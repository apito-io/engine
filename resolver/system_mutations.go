package resolver

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/driver/sql"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/services"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/apito-io/types/protobuff"
	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) GenerateTenantTokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("GenerateTenantTokenResolverFn", router)

	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = val
	} else {
		return nil, errors.New("token is Required")
	}

	var tenantID string
	if val, ok := p.Args["tenant_id"].(string); ok {
		tenantID = val
	} else {
		return nil, errors.New("tenant_id is Required")
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	_token, err := s.ApiKeyManager.GenerateTenantToken(cache.Ctx, tenantID, token)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"token": _token,
	}, nil
}

func (s *GraphQLServer) GenerateAPIKeyResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("GenerateAPIKeyResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	userID := param.UserID
	projectID := param.ProjectID

	var name string
	if val, ok := p.Args["name"].(string); ok {
		name = val
	} else {
		return nil, errors.New("name is Required")
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	project := cache.Project

	parseDuration, _ := time.Parse(time.RFC3339, duration)

	t := services.GetBrankaToken(s.Cfg, s.SystemDriver)

	apiKey, err := t.GenerateAPIKey(cache.Ctx, userID, projectID, "", "api_key", parseDuration.Unix())
	if err != nil {
		return nil, err
	}

	project.APIKeys = append(project.APIKeys, &models.APIKey{
		ProjectID: projectID,
		Name:      name,
		Token:     *apiKey,
		Expire:    duration,
	})

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token": apiKey,
		"name":  name,
	}, nil
}

func (s *GraphQLServer) GenerateAPITokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("GenerateApiTokenResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	var name string
	if val, ok := p.Args["name"].(string); ok {
		name = val
	} else {
		return nil, errors.New("name Id Required")
	}

	var role string
	if val, ok := p.Args["role"].(string); ok {
		role = val
	} else {
		return nil, errors.New("Role Id Required")
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	project := cache.Project

	parseDuration, _ := time.Parse(time.RFC3339, duration)

	// generate the token
	apiKey, err := s.ApiKeyManager.GenerateKey(&models.TokenClaims{
		Role:      role,
		UserID:    param.UserID,
		ProjectID: project.ID,
		ExpireAt:  parseDuration,
	})
	if err != nil {
		return nil, err
	}

	project.Tokens = append(project.Tokens, &models.APIToken{
		Name:   name,
		Token:  apiKey,
		Role:   role,
		Expire: duration,
	})

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token": apiKey,
	}, nil
}

