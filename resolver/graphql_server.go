package resolver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/buffers/plugins"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	_const "github.com/apito-io/databasedriver"
	"github.com/apito-io/databasedriver/cache/badger"
	memoryCache "github.com/apito-io/databasedriver/cache/memory"
	redisCache "github.com/apito-io/databasedriver/cache/redis"
	kvBadger "github.com/apito-io/databasedriver/kv/badger"
	kvRedis "github.com/apito-io/databasedriver/kv/redis"
	"github.com/apito-io/databasedriver/project"
	"github.com/apito-io/databasedriver/system"
	"github.com/apito-io/engine/executor"
	"github.com/apito-io/engine/models"
	dl "github.com/apito-io/engine/resolver/dataloader"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/apito-io/engine/services"
	"github.com/arangodb/go-driver"
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

	SystemDriver          interfaces.SystemDBInterface
	SystemDriverReadyChan chan interfaces.SystemDBInterface

	PrivateSchemaObjects *objects.SchemaObjects
	SystemDataloaders    *dl.SystemDataloader

	SystemQueries       graphql.Fields
	SystemQueriesChan   chan *graphql.Fields
	SystemMutations     graphql.Fields
	SystemMutationsChan chan *graphql.Fields

	GraphQLExecutor interfaces.GraphQLExecutorInterface

	SystemRoles []string
	//UploadService *services.UploadService
	//ProjectRawSchemas *protobuff.ProjectSchema
	BlankaTokenService *services.BrankaToken
	ApitoTokenService  *services.ApitoTokenService
	JWTTokenService    *services.JWTService

	//S3          *storage_driver.S3
	AuthService services.AuthServiceInterface

	//JwtService         *services.JWTService
	ProjectDBConnPools *sync.Map

	ProjectCache interfaces.CacheDBInterface

	// for global plugin & functions
	LocalPluginCache map[string]*models.PluginCache
	FunctionCache    map[string]*models.FunctionCache

	InstalledPluginList []string
	FunctionProviderIds []string
	StorageProviderIds  []string

	PluginsRouter *echo.Group

	LocalPluginGraphQLSchemas chan *plugins.ThirdPartyGraphQLSchemas
	LocalPluginRoutes         chan []*plugins.ThirdPartyRESTApi

	KVService interfaces.KeyValueServiceInterface

	PluginManagerSwapper *hotswap.PluginManagerSwapper
}

