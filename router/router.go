package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/apito-io/engine/controller"
	"github.com/apito-io/engine/database"

	//"github.com/apito-io/engine/controller/middleware"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	im "github.com/apito-io/engine/router/middleware"
	"github.com/apito-io/engine/utility"
	"github.com/jboursiquot/go-proverbs"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

// InitRouter #todo manage better connection in redis and gRPC
// connection pool for grpc tried but no use !!

func InitRouter(cfg *models.Config) (*echo.Echo, error) {

	fmt.Println("initializing apito engine router")
	router := echo.New()
	pprof.Register(router)
	router.Use(echoMiddleware.Recover())

	//router.Use(middleware.Logger())
	/*router.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[${time_rfc3339}] ${status} ${method} ${path} (${remote_ip}) ${latency_human}\n",
		Output: router.Logger.Output(),
	}))*/
	router.Use(im.CORSMiddleware(cfg))

	// Once it's done, you can attach the handler as one of your middleware
	//router.Use(sentryecho.New(sentryecho.Options{}))

	router.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, &models.HttpResponse{
			Message: proverbs.Random().Saying,
			Code:    http.StatusOK,
		})
	})

	router.GET("/heartbeat", func(i echo.Context) error {
		return i.JSON(http.StatusOK, &models.HttpResponse{
			Message: "Beating",
			Code:    http.StatusOK,
		})
	})

	// for google cloud run ssl
	router.GET("/.well-known/acme-challenge/:token", func(c echo.Context) error {
		return c.String(http.StatusOK, c.Param("token"))
	})

	//router.Use(middleware.Static("files/storage"))
	router.Static("/static/media", "files/storage")

	/*
		cacheDriver, err := badger.GetBadgerDriver(cfg)
		if err != nil {
			panic(err.Error())
		}
	*/

	// init the system driver first
	//systemDriver, err := firestore.GetFirestoreDriver(cfg)

	/*systemQueueDriver, err := redis.GetRedisDriver(cfg)
	if err != nil {
		panic(err.Error())
	}*/

	extensionRoute := router.Group("plugin")

	fmt.Println("building graphql server instance")
	ctx := context.Background()
	server, err := resolver.BuildGraphQLWithFactory(ctx, cfg, extensionRoute, router)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	/*fmt.Println("building system graphql queries and mutations")
	server.BuildServerQueriesAndMutations()*/

	fmt.Println(" ---> loading local plugins <--- ")
	// load the plugin
	err = server.LoadPlugins(ctx)
	if err != nil {
		fmt.Printf("Error loading plugins: %v\n", err)
		// Don't return error to allow server to continue
	}

	fmt.Println(" ---> waiting for plugins to finish loading <--- ")
	// Wait for all plugins to finish loading (they run in goroutines)
	server.WaitForPluginsToLoad()

	fmt.Println(" ---> initializing plugin health monitor <--- ")
	// Create and start the plugin monitor
	pluginMonitor := resolver.NewPluginMonitor(server.GetConcreteServer().(*resolver.GraphQLServer))

	// Register all loaded HashiCorp plugins for monitoring
	for _, pluginID := range server.GetHashiCorpPluginIDs() {
		pluginMonitor.RegisterPlugin(pluginID)
		fmt.Printf("🔍 [PLUGIN-MONITOR] Registered plugin for monitoring: %s\n", pluginID)
	}

	// Start monitoring in a separate goroutine
	go func() {
		pluginMonitor.StartMonitoring(ctx)
	}()

	// Store the monitor in the server for later access (optional)
	server.SetPluginMonitor(pluginMonitor)

	fmt.Println(" ---> initializing connection pool monitor <--- ")
	// Create and start connection pool monitoring
	concreteServer := server.GetConcreteServer().(*resolver.GraphQLServer)
	connectionManager := concreteServer.GetConnectionManager()
	if connectionManager != nil {
		connectionMonitor := database.NewConnectionMonitor(connectionManager)
		go func() {
			connectionMonitor.StartMonitoring(ctx)
		}()
		concreteServer.SetConnectionMonitor(connectionMonitor)
	} else {
		fmt.Println("⚠️ [CONNECTION-MONITOR] Connection manager not available, skipping monitor setup")
	}

	fmt.Println(" ---> initializing goroutine monitor <--- ")
	// Create and start goroutine monitoring
	goroutineMonitor := utility.NewGoroutineMonitor()
	go func() {
		goroutineMonitor.StartMonitoring(ctx)
	}()

	fmt.Println("initializing graphql & auth controller")
	graphCtrl := controller.GetGraphQLController(cfg, server.GetConcreteServer().(*resolver.GraphQLServer))
	authCtrl := controller.GetAuthController(cfg, server.GetConcreteServer().(*resolver.GraphQLServer))
	pluginV2Ctrl := controller.NewPluginV2Controller(server.GetConcreteServer().(*resolver.GraphQLServer))

	/*authv3Routes := router.Group("auth/v3")
	{
		authv3Routes.POST("/login", authCtrl.LoginV3)
		authv3Routes.POST("/register", authCtrl.RegisterV3)
		//authv3Routes.GET("/logout", authCtrl.LogoutV3)
		//authv3Routes.POST("/verify/email", authCtrl.VerifyV3)
	}*/

	authV2Routes := router.Group("auth/v2")
	{
		authV2Routes.POST("/login", authCtrl.LoginV2)
		authV2Routes.POST("/register", authCtrl.RegisterV2)
		//authV2Routes.POST("/logout", authCtrl.LogoutV2)
		authV2Routes.POST("/verify/email", authCtrl.VerifyV2)
		authV2Routes.POST("/forget/password/request", authCtrl.ForgetPasswordRequestV2)
		authV2Routes.POST("/forget/password/verify", authCtrl.ForgetPasswordConfirmedV2)

		//authV2Routes.POST("/verify/email/resend", authCtrl.ResendVerificationEmailV2)
	}

	authV2ProtectedRoutes := router.Group("auth/v2")
	authV2ProtectedRoutes.Use(server.Authorize())
	{
		authV2ProtectedRoutes.POST("/logout", authCtrl.LogoutV2)

		authV2ProtectedRoutes.POST("/change/password", authCtrl.ChangePasswordV2)
	}

	/*authv3ProtectedRoutes := router.Group("auth/v3")
	authv3ProtectedRoutes.Use(server.Authorize())
	{
		authv3ProtectedRoutes.POST("/switch", authCtrl.ProjectSwitchV3)
		authv3ProtectedRoutes.POST("/logout", authCtrl.LogoutV3)
	}*/

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
			projectRoutes.POST("/demo/switch", authCtrl.DemoProjectSwitch)
			projectRoutes.POST("/name/check", authCtrl.ProjectNameCheck)
			projectRoutes.POST("/list", authCtrl.ProjectList)
			projectRoutes.POST("/delete", authCtrl.ProjectDelete)
		}

		databaseRoutes := systemRoutes.Group("/database")
		{

			databaseRoutes.POST("/check", authCtrl.DatabaseCheck)
		}

		systemRoutes.GET("/cache/list", graphCtrl.GetSystemCacheInfo)

		systemRoutes.GET("/doc/:id", graphCtrl.RESTApiDocGenerator)

		systemRoutes.POST("/graphql", graphCtrl.SystemGraphQL)
		systemRoutes.Any("/graphql/subscription", graphCtrl.SubscriptionWrapHandler)

		// Legacy plugin upload (keeping for backwards compatibility)
		//systemRoutes.POST("/plugin/upload", graphCtrl.PluginUpload) deprecated

		systemRoutes.GET("/health", graphCtrl.SystemHealth)

		// Plugin management routes with cloud sync authentication
		//pluginRoutes := systemRoutes.Group("/plugin", middleware.CloudSyncKeyAuth())
		pluginRoutes := systemRoutes.Group("/plugin")
		{
			// Plugin CRUD operations
			pluginRoutes.POST("", pluginV2Ctrl.CreateOrUpdatePlugin)    // Create/Update plugin
			pluginRoutes.PUT("/:id", pluginV2Ctrl.CreateOrUpdatePlugin) // Update specific plugin
			pluginRoutes.GET("", pluginV2Ctrl.ListPlugins)              // List all plugins
			pluginRoutes.GET("/:id", pluginV2Ctrl.GetPluginStatus)      // Get plugin status
			pluginRoutes.DELETE("/:id", pluginV2Ctrl.DeletePlugin)      // Delete plugin

			// Platform compatibility check
			pluginRoutes.GET("/platform", pluginV2Ctrl.GetPlatformInfo) // Get server platform info

			// Plugin control operations
			pluginRoutes.POST("/:id/restart", pluginV2Ctrl.RestartPlugin) // Restart plugin
			pluginRoutes.POST("/:id/stop", pluginV2Ctrl.StopPlugin)       // Stop plugin
			pluginRoutes.POST("/:id/start", pluginV2Ctrl.RestartPlugin)   // Start plugin (alias for restart)
		}

		// Plugin health check (no auth required, placed outside the authenticated group)
		systemRoutes.GET("/plugin/health", func(c echo.Context) error {
			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Plugin management API is healthy",
				"version": "v2.0.0",
				"endpoints": map[string]string{
					"create_update": "POST /system/plugin",
					"list":          "GET /system/plugin",
					"status":        "GET /system/plugin/:id",
					"delete":        "DELETE /system/plugin/:id",
					"restart":       "POST /system/plugin/:id/restart",
					"stop":          "POST /system/plugin/:id/stop",
					"start":         "POST /system/plugin/:id/start",
				},
			})
		})
	}

	functionDirectEndpoint := router.Group("/function")
	functionDirectEndpoint.Use(server.PublicFunctionRouteAuthorize())
	{
		functionDirectEndpoint.POST("/:project_id/:fn_name", graphCtrl.FunctionExecute)
	}

	// PUBLIC ENDPOINTS
	publicProtectedEndpoint := router.Group("/secured")
	publicProtectedEndpoint.Use(server.Authorize())
	{
		// rest api
		publicProtectedEndpoint.GET("/rest/:pid/:model/:id/:relation", graphCtrl.RestToGraphQL)
		publicProtectedEndpoint.GET("/rest/:pid/:model/:id", graphCtrl.RestToGraphQL)
		publicProtectedEndpoint.GET("/rest/:pid/:model", graphCtrl.RestToGraphQL)
		publicProtectedEndpoint.POST("/rest/:pid/*model", graphCtrl.RestToGraphQL)
		publicProtectedEndpoint.PUT("/rest/:pid/:model", graphCtrl.RestToGraphQL)
		publicProtectedEndpoint.DELETE("/rest/:pid/:model", graphCtrl.RestToGraphQL)

		// graphql
		publicProtectedEndpoint.POST("/graphql", graphCtrl.PublicGraphQL)
	}

	/*subsEndpoint := router.Group("/sub")
	subsEndpoint.Use(tokenService.Authorize())
	{
		// graphql
		subsEndpoint.POST("/graphql", echo.SubscriptionWrapHandler(subHandler))
	}*/

	/*mux := http.ServeMux{}
	mux.Handle("/subscription", subHandler)
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(writer, "Hello, %q", html.EscapeString(request.URL.Path))
	})

	go func() {
		fmt.Println("serving graphql subs on : 9090")
		err = http.ListenAndServe(":9090", &mux)
		if err != nil {
			panic(err)
		}
	}()*/

	return router, nil
}