func (s *GraphQLServer) DeleteAPIKeyResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteApiKeyResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = val
	} else {
		return nil, ae.TokenIsRequired
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	var verifiedToken *models.TokenClaims
	if strings.HasPrefix(token, "ak_") {
		verifiedToken, err = s.ApiKeyManager.Validate(cache.Ctx, token, false)
		if err != nil {
			return nil, ae.InvalidToken
		}
	} else {
		verifiedToken, err = s.BlankaTokenService.Validate(cache.Ctx, token)
		if err != nil {
			return nil, ae.InvalidToken
		}
	}

	if !param.Role.IsAdmin {
		if verifiedToken.UserID != param.UserID {
			return nil, errors.New("its none of your business, Pal ")
		}
	}

	project := cache.Project

	for i, t := range project.Tokens {
		if t.Token == token {
			project.Tokens = append(project.Tokens[:i], project.Tokens[i+1:]...)
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	parseDuration, _ := time.Parse(time.RFC3339, duration)
	alreadyExpired := time.Until(parseDuration).Hours()
	if alreadyExpired > 0.0 { // expire the token
		expiredToken := map[string]interface{}{
			"id":        verifiedToken.TokenUniqueID,
			"_key":      verifiedToken.TokenUniqueID,
			"expire_at": duration,
		}

		err = s.SystemDriver.BlacklistAToken(cache.Ctx, expiredToken)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"msg": "Token Deleted",
	}, nil

}

func (s *GraphQLServer) DeleteAPITokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteApiTokenResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = val
	} else {
		return nil, ae.TokenIsRequired
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	verifiedToken, err := s.BlankaTokenService.Validate(cache.Ctx, token)
	if err != nil {
		return nil, ae.InvalidToken
	}

	if !param.Role.IsAdmin {
		if verifiedToken.UserID != param.UserID {
			return nil, errors.New("its none of your business, Pal ")
		}
	}

	project := cache.Project

	for i, t := range project.APIKeys {
		if t.Token == token {
			project.APIKeys = append(project.APIKeys[:i], project.APIKeys[i+1:]...)
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	parseDuration, _ := time.Parse(time.RFC3339, duration)
	alreadyExpired := parseDuration.Sub(time.Now()).Hours()
	if alreadyExpired > 0.0 { // expire the token
		expiredToken := map[string]interface{}{
			"id":        verifiedToken.TokenUniqueID,
			"_key":      verifiedToken.TokenUniqueID,
			"expire_at": duration,
		}

		err = s.SystemDriver.BlacklistAToken(cache.Ctx, expiredToken)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"msg": "Token Deleted",
	}, nil
}

func (s *GraphQLServer) CreateWebHookResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("CreateWebHookResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	project := cache.Project

	var model string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		model = utility.SingularResourceName(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var modelType *models.ModelType
	for _, ct := range project.Schema.Models {
		if ct.Name == model {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	cache.Param.Model = modelType

	id := uuid.New()
	uid := id.String()
	hook := &models.Webhook{
		ID:        uid,
		XKey:      uid,
		Type:      "hook",
		Model:     model,
		ProjectID: project.ID,
	}

	if val, ok := p.Args["name"].(string); ok {
		hook.Name = val
	} else {
		return nil, errors.New("Name Id Required")
	}

	if val, ok := p.Args["url"].(string); ok {
		hook.URL = val
	}

	if val, ok := p.Args["events"].([]interface{}); ok {
		var events []string
		for _, v := range val {
			events = append(events, v.(string))
		}
		hook.Events = events
	} else {
		return nil, errors.New("Events are Required")
	}

	if val, ok := p.Args["logic_executions"].([]interface{}); ok {
		var functions []string
		for _, v := range val {
			functions = append(functions, v.(string))
		}
		hook.LogicExecutions = functions
	}

	if hook.URL == "" && len(hook.LogicExecutions) == 0 {
		return nil, errors.New("Either URL OR Trigger Functions are Required")
	}

	// now append the hook info in model as well
	modelType.HookIds = append(modelType.HookIds, hook.ID)

	_, err = s.SystemDriver.AddWebhookToProject(cache.Ctx, hook)
	if err != nil {
		return nil, err
	}

	project, err = s.SystemDriver.GetProject(cache.Ctx, param.ProjectID)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	return hook, nil
}

func (s *GraphQLServer) DeleteWebHookResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteWebHookResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p

	tenantId := router.Get("tenant")
	switch param.Role.ID {
	case "tenant":
		if tenantId == nil {
			return nil, errors.New("Unable to Identify the User")
		}
		param.TenantID = tenantId.(string)
		break
	}

	if val, ok := p.Args["id"].(string); ok {
		param.DocumentID = val
	} else {
		return nil, errors.New("Hook id is Required")
	}

	hook, err := s.SystemDriver.GetWebHook(cache.Ctx, param.ProjectID, param.DocumentID)
	if err != nil {
		return nil, err
	}

	// if schema not found then create
	if cache.Project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var modelType *models.ModelType
	for _, ct := range cache.Project.Schema.Models {
		if ct.Name == hook.Model {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	// delete from model
	for i, h := range modelType.HookIds {
		if h == hook.ID {
			modelType.HookIds = append(modelType.HookIds[:i], modelType.HookIds[i+1:]...)
		}
	}

	if len(modelType.HookIds) == 0 {
		modelType.HookIds = nil
	}

	project, err := s.SystemDriver.GetProject(cache.Ctx, param.ProjectID)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// now remove from the users collection as well
	err = s.SystemDriver.DeleteWebhook(cache.Ctx, param.ProjectID, param.DocumentID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"msg": "Webhook Deleted",
	}, nil
}

/*
func (s *GraphQLServer) CreateProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

		userId := param.UserId

		user, err := s.SystemDriver.GetSystemUser(userId)
		if err != nil {
			return nil, err
		}

		if user.AccountUsage == nil {
			user.AccountUsage = &models.AccountUsage{
				XKey:            userId,
				Id:              userId,
				Type:            "usage",
				NumberOfProject: utility.AvailableProjectLimit["free"],
				Limits:          utility.DeveloperAccountPlan["free"],
			}
		}

		if user.AccountUsage != nil && (len(user.AccountUsage.Usages) >= int(user.AccountUsage.NumberOfProject)) {
			return nil, errors.New("You have reached the project limit creation. Please upgrade your plan")
		}

		projectName := p.Args["name"].(string)
		projectDescription := p.Args["description"].(string)

		project, err := s.SystemDriver.CreateProject( projectName, projectDescription, user.ID)
		if err != nil {
			return nil, err
		}

		// add the project info
		//user.Projects = append(user.Projects, projectId)

		err = s.SystemDriver.UpdateUser(user, false)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"id":          project.ID,
			"name":        project.ProjectName,
			"description": project.ProjectDescription,
		}, nil
	}
*/
func (s *GraphQLServer) UpdateProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpdateProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	var projectId string
	if val, ok := p.Args["_id"].(string); ok && val != "" {
		projectId = val

		_, err = s.SystemDriver.CheckProjectWithRoles(cache.Ctx, param.UserID, projectId)
		if err != nil {
			return nil, err
		}

	} else {
		// passing the current
		projectId = param.ProjectID
	}

	project, err := s.SystemDriver.GetProject(cache.Ctx, projectId)
	if err != nil {
		return nil, err
	}

	// hack or fix
	if project.Settings == nil {
		project.Settings = &models.ProjectSettings{
			Locals: []string{"en"},
		}
	}

	// update project
	if val, ok := p.Args["name"].(string); ok {
		project.Name = val
	}

	if val, ok := p.Args["description"].(string); ok {
		project.Description = val
	}

	if driver, ok := p.Args["driver"].(map[string]interface{}); ok {

		if project.Driver == nil {
			project.Driver = &models.DriverCredentials{Engine: "apito"}
		}

		var host string
		if val, ok := driver["host"].(string); ok {
			host = val
		}

		var port string
		if val, ok := driver["port"].(string); ok {
			port = val
		}

		var database string
		if val, ok := driver["database"].(string); ok {
			database = val
		}

		var user string
		if val, ok := driver["user"].(string); ok {
			user = val
		}

		var password string
		if val, ok := driver["password"].(string); ok {
			password = val
		}

		var db interface{}
		var err error
		switch driver["engine"] {
		case _const.PostgreSQLDriver, _const.MySQLDriver, _const.MariaDBDriver:
			db, err = sql.GetSQLDriver(&models.DriverCredentials{
				Host:     host,
				Port:     port,
				Database: database,
				User:     user,
				Password: password,
			})
		case _const.DynamoDB:
		default:
			project.Driver = &models.DriverCredentials{Engine: _const.EmbeddedDB}
		}

		if db == nil {
			return nil, errors.New("db configuration is not correct")
		}

		if err != nil {
			return nil, err
		}
	}

	if val, ok := p.Args["tenant_model_name"].(string); ok {
		var modelType *models.ModelType
		for _, ct := range project.Schema.Models {
			if ct.Name == val {
				modelType = ct
				break
			}
		}
		if modelType == nil {
			return nil, errors.New("tenant Model not found")
		}
		// search for name and logo fields
		var nameFound, logoFound bool
		for _, field := range modelType.Fields {
			if field.Identifier == "name" && field.FieldType == "text" {
				nameFound = true
			} else if field.Identifier == "logo" && field.FieldType == "media" {
				logoFound = true
			}
		}
		if !nameFound || !logoFound {
			return nil, errors.New("tenant Model must have name(string) and logo(media) fields")
		}

		// set the tenant model to false for all models
		for _, ct := range project.Schema.Models {
			ct.IsTenantModel = false
		}
		modelType.IsTenantModel = true // only one model can be tenant model
		project.TenantModelName = val
	}

	if settings, ok := p.Args["settings"].(map[string]interface{}); ok {
		if project.Settings == nil {
			project.Settings = &models.ProjectSettings{}
		}

		if val, ok := settings["locals"].([]interface{}); ok {
			var locals []string
			for _, v := range val {
				locals = append(locals, v.(string))
			}
			project.Settings.Locals = locals
		}

		if val, ok := settings["system_graphql_hooks"].(bool); ok {
			project.Settings.SystemGraphqlHooks = val
		}

		if val, ok := settings["enable_revision_history"].(bool); ok {
			project.Settings.EnableRevisionHistory = val
		}

		if val, ok := settings["default_storage_plugin"].(string); ok {
			project.Settings.DefaultStoragePlugin = val
		}

		if val, ok := settings["default_function_plugin"].(string); ok {
			project.Settings.DefaultFunctionPlugin = val
		}

		if val, ok := settings["default_locale"].(string); ok {
			project.Settings.DefaultLocale = val
		}

	}

	if val, ok := p.Args["add_team_member"].(map[string]interface{}); ok && val != nil {

		req := models.TeamMemberAddRequest{
			ProjectID: param.ProjectID,
		}

		if val, ok := val["email"].(string); ok {
			req.Email = val
		} else {
			return nil, errors.New("email is Required")
		}

		if val, ok := val["role"].(string); ok {
			req.Role = val
		} else {
			return nil, errors.New("role is Required")
		}

		if val, ok := val["team_id"].(string); ok {
			req.TeamID = val
		} /*else {
			return nil, errors.New("user ID is Required")
		}*/

		if vals, ok := val["administrative_permissions"].([]interface{}); ok {
			var permissions []string
			for _, v := range vals {
				permissions = append(permissions, v.(string))
			}
			req.Permissions = permissions
		}

		user, err := s.SystemDriver.GetSystemUserByEmail(cache.Ctx, req.Email)
		if err != nil {
			return nil, err
		}

		if user == nil { // new user
			_tempPass := utility.RandomStringGenerator(10)
			registerRequest := &models.RegisterRequest{
				User: &models.SystemUser{
					Email:            req.Email,
					RegisterProvider: "system",
					TempPassword:     _tempPass,
					CurrentProjectID: projectId,
				},
			}
			user, err = s.AuthService.Signup(cache.Ctx, registerRequest)
			if err != nil {
				return nil, err
			}
			user, err = s.SystemDriver.CreateSystemUser(cache.Ctx, user)
			if err != nil {
				return nil, err
			}
		}

		req.UserID = user.ID

		err = s.SystemDriver.AddATeamMemberToProject(cache.Ctx, &req)
		if err != nil {
			return nil, err
		}

		// send email to the user
		go func(_user *models.SystemUser) {
			// send email to the user
			ctx := context.Background()
			req := &models.EmailSendRequest{
				AppURL:       s.Cfg.CORSOrigin,
				Sender:       "no-reply@apito.io",
				Recipients:   []string{_user.Email},
				TempPassword: _user.TempPassword,
				ProjectName:  project.Name,
			}
			err := services.SendTeamAddEmail(ctx, s.AwsConfig, req)
			if err != nil {
				fmt.Println(err.Error())
			}
		}(user)
	}

	if val, ok := p.Args["remove_team_member"].(map[string]interface{}); ok {
		var memberId string
		if val, ok := val["member_id"].(string); ok {
			memberId = val
		} else {
			return nil, errors.New("member ID is Required")
		}
		err := s.SystemDriver.RemoveATeamMemberFromProject(cache.Ctx, param.ProjectID, memberId)
		if err != nil {
			return nil, err
		}
	}

	if plugins, ok := p.Args["plugins"].(map[string]interface{}); ok {
		if project.Plugins == nil {
			project.Plugins = []*protobuff.PluginDetails{}
		}

		switch plugins["name"] {
		case "aws":
			details := &protobuff.PluginDetails{
				Id:          "aws",
				Description: "Aws Lambda Functions",
				EnvVars:     []*protobuff.EnvVariable{},
			}
			if val, ok := plugins["details"].(map[string]interface{}); ok {
				if accessKey, ok := val["access_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "ACCESS_KEY",
						Value: accessKey,
					})
				}
				if secretKey, ok := val["secret_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "SECRET_KEY",
						Value: secretKey,
					})
				}
				if region, ok := val["region"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "RELIGION",
						Value: region,
					})
				}
			}

			/*// validate the creds first
			sess, err := session.NewSession(&aws.Config{
				Region:      aws.String(details.Configs.Credentials.Region),
				Credentials: credentials.NewStaticCredentials(details.Configs.Credentials.AccessKey, details.Configs.Credentials.SecretKey, ""),
			})
			if err != nil {
				return nil, err
			}
			_, err = sess.Config.Credentials.Get()
			if err != nil {
				return nil, err
			}

			svc := iam.New(sess)

			arn := "arn:aws:iam::aws:policy/AWSLambdaExecute"
			result, err := svc.GetPolicy(&iam.GetPolicyInput{
				PolicyArn: &arn,
			})
			if err != nil {
				return nil, err
			}

			fmt.Printf("%s - %s\n", arn, *result.Policy.Description)*/

			project.Plugins = append(project.Plugins, details)
			break
		case "apitofunc":

			details := &protobuff.PluginDetails{
				Id:          "apitofunc",
				Description: "Apito Functions",
				EnvVars:     []*protobuff.EnvVariable{},
			}
			if val, ok := plugins["details"].(map[string]interface{}); ok {
				if accessKey, ok := val["access_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "ACCESS_KEY",
						Value: accessKey,
					})
				}
				if secretKey, ok := val["secret_key"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "SECRET_KEY",
						Value: secretKey,
					})
				}
				if region, ok := val["region"].(string); ok {
					details.EnvVars = append(details.EnvVars, &protobuff.EnvVariable{
						Key:   "RELIGION",
						Value: region,
					})
				}
			}

			out, err := exec.Command("bash", "-c", "docker version").Output()

			// if there is an error with our execution
			// handle it here
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Println("Command Successfully Executed")
			output := strings.Split(string(out), "\n")
			fmt.Println(output)
			var isDockerRunning bool
			for _, o := range output {
				if strings.TrimSpace(o) == "Engine:" {
					isDockerRunning = true
					break
				}
			}

			if isDockerRunning {
				project.Plugins = append(project.Plugins, details)
			} else {
				return nil, errors.New("docker Service is Not Running on this Machine. Please Start")
			}
		default:
			return nil, errors.New("invalid Extension Type")
		}
	}

	if project.ProjectSecretKey == "" {
		project.ProjectSecretKey = utility.RandomStringGenerator(25)
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// hide the schema
	project.Schema = nil
	return project, nil
}

func (s *GraphQLServer) UpdateProfileResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpdateProfileResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	userId := param.UserID
	user, err := s.SystemDriver.GetSystemUser(cache.Ctx, userId)
	if err != nil {
		return nil, err
	}

	// update project
	if val, ok := p.Args["first_name"].(string); ok {
		user.FirstName = val
		if val == "fahim" {
			return nil, errors.New("fahim is a reserved word")
		}
	}

	if val, ok := p.Args["last_name"].(string); ok {
		user.LastName = val
	}

	if val, ok := p.Args["role"].(string); ok {
		if user.IsAdmin {
			user.Role = val
		} else {
			return nil, errors.New("you are not allowed to change the role")
		}
	}

	if val, ok := p.Args["username"].(string); ok {
		user.Username = val
	}

	if val, ok := p.Args["old_pass"].(string); ok {
		if newPass, ok := p.Args["new_pass"].(string); ok {
			user, err = s.AuthService.ChangePassword(cache.Ctx, user, val, newPass)
			if err != nil {
				return nil, err
			}
		}
	}

	err = s.SystemDriver.UpdateSystemUser(cache.Ctx, user, true)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *GraphQLServer) UpsertPluginResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpsertPluginResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := *cache.Project

	var id string
	if val, ok := p.Args["id"].(string); ok && val != "" {
		id = val
	} else {
		return nil, errors.New("plugin id is required")
	}

	var _pluginDetails *protobuff.PluginDetails

	// First check if plugin already exists in project
	for _, plugin := range project.Plugins {
		if plugin.Id == id {
			_pluginDetails = plugin
			break
		}
	}

	// If not found in project, load from HashiCorp registry
	if _pluginDetails == nil {
		_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(s.Cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load HashiCorp plugin registry: %w", err)
		}

		if plugin, exists := _hashiCorpPlugins[id]; exists {
			_pluginDetails = plugin
		} else {
			return nil, errors.New("plugin not found in HashiCorp registry")
		}
	}

	if val, ok := p.Args["env_vars"].([]interface{}); ok && val != nil && len(val) > 0 {
		for _, v := range val {
			_env := v.(map[string]interface{})
			for _, env := range _pluginDetails.EnvVars {
				if env.Key == _env["key"].(string) {
					env.Value = _env["value"].(string)
					break
				}
			}
		}
	}

	if val, ok := p.Args["activate_status"].(protobuff.PluginActivateStatus); ok {
		switch val {
		case protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED:
			_pluginDetails.ActivateStatus = protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED
			project.Settings.DefaultStoragePlugin = ""
		case protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED:
			_pluginDetails.ActivateStatus = 1
			project.Settings.DefaultStoragePlugin = id
		}
	}

	if len(project.Plugins) == 0 {
		project.Plugins = append(project.Plugins, _pluginDetails)
	} else {
		for i, plugin := range project.Plugins {
			if plugin.Id == id {
				project.Plugins[i] = _pluginDetails
				break
			}
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, &project, true)
	if err != nil {
		return nil, err
	}

	return _pluginDetails, nil
}

