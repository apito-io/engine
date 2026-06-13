package utility

import (
	"net/http"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

func SetTokenClaimsToRouter(ctx echo.Context, tokenClaims *models.TokenClaims) error {

	if tokenClaims != nil {

		// user id is set by access token not id token
		if tokenClaims.UserID != "" {
			ctx.Set("user", tokenClaims.UserID)
		} else {
			return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": "invalid token, without user"})
		}

		// rest is set using id token
		if tokenClaims.ProjectID != "" {
			ctx.Set("project", tokenClaims.ProjectID)
		} else if len(tokenClaims.ProjectIDs) > 0 && tokenClaims.ProjectIDs[0] != "" {
			// Sync tokens may carry project_ids[] without ProjectID.
			// Keep backward compatibility by setting the primary project to the first item.
			ctx.Set("project", tokenClaims.ProjectIDs[0])
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
