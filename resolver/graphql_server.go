package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	_const "github.com/apito-io/engine/const"
	badgerCache "github.com/apito-io/engine/database/cache/badger"
	memoryCache "github.com/apito-io/engine/database/cache/memory"
	redisCache "github.com/apito-io/engine/database/cache/redis"
	kvBadger "github.com/apito-io/engine/database/kv/badger"
	kvMemory "github.com/apito-io/engine/database/kv/memory"
	kvRedis "github.com/apito-io/engine/database/kv/redis"
	queueRedis "github.com/apito-io/engine/database/queue/redis"
	"github.com/apito-io/engine/database/system"
	"github.com/apito-io/engine/executor"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	dl "github.com/apito-io/engine/resolver/dataloader"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/edwingeng/hotswap"
	svg "github.com/h2non/go-is-svg"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"gopkg.in/h2non/filetype.v1"
)

type GraphQLServer struct {
	sync.Mutex
	wg sync.WaitGroup

	Cfg *models.Config

	SystemDriver          interfaces.ApitoSystemDB
	SystemDriverReadyChan chan interfaces.ApitoSystemDB

	PrivateSchemObjects *objects.SchemaObjects
	SystemDataloaders   *dl.SystemDataloader

	SystemQueries       graphql.Fields
	SystemQueriesChan   chan *graphql.Fields
	SystemMutations     graphql.Fields
	SystemMutationsChan chan *graphql.Fields

	GraphQLExecutor interfaces.GraphQLExecutorInterface

	//SystemRoles []string
	//UploadService *services.UploadService
	//ProjectRawSchemas *protobuff.ProjectSchema

	BlankaTokenService *services.BrankaToken
	ApiKeyManager      *services.APIKeyManager

	ApitoTokenService *services.ApitoTokenService
	JWTTokenService   *services.JWTService

	//S3          *storage_driver.S3
	AuthService services.AuthServiceInterface
	//JwtService         *services.JWTService
	ProjectDBConnPools *sync.Map

	AwsConfig aws.Config

	ProjectCache interfaces.CacheDBInterface

	// for HashiCorp plugins only
	HashiCorpPluginCache map[string]*models.HashiCorpPluginCache
	PluginLoadingState   map[string]bool // Track which plugins are currently being loaded
	PluginMonitor        *PluginMonitor  // Health monitoring and auto-restart for plugins

	InstalledPluginList   []string
	InstalledHCPluginList []string

	ExtensionRouterList []string
	ExtensionRouter     *echo.Group
	MainEchoInstance    *echo.Echo // Reference to the main Echo instance for route introspection

	//LocalPluginGraphQLSchemas chan *extensions.ThirdPartyGraphQLSchemas
	//LocalPluginRoutes chan []*extensions.ThirdPartyRESTApi

	GraphQLSubscription *GraphQLSubscriptions
	PubSubService       interfaces.PubSubServiceInterface
	KVService           interfaces.KeyValueServiceInterface

	PluginManagerSwapper *hotswap.PluginManagerSwapper

	MicroServiceClient *sync.Map
}