/*func (s *GraphQLServer) DeleteMediaFileInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	_param, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := _param.Project

	if project.ActivatedPluginsIds == nil || project.ActivatedPluginsIds.Storage == "" {
		return nil, errors.New("no activated plugin found")
	}

	var docIds []string
	if ids, ok := p.Args["ids"].([]interface{}); ok {
		for _, id := range ids {
			docIds = append(docIds, id.(string))
		}
	} else if len(docIds) == 0 {
		return nil, errors.New("id is required")
	} else {
		return nil, errors.New("invalid request")
	}

	var pluginCache *models.PluginCache
	if val, ok := s.LocalPluginCache[project.ActivatedPluginsIds.Storage]; ok && val != nil {
		pluginCache = val
	}

	if pluginCache == nil {
		return nil, errors.New("media plugin is not loaded")
	}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	pluginLookUp, err := pluginCache.Plugin.Lookup(pluginCache.PluginConfigurations.ExportedVariable)
	if err != nil {
		return nil, err
	}

	var storagePlugin interfaces.StoragePluginInterface
	storagePlugin, ok := pluginLookUp.(interfaces.StoragePluginInterface)
	if !ok {
		return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, pluginCache.PluginConfigurations.ID))
	}

	// inject project id
	envs := []*extensions.EnvVariables{
		{
			Key:   "PROJECT_ID",
			Value: project.ID,
		},
	}
	err = storagePlugin.Init(envs)
	if err != nil {
		return nil, err
	}

	var deletedIds []string
	for _, docId := range docIds {
		err = storagePlugin.DeleteFile(ctx, docId)
		if err != nil {
			return nil, err
		}
		deletedIds = append(deletedIds, docId)
	}

	return map[string]interface{}{
		"message": fmt.Sprintf("%s file deleted", strings.Join(deletedIds, ",")),
	}, nil
}
*/

func (s *GraphQLServer) AddModelToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("AddModelToProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := cache.Param

	var modelName string
	if val, ok := p.Args["name"].(string); ok {
		modelName = strings.TrimSpace(utility.SingularResourceName(strcase.ToLowerCamel(val)))
		switch modelName {
		case "list":
			return nil, errors.New("naming a Model `List` is not allowed. Apito Uses List to represent plural of a resource automatically. Try another name instead")
		case "user":
			return nil, errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
		case "system":
			return nil, errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
		case "function":
			return nil, errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
		}
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	//check if model name starts with number
	var re = regexp.MustCompile(`^\d`)
	matchFound := re.FindAllString(modelName, -1)
	if len(matchFound) > 0 {
		return nil, errors.New("model name can not start with a number! use character instead")
	}

	// inject model type
	param.Model = &models.ModelType{
		Name: modelName,
	}

	// temporary fix for sql driver
	if cache.Project.Driver.Database == "sqlite" || cache.Project.Driver.Database == "mysql" || cache.Project.Driver.Database == "postgres" {
		param.ProjectID = cache.Project.Driver.Database
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	checkCollectionExists, err := driver.CheckCollectionExists(cache.Ctx, param, false)
	if err != nil {
		return nil, err
	}

	if checkCollectionExists {
		return nil, errors.New("collection already exists")
	}

	project := cache.Project

	var singleRecord bool
	if val, ok := p.Args["single_record"].(bool); ok {
		singleRecord = val
	}

	model := &models.ModelType{
		Name:       modelName,
		SinglePage: singleRecord,
	}

	// if schema not found then create
	project.Schema, err = driver.AddModel(cache.Ctx, project, model)
	if err != nil {
		return nil, err
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
	if err != nil {
		return nil, err
	}

	return project.Schema.Models, nil
}

func (s *GraphQLServer) RunModelMigrationsResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("RunModelMigrationsResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := cache.Param
	project := cache.Project

	for _, model := range project.Schema.Models {
		param.Model = model

		// temporary fix for sql driver
		param.ProjectID = cache.Project.ID

		driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
		if err != nil {
			return nil, err
		}

		checkCollectionExists, err := driver.CheckCollectionExists(cache.Ctx, param, false)
		if err != nil {
			return nil, err
		}

		if !checkCollectionExists {
			// if schema not found then create
			err = driver.AddCollection(cache.Ctx, param, false)
			if err != nil {
				return nil, err
			}
		}
	}

	// check relation collection
	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	checkRelationCollectionExists, err := driver.CheckCollectionExists(cache.Ctx, param, true)
	if err != nil {
		return nil, err
	}

	if !checkRelationCollectionExists {
		err = driver.AddCollection(cache.Ctx, param, true)
		if err != nil {
			return nil, err
		}
	}

	return project.Schema.Models, nil
}

func (s *GraphQLServer) UpdateModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpdateModelResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	if project == nil {
		return nil, errors.New("project not found to create a model")
	}

	var _type string
	if val, ok := p.Args["type"].(string); ok {
		_type = val
	} else {
		return nil, errors.New("type not found")
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var singlePageModel bool
	if val, ok := p.Args["single_page_model"].(bool); ok {
		singlePageModel = val
	}

	var resp interface{}
	switch _type {
	case "duplicate":
		var newName string
		if val, ok := p.Args["new_name"].(string); ok {
			newName = val
		} else {
			return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
		}
		resp, err = s.duplicateModel(cache.Ctx, project, newName, modelName)
	case "rename":
		var newName string
		if val, ok := p.Args["new_name"].(string); ok {
			newName = val
		} else {
			return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
		}
		resp, err = s.renameModel(cache.Ctx, project, newName, modelName, singlePageModel)
	case "convert":
		resp, err = s.convertModel(cache.Ctx, project, modelName)
	case "delete":
		resp, err = s.deleteModel(cache.Ctx, project, modelName)
	}

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *GraphQLServer) duplicateModel(ctx context.Context, project *models.Project, newName, modelName string) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	newModelName := strings.TrimSpace(utility.SingularResourceName(strcase.ToLowerCamel(newName)))
	protectedNames := map[string]string{
		"user":     "naming a Model `User` is protected. If you want to store authenticated users, try adding the Authentication module from Settings > Add-Ons.",
		"system":   "naming a Model `System` is not allowed. Try another alternate name instead.",
		"function": "naming a Model `Function` is not allowed. Try another alternate name instead.",
	}
	if msg, exists := protectedNames[newModelName]; exists {
		return nil, errors.New(msg)
	}

	var duplicatedModel *models.ModelType
	var err error

	// if schema not found then create
	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		var modelToDuplicate *models.ModelType
		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToDuplicate = ct
				break
			}
		}

		if modelToDuplicate == nil {
			return nil, errors.New("the model about to be duplicated, not found")
		}

		var found bool
		for _, ct := range project.Schema.Models {
			if ct.Name == newModelName {
				found = true
				break
			}
		}

		if !found {
			duplicatedModel = &models.ModelType{
				Name:            newModelName,
				Fields:          modelToDuplicate.Fields,
				Connections:     modelToDuplicate.Connections,
				HookIds:         modelToDuplicate.HookIds,
				Locals:          modelToDuplicate.Locals,
				RepeatedGroups:  modelToDuplicate.RepeatedGroups,
				SystemGenerated: modelToDuplicate.SystemGenerated,
				HasConnections:  modelToDuplicate.HasConnections,
			}
			if modelToDuplicate.SinglePageUUID != "" { // assign new id
				uid := uuid.New()
				duplicatedModel.SinglePage = true
				duplicatedModel.SinglePageUUID = uid.String()
			}
			project.Schema.Models = append(project.Schema.Models, duplicatedModel)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	err = s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return duplicatedModel, nil
}

func (s *GraphQLServer) renameModel(ctx context.Context, project *models.Project, newName, modelName string, singlePageModel bool) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
	}

	if newName == modelName {
		return nil, errors.New("new model name can not be the same as the old one")
	}

	var newModelName string

	newModelName = utility.SingularResourceName(newName)
	if newModelName == "user" {
		return nil, errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
	} else if newModelName == "system" {
		return nil, errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
	} else if newModelName == "function" {
		return nil, errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
	}

	var modelToRename *models.ModelType
	var err error

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToRename = ct
				break
			}
		}

		if modelToRename == nil {
			return nil, errors.New("the model about to be renamed, not found")
		}

		if len(modelToRename.Connections) > 0 {
			return nil, errors.New("can not rename model because it has relations with other models")
		}

		// check if the models has documents

		// rename
		modelToRename.Name = newModelName
	}

	err = s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return modelToRename, nil
}

func (s *GraphQLServer) convertModel(ctx context.Context, project *models.Project, modelName string) (interface{}, error) {

	var modelToConvert *models.ModelType

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	} else {

		for _, ct := range project.Schema.Models {
			if ct.Name == modelName {
				modelToConvert = ct
				break
			}
		}

		if modelToConvert == nil {
			return nil, errors.New("the model about to be renamed, not found")
		}

		if len(modelToConvert.Connections) > 0 {
			return nil, errors.New("can not convert model because it has relations with other models")
		}

		// check if the models has documents

		// convert
		if modelToConvert.SinglePage {
			// remove
			modelToConvert.SinglePage = false
			modelToConvert.SinglePageUUID = ""
		} else {
			// assign new id
			uid := uuid.New()
			modelToConvert.SinglePage = true
			modelToConvert.SinglePageUUID = uid.String()
		}
	}

	err := s.SystemDriver.UpdateProject(ctx, project, false)
	if err != nil {
		return nil, err
	}

	return modelToConvert, nil
}

