package utility

import (
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

// EchoTokenClaimsKey is the Echo context key for the parsed TokenClaims pointer.
const EchoTokenClaimsKey = "token_claims"

func resolveProjectIDFromClaims(ctx echo.Context, tokenClaims *models.TokenClaims) string {
	if tokenClaims == nil {
		return ""
	}
	// Unified access tokens are scoped only after AccessTokenService authorizes
	// the canonical project header. Never silently pick ProjectIDs[0] here.
	if tokenClaims.TokenType == "access_token" {
		return ""
	}
	if header := strings.TrimSpace(ctx.Request().Header.Get(models.ApitoProjectIDHeader)); header != "" {
		if tokenClaims.ProjectID != "" {
			if header == tokenClaims.ProjectID {
				return header
			}
		} else {
			for _, pid := range tokenClaims.ProjectIDs {
				if pid == header {
					return header
				}
			}
		}
	}
	if tokenClaims.ProjectID != "" {
		return tokenClaims.ProjectID
	}
	if len(tokenClaims.ProjectIDs) > 0 && tokenClaims.ProjectIDs[0] != "" {
		return tokenClaims.ProjectIDs[0]
	}
	return ""
}

func SetTokenClaimsToRouter(ctx echo.Context, tokenClaims *models.TokenClaims) error {

	if tokenClaims != nil {

		// user id is set by access token not id token
		if tokenClaims.UserID != "" {
			ctx.Set("user", tokenClaims.UserID)
		} else {
			return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": "invalid token, without user"})
		}

		// rest is set using id token
		if projectID := resolveProjectIDFromClaims(ctx, tokenClaims); projectID != "" {
			ctx.Set("project", projectID)
		}

		if tokenClaims.Role != "" {
			ctx.Set("role", tokenClaims.Role)
		}

		if tokenClaims.PaymentDueDate != "" {
			ctx.Set("payment_due_date", tokenClaims.PaymentDueDate)
		}

		if tokenClaims.AccessPermissions != nil {
			ctx.Set("project_access", tokenClaims.AccessPermissions)
		}

		if tokenClaims.Email != "" {
			ctx.Set("email", tokenClaims.Email)
		}

		ctx.Set("read_only", tokenClaims.IsReadOnly)
		ctx.Set("is_super_admin", tokenClaims.IsSuperAdmin)
		ctx.Set(EchoTokenClaimsKey, tokenClaims)

		if tokenClaims.IsProjectUser ||
			tokenClaims.TokenType == "user" ||
			tokenClaims.TokenType == "tenant" {
			ctx.Set("is_project_user", true)
		}
	}

	return nil
}
