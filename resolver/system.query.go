package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/faker"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/plugins"
	"github.com/jinzhu/inflection"
	"github.com/labstack/echo/v4"
	graphqlClient "github.com/machinebox/graphql"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) ConnectSupportResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	hybridAuthCallToken := ""

	// create a GraphQL client (safe to share across requests)
	client := graphqlClient.NewClient("https://api.apito.io/secured/graphql")

	// make a request
	req := graphqlClient.NewRequest(fmt.Sprintf(`
		   mutation MyMutation {
			  hybridAuth(payload: {
				user_id : "%s", 
				email: "%s" 
			  }) {
				JSON
			  }
			}`, param.UserId, param.Email))

	// set header fields
	req.Header.Set("Authorization", "Bearer "+hybridAuthCallToken)

	// run it and capture the response
	var respData map[string]interface{}
	if err := client.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	if resp, ok := respData["hybridAuth"].(map[string]interface{}); ok {
		if json, ok := resp["JSON"].(map[string]interface{}); ok {
			if login, ok := json["userLogin"].(map[string]interface{}); ok {
				return login, nil
			}
		}
	}

	return nil, nil
}

func (s *GraphQLServer) ListProjectsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "projects"

	return s.SystemDriver.ListProjects(p.Context, param)
}

func (s *GraphQLServer) ListAllProjectsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "projects"

	return s.SystemDriver.ListProjects(p.Context, param)
}

func (s *GraphQLServer) GetCurrentProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := new(protobuff.Project)

	*project = *cache.Project
	project.Schema = nil

	return project, nil
}

func (s *GraphQLServer) GetProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var projectID string
	if val, ok := p.Args["_id"].(string); ok {
		projectID = strings.TrimSpace(inflection.Singular(val))
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}
	project, err := s.SystemDriver.GetProject(p.Context, projectID)
	if err != nil {
		return nil, err
	}
	project.Schema = nil
	return project, nil
}

func (s *GraphQLServer) GetLoggedInUserFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	userID := param.UserId

	user, err := s.SystemDriver.GetSystemUser(p.Context, userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *GraphQLServer) ProjectsPlugins(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var systemType string
	if val, ok := p.Args["system_type"].(string); ok {
		systemType = val
	} else {
		return nil, errors.New("system type is required")
	}

	var _plugins []*protobuff.PluginDetails
	switch systemType {
	case "local":
		_LocalPlugins, err := plugins.LoadLocalPluginRegistry(s.Cfg, project.Driver)
		if err != nil {
			return nil, err
		}
		for _, p := range _LocalPlugins {
			switch p.Type {
			case protobuff.PluginType_Storage:
				if p.Id == project.DefaultStoragePlugin {
					p.ActivateStatus = protobuff.PluginActivateStatus_activated
				}
			}
			_plugins = append(_plugins, p)
		}
	case "third_party":
		//
		for _, a := range plugins.ThirdPartyApprovedPlugins {
			_p, err := GetPluginInfo(a)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				_plugins = append(_plugins, _p)
			}
		}
	default:
		return nil, errors.New("unsupported system type")
	}

	return _plugins, nil
}

func GetPluginInfo(repo *plugins.ApprovedRepos) (*protobuff.PluginDetails, error) {
	resp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/info.json", repo.RepositoryOwner, repo.RepositoryName, repo.BranchName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad plugin repository request or info.json file missing from root repository")
	}
	var details protobuff.PluginDetails
	err = json.NewDecoder(resp.Body).Decode(&details) // response body is []byte
	if err != nil {
		return nil, err
	}
	return &details, nil

}

func (s *GraphQLServer) SearchApitoUsersResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "users"

	resp, err := s.SystemDriver.SearchUsers(p.Context, param)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

func (s *GraphQLServer) Pixabay(p graphql.ResolveParams) ([]*protobuff.FileDetails, error) {

	arg := p.Args

	limit := 10
	if val, ok := arg["limit"]; ok {
		limit = val.(int)
	}

	page := 1
	if val, ok := arg["page"]; ok {
		page = val.(int)
	}

	var search string
	if val, ok := p.Args["search"].(string); ok {
		search = val
	}

	pix := faker.NewPixabay()
	pix.APIKey = "8621001-77c43e22995a72c7ab14f5059"
	images, err := pix.GetPhotos(&faker.PhotoParameter{
		Q:       search,
		Page:    page,
		PerPage: limit,
	})
	if err != nil {
		return nil, err
	}
	var files []*protobuff.FileDetails
	for _, img := range images {
		split := strings.Split(img.PreviewURL, "/")
		name := split[len(split)-1]
		files = append(files, &protobuff.FileDetails{
			/*			Id:            "",
						XKey:          "",
						Type:          "",*/
			FileExtension: strings.Split(name, ".")[1],
			FileName:      name,
			Size:          int64(img.ImageSize),
			Url:           img.ImageURL,
		})
	}
	return files, nil
}

func (s *GraphQLServer) GetPhotosAndCountInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return p, nil
}

func (s *GraphQLServer) ProjectFunctionsInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	results, err := s.SystemDriver.ListFunctions(p.Context, param)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}

func (s *GraphQLServer) ListExecutableFunctionsResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var model string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		model = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	// if schema not found then create
	if cache.Project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var modelType *protobuff.ModelType
	for _, ct := range cache.Project.Schema.Models {
		if ct.Name == model {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	cache.Param.Model = modelType

	var supportedFunctions []string

	for _, ct := range cache.Project.Schema.Functions {
		if ct.Request.Model == modelType.Name || ct.Request.Model == "JSON" {
			supportedFunctions = append(supportedFunctions, ct.Name)
		}
	}

	return map[string]interface{}{
		"functions": supportedFunctions,
	}, nil
}

func (s *GraphQLServer) LoadedFunctionProviderResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var _type protobuff.PluginType
	if val, ok := p.Args["type"].(protobuff.PluginType); ok {
		_type = val
	}

	var _list []string
	switch _type {
	case protobuff.PluginType_Function:
		_list = s.FunctionProviderIds
	case protobuff.PluginType_NormalPlugin:
		_list = s.InstalledPluginList
	case protobuff.PluginType_Storage:
		_list = s.StorageProviderIds
	}

	return map[string]interface{}{
		"type":    _type,
		"plugins": _list,
	}, nil
}

func (s *GraphQLServer) ListAvailableFunctionsResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	var _id string
	if val, ok := p.Args["function_id"].(string); ok {
		_id = val
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var _function *protobuff.CloudFunction
	for _, f := range cache.Project.Schema.Functions {
		if f.Name == _id {
			_function = f
			break
		}
	}

	var pluginCache *models.PluginCache
	if val, ok := s.LocalPluginCache[_function.FunctionProviderId]; ok {
		pluginCache = val
	}

	fmt.Println(pluginCache)

	return map[string]interface{}{}, nil
}