func (s *GraphQLServer) deleteModel(ctx context.Context, project *models.Project, modelName string) (interface{}, error) {

	if modelName == "user" {
		return nil, errors.New("Can not delete User Model. If you dont want it then remove User Addons")
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("nothing to Delete")
	} else {
		var index int
		var _model *models.ModelType
		for i, ct := range project.Schema.Models {
			if ct.Name == modelName {
				_model = ct
				index = i
				break
			}
		}

		if _model != nil {
			// drop the model from schema
			project.Schema.Models = append(project.Schema.Models[:index], project.Schema.Models[index+1:]...)

			if project.ProjectType == models.ProjectType_SaaS {
				// for saas it creates additional collection, if model dropped all the data is gone
				err := driver.DropModel(ctx, project, _model.Name)
				if err != nil {
					return nil, err
				}
			} else {
				// delete all the data connected to this model
				err := driver.DeleteDocumentsFromProject(ctx, &models.CommonSystemParams{ProjectID: project.ID, Model: _model})
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, errors.New("could not find model to delete")
		}
	}

	// also remove all its relations

	for _, m := range project.Schema.Models {
		for i, c := range m.Connections {
			if c.Model == modelName {
				m.Connections = append(m.Connections[:i], m.Connections[i+1:]...)
			}
		}
		if len(m.Connections) == 0 {
			m.Connections = nil
		}
	}

	err = s.SystemDriver.UpdateProject(ctx, project, true)
	if err != nil {
		return nil, err
	}

	return project.Schema.Models[len(project.Schema.Models)-1], nil
}

func (s *GraphQLServer) UpsertFunctionToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var functionName string
	if val, ok := p.Args["name"].(string); ok && val != "" {
		functionName = utility.SingularResourceName(val)
	} else {
		return nil, errors.New("function Name Required")
	}

	var isUpdate bool
	if val, ok := p.Args["update"].(bool); ok {
		if val {
			isUpdate = val
		}
	}

	if !isUpdate {
		// check for same func name
		for _, f := range project.Schema.Functions {
			if f.Name == functionName {
				return nil, errors.New("function with this name is already defined")
			}
		}
	}

	// check if function name is valid or not
	if strings.HasPrefix(functionName, "create") || strings.HasPrefix(functionName, "update") || strings.HasPrefix(functionName, "delete") {
		model := functionName[6:len(functionName)]
		for _, m := range cache.Project.Schema.Models {
			if m.Name == strings.ToLower(model) {
				return nil, errors.New(fmt.Sprintf("Function Name `%s` is auto generated by System. Cant use this name", functionName))
			}
		}
	}

	//userID := cache.Param.UserId

	var function *models.ApitoFunction

	var oldFunction bool
	// if schema not found then create
	if project.Schema == nil {
		function = &models.ApitoFunction{
			Name:      functionName,
			CreatedAt: utility.GetCurrentTime(),
			UpdatedAt: utility.GetCurrentTime(),
		}
		project.Schema = &models.ProjectSchema{
			Functions: []*models.ApitoFunction{function},
		}
	} else {
		for _, ct := range project.Schema.Functions {
			if ct.Name == functionName {
				function = ct
				function.UpdatedAt = utility.GetCurrentTime()
				oldFunction = true
				break
			}
		}
		if function == nil {
			function = &models.ApitoFunction{
				Name:      functionName,
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
			}
		}
	}

	if val, ok := p.Args["description"].(string); ok && val != "" {
		function.Description = val
	}

	if val, ok := p.Args["function_path"].(string); ok && val != "" {
		function.FunctionPath = val
	}

	if val, ok := p.Args["function_provider_id"].(string); ok && val != "" {
		function.FunctionProviderID = val
		//var _configuration models.PluginDetails
		// Local plugin cache removed - exported variable lookup disabled
		// Use HashiCorp plugins instead
	}

	if val, ok := p.Args["provider_exported_variable"].(string); ok && val != "" {
		function.ProviderExportedVariable = val
	}

	if val, ok := p.Args["function_exported_variable"].(string); ok && val != "" {
		function.FunctionExportedVariable = val
	}

	if val, ok := p.Args["graphql_schema_type"].(string); ok && val != "" {
		function.GraphQLSchemaType = val
	}

	//if val, ok := p.Args["type"]
	if val, ok := p.Args["request"].(string); ok {
		function.Request = &models.ApitoFunctionRequestResponseType{
			Model: val,
		}
	}

	if val, ok := p.Args["request_payload_is_optional"].(bool); ok {
		if function.Request == nil {
			return nil, errors.New("request model is required")
		}
		function.Request.OptionalPayload = val
	}

	if val, ok := p.Args["response"].(string); ok {
		function.Response = &models.ApitoFunctionRequestResponseType{
			Model: val,
		}
	}

	if val, ok := p.Args["response_is_array"].(bool); ok {
		if function.Response == nil {
			return nil, errors.New("response model is required")
		}
		function.Response.IsArray = val
	}

	// update config if found
	if vals, ok := p.Args["env_vars"].([]interface{}); ok && len(vals) > 0 {
		var vars []*protobuff.EnvVariable
		for _, v := range vals {
			vv := v.(map[string]interface{})
			vars = append(vars, &protobuff.EnvVariable{
				Key:   vv["key"].(string),
				Value: vv["value"].(string),
			})
		}
		function.EnvVars = vars
	}

	if val, ok := p.Args["runtime_config"].(map[string]interface{}); ok {
		if function.RuntimeConfig == nil {
			function.RuntimeConfig = &models.ApitoFunctionRuntimeConfig{}
		}
		for k, v := range val {
			switch k {
			case "runtime":
				function.RuntimeConfig.Runtime = v.(string)
				break
			case "memory":
				function.RuntimeConfig.Memory = int64(v.(int))
				break
			case "handler":
				function.RuntimeConfig.Handler = v.(string)
				break
			case "time_out":
				function.RuntimeConfig.TimeOut = int64(v.(int))
				break
			}
		}
	}

	/*switch function.FunctionProviderType {
	case models.FunctionProvider_ViaExtension:
		if plugin, ok := s.LocalPluginCache[function.FunctionProviderName]; ok {
			//if plugin.Lookup("")
			fmt.Println(plugin)
			function.FunctionConnected = true
		}

		if plugin, ok := s.FunctionCache[function.FunctionProviderName]; ok {
			//if plugin.Lookup("")
			fmt.Println(plugin)
			function.FunctionConnected = true
		}

		/*if val, ok := p.Args["remote_function_name"].(string); ok {
			function.FunctionProviderType = models.FunctionProvider_GoPlugin
			if function.ProviderConfig == nil {
				function.ProviderConfig = &models.FunctionProviderConfig{
					RemoteFunctionName: val,
				}
			} else {
				function.ProviderConfig.RemoteFunctionName = val
			}

			if val, ok := p.Args["region"].(string); ok {
				function.ProviderConfig.Region = val
			}

			// fetch all property by func name
			if function.ProviderConfig.Region == "" {
				return nil, errors.New("region is required for remove func assignment")
			}

			functions, err := s.FetchAWSLambdaFunctions(function.ProviderConfig.Region)
			if err != nil {
				return nil, err
			}
			if len(functions) == 0 {
				return nil, errors.New("No function found to connect to")
			}

			var functionToConnect *models.FunctionProviderConfig
			for _, f := range functions {
				if f.RemoteFunctionName == function.ProviderConfig.RemoteFunctionName {
					functionToConnect = f
					break
				}
			}

			if functionToConnect != nil {
				function.ProviderConfig.EnvVars = functionToConnect.EnvVars
				function.ProviderConfig.Configs = functionToConnect.Configs
				function.FunctionConnected = true
			} else {
				return nil, errors.New("Invalid function name given to connect")
			}
		}*/

	/*if function.ProviderConfig == nil {
			return nil, errors.New("Can not set env variable without connecting to a provider")
		}

		// update in aws too
		_, err = s.UpdateAWSLambdaFunctions(function)
		if err != nil {
			return nil, err
		}
		break
	case models.FunctionProvider_GoPlugin:

		// 2. look up a symbol (an exported function or variable)
		// in this case, variable Greeter
		plugin, err := s.PluginLoader(cache.Param.ProjectId, function)
		if err != nil {
			return nil, err
		}

		// 2. look up a symbol (an exported function or variable)
		// in this case, variable Greeter
		pluginLookUp, err := plugin.Lookup(function.ExportedVariable)
		if err != nil {
			return nil, err
		}

		// 3. Assert that loaded symbol is of a desired type
		// in this case interface type Greeter (defined above)
		var loadedPlugin models.Plugins
		loadedPlugin, ok := pluginLookUp.(models.Plugins)
		if !ok {
			return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, function.Name))
		}

		fmt.Println(fmt.Sprintf(`------ Loading %s Function Plugin -------`, function.Name))

		err = loadedPlugin.Init(function.EnvVars)
		if err != nil {
			return nil, err
		}

		function.FunctionConnected = true
	// #todo
	case models.FunctionProvider_NoProvider:
		function.FunctionConnected = false
	}*/

	/*if val, ok := p.Args["function_connected"].(bool); ok {
		if function.FunctionConnected && function.ProviderConfig == nil {
			return nil, errors.New("Try to connect in a proper way")
		}
		function.FunctionConnected = val
	}*/

	if oldFunction {
		err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
		if err != nil {
			return nil, err
		}
	} else {
		if function.Request == nil || function.Response == nil {
			return nil, errors.New("can not create function without proper Request & Response")
		}
		if function.RestAPISecretURLKey == "" {
			function.RestAPISecretURLKey = utility.RandomStringGenerator(25)
		}
		project.Schema.Functions = append(project.Schema.Functions, function)
		err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
		if err != nil {
			return nil, err
		}
	}

	return function, nil
}