func BuildGraphQL(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (*GraphQLServer, error) {

	_awsConfig, _ := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.AWSRegion),
		config.WithCredentialsProvider(
			credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     cfg.AWSKey,
					SecretAccessKey: cfg.AWSSecret,
				}},
		),
	)

	var kvStorage interfaces.KeyValueServiceInterface
	var err error
	switch cfg.KVStorageEngine {
	case _const.RedisDriver:
		kvStorage, err = kvRedis.GetKVRedisDriver(ctx, cfg)
	case _const.CoreDB:
		kvStorage, err = kvBadger.GetKVBadgerDriver(cfg)
	case _const.MemoryDB:
		kvStorage, err = kvMemory.GetKVMemoryDriver(cfg)
	default:
		kvStorage, err = kvMemory.GetKVMemoryDriver(cfg) // Default to memory instead of badger for local dev
	}
	if err != nil {
		return nil, err
	}

	queueRedisService, err := queueRedis.GetRedisQueueDriver(cfg)
	if err != nil {
		return nil, err
	}

	srv := GraphQLServer{

		wg:  sync.WaitGroup{},
		Cfg: cfg,

		AwsConfig: _awsConfig,

		SystemQueriesChan:   make(chan *graphql.Fields),
		SystemMutationsChan: make(chan *graphql.Fields),

		SystemDriverReadyChan: make(chan interfaces.ApitoSystemDB, 1),

		KVService:     kvStorage,
		PubSubService: queueRedisService,

		//GraphQLExecutor:    _executor,
		ProjectDBConnPools:  &sync.Map{},
		ExtensionRouterList: []string{},
		ExtensionRouter:     extensionRouter,
		MainEchoInstance:    mainEcho,

		HashiCorpPluginCache: make(map[string]*models.HashiCorpPluginCache),

		InstalledHCPluginList: []string{},

		//LocalPluginGraphQLSchemas: make(chan *extensions.ThirdPartyGraphQLSchemas),
		//LocalPluginRoutes:         make(chan []*extensions.ThirdPartyRESTApi),

		MicroServiceClient: &sync.Map{},
	}

	// Initialize SystemQueries and SystemMutations as empty maps to prevent nil panic
	srv.SystemQueries = make(graphql.Fields)
	srv.SystemMutations = make(graphql.Fields)

	srv.wg.Add(1) // global wg

	/*jwt, err := services.GetJWTService(cfg)
	if err != nil {
		return nil
	}*/

	/*	notifier, err := pub_sub.NewRabbitBus(&pub_sub.RabbitMQConfig{
			RabbitMQUser:     cfg.RabbitMQUser,
			RabbitMQPassword: cfg.RabbitMQPassword,
			RabbitMQHost:     cfg.RabbitMQHost,
			RabbitMQPort:     cfg.RabbitMQPort,
			AMQP: pub_sub.AMQPConfig{
				Exchange:     fmt.Sprintf("%s_gql_subscription", cfg.Environment),
				ExchangeType: "direct",
			},
		})
		//notifier, err := pub_sub.NewRabbitBus("amqp://localhost:5672/")
		if err != nil {
			fmt.Println(err.Error())
		}*/

	tokenWg := sync.WaitGroup{}

	// system driver
	go func() {
		tokenWg.Add(1)
		defer tokenWg.Done()
		fmt.Println("connecting to system driver")
		_cred := models.DriverCredentials{
			Engine:   cfg.SystemDatabaseEngine,
			Host:     cfg.SystemDBHost,
			Port:     cfg.SystemDBPort,
			User:     cfg.SystemDBUser,
			Password: cfg.SystemDBPassword,
			Database: cfg.SystemDBName,
		}
		systemDriver, err := system.GetSystemDriver(&_cred, cfg)
		if err != nil {
			panic(err.Error()) // sure do a panic if system db not there
		}
		srv.SystemDriverReadyChan <- systemDriver
	}()

	// redis & graphql subscription
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("GraphQL subscription goroutine panic: %v\n", r)
			}
		}()

		subs, err := GetGraphQLSubscriptions()
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		srv.GraphQLSubscription = subs

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		subscriber := srv.PubSubService.Subscribe(ctx, "system_notify_channel")

		// Add graceful shutdown channel monitoring
		shutdownChan := make(chan struct{})
		defer close(shutdownChan)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("GraphQL subscription context cancelled, exiting")
				return
			case <-shutdownChan:
				return
			default:
				msg, err := subscriber.ReceiveMessage(ctx)
				if err != nil {
					fmt.Printf("Subscription receive error: %v\n", err)
					// Exit on persistent errors to prevent goroutine accumulation
					if ctx.Err() != nil {
						return
					}
					time.Sleep(1 * time.Second) // Brief delay before retry
					continue
				}

				if msg != nil {
					var data models.SubscriptionEvent
					err = json.Unmarshal([]byte(msg.Payload), &data)
					if err != nil {
						fmt.Printf("Subscription unmarshal error: %v\n", err)
						continue
					}

					for userId, sub := range subs.getSubscribers(nil) {
						if userId == data.UserID {
							select {
							case sub.Data <- data:
							case <-ctx.Done():
								return
							default:
								// Non-blocking send to prevent deadlock
							}
						}
					}
				}
			}
		}
	}()

	// auth services
	go func(cfg *models.Config) {
		tokenWg.Add(1)
		defer tokenWg.Done()
		var authService services.AuthServiceInterface
		var err error

		tokenService := services.GetJWTServiceWithRedis(cfg, srv.KVService)

		switch cfg.AuthServiceProvider {
		case "cognito", "oauth", "third-party":
			// you can implement your own follow the official doc
		case "local":
			authService, err = services.NewLocalAuthService(cfg, tokenService)
			if err != nil {
				fmt.Println(err.Error())
			}
		default:
			authService, err = services.NewLocalAuthService(cfg, tokenService)
			if err != nil {
				fmt.Println(err.Error())
			}
		}

		srv.JWTTokenService = tokenService
		srv.AuthService = authService
	}(cfg)

	// cache driver
	go func() {
		var err error
		var _cache interfaces.CacheDBInterface
		switch cfg.CacheDriver {
		case "memory":
			_cache, err = memoryCache.GetMemoryCacheDriver(cfg)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "badger":
			_cache, err = badgerCache.GetBadgerCacheDriver(cfg)
			if err != nil {
				fmt.Println(err.Error())
			}
		case "redis":
			_cache, err = redisCache.GetRedisCacheDriver(cfg)
			if err != nil {
				fmt.Println(err.Error())
			}
		default:
			_cache, err = memoryCache.GetMemoryCacheDriver(cfg)
			if err != nil {
				fmt.Println(err.Error())
			}
		}
		if err != nil {
			fmt.Println(err.Error())
		}
		srv.ProjectCache = _cache
	}()

	// s3 driver
	/*go func() {
		srv.S3 = storage_driver.InitS3(cfg)
	}()*/

	// grpc client
	/*go func() {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())
		conn, err := grpc.Dial(fmt.Sprintf("%s:%s", cfg.CacheRPCHost, cfg.CacheRPCPort), opts...)
		if err != nil {
			fmt.Println(err.Error())
		}
		srv.CodeGenClient = codegen.NewCodeGenClient(conn)
	}()*/

	// Copy System Schema
	go func() {
		for {
			select {
			case _systemQuery := <-srv.SystemQueriesChan:
				// Merge new queries into existing map instead of replacing
				for k, v := range *_systemQuery {
					srv.SystemQueries[k] = v
				}
			case _mutationQuery := <-srv.SystemMutationsChan:
				// Merge new mutations into existing map instead of replacing
				for k, v := range *_mutationQuery {
					srv.SystemMutations[k] = v
				}
			}
		}
	}()

	// Copy Local Plugin rest api register
	/*go func() {
		select {
		case localRoutes := <-srv.LocalPluginRoutes:
			for _, _route := range localRoutes {
				time.Sleep(500 * time.Millisecond)
				srv.ExtensionRouter.Use(srv.Authorize()) // add the middleware
				fmt.Println(fmt.Sprintf("--> Registering plugin routes %s, %s to system routes", _route.Method, _route.Path))
				srv.ExtensionRouter.Add(_route.Method, _route.Path, _route.Controller)
			}
		}
	}()*/

	// system db dependent services ( payment, blanka token, apito token )
	go func() {

		systemDB := <-srv.SystemDriverReadyChan

		fmt.Println("system driver finished initialized")
		srv.SystemDriver = systemDB

		_executor := executor.GetGraphQLExecutor(cfg, systemDB)
		err := _executor.Init(ctx, &models.InitParams{
			SharedDB: &models.DriverCredentials{
				Engine:   "redis",
				Host:     cfg.KVStorageEngineHost,
				Port:     cfg.KVStorageEnginePort,
				Password: cfg.KVStorageEnginePassword,
				Database: cfg.KVStorageEngineDatabase,
			},
		})
		if err != nil {
			panic(err)
		}
		srv.GraphQLExecutor = _executor

		fmt.Println("building system graphql queries and mutations")
		srv.BuildServerQueriesAndMutations()

		fmt.Println("initializing blanka token")
		srv.BlankaTokenService = services.GetBrankaToken(cfg, systemDB)

		fmt.Println("initializing api key manager")
		apiKeyManager, err := services.NewAPIKeyManager(cfg, systemDB)
		if err != nil {
			panic(err)
		}
		srv.ApiKeyManager = apiKeyManager

		fmt.Println("initializing apito token service")
		tokenWg.Wait() // depends on systemDb & part of apito token service
		apitoTokenService, err := services.NewApitoTokenService(cfg, srv.AuthService, systemDB)
		if err != nil {
			panic(err)
		}
		srv.ApitoTokenService = apitoTokenService

		srv.wg.Done()
	}()

	//defer conn.Close()

	return &srv, nil
}

