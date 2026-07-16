package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apito-io/wsgraphql/v1"
	"github.com/apito-io/wsgraphql/v1/compat/gorillaws"
	apifn "github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/telemetry"
	"github.com/apito-io/engine/scaler"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
	"github.com/teivah/onecontext"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"

	//"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/schemas"
	"github.com/apito-io/engine/utility"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("graphql-controller")

type GraphCtrl struct {
	cfg                *models.Config
	subscriptionSchema *graphql.Schema
	gqlServer          *resolver.GraphQLServer
	srv                wsgraphql.Server
}

func GetGraphQLController(cfg *models.Config, commonFn *resolver.GraphQLServer) *GraphCtrl {

	ctx := context.Background()
	gqlSchema, err := schemas.SystemSubscriptionSchema(ctx, commonFn)
	if err != nil {
		fmt.Println(err.Error())
	}

	// subscriptions / websocket handler
	subHandler, err := newSubscriptionWSServer(*gqlSchema)
	if err != nil {
		fmt.Println(err.Error())
	}

	return &GraphCtrl{
		cfg:                cfg,
		subscriptionSchema: gqlSchema,
		gqlServer:          commonFn,
		srv:                subHandler,
	}
}

// newSubscriptionWSServer builds a wsgraphql websocket server for a given schema
// using the standard apollo graphql-ws / graphql-transport-ws configuration.
// It is reused for both the system subscription schema (built once at boot) and
// per-connection public subscription schemas.
func newSubscriptionWSServer(schema graphql.Schema) (wsgraphql.Server, error) {
	return wsgraphql.NewServer(
		schema,
		wsgraphql.WithKeepalive(time.Second*30),
		wsgraphql.WithConnectTimeout(time.Second*30),
		wsgraphql.WithUpgrader(gorillaws.Wrap(&websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
			Subprotocols: []string{
				wsgraphql.WebsocketSubprotocolGraphqlWS.String(),
				wsgraphql.WebsocketSubprotocolGraphqlTransportWS.String(),
			},
		})),
	)
}

func (g *GraphCtrl) PluginUpload(router echo.Context) error {

	req := make(map[string]interface{})
	if err := router.Bind(&req); err != nil {
		return router.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	//build a _plugin config from the req
	/*_pluginDetails := &protobuff.PluginDetails{
		Icon:             req["icon"].(string),
		Id:               req,
		Title:            "",
		Version:          "",
		Description:      "",
		Type:             0,
		Role:             "",
		EnvVars:          nil,
		ExportedVariable: "",
		Enable:           false,
		RepositoryUrl:    "",
		Branch:           "",
		Author:           "",
		LoadStatus:       0,
	}

	if val, ok := req["icon"].(string); ok {
		_pluginDetails.Icon = val
	}

	if val, ok := req["id"].(string); ok {
		_pluginDetails.Id = val
	}

	if val, ok := req["title"].(string); ok {
		_pluginDetails.Title = val
	}

	if val, ok := req["version"].(string); ok {
		_pluginDetails.Version = val
	}

	if val, ok := req["description"].(string); ok {
		_pluginDetails.Description = val
	}

	if val, ok := req["type"].(string); ok {
		_pluginDetails.Type = val
	}

	if val, ok := req["role"].(string); ok {
		_pluginDetails.Role = val
	}

	if val, ok := req["env"].(string); ok {
		_pluginDetails.Icon = val
	}

	if val, ok := req["icon"].(string); ok {
		_pluginDetails.Icon = val
	}

	if val, ok := req["icon"].(string); ok {
		_pluginDetails.Icon = val
	}
	*/

	var pluginID string
	if val, ok := req["id"].(string); ok {
		pluginID = val
	}

	cache, err := g.gqlServer.GetApplicationCache(router)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	param := cache.Param

	if param.Role.ID == "demo" && param.Role.SystemGenerated {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: "You Cant Change Anything in a Demo Project",
		})
	}

	_, buffer, err := g.gqlServer.PrepareFileInfo(router, param.ProjectID)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	reader := bytes.NewReader(buffer.Bytes())

	dir := fmt.Sprintf(`plugins/local/%s`, pluginID)

	path := fmt.Sprintf(`%s/main.so`, dir)

	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	w, err := os.Create(path)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}
	defer w.Close()

	// do the actual work
	_, err = io.Copy(w, reader)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	//ctx := router.Request().Context()

	/* _pluginDetails, err := g.gqlServer.LoadLocalPlugin(ctx, dir, nil)
	if err != nil {
		err = g.gqlServer.PublishSystemMessage(ctx, param.UserID, &protobuff.SubscriptionEvent{
			Type:    "error",
			Message: err.Error(),
		})
		if err != nil {
			return router.JSON(http.StatusBadRequest, models.HttpResponse{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			})
		}
	} */

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		//Body: _pluginDetails,
	})
}