func (s *GraphQLServer) UpsertRoleToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpsertRoleToProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var roleName string
	if val, ok := p.Args["name"].(string); ok {
		roleName = strings.ToLower(utility.SingularResourceName(val))
	} else {
		return nil, errors.New("function Name Required")
	}

	switch roleName {
	case "admin":
		return nil, errors.New("a default Role with name `Admin` already exists in your project. Choose other names")
	case "demo":
		return nil, errors.New("can not create Role named `Demo`. Its being used internally. Choose other names")
	}

	if project.Roles == nil {
		project.Roles = map[string]*models.Role{
			"admin": {
				IsAdmin: true,
			},
		}
	}

	var role *models.Role
	// if schema not found then create
	if r, ok := project.Roles[roleName]; ok {
		role = r
	} else {
		role = &models.Role{}
		project.Roles[roleName] = role
	}

	var isAdmin bool
	if val, ok := p.Args["is_admin"].(bool); ok {
		if val {
			role.IsAdmin = val
			isAdmin = val
		}
	}

	if logicExecutions, ok := p.Args["logic_executions"].([]interface{}); ok && len(logicExecutions) > 0 {
		for _, l := range logicExecutions {
			if !utility.ArrayContains(role.LogicExecutions, l.(string)) {
				role.LogicExecutions = append(role.LogicExecutions, l.(string))
			}
		}
	}

	if !isAdmin {
		//if val, ok := p.Args["type"]
		if val, ok := p.Args["api_permissions"].(map[string]interface{}); ok {
			permissions := make(map[string]*models.APIPermission)
			for k, v := range val {
				validatedPermissions, err := utility.ValidatePermissions(v.(map[string]interface{}))
				if err != nil {
					return nil, err
				}
				permissions[k] = validatedPermissions
			}
			role.APIPermissions = permissions
		}

		if val, ok := p.Args["administrative_permissions"].([]interface{}); ok {
			var pms []string
			for _, v := range val {
				pms = append(pms, v.(string))
			}
			role.AdministrativePermissions = pms
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 500)

	return role, nil
}

func (s *GraphQLServer) DeleteFunctionResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var functionName string
	if val, ok := p.Args["function"].(string); ok && val != "" {
		functionName = val
	} else {
		return nil, errors.New("function Name is Required")
	}

	// if schema not found then create
	if project.Schema == nil {
		return nil, errors.New("nothing to Delete")
	} else {
		var found bool
		var index int
		for i, ct := range project.Schema.Functions {
			if ct.Name == functionName {
				found = true
				index = i
				break
			}
		}

		if found {
			project.Schema.Functions = append(project.Schema.Functions[:index], project.Schema.Functions[index+1:]...)
		} else {
			return nil, errors.New("could not find function to delete")
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// #todo delete all the data if so or not

	return project.Schema.Functions, nil
}

func (s *GraphQLServer) DeleteRoleResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteRoleResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	roleToDelete := strings.ToLower(utility.SingularResourceName(p.Args["role"].(string)))

	// if schema not found then create
	if role, ok := project.Roles[roleToDelete]; ok {
		if role.SystemGenerated {
			return nil, errors.New("you are not allowed to delete System Generated Roles")
		}
		// check if token is generated with this roles
		var tokenRelated bool
		for _, t := range project.Tokens {
			if t.Role == roleToDelete {
				tokenRelated = true
				break
			}
		}
		if tokenRelated {
			return nil, errors.New("there are active API Secrets associated with this role. Delete those API Secrects first")
		}
		delete(project.Roles, roleToDelete)
	} else {
		return nil, errors.New("role not Found")
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	return project.Roles, nil
}

func (s *GraphQLServer) searchFields(fields []*models.FieldInfo, parentIdentifier, identifier string) *models.FieldInfo {
	// Direct search mode - find a field with specified identifier at any nesting level
	if parentIdentifier == "" && identifier != "" {
		for _, f := range fields {
			if f.Identifier == identifier {
				return f
			}

			// Recursively search in subfields
			if f.SubFieldInfo != nil && len(f.SubFieldInfo) > 0 {
				if found := s.searchFields(f.SubFieldInfo, "", identifier); found != nil {
					return found
				}
			}
		}
		return nil
	}

	// Parent-child search mode - find a field under a specific parent
	if parentIdentifier != "" {
		for _, f := range fields {
			if f.Identifier == parentIdentifier {
				// Found the parent, now search for child
				if identifier == "" {
					return f // Just return the parent if no child identifier specified
				}

				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == identifier {
						return sf
					}
				}
				return nil // Child not found under this parent
			}

			// Recursively search for parent in subfields
			if f.SubFieldInfo != nil && len(f.SubFieldInfo) > 0 {
				if found := s.searchFields(f.SubFieldInfo, parentIdentifier, identifier); found != nil {
					return found
				}
			}
		}
	}

	return nil
}