func (s *GraphQLServer) Authorize() echo.MiddlewareFunc {
	s.wg.Wait()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		err := s.ApitoTokenService.ApitoTokenHandler(next)
		return err
	}
}

func (s *GraphQLServer) PublicFunctionRouteAuthorize() echo.MiddlewareFunc {
	s.wg.Wait()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		err := s.ApitoTokenService.ApitoPublicFunctionRouteHandler(next)
		return err
	}
}

type QueryBuilderInformation struct {
	DataObjects      graphql.Fields
	AggregateObjects graphql.Fields
	//ConnectionParamObjects map[string]*graphql.InputObject
	WhereParamObjects graphql.InputObjectConfigFieldMap
	SortParamObjects  graphql.InputObjectConfigFieldMap
}

func (s *GraphQLServer) GetCacheGraphQLFieldsGeneration(ctx context.Context, projectId string, modelName string) (*QueryBuilderInformation, error) {
	id := s.cacheId(projectId, modelName)
	data, err := s.ProjectCache.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if data == nil { // send signal for cacheing
		return nil, nil
	}
	return data.(*QueryBuilderInformation), nil
}

func (s *GraphQLServer) CacheGraphQLFieldsGeneration(ctx context.Context, projectId string, modelName string, val interface{}) error {
	id := s.cacheId(projectId, modelName)
	return s.ProjectCache.Put(ctx, id, val)
}

