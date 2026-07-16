package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/database/cache"
	"github.com/apito-io/engine/database/kv"
	"github.com/apito-io/engine/database/realtime"
	"gitlab.com/apito.io/open_driver/system"
	"github.com/apito-io/engine/executor"
	apifn "github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	dl "github.com/apito-io/engine/resolver/dataloader"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/edwingeng/hotswap"
	svg "github.com/h2non/go-is-svg"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
	"gopkg.in/h2non/filetype.v1"
)

// CoreGraphQLServerFactory implements GraphQLServerFactory for open-core version
type CoreGraphQLServerFactory struct{}

// NewCoreGraphQLServerFactory creates a new core GraphQL server factory
func NewCoreGraphQLServerFactory() interfaces.GraphQLServerFactory {
	return &CoreGraphQLServerFactory{}
}

// SupportsVersion returns true if this factory can create servers for the given version/edition
func (f *CoreGraphQLServerFactory) SupportsVersion(version string) bool {
	return version == "core" || version == "open-source"
}

// CreateGraphQLServer creates a core GraphQL server instance
func (f *CoreGraphQLServerFactory) CreateGraphQLServer(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (interfaces.GraphQLServerInterface, error) {
	return BuildGraphQL(ctx, cfg, extensionRouter, mainEcho)
}

// Global factory variable that can be overridden by pro version
var globalGraphQLServerFactory interfaces.GraphQLServerFactory

// SetGraphQLServerFactory allows pro version to inject its own factory
func SetGraphQLServerFactory(factory interfaces.GraphQLServerFactory) {
	globalGraphQLServerFactory = factory
}

// GetGraphQLServerFactory returns the appropriate factory (pro if available, otherwise core)
func GetGraphQLServerFactory() interfaces.GraphQLServerFactory {
	if globalGraphQLServerFactory != nil {
		return globalGraphQLServerFactory
	}
	return NewCoreGraphQLServerFactory()
}

// BuildGraphQLWithFactory creates a GraphQL server using the factory pattern
// This is the main entry point that routers should use
func BuildGraphQLWithFactory(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (interfaces.GraphQLServerInterface, error) {
	factory := GetGraphQLServerFactory()
	return factory.CreateGraphQLServer(ctx, cfg, extensionRouter, mainEcho)
}

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
	ProjectKeyManager  *services.ProjectKeyManager

	ApitoTokenService *services.ApitoTokenService
	JWTTokenService   *services.JWTService

	//S3          *storage_driver.S3
	AuthService services.AuthServiceInterface
	//JwtService         *services.JWTService
	ProjectDBConnPools *sync.Map

	ProjectCache interfaces.CacheDBInterface

	// for HashiCorp plugins only
	HashiCorpPluginCache map[string]*models.HashiCorpPluginCache
	PluginLoadingState   map[string]bool // Track which plugins are currently being loaded
	PluginMonitor        *PluginMonitor  // Health monitoring and auto-restart for plugins

	// Connection pool monitoring
	ConnectionMonitor *database.ConnectionMonitor // Health monitoring for database connections

	InstalledPluginList   []string
	InstalledHCPluginList []string

	ExtensionRouterList []string
	ExtensionRouter     *echo.Group
	MainEchoInstance    *echo.Echo // Reference to the main Echo instance for route introspection

	//LocalPluginGraphQLSchemas chan *extensions.ThirdPartyGraphQLSchemas
	//LocalPluginRoutes chan []*extensions.ThirdPartyRESTApi

	RealtimeBus         interfaces.RealtimeBus
	KVService           interfaces.KeyValueServiceInterface

	// FunctionRuntime dispatches Apito Functions (deno/wasm). Nil disables platform functions.
	FunctionRuntime *apifn.RuntimeManager

	PluginManagerSwapper *hotswap.PluginManagerSwapper

	MicroServiceClient *sync.Map

	pluginMissCacheWarned sync.Map // tracks which "project:plugin" pairs we already warned about
}

func BuildGraphQL(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (*GraphQLServer, error) {

	kvStorage, err := kv.CreateKVDriver(cfg.KVStorageEngine, cfg)
	if err != nil {
		return nil, err
	}

	realtimeBus, err := realtime.CreateRealtimeBus(cfg.RealtimeEngine, cfg)
	if err != nil {
		return nil, err
	}

	srv := GraphQLServer{

		wg:  sync.WaitGroup{},
		Cfg: cfg,

		SystemQueriesChan:   make(chan *graphql.Fields),
		SystemMutationsChan: make(chan *graphql.Fields),

		SystemDriverReadyChan: make(chan interfaces.ApitoSystemDB, 1),

		KVService:   kvStorage,
		RealtimeBus: realtimeBus,

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

	// Apito Functions: Deno primary + wazero secondary behind a runtime-neutral manager.
	artifactRoot := ""
	if cfg != nil && cfg.DefaultDatabaseDir != "" {
		artifactRoot = cfg.DefaultDatabaseDir + "/function-artifacts"
	}
	store, _ := apifn.NewFilesystemArtifactStore(artifactRoot)
	gateway := apifn.NewEngineDataGateway()
	batch := apifn.NewMemoryBatchExecutor()
	apifn.RegisterCoreDataOps(gateway, batch)
	globalConc := 16
	if cfg != nil && cfg.FunctionGlobalConcurrency > 0 {
		globalConc = cfg.FunctionGlobalConcurrency
	}
	var limitsProvider apifn.FunctionLimitsProvider = apifn.DefaultLimitsProvider{}
	if cfg != nil && cfg.FunctionLimitsHook != nil {
		limitsProvider = apifn.HookLimitsProvider{Hook: cfg.FunctionLimitsHook}
	}
	srv.FunctionRuntime = apifn.NewRuntimeManager(
		[]apifn.RuntimeProvider{
			selectDenoProvider(),
			apifn.NewWazeroRuntimeProvider(ctx),
		},
		apifn.WithTransport(apifn.NewLocalTransport()),
		apifn.WithLimitsProvider(limitsProvider),
		apifn.WithDataGateway(gateway),
		apifn.WithArtifactStore(store),
		apifn.WithGlobalConcurrency(globalConc),
	)

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
	tokenWg.Add(1)
	go func() {
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
		systemDriver, err := system.GetSystemDriver(cfg, &_cred)
		if err != nil {
			panic(err.Error()) // sure do a panic if system db not there
		}
		srv.SystemDriverReadyChan <- systemDriver
	}()

	// auth services
	tokenWg.Add(1)
	go func(cfg *models.Config) {
		defer tokenWg.Done()
		var authService services.AuthServiceInterface
		var err error

		tokenService := services.GetJWTServiceWithRedis(cfg, srv.KVService)

		authService, err = services.NewLocalAuthService(cfg, tokenService)
		if err != nil {
			fmt.Println(err.Error())
		}

		srv.JWTTokenService = tokenService
		srv.AuthService = authService
	}(cfg)

	// cache driver
	go func() {
		_cache, err := cache.CreateCacheDriver(cfg, cfg.CacheEngine)
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

	go srv.RegisterUserSchema()

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

		migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 90*time.Second)
		if err := systemDB.RunMigration(migrateCtx); err != nil {
			migrateCancel()
			panic(fmt.Sprintf("system DB migration failed: %v", err))
		}
		migrateCancel()
		log.Println("system DB migration completed")

		bootCtx, bootCancel := context.WithTimeout(context.Background(), 120*time.Second)
		if err := systemDB.EnsureSystemBootstrap(bootCtx); err != nil {
			bootCancel()
			panic(fmt.Sprintf("system DB bootstrap failed: %v", err))
		}
		bootCancel()
		log.Println("system DB EnsureSystemBootstrap completed")

		_executor := executor.GetGraphQLExecutor(cfg, systemDB)
		err := _executor.Init(ctx, &models.InitParams{
			SharedDB: &models.DriverCredentials{
				Engine:   cfg.KVStorageEngine,
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
		apiKeyManager, err := services.NewProjectKeyManager(cfg, systemDB)
		if err != nil {
			panic(err)
		}
		srv.ProjectKeyManager = apiKeyManager

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

// QueryBuilderInformation moved to interfaces package for compatibility
// Keeping this as alias for backward compatibility
type QueryBuilderInformation = interfaces.QueryBuilderInformation

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
	if s == nil || s.RealtimeBus == nil {
		return fmt.Errorf("realtime bus is not configured")
	}

	data.UserID = userID

	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	topic := s.realtimeTopic(ctx, RealtimeSystemUserNotifyTopic(userID))
	if err := s.RealtimeBus.Publish(ctx, topic, payload); err != nil {
		return err
	}

	// Also fan-out on project-scoped subject when project + type are set.
	if data.ProjectID != "" && data.Type != "" {
		projectTopic := s.realtimeTopic(ctx, RealtimeSystemProjectEventTopic(data.ProjectID, data.Type))
		_ = s.RealtimeBus.Publish(ctx, projectTopic, payload)
	}

	return nil
}

func (s *GraphQLServer) UpdateApplicationCache(ctx context.Context, projectID string) {
	s.Lock()
	err := s.ProjectCache.Expire(ctx, projectID)
	if err != nil {
		return
	}
	s.Unlock()
}

// refreshProjectCacheFromSystem reloads the project from the system DB and upserts ProjectCache.
// Use after SystemDriver.UpdateProject when schema/models changed so queries like projectModelsInfo see fresh data.
func (s *GraphQLServer) refreshProjectAndReCache(ctx context.Context, projectID string) (*models.Project, error) {
	fresh, err := s.SystemDriver.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if fresh != nil && fresh.Schema != nil {
		models.NormalizeProjectSchemaConnectionTypes(fresh.Schema)
		applyRuntimeSchemaAugments(fresh)
	}
	if _, err := s.ProjectCache.SaveProject(ctx, fresh); err != nil {
		return nil, err
	}
	return fresh, nil
}

// RefreshProjectAndReCache reloads project from system DB into ProjectCache (exported for pro publish).
func (s *GraphQLServer) RefreshProjectAndReCache(ctx context.Context, projectID string) (*models.Project, error) {
	return s.refreshProjectAndReCache(ctx, projectID)
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

// ensureExecutorProjectDriver registers the project's driver with the in-memory ConnectionManager
// via GraphQLExecutor.Init. Project metadata may be served from Redis (or similar) while the
// executor's connection table is process-local, so this must run on cache hits as well as
// after a system-DB load (e.g. after deploy or eviction).
func (s *GraphQLServer) ensureExecutorProjectDriver(ctx context.Context, project *models.Project) error {
	if project == nil || project.Driver == nil {
		return nil
	}
	if s.Cfg != nil && s.Cfg.LoadProjectCacheHook != nil {
		s.Cfg.LoadProjectCacheHook(ctx, project)
	}
	project.Driver.ProjectID = project.ID
	return s.GraphQLExecutor.Init(ctx, &models.InitParams{
		ProjectID: project.ID,
		ProjectDB: project.Driver,
		SharedDB: &models.DriverCredentials{
			ProjectID: project.ID,
			Engine:    _const.RedisDriver,
			Host:      s.Cfg.KVStorageEngineHost,
			Port:      s.Cfg.KVStorageEnginePort,
			Password:  s.Cfg.KVStorageEnginePassword,
			Database:  s.Cfg.KVStorageEngineDatabase,
		},
	})
}

func (s *GraphQLServer) LoadProjectCache(ctx context.Context, projectID string) (*models.Project, error) {

	// get the project details
	var _project *models.Project
	if val, err := s.ProjectCache.GetProject(ctx, projectID); err == nil && val != nil && val.ID != "" {
		_project = val
	} else {
		var err error
		_project, err = s.SystemDriver.GetProject(ctx, projectID)
		if err != nil {
			return nil, err
		}

		// Local plugin cache removed - using HashiCorp plugins only
		_, err = s.ProjectCache.SaveProject(ctx, _project)
		if err != nil {
			return nil, err
		}
	}

	if err := s.ensureExecutorProjectDriver(ctx, _project); err != nil {
		return nil, err
	}

	if _project != nil {
		if _project.Schema != nil {
			models.NormalizeProjectSchemaConnectionTypes(_project.Schema)
			applyRuntimeSchemaAugments(_project)
		}
		if err := s.ApplyNamingV2AfterProjectLoad(ctx, _project); err != nil {
			return nil, err
		}
	}

	return _project, nil
}

// applyRuntimeSchemaAugments patches in-memory project schema with system models that are not persisted.
func applyRuntimeSchemaAugments(project *models.Project) {
	if project == nil || project.Schema == nil {
		return
	}
	models.EnsureProjectAuthUserModelInSchema(project.Schema)
}

func isArangoProjectEngine(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	return e == "arangodb" || e == "arango"
}

// ApplyNamingV2AfterProjectLoad runs optional Arango physical migration, then in-memory naming V2
// schema migration and persistence. Safe to call on every load (no-op when already on V2).
func (s *GraphQLServer) ApplyNamingV2AfterProjectLoad(ctx context.Context, project *models.Project) error {
	if project == nil || project.Schema == nil {
		return nil
	}
	if project.Schema.NamingSchemaVersion >= utility.NamingSchemaVersionV2 {
		return nil
	}
	pairs, err := utility.ComputeNamingV2ModelRenamePairs(project)
	if err != nil {
		return err
	}
	needsPhysical := len(pairs) > 0 && project.Driver != nil && isArangoProjectEngine(project.Driver.Engine)
	if needsPhysical {
		perModel := false
		if s.Cfg != nil && s.Cfg.NamingV2ArangoPerModelCollections != nil {
			perModel = s.Cfg.NamingV2ArangoPerModelCollections(ctx, project)
		}
		sub := context.WithValue(ctx, "project_id", project.ID)
		drv, err := s.GraphQLExecutor.GetProjectDriver(sub)
		if err != nil {
			return fmt.Errorf("naming v2: project driver: %w", err)
		}
		relationTenantModel := ""
		if s.Cfg != nil && s.Cfg.NamingV2RelationTenantModel != nil {
			relationTenantModel = strings.TrimSpace(s.Cfg.NamingV2RelationTenantModel(ctx, project))
		}
		if migrator, ok := drv.(utility.NamingV2PhysicalMigrator); ok {
			if err := migrator.ApplyNamingV2PhysicalMigration(sub, project.ID, pairs, perModel, relationTenantModel); err != nil {
				 return fmt.Errorf("naming v2 physical migration: %w", err)
			}
		}
	}
	changed, err := utility.MigrateProjectSchemaToNamingV2(project)
	if err != nil {
		return err
	}
	if changed {
		if err := s.SystemDriver.UpdateProject(ctx, project, false); err != nil {
			return err
		}
		if _, err := s.ProjectCache.SaveProject(ctx, project); err != nil {
			return err
		}
	}
	return nil
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
	if s.Cfg != nil && cache.Param != nil {
		cache.Param.RuntimeConfig = s.Cfg
	}

	// Try to reuse previously cached plugin schemas (per project) to avoid re-registering every request
	if existing, err := s.ProjectCache.GetAppCache(ctx, projectID); err == nil && existing != nil && existing.RawSchemas != nil {
		cache.RawSchemas = existing.RawSchemas
	}

	// Load project-specific plugins into the cache only when not already cached
	if cache.RawSchemas == nil || (len(cache.RawSchemas.Queries) == 0 && len(cache.RawSchemas.Mutations) == 0) {
		fmt.Printf("[DEBUG] GetApplicationCache: Loading project-specific plugins for project %s\n", _project.ID)
		err = s.LoadProjectSpecificPlugins(ctx, cache)
		if err != nil {
			fmt.Printf("[ERROR] GetApplicationCache: Failed to load project-specific plugins for project %s: %v\n", _project.ID, err)
		} else {
			_ = s.ProjectCache.PutAppCache(ctx, projectID, &models.ApplicationCache{RawSchemas: cache.RawSchemas})
			fmt.Printf("[DEBUG] GetApplicationCache: Project-specific plugins loaded and cached for project %s\n", _project.ID)
		}

		// Warn once per (project, plugin) when a project-enabled hc-* plugin wasn't in the engine cache
		for _, pd := range _project.Plugins {
			if !pd.Enable || !strings.HasPrefix(pd.ID, "hc-") {
				continue
			}
			if cache.PluginSchemasRegistered != nil && cache.PluginSchemasRegistered[pd.ID] {
				continue
			}
			warnKey := _project.ID + ":" + pd.ID
			if _, alreadyWarned := s.pluginMissCacheWarned.LoadOrStore(warnKey, true); !alreadyWarned {
				fmt.Printf("⚠️  [PLUGIN-PROJECT] project=%s plugin=%s: enabled in project document but not found in engine plugin cache — "+
					"plg_* queries/mutations from this plugin won't appear until it is loaded (startup config or PLUGIN-V2 admin API).\n",
					_project.ID, pd.ID)
			}
		}
	} else {
		fmt.Printf("[DEBUG] GetApplicationCache: Using cached project-specific plugin schemas for project %s\n", _project.ID)
	}

	if h := s.Cfg.PostApplicationCacheHook; h != nil {
		if fn, ok := h.(func(echo.Context, *models.ApplicationCache)); ok {
			fn(router, cache)
		}
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

	if s.Cfg.BuildSystemParamHook != nil {
		s.Cfg.BuildSystemParamHook(i.Request().Context(), project, param)
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
		} else if param.Role.ID == "owner" {
			// SQL CreateProject historically wrote user_projects.role = "owner" while project.roles only
			// defines "admin" (Arango/Mongo use "admin"). Treat owner like admin when that template exists.
			if val, ok := project.Roles["admin"]; ok && val != nil {
				merged := *val
				if merged.ID == "" {
					merged.ID = "owner"
				}
				param.Role = &merged
			} else {
				param.Role.IsAdmin = false
				param.Role.SystemGenerated = false
			}
		} else {
			// Project end-user roles (e.g. "none", custom app roles from loginUser tokens).
			param.Role.IsAdmin = false
			param.Role.SystemGenerated = false
		}
	}
	return param, nil
}

func (s *GraphQLServer) invokeCreateTableOrCollection(ctx context.Context, driver interfaces.ProjectDBInterface, param *models.CommonSystemParams, isRelation bool) error {
	var idx []string
	if isRelation {
		idx = []string{models.IndexesRelationCollectionToken}
	}
	return driver.CreateTableOrCollection(ctx, param, idx)
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
	if s.Cfg != nil {
		param.RuntimeConfig = s.Cfg
	}
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

// GetHashiCorpPluginIDs returns the list of loaded HashiCorp plugin IDs
func (s *GraphQLServer) GetHashiCorpPluginIDs() []string {
	var pluginIDs []string
	for pluginID := range s.HashiCorpPluginCache {
		pluginIDs = append(pluginIDs, pluginID)
	}
	return pluginIDs
}

// SetPluginMonitor sets the plugin monitor
func (s *GraphQLServer) SetPluginMonitor(monitor interface{}) {
	if pm, ok := monitor.(*PluginMonitor); ok {
		s.PluginMonitor = pm
	}
}

// SetConnectionMonitor sets the connection monitor
func (s *GraphQLServer) SetConnectionMonitor(monitor *database.ConnectionMonitor) {
	s.ConnectionMonitor = monitor
}

// GetConnectionMonitor returns the connection monitor
func (s *GraphQLServer) GetConnectionMonitor() *database.ConnectionMonitor {
	return s.ConnectionMonitor
}

// GetConnectionManager returns the connection manager from the executor
func (s *GraphQLServer) GetConnectionManager() *database.ConnectionManager {
	if exec, ok := s.GraphQLExecutor.(*executor.GraphQLExecutor); ok {
		return exec.GetConnectionManager()
	}
	return nil
}

// GetConcreteServer returns the concrete server instance for controller compatibility
func (s *GraphQLServer) GetConcreteServer() interface{} {
	return s
}

// TryGetPlugin attempts to get a plugin from cache without blocking
// This provides public access to plugin cache for controllers
func (s *GraphQLServer) TryGetPlugin(pluginID string) *models.HashiCorpPluginCache {
	return s.tryGetPluginNoBlock(pluginID)
}
