package services

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

type memSystemDB struct {
	mu    sync.Mutex
	users map[string]*models.SystemUser
	roles map[string]string // userID|projectID -> role
}

func newMemDB() *memSystemDB {
	return &memSystemDB{
		users: map[string]*models.SystemUser{},
		roles: map[string]string{},
	}
}

func (m *memSystemDB) GetSystemUser(_ context.Context, id string) (*models.SystemUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *u
	tokens := make([]*models.AccessTokenRecord, 0, len(u.AccessTokens))
	for _, t := range u.AccessTokens {
		if t == nil {
			continue
		}
		tc := *t
		tokens = append(tokens, &tc)
	}
	cp.AccessTokens = tokens
	return &cp, nil
}

func (m *memSystemDB) UpdateSystemUser(_ context.Context, user *models.SystemUser, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *user
	tokens := make([]*models.AccessTokenRecord, 0, len(user.AccessTokens))
	for _, t := range user.AccessTokens {
		if t == nil {
			continue
		}
		tc := *t
		tokens = append(tokens, &tc)
	}
	cp.AccessTokens = tokens
	m.users[user.ID] = &cp
	return nil
}

func (m *memSystemDB) FindUserProjectsWithRoles(_ context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*models.ProjectWithRoles
	prefix := userId + "|"
	for k, role := range m.roles {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			pid := k[len(prefix):]
			out = append(out, &models.ProjectWithRoles{
				Project: &models.Project{ID: pid, Name: pid},
				Role:    role,
			})
		}
	}
	return out, nil
}

func (m *memSystemDB) CheckProjectWithRoles(_ context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role, ok := m.roles[userId+"|"+projectId]
	if !ok {
		return nil, errors.New("not a member")
	}
	return &models.ProjectWithRoles{
		Project: &models.Project{ID: projectId},
		Role:    role,
	}, nil
}

func TestAccessTokenMintValidateRevokeMatrix(t *testing.T) {
	db := newMemDB()
	db.users["user1"] = &models.SystemUser{ID: "user1", Email: "a@b.com", IsActive: true}
	db.roles["user1|projA"] = "admin"
	db.roles["user1|projB"] = "editor"

	svc := NewAccessTokenService(&models.Config{}, db)

	_, _, err := svc.Mint(context.Background(), "user1", &models.CreateAccessTokenRequest{
		Name:             "bad",
		Preset:           "read_only",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"projB"},
	})
	require.Error(t, err, "non-admin project must be rejected at mint")

	raw, pub, err := svc.Mint(context.Background(), "user1", &models.CreateAccessTokenRequest{
		Name:              "ok",
		Preset:            "cli_sync",
		ProjectGrantMode:  models.AccessTokenProjectGrantSelected,
		ProjectIDs:        []string{"projA"},
		AcknowledgeDanger: true,
	})
	require.NoError(t, err)
	require.True(t, IsAccessToken(raw))
	require.NotNil(t, pub)
	require.Equal(t, models.AccessTokenStatusActive, pub.Status)
	require.NotContains(t, raw, "secret_hash")

	claims, principal, err := svc.ValidateRaw(context.Background(), raw, "127.0.0.1", "test")
	require.NoError(t, err)
	require.Equal(t, "user1", claims.UserID)
	require.False(t, claims.IsSuperAdmin)
	require.Empty(t, claims.ProjectID, "apt_ project scope must come from the canonical request header")
	require.True(t, authz.HasCapability(principal.Capabilities, authz.CapSyncWrite))

	require.NoError(t, svc.AuthorizeProject(context.Background(), principal, "projA"))
	require.Error(t, svc.AuthorizeProject(context.Background(), principal, "projB"))

	require.NoError(t, svc.Revoke(context.Background(), "user1", pub.ID, "user1"))
	_, _, err = svc.ValidateRaw(context.Background(), raw, "127.0.0.1", "test")
	require.Error(t, err)
}

func TestAllAdminProjectsDynamicGrant(t *testing.T) {
	db := newMemDB()
	db.users["user1"] = &models.SystemUser{ID: "user1", Email: "a@b.com", IsActive: true}
	db.roles["user1|projA"] = "admin"

	svc := NewAccessTokenService(&models.Config{}, db)
	raw, _, err := svc.Mint(context.Background(), "user1", &models.CreateAccessTokenRequest{
		Name:             "dyn",
		Preset:           "read_only",
		ProjectGrantMode: models.AccessTokenProjectGrantAllAdmin,
	})
	require.NoError(t, err)
	_, principal, err := svc.ValidateRaw(context.Background(), raw, "", "")
	require.NoError(t, err)
	require.NoError(t, svc.AuthorizeProject(context.Background(), principal, "projA"))

	db.roles["user1|projA"] = "editor"
	require.Error(t, svc.AuthorizeProject(context.Background(), principal, "projA"))

	db.roles["user1|projC"] = "owner"
	require.NoError(t, svc.AuthorizeProject(context.Background(), principal, "projC"))
}