func (s *GraphQLServer) ExpireGraphQLFieldCache(ctx context.Context, projectId string, modelName string) error {
	id := s.cacheId(projectId, modelName)
	return s.ProjectCache.Expire(ctx, id)
}

func (s *GraphQLServer) ExpireGraphQLProjectCache(ctx context.Context, projectId string) error {
	return s.ProjectCache.Expire(ctx, projectId)
}

func (s *GraphQLServer) cacheId(projectId string, modelName string) string {
	return fmt.Sprintf(`%s#%s`, projectId, modelName)
}

func (s *GraphQLServer) PublishSystemMessage(ctx context.Context, userID string, data *models.SubscriptionEvent) error {

	data.UserID = userID

	var err error
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = s.PubSubService.Publish(ctx, "system_notify_channel", payload)
	if err != nil {
		return err
	}

	fmt.Println(string(payload))
	return err
}

func (s *GraphQLServer) UpdateApplicationCache(ctx context.Context, projectID string) {
	s.Lock()
	err := s.ProjectCache.Expire(ctx, projectID)
	if err != nil {
		return
	}
	s.Unlock()
}

func (s *GraphQLServer) GetFunctionProvider() ([]string, error) {
	var providers []string
	// Only return HashiCorp plugins
	for _, pluginId := range s.InstalledHCPluginList {
		providers = append(providers, pluginId)
	}
	return providers, nil
}

func (s *GraphQLServer) GetStorageProvider() ([]string, error) {
	var providers []string
	// Only return HashiCorp plugins
	for _, pluginId := range s.InstalledHCPluginList {
		providers = append(providers, pluginId)
	}
	return providers, nil
}