func (g *GraphCtrl) FunctionExecute(c echo.Context) error {

	ctx := c.Request().Context()

	projectId := c.Param("project_id")

	project, err := g.gqlServer.LoadProjectCache(ctx, projectId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	fnName := c.Param("fn_name")

	var req map[string]interface{}
	if err = c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	fn := apifn.FindFunctionByName(project, fnName)
	if fn == nil {
		return c.JSON(http.StatusNotFound, &models.HttpResponse{
			Message: "function not found",
			Code:    http.StatusNotFound,
		})
	}
	if !apifn.AllowCallableRuntime(fn) {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "function runtime not callable via REST",
			Code:    http.StatusForbidden,
		})
	}

	authMode := "secret"
	if g.cfg != nil && g.cfg.FunctionCallableAuthMode != "" {
		authMode = g.cfg.FunctionCallableAuthMode
	}
	if authMode == "disabled" {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "callable REST /function is disabled",
			Code:    http.StatusForbidden,
		})
	}
	provided, _ := c.Get("function_hash").(string)
	if err := apifn.VerifyFunctionSecret(fn, provided); err != nil {
		return c.JSON(http.StatusUnauthorized, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusUnauthorized,
		})
	}

	// Deno/wasm platform functions require a configured runtime manager.
	if fn.IsApitoFunctionsRuntime() && g.gqlServer.FunctionRuntime == nil {
		return c.JSON(http.StatusServiceUnavailable, &models.HttpResponse{
			Message: "function runtime not configured",
			Code:    http.StatusServiceUnavailable,
		})
	}

	fnStart := time.Now()
	resp, _fn, err := g.gqlServer.HandleApitoFunction(ctx, &models.ApplicationCache{
		Project: project,
		Param: &models.CommonSystemParams{
			ProjectID: projectId,
		},
	}, fnName, map[string]interface{}{
		"payload": req,
	})
	telemetry.RecordFunctionExecute(ctx, g.cfg, fnName, err, time.Since(fnStart))
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if _fn.Response != nil && _fn.Response.Model == "JSON" {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"JSON": resp,
		})
	}
	return c.JSON(http.StatusOK, resp)
}