func TestApplyAccessTokenScopeUsesCanonicalProjectHeader(t *testing.T) {
	db := newMemDB()
	db.roles["user1|projA"] = "admin"
	tokenSvc := NewAccessTokenService(&models.Config{}, db)
	middleware := &ApitoTokenService{accessTokenService: tokenSvc}
	principal := &models.AccessPrincipal{
		IssuerUserID:     "user1",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"projA"},
		TenantGrantMode:  models.AccessTokenTenantGrantAll,
	}
	claims := &models.TokenClaims{UserID: "user1", TokenType: "access_token"}

	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set(models.ApitoProjectIDHeader, "projA")
	req.Header.Set(models.ApitoTenantIDHeader, "tenantA")
	ctx := e.NewContext(req, httptest.NewRecorder())
	ctx.Set("access_principal", principal)

	require.NoError(t, middleware.applyAccessTokenScope(ctx, claims, principal))
	require.Equal(t, "projA", claims.ProjectID)
	require.Equal(t, "projA", ctx.Get("project"))
	require.Equal(t, "tenantA", ctx.Get("tenant_id"))
}

func TestApplyAccessTokenScopeRejectsTenantWithoutProject(t *testing.T) {
	tokenSvc := NewAccessTokenService(&models.Config{}, newMemDB())
	middleware := &ApitoTokenService{accessTokenService: tokenSvc}
	principal := &models.AccessPrincipal{IssuerUserID: "user1", TenantGrantMode: models.AccessTokenTenantGrantAll}
	claims := &models.TokenClaims{UserID: "user1", TokenType: "access_token"}

	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set(models.ApitoTenantIDHeader, "tenantA")
	ctx := e.NewContext(req, httptest.NewRecorder())
	ctx.Set("access_principal", principal)

	err := middleware.applyAccessTokenScope(ctx, claims, principal)
	require.EqualError(t, err, "X-Apito-Project-Id is required when X-Apito-Tenant-ID is set")
}

func TestApplyAccessTokenScopeDoesNotAcceptLegacyProjectHeader(t *testing.T) {
	tokenSvc := NewAccessTokenService(&models.Config{}, newMemDB())
	middleware := &ApitoTokenService{accessTokenService: tokenSvc}
	principal := &models.AccessPrincipal{IssuerUserID: "user1"}
	claims := &models.TokenClaims{UserID: "user1", TokenType: "access_token"}

	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set("X-Project-ID", "projA")
	ctx := e.NewContext(req, httptest.NewRecorder())
	ctx.Set("access_principal", principal)

	require.NoError(t, middleware.applyAccessTokenScope(ctx, claims, principal))
	require.Empty(t, claims.ProjectID)
	require.Nil(t, ctx.Get("project"))
}

func TestApplyAccessTokenScopeReportOnlyStillProvidesRequestedRoute(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyReportOnly)
	tokenSvc := NewAccessTokenService(&models.Config{}, newMemDB())
	middleware := &ApitoTokenService{accessTokenService: tokenSvc}
	principal := &models.AccessPrincipal{
		TokenID:          "token-1",
		IssuerUserID:     "user1",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"other-project"},
	}
	claims := &models.TokenClaims{UserID: "user1", TokenType: "access_token"}

	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set(models.ApitoProjectIDHeader, "reported-project")
	ctx := e.NewContext(req, httptest.NewRecorder())
	ctx.Set("access_principal", principal)

	require.NoError(t, middleware.applyAccessTokenScope(ctx, claims, principal))
	require.Equal(t, "reported-project", claims.ProjectID)
	require.Equal(t, "reported-project", ctx.Get("project"))
}

func TestRetiredTokenRejected(t *testing.T) {
	svc := NewAccessTokenService(&models.Config{}, newMemDB())
	_, _, err := svc.ValidateRaw(context.Background(), "cli-legacy-token", "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "TOKEN_FORMAT_RETIRED")
}

func TestValidateRaw_StampsIssuerSuperAdmin(t *testing.T) {
	db := newMemDB()
	db.users["admin"] = &models.SystemUser{
		ID: "admin", Email: "admin@apito.io", IsActive: true, IsSuperAdmin: true,
	}
	db.roles["admin|projA"] = "admin"
	svc := NewAccessTokenService(&models.Config{}, db)
	raw, _, err := svc.Mint(context.Background(), "admin", &models.CreateAccessTokenRequest{
		Name:              "ops",
		Preset:            "cli_sync",
		ProjectGrantMode:  models.AccessTokenProjectGrantSelected,
		ProjectIDs:        []string{"projA"},
		AcknowledgeDanger: true,
	})
	require.NoError(t, err)
	claims, _, err := svc.ValidateRaw(context.Background(), raw, "", "")
	require.NoError(t, err)
	require.True(t, claims.IsSuperAdmin)
}