/*func (s *GraphQLServer) GetParam(router echo.Context) (*shared.CommonSystemParams, error) {

	//s.Lock()
	_projectID := router.Get("project")
	if _projectID == nil {
		return nil, errors.New("nope, Can't Do it..! Project ")
	}

	_userID := router.Get("user")
	if _userID == nil {
		return nil, errors.New("nope, Can't Do it..! Project ")
	}

	projectID := _projectID.(string)
	userID := _userID.(string)

	var loader *shared.ApplicationCache
	if val, ok := s.ProjectCache[userID][projectID]; ok {
		loader = val
	} else {
		s.Lock()
		_loader, err := s.SetProjectDriverAndParam(router, projectID)
		if err != nil {
			return nil, err
		}
		loader = _loader
		// add to cache
		if userCache, ok := s.ProjectCache[userID]; ok {
			userCache[projectID] = _loader
		} else {
			s.ProjectCache[userID] = map[string]*shared.ApplicationCache{
				projectID: _loader,
			}
		}
		s.Unlock()
	}

	return loader.Param, nil
}
*/
// used in subscription
/*func (s *GraphQLServer) GetPrimaryParam(router echo.Context) (*shared.CommonSystemParams, error) {

	//s.Lock()
	var projectID string
	_projectID := router.Get("project")
	if _projectID != nil {
		projectID = _projectID.(string)
	}

	_userID := router.Get("user")
	if _userID == nil {
		return nil, errors.New("nope, Can't Do it..! Project ")
	}

	userID := _userID.(string)

	var loader *shared.ApplicationCache
	if val, ok := s.ProjectCache[userID][projectID]; ok {
		loader = val
	} else {
		s.Lock()
		_loader, err := s.SetProjectDriverAndParam(router, projectID)
		if err != nil {
			return nil, err
		}
		loader = _loader
		// add to cache
		if userCache, ok := s.ProjectCache[userID]; ok {
			userCache[projectID] = _loader
		} else {
			s.ProjectCache[userID] = map[string]*shared.ApplicationCache{
				projectID: _loader,
			}
		}
		s.Unlock()
	}

	return loader.Param, nil
}
*/

func (s *GraphQLServer) LoadProjectCache(ctx context.Context, projectID string) (*models.Project, error) {

	// get the project details
	var _project *models.Project
	if val, err := s.ProjectCache.GetProject(ctx, projectID); err == nil && val != nil && val.ID != "" {
		_project = val
	} else {
		_project, err = s.SystemDriver.GetProject(ctx, projectID)
		if err != nil {
			return nil, err
		}

		// Local plugin cache removed - using HashiCorp plugins only
		_, err = s.ProjectCache.SaveProject(ctx, _project)
		if err != nil {
			return nil, err
		}

		if _project.Driver.Database == "" && _project.ProjectType == models.ProjectType_SaaS {
			_project.Driver.Database = s.Cfg.DefaultSaaSProjectDBName
		}

		// set the project driver and build param
		_project.Driver.ProjectID = _project.ID
		err = s.GraphQLExecutor.Init(ctx, &models.InitParams{
			ProjectID: _project.ID,
			ProjectDB: _project.Driver,
			SharedDB: &models.DriverCredentials{
				ProjectID: _project.ID,
				Engine:    _const.RedisDriver,
				Host:      s.Cfg.KVStorageEngineHost,
				Port:      s.Cfg.KVStorageEnginePort,
				Password:  s.Cfg.KVStorageEnginePassword,
				Database:  s.Cfg.KVStorageEngineDatabase,
			},
		})
		if err != nil {
			return nil, err
		}

		// load the default storage plugin in any
		/* for _, plugin := range _project.Plugins {
			if plugin.Type == protobuff.PluginType_Storage && plugin.Enable {
				//if plugin.Enable {
				// load the storage plugin
				err = s.RegisterMediaStorageRoutes(ctx, plugin)
				if err != nil {
					return nil, err
				}
			}
		} */

		/*err = s.GenerateGraphQLSchema(_project)
		if err != nil {
			return nil, err
		}*/
	}

	if _project.PaymentDueDate != "" {
		parseDuration, _ := time.Parse(time.RFC3339, _project.PaymentDueDate)
		alreadyExpired := parseDuration.Sub(time.Now()).Hours()
		if alreadyExpired <= 0 {
			return nil, errors.New("payment is due. Please pay to continue or contact administrator")
		}
	}

	return _project, nil
}

