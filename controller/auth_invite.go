package controller

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

type inviteTokenStore interface {
	LookupGrantsByInviteToken(ctx context.Context, hash string) ([]*models.UserProject, error)
	ExpireInvitesByToken(ctx context.Context, hash, nowRFC3339 string) error
	AcceptGrantsByInviteToken(ctx context.Context, hash, acceptedAt string) error
}

type inviteAcceptBody struct {
	Secret string `json:"secret"`
}

func (a *AuthController) inviteStore() inviteTokenStore {
	if a == nil || a.graphQLServer == nil || a.graphQLServer.SystemDriver == nil {
		return nil
	}
	store, _ := a.graphQLServer.SystemDriver.(inviteTokenStore)
	return store
}

func (a *AuthController) GetWorkspaceInviteV2(c echo.Context) error {
	raw := strings.TrimSpace(c.Param("token"))
	if raw == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "token is required", Code: http.StatusBadRequest})
	}
	store := a.inviteStore()
	if store == nil {
		return c.JSON(http.StatusNotImplemented, &models.HttpResponse{Message: "invite store unavailable", Code: http.StatusNotImplemented})
	}
	ctx := c.Request().Context()
	hash := models.HashInviteToken(raw)
	now := time.Now().UTC()
	_ = store.ExpireInvitesByToken(ctx, hash, now.Format(time.RFC3339))
	grants, err := store.LookupGrantsByInviteToken(ctx, hash)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
	}
	if len(grants) == 0 {
		return c.JSON(http.StatusNotFound, &models.HttpResponse{Message: "invite not found", Code: http.StatusNotFound})
	}
	payload, err := a.invitePayload(ctx, grants, now)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
	}
	return c.JSON(http.StatusOK, payload)
}

func (a *AuthController) AcceptWorkspaceInviteV2(c echo.Context) error {
	raw := strings.TrimSpace(c.Param("token"))
	if raw == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "token is required", Code: http.StatusBadRequest})
	}
	store := a.inviteStore()
	if store == nil {
		return c.JSON(http.StatusNotImplemented, &models.HttpResponse{Message: "invite store unavailable", Code: http.StatusNotImplemented})
	}
	var body inviteAcceptBody
	_ = c.Bind(&body)

	ctx := c.Request().Context()
	hash := models.HashInviteToken(raw)
	now := time.Now().UTC()
	_ = store.ExpireInvitesByToken(ctx, hash, now.Format(time.RFC3339))
	grants, err := store.LookupGrantsByInviteToken(ctx, hash)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
	}
	if len(grants) == 0 {
		return c.JSON(http.StatusNotFound, &models.HttpResponse{Message: "invite not found", Code: http.StatusNotFound})
	}

	hasPending := false
	allExpired := true
	for _, g := range grants {
		st := models.EffectiveInviteStatus(g.InviteStatus, g.InviteExpiresAt, now)
		if st == models.InviteStatusInvited {
			hasPending = true
			allExpired = false
		}
		if st != models.InviteStatusExpired {
			allExpired = false
		}
	}
	if allExpired && !hasPending {
		return c.JSON(http.StatusGone, &models.HttpResponse{Message: "invitation expired", Code: http.StatusGone})
	}

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, grants[0].UserID)
	if err != nil || user == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "user not found", Code: http.StatusBadRequest})
	}
	needsPassword := strings.TrimSpace(user.TempPassword) != ""
	secret := strings.TrimSpace(body.Secret)
	if hasPending && needsPassword {
		if len(secret) < models.MinPasswordLength {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "password is required to accept this invite",
				Code:    http.StatusBadRequest,
			})
		}
		hashPw, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
		user.Secret = string(hashPw)
		user.TempPassword = ""
		if err := a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true); err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
	}

	if hasPending {
		if err := store.AcceptGrantsByInviteToken(ctx, hash, now.Format(time.RFC3339)); err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
		grants, err = store.LookupGrantsByInviteToken(ctx, hash)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
	}

	payload, err := a.invitePayload(ctx, grants, now)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
	}
	payload["message"] = "accepted"
	return c.JSON(http.StatusOK, payload)
}

func (a *AuthController) invitePayload(ctx context.Context, grants []*models.UserProject, now time.Time) (map[string]interface{}, error) {
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, grants[0].UserID)
	if err != nil || user == nil {
		return nil, err
	}
	projects := make([]map[string]interface{}, 0, len(grants))
	overall := models.InviteStatusAccepted
	var expiresAt string
	for _, g := range grants {
		st := models.EffectiveInviteStatus(g.InviteStatus, g.InviteExpiresAt, now)
		name := g.ProjectID
		if proj, err := a.graphQLServer.SystemDriver.GetProject(ctx, g.ProjectID); err == nil && proj != nil && proj.Name != "" {
			name = proj.Name
		}
		if strings.TrimSpace(g.InviteExpiresAt) != "" && (expiresAt == "" || g.InviteExpiresAt < expiresAt) {
			expiresAt = g.InviteExpiresAt
		}
		if st == models.InviteStatusInvited {
			overall = models.InviteStatusInvited
		} else if st == models.InviteStatusExpired && overall != models.InviteStatusInvited {
			overall = models.InviteStatusExpired
		}
		projects = append(projects, map[string]interface{}{
			"project_id":     g.ProjectID,
			"project_name":   name,
			"invite_status":  st,
			"invite_expires": g.InviteExpiresAt,
		})
	}
	return map[string]interface{}{
		"code":           http.StatusOK,
		"email":          user.Email,
		"status":         overall,
		"expires_at":     expiresAt,
		"needs_password": strings.TrimSpace(user.TempPassword) != "",
		"projects":       projects,
	}, nil
}