func (s *GraphQLServer) UpsertFieldToModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpsertFieldToModelResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = utility.SingularResourceName(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *models.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var identifier string
	var label string
	if val, ok := p.Args["field_label"].(string); ok {
		label = strings.TrimSpace(val)
		m := regexp.MustCompile("[^A-Za-z0-9]+")
		identifier = strings.TrimSpace(strings.ToLower(m.ReplaceAllString(label, "_")))
		// check for valid field name. Restrict a few
		if strings.HasPrefix(identifier, "_") {
			return nil, errors.New("field can not begin with _")
		}
		if utility.ArrayContains([]string{"id", "_id"}, identifier) {
			return nil, errors.New(fmt.Sprintf("Field %s is auto generated by the System. No Need to define It", identifier))
		} else if utility.ArrayContains([]string{"status"}, identifier) {
			return nil, errors.New("status field is auto generated and reserved for document publishing status in the API. Choose other name instead")
		} else if strings.HasPrefix(identifier, "sys_") {
			return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
		}

		//check if model name starts with number
		var re = regexp.MustCompile(`^\d`)
		matchFound := re.FindAllString(identifier, -1)
		if len(matchFound) > 0 {
			return nil, errors.New("field name can not start with a number! use character instead")
		}

	} else {
		return nil, errors.New("field Label Is necessary")
	}

	var parentField string
	if val, ok := p.Args["parent_field"].(string); ok {
		if val != "_root" {
			parentField = val
		}
	}

	var isUpdate bool
	if val, ok := p.Args["is_update"].(bool); ok {
		isUpdate = val
	}

	// now search for fields
	fieldInfo := s.searchFields(modelType.Fields, parentField, identifier)

	if !isUpdate && fieldInfo != nil {
		return nil, errors.New(fmt.Sprintf("A field with identifier '%s' already exits", identifier))
	}

	if fieldInfo == nil {
		fieldInfo = &models.FieldInfo{
			Identifier: identifier,
			Label:      label,
			Serial:     uint32(len(modelType.Fields) + 1),
		}
	}

	if val, ok := p.Args["is_object_field"].(bool); ok {
		fieldInfo.IsObjectField = val
	}

	if val, ok := p.Args["field_type"].(string); ok {
		fieldInfo.FieldType = val
	}

	if val, ok := p.Args["input_type"].(string); ok {
		fieldInfo.InputType = val
	}

	// validate field & input type combination and other validation
	switch fieldInfo.FieldType {
	case "geo":
		if fieldInfo.InputType != "geo" {
			return nil, errors.New("input Type must be Geo if Field Type is Geo")
		}
		break
	case "repeated":
		fieldInfo.SubFieldInfo = []*models.FieldInfo{
			&models.FieldInfo{
				Identifier:   "_id",
				Description:  "An Auto Generated UUIDv4 Unique Identifier",
				InputType:    "string",
				FieldType:    "text",
				SubFieldInfo: nil,
				Validation: &models.Validation{
					Hide:   true,
					Unique: true,
				},
				Serial:                  1,
				Label:                   "ID",
				SystemGenerated:         true,
				//RepeatedGroupIdentifier: fieldInfo.Identifier,
				ParentField:             fieldInfo.Identifier,
			},
		}
	}

	if val, ok := p.Args["serial"].(int); ok {
		fieldInfo.Serial = uint32(val)
	}

	if val, ok := p.Args["field_description"].(string); ok {
		fieldInfo.Description = val
	}

	if validation, ok := p.Args["validation"].(map[string]interface{}); ok {
		if fieldInfo.Validation == nil {
			fieldInfo.Validation = &models.Validation{}
		}
		if v, ok := validation["required"].(bool); ok {
			fieldInfo.Validation.Required = v
		}
		/* if v, ok := validation["as_title"].(bool); ok {
			fieldInfo.Validation.AsTitle = v
		} */

		if v, ok := validation["hide"].(bool); ok {
			fieldInfo.Validation.Hide = v
		}

		if v, ok := validation["is_email"].(bool); ok {
			fieldInfo.Validation.IsEmail = v
		}

		if v, ok := validation["is_gallery"].(bool); ok {
			fieldInfo.Validation.IsGallery = v
		}

		if v, ok := validation["is_url"].(bool); ok {
			fieldInfo.Validation.IsURL = v
		}

		if v, ok := validation["unique"].(bool); ok {
			fieldInfo.Validation.Unique = v
		}

		if v, ok := validation["is_multi_choice"].(bool); ok {
			fieldInfo.Validation.IsMultiChoice = v
		}

		if v, ok := validation["placeholder"].(string); ok && v != "" {
			fieldInfo.Validation.Placeholder = v
		}

		if vals, ok := validation["locals"].([]interface{}); ok && len(vals) > 0 {
			var elements []string
			for _, v := range vals {
				elements = append(elements, v.(string))
			}
			fieldInfo.Validation.Locals = elements
		} else {
			fieldInfo.Validation.Locals = nil
		}

		if v, ok := validation["fixed_list_element_type"].(string); ok && v != "" {
			fieldInfo.Validation.FixedListElementType = v
		}

		if vals, ok := validation["fixed_list_elements"].([]interface{}); ok && len(vals) > 0 {
			/* var elements []string
			for _, v := range vals {
				elements = append(elements, v.(string))
			} */
			fieldInfo.Validation.FixedListElements = vals
		}

	}

	if parentField != "" {
		//fieldInfo.RepeatedGroupIdentifier = repeatedGroupIdentifier
		fieldInfo.ParentField = fieldInfo.ParentField
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.FieldInfo = fieldInfo

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	modelType, err = driver.AddFieldToModel(cache.Ctx, param, isUpdate, parentField)
	if err != nil {
		return nil, err
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// expire the cache
	err = s.ExpireGraphQLFieldCache(cache.Ctx, project.ID, modelType.Name)
	if err != nil {
		return nil, err
	}

	// expire the project cache
	err = s.ExpireGraphQLProjectCache(cache.Ctx, project.ID)
	if err != nil {
		return nil, err
	}

	return fieldInfo, nil
}

func (s *GraphQLServer) RearrangeFieldOfModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = utility.SingularResourceName(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *models.ModelType
	for _, ct := range cache.Project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	var oldSerial uint32
	if val, ok := p.Args["old_serial"].(int); ok {
		oldSerial = uint32(val)
	} else {
		return nil, errors.New("old serial is required")
	}

	var newSerial uint32
	if val, ok := p.Args["new_serial"].(int); ok {
		newSerial = uint32(val)
	} else {
		return nil, errors.New("new serial is required")
	}

	var parentField string
	if val, ok := p.Args["parent_field"].(string); ok {
		parentField = val
	} else {
		return nil, errors.New("parent_field is required")
	}

	var fieldName string
	if val, ok := p.Args["field_name"].(string); ok {
		fieldName = val
	} else {
		return nil, errors.New("field_name is required")
	}

	fmt.Println(fieldName)

	// now search for fields
	for _, f := range modelType.Fields {
		if parentField == "_root" { // root field rearrange
			if f.Serial == oldSerial {
				f.Serial = newSerial
			} else {
				if newSerial < oldSerial && (f.Serial >= newSerial && f.Serial < oldSerial) { // bottom up
					f.Serial = f.Serial + 1
				} else if newSerial > oldSerial && (f.Serial > oldSerial && f.Serial <= newSerial) { // top down
					f.Serial = f.Serial - 1
				}
			}
		} else if parentField != "_root" && f.SubFieldInfo != nil { // it's an array/object field rearrange
			if f.Identifier == parentField {
				for _, sf := range f.SubFieldInfo {
					if sf.Serial == oldSerial {
						sf.Serial = newSerial
					} else {
						if newSerial < oldSerial && (sf.Serial >= newSerial && sf.Serial < oldSerial) { // bottom up
							sf.Serial = sf.Serial + 1
						} else if newSerial > oldSerial && (sf.Serial > oldSerial && sf.Serial <= newSerial) { // top down
							sf.Serial = sf.Serial - 1
						}
					}
				}
				sort.Slice(f.SubFieldInfo, func(i, j int) bool {
					return f.SubFieldInfo[i].Serial < f.SubFieldInfo[j].Serial
				})
				break
			}
		}
	}

	// rearrange others
	sort.Slice(modelType.Fields, func(i, j int) bool {
		return modelType.Fields[i].Serial < modelType.Fields[j].Serial
	})

	cache.Param.Model = modelType

	project, err := s.SystemDriver.GetProject(cache.Ctx, cache.Project.ID)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema
	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	return modelType, nil
}

func (s *GraphQLServer) ModelFieldOperationResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("ModelFieldOperationResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = utility.SingularResourceName(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var singlePageModel bool
	if val, ok := p.Args["single_page_model"].(bool); ok {
		singlePageModel = val
	}

	var modelType *models.ModelType
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	var _type enums.FieldOperation
	if val, ok := p.Args["type"].(enums.FieldOperation); ok {
		_type = val
	}

	var fieldName string
	if val, ok := p.Args["field_name"].(string); ok {
		fieldName = val
	} else {
		return nil, errors.New("field name is required")
	}

	var parentField string
	if val, ok := p.Args["parent_field"].(string); ok && val != "_root" {
		parentField = val
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	var fieldInfo *models.FieldInfo

	switch _type {
	case enums.FieldOperation_Rename:
		var label string
		var newIdentifier string
		if val, ok := p.Args["new_name"].(string); ok {
			name := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(val, " ", "_")))
			// check for valid field name. Restrict a few
			if name == "id" {
				return nil, errors.New("field ID is auto generated by the System. No Need to define It")
			} else if strings.HasPrefix(name, "sys_") {
				return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
			}
			label = val
			newIdentifier = name
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// now search for fields
		for _, f := range modelType.Fields {
			if f.Identifier == fieldName && parentField == "" {
				fieldInfo = f
				// rename this
				fieldInfo.Identifier = newIdentifier
				fieldInfo.Label = label
				break
			} else if parentField != "" && f.Identifier == parentField && f.SubFieldInfo != nil {
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == fieldName {
						fieldInfo = &models.FieldInfo{
							Identifier:      newIdentifier,
							Description:     sf.Description,
							InputType:       sf.InputType,
							FieldType:       sf.FieldType,
							Validation:      sf.Validation,
							Serial:          sf.Serial,
							Label:           label,
							SystemGenerated: sf.SystemGenerated,
						}
						// rename this
						sf.Identifier = newIdentifier
						sf.Label = label

						if len(sf.SubFieldInfo) > 0 {
							for _, ssf := range sf.SubFieldInfo {
								//ssf.parent_field = newIdentifier
								ssf.ParentField = newIdentifier
							}
						}
					}
				}
			}
		}

		if fieldInfo == nil {
			return nil, errors.New("field not found to Request")
		}

		param := s.NewParam(cache.Param)
		param.Model = modelType
		param.FieldInfo = fieldInfo
		param.SinglePageData = singlePageModel

		if fieldName != param.FieldInfo.Identifier { // skip renaming if the same value is given
			err := driver.RenameField(cache.Ctx, fieldName, parentField, param)
			if err != nil {
				return nil, err
			}
		}
	case enums.FieldOperation_Duplicate:
		var label string
		var newIdentifier string
		if val, ok := p.Args["new_name"].(string); ok {
			name := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(val, " ", "_")))
			// check for valid field name. Restrict a few
			if name == "id" {
				return nil, errors.New("field ID is auto generated by the System. No Need to define It")
			} else if strings.HasPrefix(name, "sys_") {
				return nil, errors.New("field Name Starts with SYS/Sys is protected. Please Use alternative names")
			}
			label = val
			newIdentifier = name
		} else {
			return nil, errors.New("field Label Is necessary")
		}
		var newFieldInfo models.FieldInfo
		var found bool

		// Single loop to check for duplicates and create new field
		for _, f := range modelType.Fields {
			// Check for duplicates
			if f.Identifier == newIdentifier && parentField == "" {
				return nil, errors.New("field with that name already exists")
			}

			if f.Identifier == parentField && f.SubFieldInfo != nil && parentField != "" {
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == newIdentifier {
						return nil, errors.New("sub field with that name already exists")
					}
				}
			}

			// Create new field if matching field found
			if f.Identifier == fieldName && parentField == "" {
				newFieldInfo = models.FieldInfo{
					Identifier:              newIdentifier,
					Description:             f.Description,
					InputType:               f.InputType,
					FieldType:               f.FieldType,
					Validation:              f.Validation,
					Serial:                  uint32(len(modelType.Fields) + 1),
					Label:                   label,
					ParentField:             f.ParentField,
					//RepeatedGroupIdentifier: f.RepeatedGroupIdentifier,
					SystemGenerated:         f.SystemGenerated,
					SubFieldInfo:            f.SubFieldInfo,
				}
				modelType.Fields = append(modelType.Fields, &newFieldInfo)
				found = true
				break
			} else if parentField != "" && f.Identifier == parentField && f.SubFieldInfo != nil {
				var fieldToDuplicate *models.FieldInfo
				for _, sf := range f.SubFieldInfo {
					if sf.Identifier == fieldName {
						fieldToDuplicate = sf
						break
					}
				}
				// Add subfield if not found
				f.SubFieldInfo = append(f.SubFieldInfo, &models.FieldInfo{
					Identifier:              newIdentifier,
					Label:                   label,
					Description:             fieldToDuplicate.Description,
					InputType:               fieldToDuplicate.InputType,
					FieldType:               fieldToDuplicate.FieldType,
					Validation:              fieldToDuplicate.Validation,
					Serial:                  uint32(len(f.SubFieldInfo) + 1),
					ParentField:             fieldToDuplicate.ParentField,
					//RepeatedGroupIdentifier: fieldToDuplicate.RepeatedGroupIdentifier,
					SystemGenerated:         fieldToDuplicate.SystemGenerated,
				})
				found = true
			}
		}

		if !found {
			return nil, errors.New("field not found to duplicate")
		}

		/* param := s.NewParam(cache.Param)
		param.Model = modelType
		param.FieldInfo = &newFieldInfo
		param.SinglePageData = singlePageModel
		modelType, err = driver.AddFieldToModel(p.Context, param, true, parent_field)
		if err != nil {
			return nil, err
		} */

		// for response
		fieldInfo = &newFieldInfo
	}

	project, err = s.SystemDriver.GetProject(cache.Ctx, project.ID)
	if err != nil {
		return nil, err
	}

	project.Schema = cache.Project.Schema
	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// expire the cache
	err = s.ExpireGraphQLFieldCache(cache.Ctx, project.ID, modelType.Name)
	if err != nil {
		return nil, err
	}

	return fieldInfo, nil
}

/*func (s *GraphQLServer) UploadImageFromURLResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	var url string
	if val, ok := p.Args["url"].(string); ok && val != "" {
		url = val
	} else {
		return nil, errors.New("URL is Necessary")
	}
	m, err := s.GraphQLExecutor.HandleMediaURL(p.Context, param, map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}
*/

func (s *GraphQLServer) deleteFieldFromModels(models []*models.ModelType, modelID, fieldID string) *models.ModelType {
	for _, m := range models {
		if m.Name == modelID {
			for i, f := range m.Fields {
				if f.Identifier == fieldID {
					m.Fields = append(m.Fields[:i], m.Fields[i+1:]...)
					return m
				}
			}
		}
	}
	return nil
}

func (s *GraphQLServer) findAndDeleteField(fields []*models.FieldInfo, identifier string, parentPath string) (*models.FieldInfo, bool) {
	for i, f := range fields {
		// Check if this is the field to delete
		if f.Identifier == identifier {
			// Remove the field from the slice
			deletedField := f
			fields[i] = fields[len(fields)-1]
			fields = fields[:len(fields)-1]

			// Update the parent's SubFieldInfo reference if we're in a nested context
			if parentPath != "" {
				for j := range fields {
					if fields[j].Identifier == parentPath {
						fields[j].SubFieldInfo = fields
						break
					}
				}
			}

			// Adjust serials for siblings
			for j := range fields {
				if fields[j].Serial > deletedField.Serial {
					fields[j].Serial--
				}
			}

			return deletedField, true
		}

		// Recursively search in subfields
		if f.SubFieldInfo != nil && len(f.SubFieldInfo) > 0 {
			newParentPath := f.Identifier
			if parentPath != "" {
				newParentPath = parentPath + "." + f.Identifier
			}

			if deletedField, found := s.findAndDeleteField(f.SubFieldInfo, identifier, newParentPath); found {
				return deletedField, true
			}
		}
	}

	return nil, false
}

