package router

import (
	"context"
	"fmt"
	"github.com/apito-io/engine/controller"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	mm "github.com/apito-io/engine/router/middleware"
	"github.com/jboursiquot/go-proverbs"
	"github.com/labstack/echo-contrib/pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

// InitRouter #todo manage better connection in redis and gRPC
// connection pool for grpc tried but no use !!

func InitRouter(cfg *models.Config) (*echo.Echo, error) {

	fmt.Println("initializing router")
	router := echo.New()
	pprof.Register(router)
	router.Use(middleware.Recover())

	router.Use(middleware.Logger())
	router.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[${time_rfc3339}] ${status} ${method} ${path} (${remote_ip}) ${latency_human}\n",
		Output: router.Logger.Output(),
	}))
	router.Use(mm.CORSMiddleware(cfg))

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
	server, err := resolver.BuildGraphQLServer(ctx, cfg, extensionRoute)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	/*fmt.Println("building system graphql queries and mutations")
	server.BuildServerQueriesAndMutations()*/

	fmt.Println("loading local plugins")
	err = server.LoadPlugins()
	if err != nil {
		return nil, err
	}

	fmt.Println("initializing graphql & auth controller")
	graphCtrl := controller.GetGraphQLController(cfg, server)
	authCtrl := controller.GetAuthController(cfg, server)

	authV2Routes := router.Group("auth/v1")
	{
		authV2Routes.POST("/login", authCtrl.LoginV2)
		authV2Routes.POST("/register", authCtrl.RegisterV2)
		//authV2Routes.POST("/logout", authCtrl.LogoutV2)
		authV2Routes.POST("/verify/email", authCtrl.VerifyV2)
		authV2Routes.POST("/forget/password/request", authCtrl.ForgetPasswordRequestV2)
		authV2Routes.POST("/forget/password/verify", authCtrl.ForgetPasswordConfirmedV2)
	}

	authV2ProtectedRoutes := router.Group("auth/v1")
	authV2ProtectedRoutes.Use(server.Authorize())
	{
		authV2ProtectedRoutes.POST("/logout", authCtrl.LogoutV2)
		authV2ProtectedRoutes.POST("/change/password", authCtrl.ChangePasswordV2)
	}

	systemRoutes := router.Group("/system")
	systemRoutes.Use(server.Authorize())
	{

		systemUser := systemRoutes.Group("/user")
		{
			systemUser.GET("/profile", authCtrl.GetProfile)
			systemUser.POST("/profile", authCtrl.UpdateProfile)
			//systemUser.POST("/teams", authCtrl.Teams)
		}

		projectRoutes := systemRoutes.Group("/project")
		{
			projectRoutes.POST("/switch", authCtrl.ProjectSwitchV2)
			//projectRoutes.POST("/create", authCtrl.ProjectCreation)
			projectRoutes.POST("/name/check", authCtrl.ProjectNameCheck)
			projectRoutes.POST("/list", authCtrl.ProjectList)
			//projectRoutes.POST("/delete", authCtrl.ProjectDelete)
		}

		systemRoutes.GET("/cache/list", graphCtrl.GetSystemCacheInfo)

		systemRoutes.GET("/doc/:id", graphCtrl.RESTApiDocGenerator)

		systemRoutes.POST("/plugin/upload", graphCtrl.PluginUpload)

		systemRoutes.POST("/graphql", graphCtrl.SystemGraphQL)
		systemRoutes.Any("/graphql/subscription", graphCtrl.SubscriptionWrapHandler)
	}

	publicEndpoint := router.Group("/secured")
	publicEndpoint.Use(server.Authorize())
	{
		// rest api
		publicEndpoint.GET("/rest/:pid/:model/:id/:relation", graphCtrl.RestToGraphQL)
		publicEndpoint.GET("/rest/:pid/:model/:id", graphCtrl.RestToGraphQL)
		publicEndpoint.GET("/rest/:pid/:model", graphCtrl.RestToGraphQL)
		publicEndpoint.POST("/rest/:pid/*model", graphCtrl.RestToGraphQL)
		publicEndpoint.PUT("/rest/:pid/:model", graphCtrl.RestToGraphQL)
		publicEndpoint.DELETE("/rest/:pid/:model", graphCtrl.RestToGraphQL)

		// graphql
		publicEndpoint.POST("/graphql", graphCtrl.PublicGraphQL)

		// test graphql
		//publicEndpoint.POST("/graphql/v3", graphCtrl.PublicGraphQLV3)

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
