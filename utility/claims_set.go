package utility

import (
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"net/http"
)

func SetTokenClaimsToRouter(ctx echo.Context, tokenClaims *models.TokenClaims) error {
	if tokenClaims != nil {

		// user id is set by access token not id token
		if tokenClaims.UserId != "" {
			ctx.Set("user", tokenClaims.UserId)
		} else {
			return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": "invalid token, without user"})
		}

		// rest is set using id token
		if tokenClaims.ProjectId != "" {
			ctx.Set("project", tokenClaims.ProjectId)
		}

		if tokenClaims.Email != "" {
			ctx.Set("email", tokenClaims.Email)
		}

		ctx.Set("read_only", tokenClaims.IsReadOnly)
	}

	return nil
}