// PrintAllEchoRoutes prints all registered routes in the Echo instance
// This is the proper way to see all routes registered in Echo v4
func PrintAllEchoRoutes(e *echo.Echo) {
	fmt.Printf("\n🌐 [ECHO-ROUTES] Complete route listing:\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	routes := e.Routes()
	if len(routes) == 0 {
		fmt.Printf("❌ No routes registered in Echo\n")
	} else {
		fmt.Printf("✅ Total routes registered: %d\n\n", len(routes))

		// Group routes by prefix for better readability
		//systemRoutes := make([]*echo.Route, 0)
		//authRoutes := make([]*echo.Route, 0)
		pluginRoutes := make([]*echo.Route, 0)
		//publicRoutes := make([]*echo.Route, 0)
		//otherRoutes := make([]*echo.Route, 0)

		for _, route := range routes {
			switch {
			//case strings.HasPrefix(route.Path, "/system"):
			//	systemRoutes = append(systemRoutes, route)
			//case strings.HasPrefix(route.Path, "/auth"):
			//	authRoutes = append(authRoutes, route)
			case strings.HasPrefix(route.Path, "/plugin"):
				pluginRoutes = append(pluginRoutes, route)
				//case strings.HasPrefix(route.Path, "/secured"), strings.HasPrefix(route.Path, "/function"):
				//	publicRoutes = append(publicRoutes, route)
				//default:
				//	otherRoutes = append(otherRoutes, route)
			}
		}

		// Print each category
		//printRouteCategory("🔧 System Routes", systemRoutes)
		//printRouteCategory("🔐 Authentication Routes", authRoutes)
		printRouteCategory("🔌 Plugin Routes", pluginRoutes)
		//printRouteCategory("🌐 Public/Secured Routes", publicRoutes)
		//printRouteCategory("📝 Other Routes", otherRoutes)
	}

	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("🌐 [ECHO-ROUTES] End of complete route listing\n\n")
}

// printRouteCategory prints routes for a specific category
func printRouteCategory(title string, routes []*echo.Route) {
	if len(routes) > 0 {
		fmt.Printf("\n%s (%d routes):\n", title, len(routes))
		fmt.Printf("%s\n", strings.Repeat("-", 50))

		for i, route := range routes {
			fmt.Printf("%2d. %-6s %s", i+1, route.Method, route.Path)
			if route.Name != "" {
				fmt.Printf(" (name: %s)", route.Name)
			}
			fmt.Printf("\n")
		}
	}
}

/*// #todo implement log here, yea good idea !!
func clientInterceptor(
	ctx context.Context,
	method string,
	req interface{},
	reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...) // <==
	fmt.Printf("invoke remote method=%s duration=%s error=%v", method,
		time.Since(start), err)
	return err
}*/
