package controller

import (
	"bytes"
	"fmt"
	"github.com/apito-cms/wsgraphql/v1"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/schemas"
	"github.com/apito-io/engine/utility"
	"github.com/getsentry/sentry-go"
	"github.com/jinzhu/inflection"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"github.com/teivah/onecontext"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"go.opentelemetry.io/otel"
	"golang.org/x/net/context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var tracer = otel.Tracer("graphql-controller")

type GraphCtrl struct {
	cfg       *models.Config
	gqlServer *resolver.GraphQLServer
	srv       wsgraphql.Server
}

func GetGraphQLController(cfg *models.Config, commonFn *resolver.GraphQLServer) *GraphCtrl {

	return &GraphCtrl{
		cfg:       cfg,
		gqlServer: commonFn,
	}
}

func (g *GraphCtrl) PluginUpload(router echo.Context) error {

	req := make(map[string]interface{})
	if err := router.Bind(&req); err != nil {
		return router.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

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

	_, buffer, err := g.gqlServer.PrepareFileInfo(router, param.ProjectId)
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

	ctx := router.Request().Context()

	_pluginDetails, err := g.gqlServer.LoadLocalPlugin(ctx, dir, cache.Project.Driver)
	if err != nil {
		return router.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	return router.JSON(http.StatusOK, models.HttpResponse{
		Code: http.StatusOK,
		Body: _pluginDetails,
	})
}

func (g *GraphCtrl) RestToGraphQL(c echo.Context) error {

	cache, err := g.gqlServer.GetApplicationCache(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	projectId := c.Param("pid")
	model := inflection.Singular(strings.TrimLeft(c.Param("model"), "/"))
	docId := c.Param("id")
	relation := inflection.Singular(c.Param("relation"))

	param := *cache.Param
	param.ProjectId = projectId
	param.RelationModel = relation
	param.DocumentId = docId

	q := c.Request().URL.Query()

	project := cache.Project

	// set the project driver first
	err = g.gqlServer.SetOnlyProjectDriver(ctx, project.Driver)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if param.DocumentId != "" {
		q.Add("_id", param.DocumentId)
	}

	if param.RelationModel != "" {
		q.Add("relation", param.RelationModel)
	}

	if strings.HasPrefix(model, "system") {
		_case := strings.Split(model, "/")
		if len(_case) > 1 {
			switch _case[1] {
			case "media":
				fileInfo, buf, err := g.gqlServer.PrepareFileInfo(c, param.ProjectId)
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
				builderResponse, err := utility.HandleFunction(c, project.Schema, model)
				if err != nil {
					return c.JSON(http.StatusBadRequest, &models.HttpResponse{
						Message: err.Error(),
						Code:    http.StatusBadRequest,
					})
				}
				req := &models.GraphQLIncomingRequest{Query: builderResponse.Query, OperationName: builderResponse.OperationName}
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
				return c.JSON(http.StatusOK, req)
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

		res, err := g.exePublicGraphql(req, c)
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

	/*fmt.Println(fmt.Sprintf("req %s", req.Query))
	_var, _ := json.Marshal(req.Variables)
	fmt.Println(fmt.Sprintf("variable %s", string(_var)))*/

	res, err := g.exePublicGraphql(&req, i)
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

func (g *GraphCtrl) exePublicGraphql(req *models.GraphQLIncomingRequest, i echo.Context) (*graphql.Result, error) {
	cache, err := g.gqlServer.GetApplicationCache(i)
	if err != nil {
		return nil, err
	}

	ctx := i.Request().Context()
	/*err = g.gqlServer.CheckLimit(cache.Project)
	if err != nil {
		return nil, err
	}*/

	incomingRequest, err := utility.ExtractGraphQLOperationName(req.Query, cache.Project.Schema, false)
	if err != nil {
		return nil, err
	}
	// inject request
	//fmt.Println(incomingRequest)
	cache.IncomingRequest = incomingRequest

	_cache, err := g.publicSchemaBuilder(ctx, cache)
	if err != nil {
		return nil, err
	}

	// transfer the goods
	cache.RawSchemas = _cache.RawSchemas
	cache.Dataloaders = _cache.Dataloaders

	schemaConfig := graphql.SchemaConfig{}
	if len(cache.RawSchemas.Queries) > 0 {
		schemaConfig.Query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "QueryType",
			Fields: cache.RawSchemas.Queries,
		},
		)
	}
	if len(cache.RawSchemas.Mutations) > 0 {
		schemaConfig.Mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "MutationQuery",
			Fields: cache.RawSchemas.Mutations,
		})
	}

	qq, err := parser.ParseQuery(&ast.Source{Input: req.Query})
	if err != nil {
		return nil, err
	}
	xx := qq.Operations.ForName("")

	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		return nil, err
	}

	//loaderCtx := context.WithValue(ctx, "loaders", cache.Dataloaders)
	reqCtx := context.WithValue(ctx, "selectionSet", xx.SelectionSet) // pass the schema for input validation purpose
	cacheCtx := context.WithValue(ctx, "cache", cache)

	ctx, closeContext := onecontext.Merge(reqCtx, cacheCtx)
	defer closeContext()

	res, err := graphql.Do(graphql.Params{
		Context:        ctx,
		Schema:         schema,
		RequestString:  req.Query,
		VariableValues: req.Variables,
		OperationName:  req.OperationName,
	}), nil

	// catch all the internal server error
	if len(res.Errors) > 0 {
		utility.CaptureInternalServerError(err, map[string]interface{}{
			"loader": cache,
			"req":    i.Request,
		})
	}

	return res, err
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

func (g *GraphCtrl) SystemGraphQL(i echo.Context) error {

	var req models.GraphQLIncomingRequest
	if err := i.Bind(&req); err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid Json",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := i.Request().Context()

	cache, err := g.gqlServer.GetApplicationCache(i)
	if err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	res, err := schemas.SystemSchema(ctx, cache, &req, g.gqlServer, i)
	if err != nil {
		return i.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: utility.CaptureInternalServerError(err, map[string]interface{}{
				"req": cache,
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

	qq, err := parser.ParseQuery(&ast.Source{Input: req.Query})
	if err != nil {
		fmt.Println(err)
	}

	xx := qq.Operations.ForName("")
	if xx.Operation == "mutation" {
		// for each system mutation invalidate the application cache
		err = g.gqlServer.ExpireGraphQLProjectCache(ctx, cache.Project.Id)
		if err != nil {
			return err
		}
		/*var mutationName string
		for _, yy := range xx.SelectionSet {
			zz := yy.(*ast.Field)
			mutationName = zz.Name
			/*for _, ww := range zz.SelectionSet {
				tt := ww.(*ast.Field)
				fmt.Println(tt.Name)
			}
		}*/
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
