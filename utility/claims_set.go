package utility

import (
	"fmt"
	"net/http"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

func SetTokenClaimsToRouter(ctx echo.Context, tokenClaims *models.TokenClaims) error {

	if tokenClaims != nil {

		// inject tenant id to context for system for queyr and mutation
		if tokenClaims.TenantID != "" { // for apito saas project
			fmt.Println("setting temp_tenant_id", tokenClaims.TenantID)
			ctx.Set("temp_tenant_id", tokenClaims.TenantID)
		} /* else if tempTenantID != "" { // passed via cookie
			ctx.Set("temp_tenant_id", tempTenantID)
		} */

		// user id is set by access token not id token
		if tokenClaims.UserID != "" {
			ctx.Set("user", tokenClaims.UserID)
		} else {
			return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": "invalid token, without user"})
		}

		// rest is set using id token
		if tokenClaims.ProjectID != "" {
			ctx.Set("project", tokenClaims.ProjectID)
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
	}

	return nil
}