func (s *GraphQLServer) DeleteFieldTypeResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteFieldTypeResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = strings.TrimSpace(utility.SingularResourceName(val))
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *models.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}
	if modelType == nil {
		return nil, errors.New(ae.MODEL_IS_REQUIRED)
	}

	param := s.NewParam(cache.Param)

	param.Model = modelType
	identifier := p.Args["identifier"].(string)

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	if isRelation, ok := p.Args["is_relation"].(bool); ok && isRelation {

		// struct the connection type before removing from schema
		var fromConnectionType *models.ConnectionType
		// delete the forward relation
		for i, r := range modelType.Connections {
			if r.Model == identifier {
				fromConnectionType = r
				modelType.Connections = append(modelType.Connections[:i], modelType.Connections[i+1:]...)
				break
			}
		}
		if len(modelType.Connections) == 0 {
			modelType.Connections = nil
		}

		// struct the connection type before removing from schema
		var toConnectionType *models.ConnectionType

		// delete the backward relation
		for _, ct := range project.Schema.Models {
			if ct.Name == identifier {
				for i, r := range ct.Connections {
					if r.Model == modelName {
						toConnectionType = r
						ct.Connections = append(ct.Connections[:i], ct.Connections[i+1:]...)
						break
					}
				}
				if len(ct.Connections) == 0 {
					ct.Connections = nil
				}
				break
			}
		}

		// delete system has one identifer
		if fromConnectionType.Relation == "has_one" {
			s.deleteFieldFromModels(project.Schema.Models, toConnectionType.Model, fmt.Sprintf(`system_%s_id`, fromConnectionType.Model))
		}

		if toConnectionType.Relation == "has_one" {
			s.deleteFieldFromModels(project.Schema.Models, fromConnectionType.Model, fmt.Sprintf(`system_%s_id`, toConnectionType.Model))
		}

		// drop it from db
		param.FieldInfo = &models.FieldInfo{Identifier: identifier}

		err = driver.DeleteRelationDocuments(cache.Ctx, param.ProjectID,
			fromConnectionType,
			toConnectionType,
		)
		if err != nil {
			return nil, err
		}
	} else {

		var parentIdentifier string
		if val, ok := p.Args["parent_field"].(string); ok {
			if val != "_root" { // skip if root
				parentIdentifier = val
			}
		}

		// Handle parent-child explicit relationship if parentIdentifier is provided
		if parentIdentifier != "" {
			// Find parent field recursively
			parentField := s.searchFields(modelType.Fields, "", parentIdentifier)

			if parentField == nil {
				return nil, errors.New("parent field not found")
			}

			// Find and remove child field
			var deletedField *models.FieldInfo
			for i, sf := range parentField.SubFieldInfo {
				if sf.Identifier == identifier {
					deletedField = sf
					// Remove the field
					parentField.SubFieldInfo = append(parentField.SubFieldInfo[:i], parentField.SubFieldInfo[i+1:]...)

					// Adjust serial numbers for siblings
					for j := range parentField.SubFieldInfo {
						if parentField.SubFieldInfo[j].Serial > deletedField.Serial {
							parentField.SubFieldInfo[j].Serial--
						}
					}

					// Prepare field info for database update
					param.FieldInfo = &models.FieldInfo{
						Identifier:              deletedField.Identifier,
						Description:             deletedField.Description,
						InputType:               deletedField.InputType,
						FieldType:               deletedField.FieldType,
						Validation:              deletedField.Validation,
						Serial:                  deletedField.Serial,
						Label:                   deletedField.Label,
						SystemGenerated:         deletedField.SystemGenerated,
						ParentField:             parentIdentifier,
						//RepeatedGroupIdentifier: parentIdentifier,
					}

					// Drop it from database
					err = driver.DropField(cache.Ctx, param)
					if err != nil {
						return nil, err
					}
					break
				}
			}

			if deletedField == nil {
				return nil, errors.New("field not found in parent")
			}
		} else {
			// Use recursive approach for fields at any nesting level
			var deletedField *models.FieldInfo
			var found bool

			// Check for top-level fields first for backward compatibility
			for i, f := range modelType.Fields {
				if f.Identifier == identifier {
					deletedField = f
					modelType.Fields = append(modelType.Fields[:i], modelType.Fields[i+1:]...)
					found = true

					// Adjust serial numbers
					for j := range modelType.Fields {
						if modelType.Fields[j].Serial > deletedField.Serial {
							modelType.Fields[j].Serial--
						}
					}
					break
				}
			}

			// If not found at top level, search recursively
			if !found {
				deletedField, found = s.findAndDeleteField(modelType.Fields, identifier, "")
			}

			if deletedField != nil {
				// Prepare field info for database update
				param.FieldInfo = deletedField

				// Drop it from database
				err = driver.DropField(cache.Ctx, param)
				if err != nil {
					return nil, err
				}
			} else if !found {
				return nil, errors.New("field not found")
			}
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	// expire the project cache
	err = s.ExpireGraphQLProjectCache(cache.Ctx, project.ID)
	if err != nil {
		return nil, err
	}

	return modelType, nil
}

func (s *GraphQLServer) CreateConnectionTypeResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("CreateConnectionTypeResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	
	project := cache.Project

	var fromResource string
	if val, ok := p.Args["from"].(string); ok {
		fromResource = strings.TrimSpace(utility.SingularResourceName(strcase.ToLowerCamel(val)))
	} else {
		return nil, errors.New("from Model Needed")
	}

	var toResource string
	if val, ok := p.Args["to"].(string); ok {
		toResource = strings.TrimSpace(utility.SingularResourceName(strcase.ToLowerCamel(val)))
	} else {
		return nil, errors.New("to Model Needed")
	}

	var knownAs string
	if val, ok := p.Args["known_as"].(string); ok {
		knownAs = strings.TrimSpace(utility.SingularResourceName(strcase.ToLowerCamel(val)))
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	var connections []*models.ConnectionType

	var fromModelType *models.ModelType
	var toModelType *models.ModelType
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	} else {
		for _, ct := range project.Schema.Models {
			switch ct.Name {
			case fromResource:
				fromModelType = ct
			case toResource:
				toModelType = ct
			}
		}

		if fromModelType == nil || toModelType == nil {
			return nil, errors.New("model Not Found")
		}

		// dont let insert relations without defining any fields
		if len(fromModelType.Fields) == 0 {
			return nil, fmt.Errorf("can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(fromModelType.Name)))
		} else if len(toModelType.Fields) == 0 {
			return nil, fmt.Errorf("can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(toModelType.Name)))
		}

		// from
		var fromConnectionInfo *models.ConnectionType
		for _, f := range fromModelType.Connections {
			if f.Model == toResource && f.KnownAs == knownAs {
				fromConnectionInfo = f
				break
			}
		}

		if fromConnectionInfo == nil {
			fromConnectionInfo = &models.ConnectionType{
				Model:   toResource,
				Type:    "forward",
				KnownAs: knownAs,
			}
			if val, ok := p.Args["forward_connection_type"]; ok {
				fromConnectionInfo.Relation = val.(string)
			}
			fromModelType.Connections = append(fromModelType.Connections, fromConnectionInfo)
		} else {
			if val, ok := p.Args["forward_connection_type"]; ok {
				fromConnectionInfo.Relation = val.(string)
			}
		}

		connections = append(connections, fromConnectionInfo)

		// to
		var toConnectionInfo *models.ConnectionType
		for _, f := range toModelType.Connections {
			if f.Model == fromResource && f.KnownAs == knownAs {
				toConnectionInfo = f
				break
			}
		}

		if toConnectionInfo == nil {
			toConnectionInfo = &models.ConnectionType{
				Model:   fromResource,
				Type:    "backward",
				KnownAs: knownAs,
			}
			if val, ok := p.Args["reverse_connection_type"]; ok {
				toConnectionInfo.Relation = val.(string)
			}
			toModelType.Connections = append(toModelType.Connections, toConnectionInfo)
		} else {
			if val, ok := p.Args["reverse_connection_type"]; ok {
				toConnectionInfo.Relation = val.(string)
			}
		}

		// For Has One Relation Add a `system_model_id` field by default in that model for
		// Easy filter purposes
		if fromConnectionInfo.Relation == "has_one" {
			// check for already existed id
			identifier := fmt.Sprintf(`system_%s_id`, fromConnectionInfo.Model)
			var found bool
			for _, f := range fromModelType.Fields {
				if f.Identifier == identifier {
					found = true
					break
				}
			}
			if !found {
				fromModelType.Fields = append(fromModelType.Fields, &models.FieldInfo{
					Identifier:   identifier,
					Description:  "An Auto Generated Relation Identifier for Easy Filter Purposes",
					InputType:    "string",
					FieldType:    "text",
					SubFieldInfo: nil,
					Validation: &models.Validation{
						Hide:   true,
						Unique: true,
					},
					Serial:          1,
					Label:           fmt.Sprintf(`System %s ID`, strings.Title(fromConnectionInfo.Model)),
					SystemGenerated: true,
				})
			}
		}

		if toConnectionInfo.Relation == "has_one" {
			// check for already existed id
			identifier := fmt.Sprintf(`system_%s_id`, toConnectionInfo.Model)
			var found bool
			for _, f := range toModelType.Fields {
				if f.Identifier == identifier {
					found = true
					break
				}
			}
			if !found {
				toModelType.Fields = append(toModelType.Fields, &models.FieldInfo{
					Identifier:   identifier,
					Description:  "An Auto Generated Relation Identifier for Easy Filter Purposes",
					InputType:    "string",
					FieldType:    "text",
					SubFieldInfo: nil,
					Validation: &models.Validation{
						Hide:   true,
						Unique: true,
					},
					Serial:          1,
					Label:           fmt.Sprintf(`System %s ID`, strings.Title(toConnectionInfo.Model)),
					SystemGenerated: true,
				})
			}
		}

		// #todo we should rearrange all the serial after inserting this at the top

		// used for SQL type driver. For nosql it's not implemented or needed
		err = driver.AddRelationFields(cache.Ctx, fromConnectionInfo, toConnectionInfo)
		if err != nil {
			return nil, err
		}

		err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
		if err != nil {
			return nil, err
		}

		// for ui purpose
		toConnectionInfo.Model = toResource
		connections = append(connections, toConnectionInfo)
	}

	return connections, nil
}

func (s *GraphQLServer) UpsertModelDataFnFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("UpsertModelDataFnFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p

	/*tenantId := router.Get("tenant")
	switch param.Role.ID {
	case "tenant":
		if tenantId == nil {
			return nil, errors.New("unable to Identify the User")
		}
		param.TenantId = tenantId.(string)
		break
	}*/

	project := cache.Project
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		modelName = val
	} else {
		return nil, errors.New("model Name is Necessary")
	}

	var modelType *models.ModelType
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New("model Not Found")
	}

	param.Model = modelType

	/* var tempTenantId string
	// inject temp tenant id to fetch specific tenant data
	if val := router.Get("temp_tenant_id"); val != nil {
		tempTenantId = val.(string)
	}
	param.TenantId = tempTenantId
	*/

	if val, ok := p.Args["status"].(string); ok {
		param.DocPublishStatus = val
	} else {
		param.DocPublishStatus = "published" // default is published
	}

	var forceUpdate bool
	if val, ok := p.Args["force_update"].(bool); ok {
		forceUpdate = val
	} else {
		forceUpdate = false
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	var doc *types.DefaultDocumentStructure

	if val, ok := p.Args["_id"]; ok {

		var isSinglePageData bool
		if val, ok := p.Args["single_page_data"].(bool); ok {
			isSinglePageData = val
		}

		param.DocumentID = val.(string)
		param.ResolveParams = &p
		param.Model = modelType
		param.SinglePageData = isSinglePageData

		param.SkipPagination = true
		param.SkipSort = true

		if modelType.SinglePage { // overwrite the input if the model itself is single page
			param.SinglePageData = true
		}

		raw, err := driver.GetSingleRawDocumentFromProject(cache.Ctx, param)
		if err != nil {
			return nil, err
		}
		doc = raw.(*types.DefaultDocumentStructure)

		// got the doc but doc doesn't belong to specific model
		if doc.Type != modelName {
			return nil, fmt.Errorf("document does not belongs to %s", modelName)
		}

		if len(modelType.Connections) > 0 {
			if disconnects, ok := p.Args["disconnect"].(map[string]interface{}); ok && len(disconnects) > 0 {
				cdp, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, disconnects, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = cdp
				err = driver.DisconnectBuilder(cache.Ctx, param)
				if err != nil {
					return nil, err
				}
			}

			if connectionIds, ok := p.Args["connect"].(map[string]interface{}); ok && len(connectionIds) > 0 {
				cdp, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, connectionIds, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = cdp
				err = driver.ConnectBuilder(cache.Ctx, param)
				if err != nil {
					return nil, err
				}
			}
		}

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			// local support
			local := "en"
			if val, ok := input["local"].(string); ok {
				local = val
			}

			//#todo need image param validation
			modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(cache.Ctx, param, local, modelType.Fields, inputPayload, doc.Data)
			if err != nil {
				return nil, err
			}
			doc.Data = modifiedPayload

			if param.TenantID != "" {
				doc.TenantID = types.ID(param.TenantID)
				doc.TenantModel = project.TenantModelName
			}

			// replacing the doc might case the local field to disappear. don't replace the old doc
			// fixed it later !!
			err = driver.UpdateDocumentOfProject(cache.Ctx, param, doc, forceUpdate)
			if err != nil {
				return nil, err
			}
		}

	} else {

		//#todo replace these operation with transaction

		id := uuid.New()
		uid := id.String()

		doc = &types.DefaultDocumentStructure{
			ID:   uid,
			Key:  uid,
			Type: modelName,
			Meta: &types.MetaField{
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
				CreatedBy: &types.SystemUser{
					ID: param.UserID,
				},
				LastModifiedBy: &types.SystemUser{
					ID: param.UserID,
				},
				Status: param.DocPublishStatus,
			},
		}

		local := "en"
		var inputPayload map[string]interface{}

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			input := param.ResolveParams.Args
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}
			// local support
			if val, ok := input["local"].(string); ok {
				local = val
			}
		}

		//#todo need image param validation
		modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(cache.Ctx, param, local, modelType.Fields, inputPayload, make(map[string]interface{}))
		if err != nil {
			return nil, err
		}
		doc.Data = modifiedPayload

		if param.TenantID != "" {
			doc.TenantID = types.ID(param.TenantID)
			doc.TenantModel = project.TenantModelName
		}

		//_, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(p.Context, param.ProjectId, modelName, doc)
		_, err = driver.AddDocumentToProject(cache.Ctx, param, doc)
		if err != nil {
			return nil, err
		}

		// for new document also check for connect disconnect
		if len(modelType.Connections) > 0 {
			if connections, ok := p.Args["connect"].(map[string]interface{}); ok && len(connections) > 0 {
				v, err := utility.ConnectDisconnectParamBuilder(cache.Project, uid, connections, modelType)
				if err != nil {
					// if relation error at first then remove the document
					param.DocumentID = doc.ID
					err = driver.DeleteDocumentFromProject(cache.Ctx, param)
					return nil, err
				}
				param.ConDisParam = v
				err = driver.ConnectBuilder(cache.Ctx, param)
				if err != nil {
					// if relation error at first then remove the document
					param.DocumentID = doc.ID
					err = driver.DeleteDocumentFromProject(cache.Ctx, param)
					return nil, err
				}
			}
		}
	}

	/* // add the meta
	docWithMeta, err := s.SystemDriver.AddSystemUserMetaInfo(p.Context, doc)
	if err != nil {
		return nil, err
	} */

	return doc, nil
}