func (s *GraphQLServer) GetApplicationCache(router echo.Context) (*models.ApplicationCache, error) {

	_projectID := router.Get("project")
	if _projectID == nil {
		return nil, errors.New("project id is required for this action")
	}

	_userID := router.Get("user")
	if _userID == nil {
		return nil, errors.New("user has to be logged in for this action")
	}

	projectID := _projectID.(string)
	//userID := _userID.(string)

	ctx := router.Request().Context()

	//s.wg.Add(1)
	//defer s.wg.Done()

	// get the project details
	_project, err := s.LoadProjectCache(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// inject project id to context value
	ctx = context.WithValue(ctx, "project_id", _project.ID)

	/*if _project.Driver == nil && _project.Driver.Engine == "" {
		return nil, errors.New("please select an engine")
	}*/

	param, err := s.BuildSystemParam(router, _project)
	if err != nil {
		return nil, err
	}

	cache := &models.ApplicationCache{
		Ctx:     ctx,
		Project: _project,
		Param:   param,
	}

	// Load project-specific plugins into the cache
	fmt.Printf("[DEBUG] GetApplicationCache: About to load project-specific plugins for project %s\n", _project.ID)
	err = s.LoadProjectSpecificPlugins(ctx, cache)
	if err != nil {
		// Log error but don't fail - plugins are optional
		fmt.Printf("[ERROR] GetApplicationCache: Failed to load project-specific plugins for project %s: %v\n", _project.ID, err)
	} else {
		fmt.Printf("[DEBUG] GetApplicationCache: Successfully loaded project-specific plugins for project %s\n", _project.ID)
	}

	return cache, nil
}

func (s *GraphQLServer) BuildSystemParam(i echo.Context, project *models.Project) (*models.CommonSystemParams, error) {

	param, err := s.buildCommonSystemParam(i)
	if err != nil {
		return nil, err
	}
	/*	// fetch the results
		if project.Limits == nil {
			project.Limits = utility.DeveloperProjectPlans[project.UserSubscriptionType]
		}
		//param.Limit = project.Limits
	*/

	param.ProjectType = project.ProjectType
	if project.TenantModelName != "" {
		param.TenantModel = project.TenantModelName
	}

	/* var roles []string
	for role, _ := range project.Roles {
		roles = append(roles, role)
	} */

	if param.Role.ID == "admin" {
		param.Role.IsAdmin = true
		//s.SystemRoles = roles
	} else {
		if val, ok := project.Roles[param.Role.ID]; ok {
			val.ID = param.Role.ID
			param.Role = val
			//s.SystemRoles = roles
		} else if param.Role.ID == "demo" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		} else if param.Role.ID == "team" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		} else {
			return nil, errors.New("this Role does not exits")
		}
	}
	return param, nil
}

/*
func (s *GraphQLServer) GetApplicationCacheOld(router echo.Context) (*shared.ApplicationCache, error) {

	_projectID := router.Get("project")
	if _projectID == nil {
		return nil, errors.New("nope, Can't Do it..! Project ")
	}

	_userID := router.Get("user")
	if _userID == nil {
		return nil, errors.New("nope, Can't Do it..! Project ")
	}

	projectID := _projectID.(string)
	//userID := _userID.(string)

	ctx := router.Request().Context()

	var __cache *shared.ApplicationCache
	if val, err := s.ProjectCache.Get(projectID); err == nil && val != nil && val.Project != nil {
		__cache = val

		if s.ProjectDriver == nil {
			_project := __cache.Project
			projectDriver, err := project.GetProjectDriver(&sync.Map{}, _project.Driver, _project.ID, s.Cfg)
			if err != nil {
				return nil, err
			}
			s.ProjectDriver = projectDriver
		}
	} else {
		//s.Lock()

		// get the project details
		_project, err := s.SystemDriver.GetProject(ctx, projectID)
		if err != nil {
			if driver.IsNoMoreDocuments(err) {
				return nil, errors.New(fmt.Sprintf("project not found with the id : %s", projectID))
			} else {
				return nil, err
			}
		}

		// set the project driver and build param
		_cache, err := s.SetProjectDriverAndParam(router, _project)
		if err != nil {
			return nil, err
		}
		__cache = _cache

		// add to cache
		err = s.ProjectCache.Put(projectID, __cache)
		if err != nil {
			return nil, err
		}

		// load the _plugin
		err = s.LoadPlugins()
		if err != nil {
			return nil, err
		}

		//s.LoadedPluginCache = _plugins

		//s.Unlock()
	}

	return __cache, nil

}
*/