func (g *GraphCtrl) RestToGraphQL(c echo.Context) error {

	cache, err := g.gqlServer.GetApplicationCache(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	projectId := c.Param("pid")
	model := utility.SingularResourceName(strings.TrimLeft(c.Param("model"), "/"))
	docId := c.Param("id")
	relation := utility.SingularResourceName(c.Param("relation"))

	param := *cache.Param
	param.ProjectID = projectId
	param.RelationModel = relation
	param.DocumentID = docId

	q := c.Request().URL.Query()

	project := cache.Project

	// set the project driver first
	/* err = g.gqlServer.SetOnlyProjectDriver(ctx, project, project.Driver)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	} */

	if param.DocumentID != "" {
		q.Add("_id", param.DocumentID)
	}

	if param.RelationModel != "" {
		q.Add("relation", param.RelationModel)
	}

	if param.Role == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Could not Found Role",
			Code:    http.StatusBadRequest,
		})
	}

	if strings.HasPrefix(model, "system") {
		_case := strings.Split(model, "/")
		if len(_case) > 1 {
			switch _case[1] {
			case "media":
				fileInfo, buf, err := g.gqlServer.PrepareFileInfo(c, param.ProjectID)
				if err != nil {
					return c.JSON(http.StatusBadRequest, models.HttpResponse{
						Code:    http.StatusBadRequest,
						Message: err.Error(),
					})
				}
				fmt.Println(fileInfo)
				fmt.Println(buf)
				/*send, err := g.gqlServer.UploadService.UploadFile(fileInfo, buf.Bytes())
				if err != nil {
					return c.JSON(http.StatusBadRequest, models.HttpResponse{
						Code:    http.StatusBadRequest,
						Message: err.Error(),
					})
				}*/

				/*err = g.gqlServer.TrackUploadHistory(ctx, &param, nil) // send
				if err != nil {
					return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
						Message: captureInternalServerError(err).Error(),
						Code:    http.StatusInternalServerError,
					})
				}*/

				return c.JSON(http.StatusOK, models.HttpResponse{
					Code: http.StatusOK,
					Body: nil, // send
				})
			case "auth":
				var req map[string]interface{}
				if err := c.Bind(&req); err != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: err.Error(),
						Code:    http.StatusBadRequest,
					})
				}
				if len(req) == 0 {
					if err != nil {
						return c.JSON(http.StatusBadRequest, &models.HttpResponse{
							Message: "Request Body is Required",
							Code:    http.StatusBadRequest,
						})
					}
				}
				/*resp, err := g.gqlServer.HandleAuth(_case[2], req)
				if err != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: err.Error(),
						Code:    http.StatusBadRequest,
					})
				}*/
				c.JSON(http.StatusOK, nil)
			case "function":
				/*builderResponse, err := utility.HandleFunction(c, project.Schema, model)
				if err != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: err.Error(),
						Code:    http.StatusBadRequest,
					})
				}
				req := &shared.GraphQLIncomingRequest{Query: builderResponse.Query, OperationName: builderResponse.OperationName}*/
				/*res, err := schemas.SchemaBuilder(&shared.ApplicationCache{Project: project}, req, g.gqlServer, c)
				if err != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: err.Error(),
						Code:    http.StatusBadRequest,
					})
				}
				if res.Errors != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: "Query Error -> " + res.Errors[0].Message,
						Code:    http.StatusBadRequest,
					})
				}
				response := res.Data.(map[string]interface{})[builderResponse.QueryName]*/
				//return c.JSON(http.StatusOK, req)
			}
		} else {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Invalid Route",
				Code:    http.StatusBadRequest,
			})
		}
	} else {

		builderResponse, err := utility.RESTtoGraphQL(c, project.Schema, model, q, false)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusBadRequest,
			})
		}
		req := &models.GraphQLIncomingRequest{Query: builderResponse.Query, OperationName: builderResponse.OperationName}

		restStart := time.Now()
		res, err := g.exePublicGraphql(c, req)
		st := http.StatusOK
		if err != nil {
			st = http.StatusInternalServerError
		}
		telemetry.RecordRESTToGraphQL(c.Request().Context(), g.cfg, model, c.Request().Method, st, time.Since(restStart))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		return c.JSON(http.StatusOK, res)
	}
	return nil
}

func (g *GraphCtrl) RESTApiDocGenerator(c echo.Context) error {

	cache, err := g.gqlServer.GetApplicationCache(c)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	val, err := utility.OpenApiSpecGenerator(g.cfg.Environment, cache)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})

	}
	return c.JSON(200, val)
}

func (g *GraphCtrl) PublicGraphQL(i echo.Context) error {

	var req models.GraphQLIncomingRequest
	if err := i.Bind(&req); err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if req.Query == "" {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Query can not be empty!",
			Code:    http.StatusBadRequest,
		})
	}

	/*fmt.Println(fmt.Sprintf("req %s", req.Query))
	_var, _ := json.Marshal(req.Variables)
	fmt.Println(fmt.Sprintf("variable %s", string(_var)))*/

	if strings.HasPrefix(req.Query, "mutation") {
		req.QueryType = "mutation"
	} else if strings.HasPrefix(req.Query, "subscription") {
		req.QueryType = "subscription"
	} else {
		req.QueryType = "query"
	}

	res, err := g.exePublicGraphql(i, &req)
	if err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: utility.CaptureInternalServerError(err, map[string]interface{}{
				"req": req,
			}).Error(),
			Code: http.StatusBadRequest,
		})
	}

	return i.JSON(http.StatusOK, res)
}

