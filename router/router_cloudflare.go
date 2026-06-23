//go:build cloudflare

package router

import (
	"context"
	"fmt"
	"net/http"

	"github.com/apito-io/engine/controller"
	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	im "github.com/apito-io/engine/router/middleware"
	"github.com/jboursiquot/go-proverbs"
	"github.com/labstack/echo/v4"
)

// InitCloudflareRouter registers the minimal HTTP surface for Workers (console on Pages).
func InitCloudflareRouter(cfg *models.Config) (*echo.Echo, error) {
	fmt.Println("initializing apito cloudflare router")
	router := echo.New()

	var connectionManagerRef *database.ConnectionManager
	router.Use(im.RecoverWithConnectionEvict(&connectionManagerRef))
	router.Use(im.CORSMiddleware(cfg))

	router.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, &models.HttpResponse{
			Message: proverbs.Random().Saying,
			Code:    http.StatusOK,
		})
	})

	router.GET("/heartbeat", func(c echo.Context) error {
		return c.JSON(http.StatusOK, &models.HttpResponse{
			Message: "Beating",
			Code:    http.StatusOK,
		})
	})

	registerMediaRoutes(router)

	extensionRoute := router.Group("plugin")
	ctx := context.Background()
	server, err := resolver.BuildGraphQLWithFactory(ctx, cfg, extensionRoute, router)
	if err != nil {
		return nil, err
	}

	concreteServer := server.GetConcreteServer().(*resolver.GraphQLServer)
	connectionManager := concreteServer.GetConnectionManager()
	if connectionManager != nil {
		connectionManagerRef = connectionManager
	}

	graphCtrl := controller.GetGraphQLController(cfg, concreteServer)
	authCtrl := controller.GetAuthController(cfg, concreteServer)

	authV2Routes := router.Group("auth/v2")
	authV2Routes.POST("/login", authCtrl.LoginV2)

	authV2ProtectedRoutes := router.Group("auth/v2")
	authV2ProtectedRoutes.Use(server.Authorize())
	{
		authV2ProtectedRoutes.POST("/logout", authCtrl.LogoutV2)
		authV2ProtectedRoutes.POST("/change/password", authCtrl.ChangePasswordV2)
	}

	systemRoutes := router.Group("/system")
	systemRoutes.Use(server.Authorize())
	{
		syncRoutes := systemRoutes.Group("/sync")
		{
			syncRoutes.GET("/token/list", authCtrl.ListSyncTokens)
			syncRoutes.POST("/token/create", authCtrl.GenerateSyncToken)
			syncRoutes.POST("/token/delete", authCtrl.DeleteSyncToken)
			syncRoutes.POST("/project", authCtrl.SyncProject)
		}

		systemUser := systemRoutes.Group("/user")
		{
			systemUser.GET("/profile", authCtrl.GetProfile)
			systemUser.POST("/profile", authCtrl.UpdateProfile)
		}

		projectRoutes := systemRoutes.Group("/project")
		{
			projectRoutes.POST("/switch", authCtrl.ProjectSwitchV2)
			projectRoutes.POST("/create", authCtrl.ProjectCreation)
			projectRoutes.POST("/list", authCtrl.ProjectList)
			projectRoutes.POST("/delete", authCtrl.ProjectDelete)
			projectRoutes.POST("/name/check", authCtrl.ProjectNameCheck)
		}

		databaseRoutes := systemRoutes.Group("/database")
		{
			dbCheck := authCtrl.DatabaseCheck
			if cfg.DatabaseCheckWrapper != nil {
				if wrap, ok := cfg.DatabaseCheckWrapper.(func(any) echo.HandlerFunc); ok {
					dbCheck = wrap(authCtrl)
				}
			}
			databaseRoutes.POST("/check", dbCheck)
		}

		systemRoutes.POST("/graphql", graphCtrl.SystemGraphQL)
		systemRoutes.GET("/health", graphCtrl.SystemHealth)
	}

	publicProtectedEndpoint := router.Group("/secured")
	publicProtectedEndpoint.Use(server.Authorize())
	{
		publicProtectedEndpoint.POST("/graphql", graphCtrl.PublicGraphQL)

		filesCtrl := controller.NewFilesController(cfg, concreteServer)
		fileRoutes := publicProtectedEndpoint.Group("/files")
		{
			fileRoutes.POST("/upload", filesCtrl.Upload)
			fileRoutes.GET("/list", filesCtrl.List)
			fileRoutes.POST("/delete", filesCtrl.Delete)
		}
	}

	return router, nil
}