func (s *GraphQLServer) NewParam(_param *models.CommonSystemParams) *models.CommonSystemParams {
	param := new(models.CommonSystemParams)
	*param = *_param
	return param
}

func (s *GraphQLServer) injectMetaData(functionName string, i echo.Context) {
	/*// Use runtime.Caller to get the program counter of the current function
	pc, _, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Println("Could not get caller information")
	}

	// Use runtime.FuncForPC to get the function information
	fullFuncName := runtime.FuncForPC(pc).Name()

	// Extract just the function name using string manipulation
	parts := strings.Split(fullFuncName, ".")
	funcName := parts[len(parts)-1]

	// Print the function name
	fmt.Println("Function name:", funcName)*/

	newCtx := context.WithValue(i.Request().Context(), "meta", map[string]interface{}{
		"function": functionName,
		"activity": utility.MapActionName[functionName],
	})

	// Create a new request with the new context
	req := i.Request().WithContext(newCtx)

	// Set the new request to the echo context
	i.SetRequest(req)
}

func (s *GraphQLServer) buildCommonSystemParam(i echo.Context) (*models.CommonSystemParams, error) {

	param := models.CommonSystemParams{}

	projectID := i.Get("project")
	if projectID != nil {
		param.ProjectID = projectID.(string)
	}

	projectPlan := i.Get("plan")
	if projectPlan != nil {
		param.Plan = projectPlan.(string)
	}

	userId := i.Get("user")
	if userId != nil {
		param.UserID = userId.(string)
	}

	email := i.Get("email")
	if email != nil {
		param.Email = email.(string)
	}

	tenantID := i.Get("tenant")
	if tenantID != nil {
		param.TenantID = tenantID.(string)
	}

	tempTenantID := i.Get("temp_tenant_id")
	if tempTenantID != nil {
		fmt.Println("temp_tenant_id", tempTenantID)
		param.TenantID = tempTenantID.(string)
	}

	role := i.Get("role")
	if role == nil || role == "" {
		return nil, errors.New("invalid Role, Can't Do it")
	}
	param.Role = &models.Role{ID: role.(string)}

	readOnly := i.Get("read_only")
	if readOnly != nil {
		param.Role.ReadOnlyProject = readOnly.(bool)
	}

	isProjectUser := i.Get("is_project_user")
	if isProjectUser != nil {
		param.Role.IsProjectUser = isProjectUser.(bool)
	}

	if param.Role.ID == "admin" {
		param.Role.IsAdmin = true

	} else {
		if param.Role.ID == "demo" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		} else if param.Role.ID == "team" {
			param.Role.SystemGenerated = true
			param.Role.IsAdmin = false
		}
	}
	return &param, nil
}

/*func (s *GraphQLServer) SetUploadService(service *services.UploadService) {
	s.UploadService = service
}*/

// upload
func (s *GraphQLServer) GatherFileInfo(image []byte) (*models.FileDetails, error) {
	fileInfo := models.FileDetails{}
	kind, unknown := filetype.Match(image)
	if unknown != nil {
		return nil, errors.New(fmt.Sprintf(`No Upload File1 %s`, unknown))
	}
	if kind.Extension == "unknown" {
		if svg.Is(image) {
			fileInfo.FileExtension = "svg"
			fileInfo.ContentType = "image/svg+xml"
		} else {
			fmt.Println("Unknown File Type")
		}
	} else {
		fileInfo.FileExtension = kind.Extension
		fileInfo.ContentType = kind.MIME.Value
	}
	return &fileInfo, nil
}