func (g *GraphCtrl) exePublicGraphql(i echo.Context, req *models.GraphQLIncomingRequest) (*graphql.Result, error) {
	cache, err := g.gqlServer.GetApplicationCache(i)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	if project.Schema == nil {
		return nil, errors.New("schema Not Found")
	}

	// add the req to the cache
	cache.GraphqlRequest = req

	ctx := i.Request().Context()
	/*err = g.gqlServer.CheckLimit(cache.Project)
	if err != nil {
		return nil, err
	}*/

	if cache.GraphqlRequest.QueryType == "mutation" && cache.Param.Role.ID == "demo" && cache.Param.Role.SystemGenerated {
		return nil, errors.New("you Cant Change Anything in a Demo Project")
	}

	queryDoc, err := parser.ParseQuery(&ast.Source{Input: req.Query})
	if err != nil {
		return nil, fmt.Errorf("Error parsing query: %v\n", err)
	}

	xx := queryDoc.Operations.ForName("")

	// combine the query with any fragments and merge into a single query
	if len(queryDoc.Fragments) > 0 && xx.Name != "IntrospectionQuery" {
		// Create a map of fragment definitions for easy lookup
		fragments := make(map[string]*ast.FragmentDefinition)
		for _, fragment := range queryDoc.Fragments {
			fragments[fragment.Name] = fragment
		}
		req.Query = utility.GenerateCombinedQuery(queryDoc, fragments)
	}

	incomingRequest, isPluginRequest, err := utility.ExtractModelNames(project.Schema, queryDoc)
	if err != nil {
		return nil, err
	}

	/*iq, err := utility.ExtractGraphQLOperationName(req.Query, cache.Project.Schema, false)
	if err != nil {
		return nil, err
	}
	// inject request
	fmt.Println(iq)*/
	cache.IncomingRequest = incomingRequest

	if !isPluginRequest {
		_cache, err := g.publicSchemaBuilder(ctx, cache)
		if err != nil {
			return nil, err
		}
		// transfer the goods
		cache.RawSchemas = _cache.RawSchemas
		cache.Dataloaders = _cache.Dataloaders
	}

	// Inject the generic realtime publish mutation (broadcast layer) so apps can
	// send messages on a channel that broadcast subscribers receive.
	if cache.RawSchemas != nil {
		if cache.RawSchemas.Mutations == nil {
			cache.RawSchemas.Mutations = graphql.Fields{}
		}
		cache.RawSchemas.Mutations["publish"] = g.publishMutationField()
	}

	schemaConfig := graphql.SchemaConfig{}
	if len(cache.RawSchemas.Queries) > 0 {
		schemaConfig.Query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "QueryType",
			Fields: cache.RawSchemas.Queries,
		})
	}
	if len(cache.RawSchemas.Mutations) > 0 {
		schemaConfig.Mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "MutationType",
			Fields: cache.RawSchemas.Mutations,
		})
	}

	// Add custom scalars to the schema
	schemaConfig.Types = []graphql.Type{
		scaler.ScalarJSON,
		scaler.ScalarJSONArray,
		// Add built-in scalar types to ensure they are registered
		graphql.String,
		graphql.Int,
		graphql.Float,
		graphql.Boolean,
		graphql.ID,
	}

	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		return nil, err
	}

	token := i.Get("token")

	//loaderCtx := context.WithValue(ctx, "loaders", cache.Dataloaders)
	reqCtx := utility.WithSelectionSet(ctx, xx.SelectionSet)
	cacheCtx := utility.WithApplicationCache(ctx, cache)
	projectID := context.WithValue(ctx, "project_id", cache.Param.ProjectID)
	userID := context.WithValue(ctx, "user_id", cache.Param.UserID)
	tokenCtx := context.WithValue(ctx, "token", token)
	requestVar := context.WithValue(ctx, "variableValues", req.Variables)

	ctx, closeContext := onecontext.Merge(reqCtx, cacheCtx, userID, projectID, tokenCtx, requestVar)
	defer closeContext()

	gqlStart := time.Now()
	opName := req.OperationName
	if opName == "" {
		opName = "anonymous"
	}
	res := graphql.Do(graphql.Params{
		Context:        ctx,
		Schema:         schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
	})
	var gqlErr error
	if len(res.Errors) > 0 {
		gqlErr = fmt.Errorf("%s", res.Errors[0].Message)
	}
	telemetry.RecordGraphQLOperation(ctx, g.cfg, "public", req.QueryType, opName, gqlErr, time.Since(gqlStart))

	// catch all the internal server error
	if len(res.Errors) > 0 {
		utility.CaptureInternalServerError(err, map[string]interface{}{
			"loader": cache,
			"req":    i.Request,
		})
		/* var errMsg string
		for _, err := range res.Errors {
			errMsg += err.Error()
		}
		return nil, errors.New(errMsg) */
	}

	return res, nil
}

func (g *GraphCtrl) GetSystemCacheInfo(i echo.Context) error {
	ctx := i.Request().Context()

	list, err := g.gqlServer.ProjectCache.ListKeys(ctx)
	if err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	return i.JSON(http.StatusBadRequest, map[string]interface{}{
		"keys": list,
	})
}

