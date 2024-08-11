package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
)

type CustomTokenMiddleware struct {
	conf         *models.Config
	tokenService *services.BrankaToken
	driver       interfaces.SystemDBInterface
}

func GetBrankaTokenMiddleware(cfg *models.Config, driver interfaces.SystemDBInterface) *CustomTokenMiddleware {
	return &CustomTokenMiddleware{
		conf:         cfg,
		tokenService: services.GetBrankaToken(cfg, driver),
		driver:       driver,
	}
}

func (c *CustomTokenMiddleware) GetBrankaMiddleware(ctx echo.Context) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			tokenString := ctx.Request().Header.Get("Authorization")
			tokenType := "Bearer"
			// Missing Token
			if tokenString == "" {
				return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Token is missing"})
			}

			// Check for tempered token , check with signing method RSA
			if strings.HasPrefix(tokenString, tokenType) {
				tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, tokenType))
				if tokenString != "" {

					ext, err := c.tokenService.Validate(nil, tokenString)
					if err != nil {
						return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Claims ! WTF !!"})
					}

					if ext.UserId != "" && ext.ProjectId == "" {
						//ctx.Set("user_id", )
						ctx.Set("user", ext.UserId)
						next(ctx)
					} else if ext.UserId != "" && ext.ProjectId != "" {
						ctx.Set("user", ext.UserId)
						ctx.Set("project", ext.ProjectId)
						next(ctx)
					} else {
						return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Claims ! WTF !!"})
					}
				} else {
					return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Where is the token ? WTF Dude!!"})
				}
			} else {
				return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Type."})
			}
			return nil
		}
	}
}

type bodyLogWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (c *CustomTokenMiddleware) GetBrankaMiddlewareWithLog() echo.MiddlewareFunc {

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			if ctx.Request().URL.RawQuery == "v=1" {
				// ignore with jwt token and flag is v=1
				next(ctx)
			} else {
				tokenString := ctx.Request().Header.Get("Authorization")
				tokenType := "Bearer"
				// Missing Token
				if tokenString == "" {
					return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Token is missing"})
				}

				// Check for tempered token , check with signing method RSA
				if strings.HasPrefix(tokenString, tokenType) {
					tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, tokenType))
					if tokenString != "" {

						ext, err := c.tokenService.Validate(nil, tokenString)
						if err != nil {
							return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Claims ! WTF !!"})
						}

						if ext.UserId != "" && ext.ProjectId == "" {
							//ctx.Set("user_id", )
							ctx.Set("user", ext.UserId)
							next(ctx)
						} else if ext.UserId != "" && ext.ProjectId != "" {
							ctx.Set("user", ext.UserId)
							ctx.Set("project", ext.ProjectId)

							// start a log
							blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: ctx.Response().Writer}
							ctx.Response().Writer = blw
							next(ctx)

							//inMb := (float64(ctx.Writer.Size()) / 1024.0) / 1024.0 // in mb
							//err := c.driver.UpdateUsageBandwidth(&protobuff.ProjectUsages{}, inMb)
							if err != nil {
								return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": err.Error()})
							}

							statusCode := ctx.Response().Status
							if statusCode >= 400 {
								//ok this is an request with error, let's make a record for it
								// now print body (or log in your preferred way)
								fmt.Println("Response body: " + blw.body.String())
							}

						} else {
							return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Claims ! WTF !!"})
						}
					} else {
						return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Where is the token ? WTF Dude!!"})
					}
				} else {
					return ctx.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": "Invalid token Type."})
				}
			}
			return nil
		}
	}
}