func (s *GraphQLServer) DuplicateModelDataFnFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DuplicateModelDataFnFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)
	param.ResolveParams = &p

	tenantID := router.Get("tenant")

	switch param.Role.ID {
	case "tenant":
		if tenantID == nil {
			return nil, errors.New("unable to Identify the User")
		}
		param.TenantID = tenantID.(string)
		break
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		modelName = strings.TrimSpace(utility.SingularResourceName(val))
	} else {
		return nil, errors.New("model Name is Necessary")
	}

	var modelType *models.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New("model Not Found")
	}

	var docId string
	if val, ok := p.Args["_id"]; ok {
		docId = val.(string)
		param.DocumentID = docId
		param.ResolveParams = &p
		param.Model = modelType
		param.ProjectType = project.ProjectType

		driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
		if err != nil {
			return nil, err
		}

		exists, err := driver.GetSingleProjectDocument(cache.Ctx, param)
		if err != nil {
			return nil, err
		}

		if exists != nil {
			id := uuid.New()
			exists.Key = id.String()
			exists.ID = exists.Key
			exists.Meta.CreatedAt = utility.GetCurrentTime()
			exists.Meta.UpdatedAt = utility.GetCurrentTime()

			//_, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(p.Context, param.ProjectId, modelName, doc)
			_, err = driver.AddDocumentToProject(cache.Ctx, param, exists)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("doc not found to duplicate")
		}
	} else {
		return nil, errors.New("_id is required for delete")
	}

	return map[string]interface{}{
		"id": docId,
	}, nil
}

func (s *GraphQLServer) DeleteModelDataFnFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteModelDataFnFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		modelName = strings.TrimSpace(utility.SingularResourceName(val))
	} else {
		return nil, errors.New("model Name is Necessary")
	}

	var modelType *models.ModelType
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			modelType = ct
			break
		}
	}

	if modelType == nil {
		return nil, errors.New("model Not Found")
	}

	param := s.NewParam(cache.Param)

	var docId string
	if val, ok := p.Args["_id"]; ok {
		docId = val.(string)
		param.DocumentID = docId
		param.Model = modelType

		driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
		if err != nil {
			return nil, err
		}

		param.DocPublishStatus = "all"

		exists, err := driver.GetSingleProjectDocument(cache.Ctx, param)
		if err != nil {
			return nil, err
		}

		if exists != nil {
			err = driver.DeleteDocumentFromProject(cache.Ctx, param)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("doc not found to delete")
		}

	} else {
		return nil, errors.New("_id is required for delete")
	}

	return map[string]interface{}{
		"id": docId,
	}, nil
}

/*func (s *GraphQLServer) UpdateAWSLambdaFunctions(apitoFunc *models.CloudFunction) (*models.FunctionProviderConfig, error) {

	/*var cred *models.ThirdPartyCredential
	if val, ok := s.PluginConfigurations["aws"]; ok {
		cred = val.Credentials
	} else {
		return nil, errors.New("AWS Credentials are not Set")
	}

	if cred != nil {

		sess, err := session.NewSession(&aws.Config{
			Region:      aws.String(apitoFunc.ProviderConfig.Region),
			Credentials: credentials.NewStaticCredentials(cred.AccessKey, cred.SecretKey, ""),
		})

		svc := lambda.New(sess)
		input := &lambda.UpdateFunctionConfigurationInput{
			FunctionName: aws.String(apitoFunc.ProviderConfig.RemoteFunctionName),
		}

		if apitoFunc.ProviderConfig.Configs != nil {
			input.MemorySize = aws.Int64(apitoFunc.ProviderConfig.Configs.Memory)
			input.Handler = aws.String(apitoFunc.ProviderConfig.Configs.Handler)
			input.Runtime = aws.String(apitoFunc.ProviderConfig.Configs.Runtime)
			input.Timeout = aws.Int64(apitoFunc.ProviderConfig.Configs.TimeOut)
		}

		if apitoFunc.ProviderConfig.EnvVars != nil {
			var envs = make(map[string]*string)
			for _, v := range apitoFunc.ProviderConfig.EnvVars {
				envs[v.Key] = &v.Value
			}
			input.Environment = &lambda.Environment{Variables: envs}
		}

		_, err = svc.UpdateFunctionConfiguration(input)
		if err != nil {
			return nil, err
		}
	}

	return apitoFunc.ProviderConfig, nil
}
*/