func (g *GraphCtrl) SystemHealth(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	if userId.(string) == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "system health check is only allowed for authenticated users",
			Code:    http.StatusBadRequest,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "System is healthy",
		"version": "v2.0.0",
	})
}

func (g *GraphCtrl) SystemGraphQL(i echo.Context) error {

	var req models.GraphQLIncomingRequest
	if err := i.Bind(&req); err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid Json",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := i.Request().Context()

	sysStart := time.Now()
	res, err := schemas.SystemSchema(ctx, &req, g.gqlServer, i)
	opName := req.OperationName
	if opName == "" {
		opName = "anonymous"
	}
	qtype := req.QueryType
	if qtype == "" {
		qtype = "query"
	}
	var gqlErr error
	if err != nil {
		gqlErr = err
	} else if res != nil && len(res.Errors) > 0 {
		gqlErr = fmt.Errorf("%s", res.Errors[0].Message)
	}
	telemetry.RecordGraphQLOperation(ctx, g.cfg, "system", qtype, opName, gqlErr, time.Since(sysStart))
	if err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: utility.CaptureInternalServerError(err, map[string]interface{}{
				"req": req,
			}).Error(),
			Code: http.StatusBadRequest,
		})
	}
	// catch all the internal server error
	if len(res.Errors) > 0 {
		utility.CaptureInternalServerError(err, map[string]interface{}{
			"req": req,
		})
	}

	return i.JSON(http.StatusOK, res)
}

func (g *GraphCtrl) SubscriptionWrapHandler(c echo.Context) error {

	// inject router context with context
	ctx := context.WithValue(c.Request().Context(), "router", c)
	resp := c.Response()
	req := c.Request().WithContext(ctx)

	g.srv.ServeHTTP(resp, req)

	return nil
}

// publishMutationField builds the generic `publish(channel, event, payload)`
// mutation that broadcasts a message to all `broadcast(channel)` subscribers.
func (g *GraphCtrl) publishMutationField() *graphql.Field {
	resultObj := graphql.NewObject(graphql.ObjectConfig{
		Name: "BroadcastPublishResult",
		Fields: graphql.Fields{
			"success": &graphql.Field{Type: graphql.Boolean},
			"channel": &graphql.Field{Type: graphql.String},
		},
	})
	return &graphql.Field{
		Type: resultObj,
		Args: graphql.FieldConfigArgument{
			"channel": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"event":   &graphql.ArgumentConfig{Type: graphql.String},
			"payload": &graphql.ArgumentConfig{Type: scaler.ScalarJSON},
		},
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			channel, _ := p.Args["channel"].(string)
			event, _ := p.Args["event"].(string)
			payload := p.Args["payload"]
			cache, ok := utility.LegacyApplicationCache(p.Context)
			if !ok || cache == nil || cache.Param == nil {
				return nil, errors.New("application cache missing")
			}
			if err := g.gqlServer.PublishBroadcast(p.Context, cache.Param.ProjectID, channel, event, payload); err != nil {
				return nil, err
			}
			return map[string]interface{}{"success": true, "channel": channel}, nil
		},
	}
}

// PublicSubscriptionWrapHandler serves per-project public GraphQL subscriptions
// over websockets. The subscription schema (auto-generated <model>Changed fields
// + broadcast) is built per connection from the authenticated project/role, then
// served via a per-connection wsgraphql server.
func (g *GraphCtrl) PublicSubscriptionWrapHandler(c echo.Context) error {
	cache, err := g.gqlServer.GetApplicationCache(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	schema, err := schemas.PublicSubscriptionSchema(g.gqlServer, cache)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	srv, err := newSubscriptionWSServer(*schema)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	// Prefer cache.Ctx (enriched with tenant/routing keys by the pro layer) so
	// the realtime topic hook resolves the same scoped topic as the emit side.
	base := cache.Ctx
	if base == nil {
		base = c.Request().Context()
	}
	ctx := context.WithValue(base, "router", c)
	resp := c.Response()
	req := c.Request().WithContext(ctx)

	srv.ServeHTTP(resp, req)
	return nil
}

func CaptureInternalServerError(err error, scopes map[string]interface{}) error {
	sentry.WithScope(func(scope *sentry.Scope) {
		if len(scopes) > 0 {
			for k, v := range scopes {
				scope.SetExtra(k, v)
			}
		}
		sentry.CaptureException(err)
	})
	sentry.Flush(time.Second * 2)
	return err
}
