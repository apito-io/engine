package utility

import (
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

const apitoProjectIDHeader = "X-Apito-Project-Id"

func resolveProjectIDFromClaims(ctx echo.Context, tokenClaims *models.TokenClaims) string {
	if tokenClaims == nil {
		return ""
	}
	if header := strings.TrimSpace(ctx.Request().Header.Get(apitoProjectIDHeader)); header != "" {
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

		if tokenClaims.IsProjectUser ||
			tokenClaims.TokenType == "user" ||
			tokenClaims.TokenType == "tenant" {
			ctx.Set("is_project_user", true)
		}
	}

	return nil
}