func (s *GraphQLServer) PrepareFileInfo(router echo.Context, projectID string) (*models.FileDetails, *bytes.Buffer, error) {
	file, err := router.FormFile("file")
	if err != nil {
		return nil, nil, errors.New("no Upload File")
	}

	buf := bytes.NewBuffer(nil)
	f, _ := file.Open()
	defer f.Close()

	if _, err := io.Copy(buf, f); err != nil {
		panic(err)
	}
	image := buf.Bytes()

	fileInfo, err := s.GatherFileInfo(image)
	if err != nil {
		return nil, nil, err
	}
	var re = regexp.MustCompile(`[^a-zA-Z0-9]`)
	fileName := re.ReplaceAllString(strings.Split(file.Filename, ".")[0], `_$1`)
	fileInfo.FileName = fileName
	fileInfo.Size = file.Size

	modelName := router.FormValue("model")
	fileInfo.UploadParam = &models.UploadParams{
		ModelName: modelName,
	}

	// get the id
	docId := router.FormValue("id")
	if docId != "" {
		fileInfo.UploadParam.DocID = docId
	}

	fieldName := router.FormValue("field_name")
	if fieldName != "" {
		fileInfo.UploadParam.FieldName = fieldName
	}

	provider := router.FormValue("provider")
	if provider != "" {
		fileInfo.UploadParam.Provider = provider
	}

	fileInfo.UploadParam.ProjectID = projectID
	return fileInfo, buf, nil
}

// common
func (s *GraphQLServer) errorHandler(router echo.Context, response *models.HttpResponse) {
	router.JSON(int(response.Code), response)
}

// PrintAllPluginRoutes prints only plugin routes from both main Echo instance and extension router
// Plugin routes are identified by paths that start with HashiCorp plugin IDs (hc-*)
func (s *GraphQLServer) PrintAllPluginRoutes() {
	// Collect routes from both main instance and extension router
	allRoutes := s.MainEchoInstance.Routes()

	// Filter to show only plugin routes
	pluginRoutes := make([]*echo.Route, 0)
	for _, route := range allRoutes {
		// Plugin routes start with /{pluginID}/ where pluginID typically starts with "hc-"
		if s.isPluginRoute(route.Path) {
			pluginRoutes = append(pluginRoutes, route)
		}
	}

	// Also check the extension router list for registered plugin paths
	if len(pluginRoutes) == 0 && len(s.ExtensionRouterList) > 0 {
		fmt.Printf("🔌 Plugin Routes (%d routes from ExtensionRouter):\n", len(s.ExtensionRouterList))
		fmt.Printf("%s\n", strings.Repeat("-", 50))

		for i, routePath := range s.ExtensionRouterList {
			fmt.Printf("%2d. %-6s %s\n", i+1, "N/A", routePath)
		}
		return
	}

	if len(pluginRoutes) == 0 {
		fmt.Printf("❌ No plugin routes found\n")
		return
	}

	fmt.Printf("🔌 Plugin Routes (%d routes):\n", len(pluginRoutes))
	fmt.Printf("%s\n", strings.Repeat("-", 50))

	for i, route := range pluginRoutes {
		fmt.Printf("%2d. %-6s %s", i+1, route.Method, route.Path)
		if route.Name != "" {
			fmt.Printf(" (name: %s)", route.Name)
		}
		fmt.Printf("\n")
	}
}

// isPluginRoute checks if a route path belongs to a plugin
// Plugin routes are registered with pattern /{pluginID}{route.Path}
// Most HashiCorp plugins start with "hc-" prefix
func (s *GraphQLServer) isPluginRoute(path string) bool {
	// Remove leading slash and split by slash to get first segment
	if !strings.HasPrefix(path, "/") {
		return false
	}

	pathParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(pathParts) == 0 {
		return false
	}

	firstSegment := pathParts[0]

	// Check if the first segment matches known plugin patterns
	// HashiCorp plugins typically start with "hc-"
	if strings.HasPrefix(firstSegment, "hc-") {
		return true
	}

	// Also check against the list of installed HashiCorp plugins
	for _, pluginID := range s.InstalledHCPluginList {
		if firstSegment == pluginID {
			return true
		}
	}

	return false
}

// WaitForPluginsToLoad waits for all plugins to finish loading
func (s *GraphQLServer) WaitForPluginsToLoad() {
	s.wg.Wait()
}