func BuildGraphQLServer(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group) (*GraphQLServer, error) {

	var kvStorage interfaces.KeyValueServiceInterface
	var err error
	switch cfg.KeyValueEngine {
	case _const.RedisDriver:
		kvStorage, err = kvRedis.GetKVRedisDriver(ctx, cfg.KeyValueDBConfig)
	case _const.EmbeddedDB:
		kvStorage, err = kvBadger.GetKVBadgerDriver()
	default:
		kvStorage, err = kvBadger.GetKVBadgerDriver()
	}
	if err != nil {
		return nil, err
	}

	// set the project driver and build param
	_executor := executor.GetGraphQLExecutor()
	err = _executor.Init(ctx, &protobuff.InitParams{
		ProjectDB: &protobuff.DriverCredentials{
			Engine:   cfg.ProjectDatabaseEngine,
			Host:     cfg.ProjectDatabaseDBConfig.Host,
			Port:     cfg.ProjectDatabaseDBConfig.Port,
			User:     cfg.ProjectDatabaseDBConfig.User,
			Password: cfg.ProjectDatabaseDBConfig.Password,
			Database: cfg.ProjectDatabaseDBConfig.Database,
		},
	})
	if err != nil {
		return nil, err
	}

	srv := GraphQLServer{

		wg:  sync.WaitGroup{},
		Cfg: cfg,

		SystemQueriesChan:   make(chan *graphql.Fields),
		SystemMutationsChan: make(chan *graphql.Fields),

		SystemDriverReadyChan: make(chan interfaces.SystemDBInterface, 1),

		KVService: kvStorage,

		GraphQLExecutor: _executor,

		ProjectDBConnPools: &sync.Map{},
		PluginsRouter:      extensionRouter,

		LocalPluginCache: make(map[string]*models.PluginCache),
		FunctionCache:    make(map[string]*models.FunctionCache),

		InstalledPluginList: []string{},
		FunctionProviderIds: []string{},
		StorageProviderIds:  []string{},

		LocalPluginGraphQLSchemas: make(chan *plugins.ThirdPartyGraphQLSchemas),
		LocalPluginRoutes:         make(chan []*plugins.ThirdPartyRESTApi),
	}

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
		_cred := protobuff.DriverCredentials{
			Engine:   cfg.SystemDatabaseEngine,
			Host:     cfg.SystemDatabaseDBConfig.Host,
			Port:     cfg.SystemDatabaseDBConfig.Port,
			User:     cfg.ProjectDatabaseDBConfig.User,
			Password: cfg.ProjectDatabaseDBConfig.Password,
			Database: cfg.ProjectDatabaseDBConfig.Database,
		}
		systemDriver, err := system.GetSystemDriver(&_cred, cfg.SystemDatabaseDBConfig)
		if err != nil {
			panic(err.Error()) // sure do a panic if system db not there
		}
		srv.SystemDriverReadyChan <- systemDriver
	}()

	// auth services
	go func(cfg *models.Config) {
		tokenWg.Add(1)
		defer tokenWg.Done()
		var authService services.AuthServiceInterface
		var err error

		tokenService := services.GetJWTServiceWithRedis(cfg, srv.KVService)

		switch cfg.AuthServiceProvider {
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
		var _cache interfaces.CacheDBInterface
		switch cfg.CacheDBEngine {
		case _const.MemoryDb:
			_cache, err = memoryCache.GetCacheDriver()
			if err != nil {
				fmt.Println(err.Error())
			}
		case _const.EmbeddedDB:
			_cache, err = badger.GetCacheDriver()
			if err != nil {
				fmt.Println(err.Error())
			}
		case _const.RedisDriver:
			_cache, err = redisCache.GetCacheDriver(&shared.CacheDBConfig{
				DB:          cfg.CacheDBConfig,
				CacheDriver: "",
				CacheTTL:    "3600",
			})
			if err != nil {
				fmt.Println(err.Error())
			}
		default:
			_cache, err = memoryCache.GetCacheDriver()
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

	// Copy Local Plugin Queries & Mutations
	go func() {
		isPluginLoaded := false

		var localGraphQLSchemas *plugins.ThirdPartyGraphQLSchemas

		for {
			select {
			case localGraphQLSchemas = <-srv.LocalPluginGraphQLSchemas:
				isPluginLoaded = true
			case _systemQuery := <-srv.SystemQueriesChan:
				srv.SystemQueries = *_systemQuery
			case _mutationQuery := <-srv.SystemMutationsChan:
				srv.SystemMutations = *_mutationQuery
			}

			if isPluginLoaded && srv.SystemQueries != nil && srv.SystemMutations != nil {
				for k, v := range localGraphQLSchemas.Queries {
					if _, ok := srv.SystemQueries[k]; !ok && v != nil {
						srv.SystemQueries[k] = v
					} else {
						fmt.Println(fmt.Sprintf(`the system already has a query named '%s'. please check/choose a different name for extension query. ignoring this one.`, k))
					}
				}

				for k, v := range localGraphQLSchemas.Mutations {
					srv.SystemMutations[k] = v
				}
				break
			}
		}

	}()

	// Copy Local Plugin rest api register
	go func() {
		select {
		case localRoutes := <-srv.LocalPluginRoutes:
			for _, _route := range localRoutes {
				time.Sleep(500 * time.Millisecond)
				srv.PluginsRouter.Use(srv.Authorize()) // add the middleware
				srv.PluginsRouter.Add(_route.Method, _route.Path, _route.Controller)
			}
		}
	}()

	// system db dependent services ( payment, blanka token, apito token )
	go func() {

		select {
		case systemDB := <-srv.SystemDriverReadyChan:

			fmt.Println("system driver finished initialized")
			srv.SystemDriver = systemDB

			fmt.Println("building system graphql queries and mutations")
			srv.BuildServerQueriesAndMutations()

			fmt.Println("initializing blanka token")
			srv.BlankaTokenService = services.GetBrankaToken(cfg, systemDB)

			fmt.Println("initializing apito token service")
			tokenWg.Wait() // depends on systemDb & part of apito token service
			apitoTokenService, err := services.NewApitoTokenService(cfg, srv.AuthService, systemDB)
			if err != nil {
				panic(err)
			}
			srv.ApitoTokenService = apitoTokenService

			srv.wg.Done()
		}

	}()
	//defer conn.Close()

	return &srv, nil
}

type QueryBuilderInformation struct {
	DataObjects       graphql.Fields
	WhereParamObjects graphql.InputObjectConfigFieldMap
	SortParamObjects  graphql.InputObjectConfigFieldMap
}

func (s *GraphQLServer) Authorize() echo.MiddlewareFunc {
	s.wg.Wait()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		err := s.ApitoTokenService.ApitoTokenHandlr(next)
		return err
	}
}

func (s *GraphQLServer) SetOnlyProjectDriver(ctx context.Context, credentials *protobuff.DriverCredentials) error {
	projectDriver, err := project.GetProjectDriver(credentials)
	if err != nil {
		return err
	}
	s.GraphQLExecutor.SetProjectDriver(ctx, projectDriver)
	return nil
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
	s.Lock()
	err := s.ProjectCache.Expire(ctx, projectId)
	if err != nil {
		return err
	}
	s.Unlock()
	return nil
}

func (s *GraphQLServer) cacheId(projectId string, modelName string) string {
	return fmt.Sprintf(`%s#%s`, projectId, modelName)
}

func (s *GraphQLServer) UpdateApplicationCache(ctx context.Context, project *protobuff.Project) (*protobuff.Project, error) {
	s.Lock()
	_project, err := s.ProjectCache.SaveProject(ctx, project)
	if err != nil {
		return nil, err
	}
	s.Unlock()
	return _project, nil
}

func (s *GraphQLServer) GetFunctionProvider() ([]string, error) {
	return nil, nil
}

func (s *GraphQLServer) GetStorageProvider() ([]string, error) {
	return nil, nil
}

func (s *GraphQLServer) GetApplicationCache(router echo.Context) (*shared.ApplicationCache, error) {

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

	//s.Lock()

	// get the project details
	var _project *protobuff.Project
	if val, err := s.ProjectCache.GetProject(ctx, projectID); err == nil && val != nil && val.Id != "" {
		_project = val
	} else {
		_project, err = s.SystemDriver.GetProject(ctx, projectID)
		if err != nil {
			if driver.IsNoMoreDocuments(err) {
				return nil, errors.New(fmt.Sprintf("project not found with the id : %s", projectID))
			} else {
				return nil, err
			}
		}
		_, err = s.ProjectCache.SaveProject(ctx, _project)
		if err != nil {
			return nil, err
		}
	}

	param, err := s.buildSystemParam(router, _project)
	if err != nil {
		return nil, err
	}

	return &shared.ApplicationCache{
		Project: _project,
		Param:   param,
	}, nil
}

func (s *GraphQLServer) buildSystemParam(i echo.Context, project *protobuff.Project) (*shared.CommonSystemParams, error) {
	param, err := s.buildCommonSystemParam(i)
	if err != nil {
		return nil, err
	}
	return param, nil
}

func (s *GraphQLServer) NewParam(_param *shared.CommonSystemParams) *shared.CommonSystemParams {
	param := new(shared.CommonSystemParams)
	*param = *_param
	return param
}

func (s *GraphQLServer) buildCommonSystemParam(i echo.Context) (*shared.CommonSystemParams, error) {

	param := shared.CommonSystemParams{}

	projectID := i.Get("project")
	if projectID != nil {
		param.ProjectId = projectID.(string)
	}

	userId := i.Get("user")
	if userId != nil {
		param.UserId = userId.(string)
	}

	email := i.Get("email")
	if email != nil {
		param.Email = email.(string)
	}

	return &param, nil
}

// upload
func (s *GraphQLServer) GatherFileInfo(image []byte) (*protobuff.FileDetails, error) {
	fileInfo := protobuff.FileDetails{}
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

func (s *GraphQLServer) PrepareFileInfo(router echo.Context, projectID string) (*protobuff.FileDetails, *bytes.Buffer, error) {
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
	fileInfo.UploadParam = &protobuff.UploadParams{
		ModelName: modelName,
	}

	// get the id
	docId := router.FormValue("id")
	if docId != "" {
		fileInfo.UploadParam.DocId = docId
	}

	fieldName := router.FormValue("field_name")
	if fieldName != "" {
		fileInfo.UploadParam.FieldName = fieldName
	}

	provider := router.FormValue("provider")
	if provider != "" {
		fileInfo.UploadParam.Provider = provider
	}

	fileInfo.UploadParam.ProjectId = projectID
	return fileInfo, buf, nil
}

// common
func (s *GraphQLServer) errorHandler(router echo.Context, response *models.HttpResponse) {
	router.JSON(int(response.Code), response)
}
