package resolver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/schemas/enums"
	"github.com/apito-io/engine/services"
	pluginService "github.com/apito-io/engine/services/plugin"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
	projdriver "gitlab.com/apito.io/open_driver/project"
)

func (s *GraphQLServer) GenerateProjectTokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapProjectsWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("GenerateApiTokenResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
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
		return nil, errors.New("role is Required")
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	if cache.Project == nil {
		return nil, errors.New("project is required for project token generation")
	}

	project := cache.Project

	if len(role) > 64 {
		return nil, errors.New("role name exceeds maximum length of 64 characters")
	}
	roleKey := strings.ToLower(utility.SingularResourceName(role))
	if roleKey == "admin" || roleKey == "owner" {
		return nil, errors.New("cannot mint project tokens for admin or owner roles")
	}
	roleDef, ok := project.Roles[roleKey]
	if !ok {
		roleDef, ok = project.Roles[role]
		if ok {
			roleKey = role
		}
	}
	if !ok || roleDef == nil {
		return nil, errors.New("role does not exist on this project")
	}
	if roleDef.IsAdmin {
		return nil, errors.New("cannot mint project tokens for admin roles")
	}
	role = roleKey

	// Parse the date string and set it to end of day
	parseDuration, err := time.Parse("2006-01-02", duration)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format: %v", err)
	}
	// Set to end of day (23:59:59) in UTC
	parseDuration = time.Date(parseDuration.Year(), parseDuration.Month(), parseDuration.Day(), 23, 59, 59, 0, time.UTC)

	// generate the token
	tokenClaims := &models.TokenClaims{
		Role:      role,
		UserID:    param.UserID,
		ProjectID: project.ID,
		ExpireAt:  parseDuration.Unix(),
	}
	if s.Cfg != nil && s.Cfg.ProjectAPITokenClaimsHook != nil {
		s.Cfg.ProjectAPITokenClaimsHook(router, project, tokenClaims)
	}
	apiKey, err := s.ProjectKeyManager.GenerateKey(tokenClaims)
	if err != nil {
		return nil, err
	}

	// Persist metadata only — never store the full secret again.
	project.Tokens = append(project.Tokens, &models.ProjectToken{
		Name:             name,
		Token:            "",
		TokenID:          tokenClaims.TokenUniqueID,
		TokenPrefix:      services.TokenPrefix(apiKey),
		TokenFingerprint: services.TokenFingerprint(apiKey),
		Role:             role,
		Expire:           duration,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	})

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, false)
	if err != nil {
		return nil, err
	}

	// invalidate the project cache
	err = s.ExpireGraphQLProjectCache(cache.Ctx, project.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"token": apiKey,
	}, nil
}

func (s *GraphQLServer) DeleteProjectTokenResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapProjectsWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("DeleteApiTokenResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	var tokenID string
	if val, ok := p.Args["token_id"].(string); ok {
		tokenID = strings.TrimSpace(val)
	}
	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = strings.TrimSpace(val)
	}
	if tokenID == "" && token == "" {
		return nil, errors.New("token_id or token is required")
	}

	var duration string
	if val, ok := p.Args["duration"].(string); ok {
		duration = val
	} else {
		return nil, errors.New("duration is Required")
	}

	project := cache.Project
	blacklistID := tokenID

	if tokenID != "" {
		// Preferred revoke path: match persisted TokenID metadata (no secret required).
		found := false
		remaining := make([]*models.ProjectToken, 0, len(project.Tokens))
		for _, t := range project.Tokens {
			if t != nil && t.TokenID == tokenID {
				found = true
				continue
			}
			remaining = append(remaining, t)
		}
		if !found {
			return nil, errors.New("token not found")
		}
		project.Tokens = remaining
	} else {
		// Legacy path: revoke by full secret (still supported for older clients).
		var verifiedToken *models.TokenClaims
		if strings.HasPrefix(token, "ak_") {
			verifiedToken, err = s.ProjectKeyManager.Validate(cache.Ctx, token, false)
			if err != nil {
				if err.Error() == "This token is blacklisted" || err.Error() == "key has expired" {
					// do nothing — still remove from project inventory
				} else {
					return nil, err
				}
			}
		} else {
			verifiedToken, err = s.BlankaTokenService.Validate(cache.Ctx, token)
			if err != nil {
				return nil, ae.InvalidToken
			}
		}

		if verifiedToken != nil {
			if !param.Role.IsAdmin {
				if verifiedToken.UserID != param.UserID {
					return nil, errors.New("its none of your business, Pal ")
				}
			}
			blacklistID = verifiedToken.TokenUniqueID
		}

		fp := services.TokenFingerprint(token)
		remaining := make([]*models.ProjectToken, 0, len(project.Tokens))
		for _, t := range project.Tokens {
			if t == nil {
				continue
			}
			match := t.Token == token ||
				(blacklistID != "" && t.TokenID == blacklistID) ||
				(t.TokenFingerprint != "" && t.TokenFingerprint == fp)
			if match {
				if blacklistID == "" && t.TokenID != "" {
					blacklistID = t.TokenID
				}
				continue
			}
			remaining = append(remaining, t)
		}
		project.Tokens = remaining
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	if blacklistID != "" {
		parseDuration, _ := time.Parse(time.RFC3339, duration)
		alreadyExpired := time.Until(parseDuration).Hours()
		if alreadyExpired > 0.0 { // expire the token
			expiredToken := map[string]interface{}{
				"id":        blacklistID,
				"_key":      blacklistID,
				"expire_at": duration,
			}

			err = s.SystemDriver.BlacklistAToken(cache.Ctx, expiredToken)
			if err != nil {
				return nil, err
			}
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
		var cErr error
		model, cErr = utility.CanonicalizeModelName(val)
		if cErr != nil {
			return nil, cErr
		}
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

	uid := utility.NewID()
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
		return nil, errors.New("either URL OR Trigger Functions are Required")
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
	if err := requireAccessCapability(router, CapProjectsWrite); err != nil {
		return nil, err
	}

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

	if _, ok := p.Args["project_icon"]; ok {
		if val, ok := p.Args["project_icon"].(string); ok {
			project.ProjectIcon = strings.TrimSpace(val)
		} else {
			project.ProjectIcon = ""
		}
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
			db, err = projdriver.GetProjectSQLDriver(s.Cfg, &models.DriverCredentials{
				Host:     host,
				Port:     port,
				Database: database,
				User:     user,
				Password: password,
			})
		default:
			project.Driver = &models.DriverCredentials{Engine: _const.CoreDB}
		}

		if db == nil {
			return nil, errors.New("db configuration is not correct")
		}

		if err != nil {
			return nil, err
		}
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

		if val, ok := settings["idle_tenant_retention_days"]; ok && val != nil {
			days := 0
			switch n := val.(type) {
			case int:
				days = n
			case int32:
				days = int(n)
			case int64:
				days = int(n)
			case float64:
				days = int(n)
			default:
				return nil, errors.New("idle_tenant_retention_days must be an integer")
			}
			if days < models.MinIdleTenantRetentionDays {
				return nil, fmt.Errorf("idle_tenant_retention_days must be at least %d", models.MinIdleTenantRetentionDays)
			}
			project.Settings.IdleTenantRetentionDays = days
		}

		if val, ok := settings["auto_soft_delete_idle_tenants"].(bool); ok {
			project.Settings.AutoSoftDeleteIdleTenants = val
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
		}
		req.Role = models.NormalizeMembershipRole(req.Role, false)

		if vals, ok := val["administrative_permissions"].([]interface{}); ok {
			var permissions []string
			for _, v := range vals {
				permissions = append(permissions, v.(string))
			}
			req.Permissions = models.MembershipPermissions(permissions, false)
		}

		user, created, err := s.resolveOrCreateSystemUserByEmail(cache.Ctx, req.Email, projectId)
		if err != nil {
			return nil, err
		}

		req.UserID = user.ID

		adminCheck, err := s.SystemDriver.CheckProjectWithRoles(cache.Ctx, param.UserID, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if err := requireWorkspaceMemberAdmin(adminCheck); err != nil {
			return nil, err
		}

		rawToken, tokenHash, err := models.NewInviteToken()
		if err != nil {
			return nil, err
		}
		existing, _ := s.lookupUserProjectGrant(cache.Ctx, user.ID, req.ProjectID)
		models.StampInviteOnRequest(&req, existing, tokenHash, time.Now().UTC(), s.inviteTTL())

		err = s.SystemDriver.AddATeamMemberToProject(cache.Ctx, &req)
		if err != nil {
			return nil, err
		}

		tempPass := ""
		if created {
			tempPass = user.TempPassword
		}
		inviteNames := []string{}
		if project != nil && strings.TrimSpace(project.Name) != "" {
			inviteNames = []string{project.Name}
		}
		s.sendTeamInviteEmail(user, inviteNames, tempPass, models.AcceptInviteURL(s.corsOrigin(), rawToken))
	}

	if val, ok := p.Args["remove_team_member"].(map[string]interface{}); ok {
		var memberId string
		if val, ok := val["member_id"].(string); ok {
			memberId = val
		} else {
			return nil, errors.New("member ID is Required")
		}
		adminCheck, err := s.SystemDriver.CheckProjectWithRoles(cache.Ctx, param.UserID, projectId)
		if err != nil {
			return nil, err
		}
		if err := requireWorkspaceMemberAdmin(adminCheck); err != nil {
			return nil, err
		}
		err = s.SystemDriver.RemoveATeamMemberFromProject(cache.Ctx, projectId, memberId)
		if err != nil {
			return nil, err
		}
	}

	/* 	if plugins, ok := p.Args["plugins"].(map[string]interface{}); ok {
		if project.Plugins == nil {
			project.Plugins = []*models.SavedPluginDetails{}
		}

		switch plugins["name"] {
		case "aws":
			details := &models.SavedPluginDetails{
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

			/* validate the creds first
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

			fmt.Printf("%s - %s\n", arn, *result.Policy.Description)

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
	} */

	if project.ProjectSecretKey == "" {
		project.ProjectSecretKey = utility.RandomStringGenerator(25)
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	s.EmitProjectLifecycle(cache.Ctx, param.UserID, project.ID, project.Name, models.SystemEventProjectUpdated)

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

func (s *GraphQLServer) RemoveProjectSpecificPluginResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapPluginsWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("DeletePluginResolverFn", router)

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

	for i, plugin := range project.Plugins {
		if plugin.ID == id {
			project.Plugins = append(project.Plugins[:i], project.Plugins[i+1:]...)
			break
		}
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, &project, true)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"message": "Plugin deleted successfully",
	}, nil
}

func (s *GraphQLServer) UpsertPluginResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapPluginsWrite); err != nil {
		return nil, err
	}

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

	var _pluginDetails *models.SavedPluginDetails

	// First check if plugin already exists in project
	for _, plugin := range project.Plugins {
		if plugin.ID == id {
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
			_pluginDetails = &models.SavedPluginDetails{
				ProjectID:      project.ID,
				ID:             id,
				EnvVars:        plugin.EnvVars,
				ActivateStatus: plugin.ActivateStatus,
				LoadStatus:     plugin.LoadStatus,
				Enable:         plugin.Enable,
			}
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
			if plugin.ID == id {
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
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("AddModelToProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["name"].(string); ok {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
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

	var singleRecord bool
	if val, ok := p.Args["single_record"].(bool); ok {
		singleRecord = val
	}

	model := &models.ModelType{
		Name:       modelName,
		SinglePage: singleRecord,
	}
	if s.Cfg != nil && s.Cfg.PreCreateModelHook != nil {
		s.Cfg.PreCreateModelHook(model, p.Args)
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeCreateModel, graphqlArgsMap(p), func() (interface{}, error) {
		param := s.NewParam(cache.Param)
		param.Model = model
		if cache.Project.Driver != nil {
			dbKind := strings.ToLower(strings.TrimSpace(cache.Project.Driver.Database))
			if dbKind == "sqlite" || dbKind == "mysql" || dbKind == "postgres" {
				param.ProjectID = cache.Project.Driver.Database
			}
		}

		driver, skipBaseDDL, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project)
		if err != nil {
			return nil, err
		}

		if !skipBaseDDL {
			checkCollectionExists, err := driver.CheckTableOrCollectionExists(cache.Ctx, param)
			if err != nil {
				return nil, err
			}
			if checkCollectionExists {
				return nil, errors.New("collection/table already exists")
			}
		}
		err = s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx:           cache.Ctx,
			Project:       project,
			OperationType: models.SchemaOpTypeCreateModel,
			Request:       map[string]interface{}{"name": modelName, "single_record": singleRecord},
			BaseDriver:    driver,
			SkipBaseDDL:   skipBaseDDL,
			ApplyDDL: func(d interfaces.ProjectDBInterface) error {
				schema, ddlErr := d.AddModel(cache.Ctx, project, model)
				if ddlErr != nil {
					return ddlErr
				}
				project.Schema = schema
				return nil
			},
			PersistSystem: func() error {
				return s.SystemDriver.UpdateProject(cache.Ctx, project, false)
			},
			RefreshCache: func() error {
				_, refreshErr := s.refreshProjectAndReCache(cache.Ctx, project.ID)
				return refreshErr
			},
			Compensate: func(d interfaces.ProjectDBInterface) error {
				return d.DropModel(cache.Ctx, project, modelName)
			},
		})
		if err != nil {
			return nil, err
		}

		fresh, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
		if err != nil {
			return nil, err
		}
		if fresh.Schema == nil {
			return []*models.ModelType{}, nil
		}
		return fresh.Schema.Models, nil
	})
}

func (s *GraphQLServer) RunModelMigrationsResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaPublish); err != nil {
		return nil, err
	}

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

		checkCollectionExists, err := driver.CheckTableOrCollectionExists(cache.Ctx, param)
		if err != nil {
			return nil, err
		}

		if !checkCollectionExists {
			// if schema not found then create
			err = s.invokeCreateTableOrCollection(cache.Ctx, driver, param, false)
			if err != nil {
				return nil, err
			}
		}
	}

	// check relation collection
	driver, err := s.getSchemaBaseProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	if param.Ext == nil {
		param.Ext = make(map[string]interface{})
	}
	param.Ext[models.ExtKeyRelationCollectionCheck] = true
	checkRelationCollectionExists, err := driver.CheckTableOrCollectionExists(cache.Ctx, param)
	delete(param.Ext, models.ExtKeyRelationCollectionCheck)
	if err != nil {
		return nil, err
	}

	if !checkRelationCollectionExists {
		err = s.invokeCreateTableOrCollection(cache.Ctx, driver, param, true)
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
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

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

	opType := models.SchemaOpTypeUpdateModel
	switch _type {
	case "duplicate":
		opType = models.SchemaOpTypeDuplicateModel
	case "rename":
		opType = models.SchemaOpTypeRenameModel
	case "convert":
		opType = models.SchemaOpTypeConvertModel
	case "delete":
		opType = models.SchemaOpTypeDeleteModel
	}
	if _type == "update" {
		if _, ok := p.Args["is_common_model"].(bool); ok {
			return s.updateModel(cache, project, modelName, p.Args)
		}
	}
	return s.tryStageSchemaMutation(cache, project, opType, graphqlArgsMap(p), func() (interface{}, error) {
		var resp interface{}
		switch _type {
		case "update":
			resp, err = s.updateModel(cache, project, modelName, p.Args)
			if err != nil {
				return nil, err
			}
			return resp, nil
		case "duplicate":
			var newName string
			if val, ok := p.Args["new_name"].(string); ok {
				newName = val
			} else {
				return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
			}
			resp, err = s.duplicateModel(cache.Ctx, project, newName, modelName)
			if err != nil {
				return nil, err
			}
			return resp, nil
		case "rename":
			var newName string
			if val, ok := p.Args["new_name"].(string); ok {
				newName = val
			} else {
				return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
			}
			resp, err = s.renameModel(cache, project, newName, modelName, singlePageModel)
			if err != nil {
				return nil, err
			}
			return resp, nil
		case "convert":
			resp, err = s.convertModel(cache, project, modelName)
			if err != nil {
				return nil, err
			}
			return resp, nil
		case "delete":
			resp, err := s.deleteModel(cache, project, modelName)
			if err != nil {
				return nil, err
			}
			return resp, nil
		default:
			return nil, errors.New("invalid update model request")
		}
	})
}

func (s *GraphQLServer) updateModel(cache *models.ApplicationCache, project *models.Project, modelName string, args map[string]interface{}) (*models.ModelType, error) {

	if modelName == "" {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var updatedModel *models.ModelType

	if project.Schema == nil {
		return nil, errors.New("please create a model first")
	}
	for _, ct := range project.Schema.Models {
		if ct.Name == modelName {
			updatedModel = ct
			break
		}
	}
	if updatedModel == nil {
		return nil, ae.ModelTypeNotFound
	}

	changed := false
	if s.Cfg != nil && s.Cfg.ApplyModelUpdateHook != nil {
		changed = s.Cfg.ApplyModelUpdateHook(updatedModel, args)
	}
	if !changed {
		return updatedModel, nil
	}

	err := s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
		Ctx:           cache.Ctx,
		Project:       project,
		OperationType: models.SchemaOpTypeUpdateModel,
		Request:       args,
		BaseDriver:    nil,
		SkipBaseDDL:   true,
		ApplyDDL: func(interfaces.ProjectDBInterface) error {
			return nil
		},
		PersistSystem: func() error {
			return s.SystemDriver.UpdateProject(cache.Ctx, project, false)
		},
		RefreshCache: func() error {
			_, refreshErr := s.refreshProjectAndReCache(cache.Ctx, project.ID)
			return refreshErr
		},
	})
	if err != nil {
		return nil, err
	}
	return updatedModel, nil
}

func (s *GraphQLServer) duplicateModel(ctx context.Context, project *models.Project, newName, modelName string) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	newModelName, err := utility.CanonicalizeModelName(newName)
	if err != nil {
		return nil, err
	}

	var duplicatedModel *models.ModelType

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
				duplicatedModel.SinglePage = true
				duplicatedModel.SinglePageUUID = utility.NewID()
			}
			project.Schema.Models = append(project.Schema.Models, duplicatedModel)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	return duplicatedModel, nil
}

func (s *GraphQLServer) renameModel(cache *models.ApplicationCache, project *models.Project, newName, modelName string, singlePageModel bool) (interface{}, error) {

	if newName == "" {
		return nil, errors.New(ae.NEW_MODEL_NAME_REQUIRED)
	}

	if newName == modelName {
		return nil, errors.New("new model name can not be the same as the old one")
	}

	newModelName, err := utility.CanonicalizeModelName(newName)
	if err != nil {
		return nil, err
	}

	driver, err := s.getSchemaBaseProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	checkCollectionExists, err := driver.CheckTableOrCollectionExists(cache.Ctx, cache.Param)
	if err != nil {
		return nil, err
	}

	if !checkCollectionExists {
		return nil, errors.New("collection/table does not exist")
	}

	var modelToRename *models.ModelType

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
		utility.RewriteProjectPermissionKeys(project, modelName, newModelName)

		// call rename model in database
		err = driver.RenameModel(cache.Ctx, project, modelName, newModelName)
		if err != nil {
			return nil, err
		}
	}

	return modelToRename, nil
}

func (s *GraphQLServer) convertModel(cache *models.ApplicationCache, project *models.Project, modelName string) (interface{}, error) {

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
			modelToConvert.SinglePage = true
			modelToConvert.SinglePageUUID = utility.NewID()
		}
	}

	return modelToConvert, nil
}

func (s *GraphQLServer) deleteModel(cache *models.ApplicationCache, project *models.Project, modelName string) (interface{}, error) {

	if modelName == "user" {
		return nil, errors.New("can not delete User Model. If you dont want it then remove User Addons")
	}
	if models.ModelNameIsReservedProjectAuthUser(modelName) {
		return nil, errors.New("can not delete the application users model")
	}

	driver, err := s.getSchemaBaseProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	var _model *models.ModelType

	if project.Schema == nil {
		return nil, errors.New("nothing to Delete")
	}

	var index int
	for i, ct := range project.Schema.Models {
		if ct.Name == modelName {
			_model = ct
			index = i
			break
		}
	}

	if _model == nil {
		return nil, errors.New("could not find model to delete")
	}

	if len(_model.Connections) > 0 {
		return nil, fmt.Errorf(
			"model %q still has %d outgoing schema connection(s); remove each with deleteConnectionFromModel (system GraphQL) before deleting the model",
			modelName, len(_model.Connections),
		)
	}

	for _, m := range project.Schema.Models {
		if m == nil || m.Name == modelName {
			continue
		}
		for _, c := range m.Connections {
			if c != nil && c.Model == modelName {
				return nil, fmt.Errorf(
					"model %q is still referenced from schema model %q (connection to %q); remove that relation with deleteConnectionFromModel before deleting the model",
					modelName, m.Name, modelName,
				)
			}
		}
	}

	if err := driver.DeleteDocumentsFromProject(cache.Ctx, &models.CommonSystemParams{ProjectID: project.ID, Model: _model}); err != nil {
		return nil, err
	}

	project.Schema.Models = append(project.Schema.Models[:index], project.Schema.Models[index+1:]...)

	if err := driver.DropModel(cache.Ctx, project, modelName); err != nil {
		return nil, err
	}

	// Persist schema change without full UpdateProject: SQL deletes model_types row;
	// document stores load project, splice model from embedded schema, then save.
	if err := s.SystemDriver.DeleteModelType(cache.Ctx, project.ID, modelName); err != nil {
		return nil, err
	}
	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}
	if err := s.ExpireGraphQLProjectCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}

	return _model, nil
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
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapFunctionsWrite); err != nil {
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
				rt, _ := v.(string)
				if !models.IsValidFunctionRuntime(rt) {
					return nil, fmt.Errorf("unsupported function runtime %q (use deno, wasm, or hashicorp)", rt)
				}
				function.RuntimeConfig.Runtime = rt
			case "memory":
				switch n := v.(type) {
				case int:
					function.RuntimeConfig.Memory = int64(n)
				case int64:
					function.RuntimeConfig.Memory = n
				case float64:
					function.RuntimeConfig.Memory = int64(n)
				}
			case "handler":
				function.RuntimeConfig.Handler, _ = v.(string)
			case "time_out":
				switch n := v.(type) {
				case int:
					function.RuntimeConfig.TimeOut = int64(n)
				case int64:
					function.RuntimeConfig.TimeOut = n
				case float64:
					function.RuntimeConfig.TimeOut = int64(n)
				}
			}
		}
	}

	if val, ok := p.Args["language"].(string); ok && val != "" {
		function.Language = val
	}
	if val, ok := p.Args["binary_url"].(string); ok && val != "" {
		function.BinaryURL = val
	}
	if val, ok := p.Args["source"].(string); ok {
		function.Source = val
	}
	if val, ok := p.Args["trigger_type"].(string); ok && val != "" {
		function.TriggerType = val
	}
	if vals, ok := p.Args["capabilities"].([]interface{}); ok {
		var caps []string
		for _, c := range vals {
			if s, ok := c.(string); ok && s != "" {
				caps = append(caps, s)
			}
		}
		function.Capabilities = caps
	}

	function.ProjectID = project.ID
	if function.ID == "" {
		function.ID = utility.NewID()
	}
	if function.TriggerType == "" {
		function.TriggerType = models.FunctionTriggerCallable
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

	// Invalidate project GraphQL cache so new/updated functions appear in the public schema.
	_ = s.ExpireGraphQLProjectCache(cache.Ctx, project.ID)

	return function, nil
}

func (s *GraphQLServer) UpsertRoleToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapRolesWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("UpsertRoleToProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
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
	existing := false
	// if schema not found then create
	if r, ok := project.Roles[roleName]; ok {
		role = r
		existing = true
	} else {
		role = &models.Role{}
		project.Roles[roleName] = role
	}

	if existing && role.SystemGenerated {
		return nil, errors.New("cannot edit system generated roles")
	}

	if err := applyRoleUpsertIsAdmin(role, p.Args); err != nil {
		return nil, err
	}

	if _, ok := p.Args["logic_executions"]; ok {
		role.LogicExecutions = nil
		if logicExecutions, ok := p.Args["logic_executions"].([]interface{}); ok {
			role.LogicExecutions = make([]string, 0, len(logicExecutions))
			for _, l := range logicExecutions {
				if s, ok := l.(string); ok {
					role.LogicExecutions = append(role.LogicExecutions, s)
				}
			}
		}
	}

	if !role.IsAdmin {
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

	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *GraphQLServer) DuplicateRoleInProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapRolesWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("DuplicateRoleInProjectResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}

	project := cache.Project

	sourceRole, ok := p.Args["source_role"].(string)
	if !ok || strings.TrimSpace(sourceRole) == "" {
		return nil, errors.New("source role is required")
	}
	newName, ok := p.Args["new_name"].(string)
	if !ok || strings.TrimSpace(newName) == "" {
		return nil, errors.New("new role name is required")
	}

	sourceKey := strings.ToLower(utility.SingularResourceName(sourceRole))
	newKey := strings.ToLower(utility.SingularResourceName(newName))

	switch newKey {
	case "admin":
		return nil, errors.New("a default Role with name `Admin` already exists in your project. Choose other names")
	case "demo":
		return nil, errors.New("can not create Role named `Demo`. Its being used internally. Choose other names")
	}

	if project.Roles == nil {
		return nil, errors.New("source role not found")
	}
	src, ok := project.Roles[sourceKey]
	if !ok || src == nil {
		return nil, errors.New("source role not found")
	}
	if _, exists := project.Roles[newKey]; exists {
		return nil, errors.New("a role with this name already exists")
	}

	copy := utility.CloneRole(src)
	project.Roles[newKey] = copy

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}

	return copy, nil
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
	if err := requireAccessCapability(router, CapFunctionsDelete); err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
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

	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}

	return project.Schema.Functions, nil
}

func (s *GraphQLServer) DeleteRoleResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapRolesWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("DeleteRoleResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireProjectAdmin(cache); err != nil {
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
		svc, err := s.ProjectUserService(cache, cache.Ctx)
		if err != nil {
			return nil, err
		}
		counts, err := svc.CountUsersByRole()
		if err != nil {
			return nil, err
		}
		if counts[roleToDelete] > 0 {
			return nil, errors.New("there are app users assigned to this role. Reassign or remove those users first")
		}
		delete(project.Roles, roleToDelete)
	} else {
		return nil, errors.New("role not Found")
	}

	err = s.SystemDriver.UpdateProject(cache.Ctx, project, true)
	if err != nil {
		return nil, err
	}

	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		return nil, err
	}

	return project.Roles, nil
}

func (s *GraphQLServer) UpsertFieldToModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("UpsertFieldToModelResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			// Draft-only models may already be stored as long single-segment
			// ids (indication, practitioner). Accept those for field ops.
			modelName, err = utility.LegacyStoredNameToCanonical(val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var identifier string
	var label string
	if val, ok := p.Args["field_label"].(string); ok && val != "" {
		validIdentifier, err := utility.IsValidIdentifier(val)
		if err != nil {
			return nil, err
		}
		identifier = validIdentifier.Identifier
		label = validIdentifier.Label
	} else {
		return nil, errors.New("field Label Is necessary")
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
		if s.schemaVersioningActive() || s.schemaMutationHook() != nil {
			args := graphqlArgsMap(p)
			args["model_name"] = modelName
			args["field"] = identifier
			args["field_label"] = label
			return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeAddField, args, func() (interface{}, error) {
				return nil, errors.New(ae.MODEL_NAME_REQUIRED)
			})
		}
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
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
	fieldInfo, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
		Identifier:  identifier,
		ParentField: parentField,
	}, nil, nil)
	if err != nil {
		if parentField != "" && s.schemaVersioningActive() && errors.Is(err, ae.ErrParentFieldNotFound) {
			err = nil
			fieldInfo = nil
		} else {
			return nil, err
		}
	}

	if !isUpdate && fieldInfo != nil {
		return nil, fmt.Errorf("a field with identifier '%s' already exits", identifier)
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

	if val, ok := p.Args["field_sub_type"].(string); ok && val != "" {
		fieldInfo.FieldSubType = val
	}

	// validate field & input type combination and other validation
	switch fieldInfo.FieldType {
	case "geo":
		if fieldInfo.InputType != "geo" {
			return nil, errors.New("input Type must be Geo if Field Type is Geo")
		}
	case "repeated":
		fieldInfo.SubFieldInfo = []*models.FieldInfo{
			{
				Identifier:   "_id",
				Description:  "An auto-generated ULID unique identifier",
				InputType:    "string",
				FieldType:    "text",
				SubFieldInfo: nil,
				Validation: &models.Validation{
					Hide:   true,
					Unique: true,
				},
				Serial:          1,
				Label:           "ID",
				SystemGenerated: true,
				//RepeatedGroupIdentifier: fieldInfo.Identifier,
				ParentField: fieldInfo.Identifier,
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
		fieldInfo.ParentField = parentField
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.FieldInfo = fieldInfo

	driver, skipBaseDDL, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project)
	if err != nil {
		return nil, err
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeAddField, graphqlArgsMap(p), func() (interface{}, error) {
		err = s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx:           cache.Ctx,
			Project:       project,
			OperationType: models.SchemaOpTypeAddField,
			Request: map[string]interface{}{
				"model_name": modelName, "field": identifier, "is_update": isUpdate,
			},
			BaseDriver:  driver,
			SkipBaseDDL: skipBaseDDL,
			ApplyDDL: func(d interfaces.ProjectDBInterface) error {
				updated, ddlErr := d.AddFieldToModel(cache.Ctx, param, isUpdate, parentField)
				if ddlErr != nil {
					return ddlErr
				}
				modelType = updated
				return nil
			},
			PersistSystem: func() error {
				return s.SystemDriver.UpdateProject(cache.Ctx, project, true)
			},
			RefreshCache: func() error {
				if err := s.ExpireGraphQLFieldCache(cache.Ctx, project.ID, modelType.Name); err != nil {
					return err
				}
				_, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
				return err
			},
			Compensate: func(d interfaces.ProjectDBInterface) error {
				if isUpdate {
					return nil
				}
				return d.DropField(cache.Ctx, param)
			},
		})
		if err != nil {
			return nil, err
		}
		return fieldInfo, nil
	})
}

type InputSerialPayload struct {
	Field     string               `json:"field"`
	Serial    int                  `json:"serial"`
	FieldType string               `json:"field_type"`
	Children  []InputSerialPayload `json:"children"`
}

func (s *GraphQLServer) RearrangeFieldOfModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("model Name is Necessary")
	}

	project := cache.Project

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

	// Extract field rearrangement parameters (schema arg is field_name; dragged_field kept for legacy clients)
	var draggedFieldName string
	if val, ok := p.Args["field_name"].(string); ok && val != "" {
		draggedFieldName = val
	} else if val, ok := p.Args["dragged_field"].(string); ok && val != "" {
		draggedFieldName = val
	} else {
		return nil, errors.New("field_name is required")
	}

	var parentId string
	if val, ok := p.Args["parent_id"].(string); ok && val != "" {
		parentId = val
	}

	var moveType string
	if val, ok := p.Args["move_type"].(string); ok && val != "" {
		moveType = val
	} else {
		return nil, errors.New("move_type is required")
	}

	var newPosition int
	if val, ok := p.Args["new_position"].(int); ok {
		newPosition = val
	}

	var draggedField *models.FieldInfo
	var currentPosition int
	var targetFields *[]*models.FieldInfo

	// Find the dragged field
	if parentId != "" {
		// Nested field - find parent and then the field
		sourceParent, _ := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier: parentId,
		}, nil, nil)
		if sourceParent == nil {
			return nil, errors.New("parent field not found")
		}

		for i, field := range sourceParent.SubFieldInfo {
			if field.Identifier == draggedFieldName {
				draggedField = field
				currentPosition = i
				targetFields = &sourceParent.SubFieldInfo
				break
			}
		}
	} else {
		// Root level field
		for i, field := range modelType.Fields {
			if field.Identifier == draggedFieldName {
				draggedField = field
				currentPosition = i
				targetFields = &modelType.Fields
				break
			}
		}
	}

	if draggedField == nil {
		return nil, errors.New("dragged field not found")
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeRearrangeField, graphqlArgsMap(p), func() (interface{}, error) {
		switch moveType {
		case "reorder":
			if currentPosition == newPosition {
				return modelType, nil
			}
			*targetFields = s.moveFieldInSlice(*targetFields, currentPosition, newPosition)
		case "child_to_parent":
			if parentId == "" {
				return nil, errors.New("cannot move to root when parent_id is empty")
			}
			sourceParent, _ := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
				Identifier: parentId,
			}, nil, nil)
			if sourceParent == nil {
				return nil, errors.New("source parent field not found")
			}
			for i, field := range sourceParent.SubFieldInfo {
				if field.Identifier == draggedFieldName {
					sourceParent.SubFieldInfo = append(sourceParent.SubFieldInfo[:i], sourceParent.SubFieldInfo[i+1:]...)
					break
				}
			}
			modelType.Fields = s.insertFieldAtPosition(modelType.Fields, draggedField, newPosition)
			draggedField.ParentField = ""
		case "parent_to_child":
			if parentId == "" {
				return nil, errors.New("parent_id is required for parent_to_child move")
			}
			for i, field := range modelType.Fields {
				if field.Identifier == draggedFieldName {
					modelType.Fields = append(modelType.Fields[:i], modelType.Fields[i+1:]...)
					break
				}
			}
			targetParent, _ := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
				Identifier: parentId,
			}, nil, nil)
			if targetParent == nil {
				return nil, errors.New("target parent field not found")
			}
			targetParent.SubFieldInfo = s.insertFieldAtPosition(targetParent.SubFieldInfo, draggedField, newPosition)
			draggedField.ParentField = parentId
		default:
			return nil, errors.New("invalid move type. Use 'reorder', 'child_to_parent', or 'parent_to_child'")
		}

		s.updateFieldSerials(modelType.Fields, 1)
		if err := s.SystemDriver.UpdateProject(cache.Ctx, project, true); err != nil {
			return nil, err
		}
		return modelType, nil
	})
}

func (s *GraphQLServer) DeleteModelDataFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	s.injectMetaData("DeleteModelDataFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
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
	if val, ok := p.Args["_id"]; ok && val != nil {
		if id, ok := val.(string); ok && id != "" {
			docId = id
		}
	}
	if docId == "" {
		return nil, errors.New("_id is required for delete")
	}

	param.DocumentID = docId
	param.Model = modelType

	driver, err := s.getSchemaBaseProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	param.DocPublishStatus = "all"
	param.ResolveParams = &p

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

	return map[string]interface{}{
		"id": docId,
	}, nil
}

// insertFieldAtPosition inserts a field at the specified position
func (s *GraphQLServer) insertFieldAtPosition(fields []*models.FieldInfo, field *models.FieldInfo, position int) []*models.FieldInfo {
	if position < 0 {
		position = 0
	}
	if position > len(fields) {
		position = len(fields)
	}

	// Insert at position
	fields = append(fields, nil)
	copy(fields[position+1:], fields[position:])
	fields[position] = field

	return fields
}

// moveFieldInSlice moves a field from one position to another within the same slice
func (s *GraphQLServer) moveFieldInSlice(fields []*models.FieldInfo, fromPos, toPos int) []*models.FieldInfo {
	if fromPos < 0 || fromPos >= len(fields) || toPos < 0 || toPos >= len(fields) {
		return fields
	}

	// Get the field to move
	fieldToMove := fields[fromPos]

	// Remove from current position
	fields = append(fields[:fromPos], fields[fromPos+1:]...)

	// Insert at new position
	fields = append(fields, nil)
	copy(fields[toPos+1:], fields[toPos:])
	fields[toPos] = fieldToMove

	return fields
}

// updateFieldSerials updates serial numbers for fields recursively
func (s *GraphQLServer) updateFieldSerials(fields []*models.FieldInfo, startSerial uint32) {
	for i, field := range fields {
		field.Serial = startSerial + uint32(i)

		// Update subfields recursively
		if len(field.SubFieldInfo) > 0 {
			s.updateFieldSerials(field.SubFieldInfo, 1)
		}
	}
}

func (s *GraphQLServer) ModelFieldOperationResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("ModelFieldOperationResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
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

	driver, skipBaseDDL, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project)
	if err != nil {
		return nil, err
	}

	var affectedField *models.FieldInfo
	var applyDDL func(interfaces.ProjectDBInterface) error
	var compensate func(interfaces.ProjectDBInterface) error
	var beforePersist func() error

	switch _type {
	case enums.FieldOperation_Rename:
		var newField *models.ValidIdentifier
		if val, ok := p.Args["new_name"].(string); ok {
			newField, err = utility.IsValidIdentifier(val)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// now search if the renamed field already exists in the model
		_fieldInfo, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  newField.Identifier,
			ParentField: parentField,
		}, nil, nil)
		if err != nil {
			return nil, err
		}

		if _fieldInfo != nil {
			return nil, errors.New("field you are trying to rename to already exists. choose a different name")
		}

		fieldInfo, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  fieldName,
			ParentField: parentField,
		}, nil, nil)
		if err != nil {
			return nil, err
		}
		if fieldInfo == nil {
			if s.schemaVersioningActive() {
				applyDDL = func(interfaces.ProjectDBInterface) error { return nil }
				beforePersist = func() error { return nil }
				affectedField = &models.FieldInfo{
					Identifier:  newField.Identifier,
					Label:       newField.Label,
					ParentField: parentField,
				}
				break
			}
			return nil, errors.New("field to rename not found")
		}

		param := s.NewParam(cache.Param)
		param.Model = modelType
		param.FieldInfo = &models.FieldInfo{
			Identifier:      newField.Identifier,
			Label:           newField.Label,
			Description:     fieldInfo.Description,
			InputType:       fieldInfo.InputType,
			FieldType:       fieldInfo.FieldType,
			Validation:      fieldInfo.Validation,
			Serial:          fieldInfo.Serial,
			ParentField:     fieldInfo.ParentField,
			SystemGenerated: fieldInfo.SystemGenerated,
			SubFieldInfo:    fieldInfo.SubFieldInfo,
		}
		param.SinglePageData = singlePageModel
		oldName := fieldName

		if fieldName != newField.Identifier {
			applyDDL = func(d interfaces.ProjectDBInterface) error {
				return d.RenameField(cache.Ctx, oldName, parentField, param)
			}
			compensate = func(d interfaces.ProjectDBInterface) error {
				back := &models.FieldInfo{
					Identifier:  oldName,
					Label:       fieldInfo.Label,
					Description: fieldInfo.Description,
					InputType:   fieldInfo.InputType,
					FieldType:   fieldInfo.FieldType,
					Validation:  fieldInfo.Validation,
					Serial:      fieldInfo.Serial,
					ParentField: fieldInfo.ParentField,
				}
				rev := s.NewParam(cache.Param)
				rev.Model = modelType
				rev.FieldInfo = back
				rev.SinglePageData = singlePageModel
				return d.RenameField(cache.Ctx, newField.Identifier, parentField, rev)
			}
		}
		beforePersist = func() error {
			affectedField, err = s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
				Identifier:  fieldName,
				ParentField: parentField,
			}, newField, &_type)
			return err
		}
	case enums.FieldOperation_Duplicate:
		var newField *models.ValidIdentifier
		if val, ok := p.Args["new_name"].(string); ok {
			newField, err = utility.IsValidIdentifier(val)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// now search if the renamed field already exists in the model
		_fieldInfo, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  newField.Identifier,
			ParentField: parentField,
		}, nil, nil)
		if err != nil {
			return nil, err
		}

		if _fieldInfo != nil {
			return nil, errors.New("field you are trying to duplicate already exits with this name")
		}

		// now search for fields
		affectedField, err = s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  fieldName,
			ParentField: parentField,
		}, newField, &_type)
		if err != nil {
			return nil, err
		}

		/* param := s.NewParam(cache.Param)
		param.Model = modelType
		param.FieldInfo = affectedField
		param.SinglePageData = singlePageModel */

	case enums.FieldOperation_ChangeType:
		var changedType string
		if val, ok := p.Args["changed_type"].(string); ok {
			changedType = val
		} else {
			return nil, errors.New("field Label Is necessary")
		}

		// now search if the renamed field already exists in the model
		fieldInfo, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  fieldName,
			ParentField: parentField,
		}, nil, nil)
		if err != nil {
			return nil, err
		}

		if fieldInfo == nil {
			return nil, errors.New("field you are trying to chnage type of does not exist")
		}

		fieldInfo.FieldType = changedType
		fieldInfo.InputType = _const.GetInputTypebyFieldType(changedType)
		fieldInfo.Validation = nil // rest the validatioan now the field type has changed

		affectedField = fieldInfo

	case enums.FieldOperation_Delete:
		deletedField, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier:  fieldName,
			ParentField: parentField,
		}, nil, nil)
		if err != nil {
			return nil, err
		}
		if deletedField == nil {
			if s.schemaVersioningActive() {
				applyDDL = func(interfaces.ProjectDBInterface) error { return nil }
				beforePersist = func() error { return nil }
				affectedField = &models.FieldInfo{
					Identifier:  fieldName,
					Label:       fieldName,
					ParentField: parentField,
				}
			} else {
				return nil, errors.New("field to delete not found")
			}
		} else {
			param := s.NewParam(cache.Param)
			param.FieldInfo = deletedField
			param.Model = modelType

			applyDDL = func(d interfaces.ProjectDBInterface) error {
				return d.DropField(cache.Ctx, param)
			}
			beforePersist = func() error {
				affectedField, err = s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
					Identifier:  fieldName,
					ParentField: parentField,
				}, nil, &_type)
				return err
			}
		}

	default:
		applyDDL = func(interfaces.ProjectDBInterface) error { return nil }
	}

	if applyDDL == nil {
		applyDDL = func(interfaces.ProjectDBInterface) error { return nil }
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeModelFieldOp, graphqlArgsMap(p), func() (interface{}, error) {
		err = s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx:           cache.Ctx,
			Project:       project,
			OperationType: models.SchemaOpTypeModelFieldOp,
			Request: map[string]interface{}{
				"model_name": modelName, "type": _type, "field_name": fieldName,
			},
			BaseDriver:  driver,
			SkipBaseDDL: skipBaseDDL,
			ApplyDDL:    applyDDL,
			PersistSystem: func() error {
				if beforePersist != nil {
					if err := beforePersist(); err != nil {
						return err
					}
				}
				if err := s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, cache.Project.ID); err != nil {
					return err
				}
				return s.SystemDriver.UpsertModelType(cache.Ctx, cache.Project.ID, modelType)
			},
			RefreshCache: func() error {
				return s.ExpireGraphQLProjectCache(cache.Ctx, project.ID)
			},
			Compensate: compensate,
		})
		if err != nil {
			return nil, err
		}
		return affectedField, nil
	})
}

func (s *GraphQLServer) searchAndOperateOnFields(fields *[]*models.FieldInfo, existingField, newField *models.ValidIdentifier, operationType *enums.FieldOperation) (*models.FieldInfo, error) {
	if existingField.Identifier == "" {
		return nil, errors.New("identifier is required")
	}
	if len(*fields) == 0 {
		return nil, nil
	}

	// Helper to delete a field from a slice by identifier
	deleteFieldByIdentifier := func(fields []*models.FieldInfo, identifier string) (int, *models.FieldInfo, bool) {
		for i, f := range fields {
			if f.Identifier == identifier {
				deleted := f
				return i, deleted, true
			}
		}
		return -1, nil, false
	}

	duplicateFieldByIdentifier := func(fields []*models.FieldInfo, identifier string) (*models.FieldInfo, error) {
		for _, f := range fields {
			if f.Identifier == identifier {

				if f.SystemGenerated {
					return nil, errors.New("can not dupliacte a system generated field")
				}

				// Create a deep copy of the field
				duplicated := &models.FieldInfo{
					Label:      newField.Label,
					Identifier: newField.Identifier,
					// copy the rest
					Description:     f.Description,
					InputType:       f.InputType,
					FieldType:       f.FieldType,
					Validation:      f.Validation,
					Serial:          f.Serial,
					ParentField:     f.ParentField,
					SystemGenerated: f.SystemGenerated,
				}
				// Deep copy SubFieldInfo if it exists
				if f.SubFieldInfo != nil {
					duplicated.SubFieldInfo = f.SubFieldInfo
				}
				return duplicated, nil
			}
		}
		return nil, nil
	}

	// If parentIdentifier is provided, find the parent field then search its
	// subfields. Prefer the shallowest match (root before nested) so a top-level
	// repeated group is not shadowed by a nested group with the same identifier
	// (e.g. class.sections vs class.divisions.sections).
	if existingField.ParentField != "" {
		var varSearch func(fs []*models.FieldInfo) *models.FieldInfo
		varSearch = func(fs []*models.FieldInfo) *models.FieldInfo {
			for _, f := range fs {
				if f.Identifier == existingField.ParentField {
					return f
				}
			}
			for _, f := range fs {
				if len(f.SubFieldInfo) > 0 {
					if found := varSearch(f.SubFieldInfo); found != nil {
						return found
					}
				}
			}
			return nil
		}
		parentField := varSearch(*fields)
		if parentField == nil {
			return nil, ae.ErrParentFieldNotFound
		}
		// Now search in parent's subfields for the identifier
		for _, sub := range parentField.SubFieldInfo {
			if sub.Identifier == existingField.Identifier {
				if operationType != nil {
					switch *operationType {
					case enums.FieldOperation_Delete:
						// Delete the field and update parent's SubFieldInfo
						index, deleted, found := deleteFieldByIdentifier(parentField.SubFieldInfo, existingField.Identifier)
						if found {
							// Remove the element at the found index
							parentField.SubFieldInfo = append(parentField.SubFieldInfo[:index], parentField.SubFieldInfo[index+1:]...)
							return deleted, nil
						}
						return nil, nil
					case enums.FieldOperation_Rename:
						// Rename the field
						if newField != nil {
							sub.Label = newField.Label
							sub.Identifier = newField.Identifier
							return sub, nil
						}
						return nil, errors.New("new field information required for rename")
					case enums.FieldOperation_Duplicate:
						// Duplicate the field and add to parent's SubFieldInfo
						duplicated, err := duplicateFieldByIdentifier(parentField.SubFieldInfo, existingField.Identifier)
						if err != nil {
							return nil, err
						}
						if duplicated != nil {
							// Update the duplicated field's identifier and serial
							duplicated.Serial = uint32(len(parentField.SubFieldInfo) + 1)
							parentField.SubFieldInfo = append(parentField.SubFieldInfo, duplicated)
							return duplicated, nil
						}
						return nil, errors.New("field not found to duplicate")
					}
				}
				return sub, nil
			}
			if len(sub.SubFieldInfo) > 0 {
				if found, err := s.searchAndOperateOnFields(&sub.SubFieldInfo, existingField, newField, operationType); err == nil && found != nil {
					return found, nil
				}
			}
		}
		return nil, nil
	}

	// If no parentIdentifier, search in root fields for the identifier (deeply)
	for _, f := range *fields {
		if f.Identifier == existingField.Identifier {
			if operationType != nil {
				switch *operationType {
				case enums.FieldOperation_Delete:
					index, deleted, found := deleteFieldByIdentifier(*fields, existingField.Identifier)
					if found {
						// Remove the element at the found index
						*fields = append((*fields)[:index], (*fields)[index+1:]...)
						return deleted, nil
					}
					return nil, nil
				case enums.FieldOperation_Rename:
					// Rename the field
					if newField != nil {
						f.Label = newField.Label
						f.Identifier = newField.Identifier
						return f, nil
					}
					return nil, errors.New("new field information required for rename")
				case enums.FieldOperation_Duplicate:
					// Duplicate the field and add to root fields
					duplicated, err := duplicateFieldByIdentifier(*fields, existingField.Identifier)
					if err != nil {
						return nil, err
					}
					if duplicated != nil {
						// Update the duplicated field's identifier and serial
						duplicated.Serial = uint32(len(*fields) + 1)
						*fields = append(*fields, duplicated)
						return duplicated, nil
					}
					return nil, errors.New("field not found to duplicate")
				}
			}
			return f, nil
		}
	}
	return nil, nil
}

func (s *GraphQLServer) deleteRelationField(ctx context.Context, project *models.Project, modelType *models.ModelType, identifier, knownAs string) (*models.ConnectionType, *models.ConnectionType, error) {

	if project != nil && project.Schema != nil {
		models.EnsureProjectAuthUserModelInSchema(project.Schema)
	}

	// struct the connection type before removing from schema
	var fromConnectionType *models.ConnectionType
	// delete the forward relation
	for i, r := range modelType.Connections {
		if r.Model == identifier && r.KnownAs == knownAs {
			fromConnectionType = r
			modelType.Connections = append(modelType.Connections[:i], modelType.Connections[i+1:]...)
			break
		}
	}
	if len(modelType.Connections) == 0 {
		modelType.Connections = nil
	}

	if fromConnectionType == nil {
		return nil, nil, errors.New("from connection type not found")
	}

	// struct the connection type before removing from schema
	var toConnectionType *models.ConnectionType
	var peerModel *models.ModelType

	// delete the backward relation
	for _, ct := range project.Schema.Models {
		if ct.Name == identifier {
			peerModel = ct
			for i, r := range ct.Connections {
				if r.Model == modelType.Name && r.KnownAs == knownAs {
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

	if toConnectionType == nil {
		return nil, nil, errors.New("to connection type not found")
	}
	if peerModel == nil {
		return nil, nil, errors.New("peer model not found")
	}

	// delete system has one identifer
	if fromConnectionType.Relation == "has_one" {

		syntheticID := utility.SyntheticSystemRelationFieldIdentifier(fromConnectionType.Model, fromConnectionType.KnownAs)

		_, err := s.searchAndOperateOnFields(&modelType.Fields, &models.ValidIdentifier{
			Identifier: syntheticID,
		}, nil, func() *enums.FieldOperation { op := enums.FieldOperation_Delete; return &op }())
		if err != nil {
			return nil, nil, err
		}
	}

	if toConnectionType.Relation == "has_one" {

		syntheticID := utility.SyntheticSystemRelationFieldIdentifier(toConnectionType.Model, knownAs)

		_, err := s.searchAndOperateOnFields(&peerModel.Fields, &models.ValidIdentifier{
			Identifier: syntheticID,
		}, nil, func() *enums.FieldOperation { op := enums.FieldOperation_Delete; return &op }())
		if err != nil {
			return nil, nil, err
		}
	}

	return fromConnectionType, toConnectionType, nil
}

func (s *GraphQLServer) DeleteConnectionFromModelResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("DeleteConnectionFromModelResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var fromResource string
	if val, ok := p.Args["from"].(string); ok && val != "" {
		var err error
		fromResource, err = utility.CanonicalizeModelName(val)
		if err != nil {
			fromResource, err = utility.LegacyStoredNameToCanonical(val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New("from Model Needed")
	}

	var toResource string
	if val, ok := p.Args["to"].(string); ok && val != "" {
		var err error
		toResource, err = utility.CanonicalizeModelName(val)
		if err != nil {
			toResource, err = utility.LegacyStoredNameToCanonical(val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New("to Model Needed")
	}

	var knownAs string
	if val, ok := p.Args["known_as"].(string); ok && val != "" {
		var err error
		knownAs, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
	}

	var fromModelType *models.ModelType
	for _, ct := range project.Schema.Models {
		if ct != nil && ct.Name == fromResource {
			fromModelType = ct
			break
		}
	}
	if fromModelType == nil {
		return nil, errors.New("model Not Found")
	}

	authUsersPersisted := false
	for _, m := range project.Schema.Models {
		if m != nil && models.ModelIsProjectAuthUserModel(m) {
			authUsersPersisted = true
			break
		}
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeDeleteConnection, graphqlArgsMap(p), func() (interface{}, error) {
		driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
		if err != nil {
			return nil, err
		}

		fromConnectionType, toConnectionType, err := s.deleteRelationField(cache.Ctx, project, fromModelType, toResource, knownAs)
		if err != nil {
			return nil, err
		}

		if err := driver.DeleteRelationDocuments(cache.Ctx, project.ID, fromConnectionType, toConnectionType); err != nil {
			return nil, err
		}

		var toModelType *models.ModelType
		for _, ct := range project.Schema.Models {
			if ct != nil && ct.Name == toResource {
				toModelType = ct
				break
			}
		}

		if err := s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, project.ID); err != nil {
			return nil, err
		}
		if err := s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, fromModelType); err != nil {
			return nil, err
		}
		if toModelType != nil && (!models.ModelIsProjectAuthUserModel(toModelType) || authUsersPersisted) {
			if err := s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, toModelType); err != nil {
				return nil, err
			}
		}

		if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
			return nil, err
		}

		return []*models.ConnectionType{fromConnectionType, toConnectionType}, nil
	})
}

func (s *GraphQLServer) CreateConnectionTypeResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaWrite); err != nil {
		return nil, err
	}

	s.injectMetaData("CreateConnectionTypeResolverFn", router)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	var fromResource string
	if val, ok := p.Args["from"].(string); ok && val != "" {
		var err error
		fromResource, err = utility.CanonicalizeModelName(val)
		if err != nil {
			fromResource, err = utility.LegacyStoredNameToCanonical(val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New("from Model Needed")
	}

	var toResource string
	if val, ok := p.Args["to"].(string); ok && val != "" {
		var err error
		toResource, err = utility.CanonicalizeModelName(val)
		if err != nil {
			toResource, err = utility.LegacyStoredNameToCanonical(val)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New("to Model Needed")
	}

	var knownAs string
	if val, ok := p.Args["known_as"].(string); ok && val != "" {
		var err error
		knownAs, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
	}

	return s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeCreateConnection, graphqlArgsMap(p), func() (interface{}, error) {
		driver, skipBaseDDL, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project)
		if err != nil {
			return nil, err
		}

		var connections []*models.ConnectionType
		if project.Schema == nil {
			return nil, ae.SchemaIsNil
		}

		var fromModelType, toModelType *models.ModelType
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
		if len(fromModelType.Fields) == 0 {
			return nil, fmt.Errorf("can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(fromModelType.Name)))
		}
		if len(toModelType.Fields) == 0 {
			return nil, fmt.Errorf("can not create relations with %s, because it has no fields.", strings.Title(strings.ToLower(toModelType.Name)))
		}

		var fromConnectionInfo *models.ConnectionType
		for _, f := range fromModelType.Connections {
			if f.Model == toResource && f.KnownAs == knownAs {
				fromConnectionInfo = f
				break
			}
		}
		if fromConnectionInfo == nil {
			fromConnectionInfo = &models.ConnectionType{Model: toResource, Type: "forward", KnownAs: knownAs}
			fromModelType.Connections = append(fromModelType.Connections, fromConnectionInfo)
		}
		// Always normalize ownership direction on upsert (fixes flipped metadata).
		fromConnectionInfo.Type = "forward"
		if val, ok := p.Args["forward_connection_type"]; ok {
			fromConnectionInfo.Relation = val.(string)
		}
		connections = append(connections, fromConnectionInfo)

		var toConnectionInfo *models.ConnectionType
		for _, f := range toModelType.Connections {
			if f.Model == fromResource && f.KnownAs == knownAs {
				toConnectionInfo = f
				break
			}
		}
		if toConnectionInfo == nil {
			toConnectionInfo = &models.ConnectionType{Model: fromResource, Type: "backward", KnownAs: knownAs}
			toModelType.Connections = append(toModelType.Connections, toConnectionInfo)
		}
		toConnectionInfo.Type = "backward"
		if val, ok := p.Args["reverse_connection_type"]; ok {
			toConnectionInfo.Relation = val.(string)
		}

		if fromConnectionInfo.Relation == "has_one" {
			identifier := utility.SyntheticSystemRelationFieldIdentifier(fromConnectionInfo.Model, knownAs)
			label := fmt.Sprintf(`System %s ID`, strings.Title(fromConnectionInfo.Model))
			if knownAs != "" {
				label = fmt.Sprintf(`System %s ID`, strings.Title(knownAs))
			}
			var found bool
			for _, f := range fromModelType.Fields {
				if f.Identifier == identifier {
					found = true
					break
				}
			}
			if !found {
				fromModelType.Fields = append(fromModelType.Fields, &models.FieldInfo{
					Identifier: identifier, Description: "An Auto Generated Relation Identifier for Easy Filter Purposes",
					InputType: "string", FieldType: "text",
					Validation: &models.Validation{Hide: true, Unique: true},
					Serial:     1, Label: label, SystemGenerated: true,
				})
			}
		}
		if toConnectionInfo.Relation == "has_one" {
			identifier := utility.SyntheticSystemRelationFieldIdentifier(toConnectionInfo.Model, knownAs)
			label := fmt.Sprintf(`System %s ID`, strings.Title(toConnectionInfo.Model))
			if knownAs != "" {
				label = fmt.Sprintf(`System %s ID`, strings.Title(knownAs))
			}
			var found bool
			for _, f := range toModelType.Fields {
				if f.Identifier == identifier {
					found = true
					break
				}
			}
			if !found {
				toModelType.Fields = append(toModelType.Fields, &models.FieldInfo{
					Identifier: identifier, Description: "An Auto Generated Relation Identifier for Easy Filter Purposes",
					InputType: "string", FieldType: "text",
					Validation: &models.Validation{Hide: true, Unique: true},
					Serial:     1, Label: label, SystemGenerated: true,
				})
			}
		}

		fwdConn := fromConnectionInfo
		revConn := toConnectionInfo
		err = s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx: cache.Ctx, Project: project,
			OperationType: models.SchemaOpTypeCreateConnection,
			Request:       map[string]interface{}{"from": fromResource, "to": toResource, "known_as": knownAs},
			BaseDriver:    driver, SkipBaseDDL: skipBaseDDL,
			ApplyDDL: func(d interfaces.ProjectDBInterface) error {
				return d.AddRelationFields(cache.Ctx, fwdConn, revConn)
			},
			PersistSystem: func() error {
				if err := s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, project.ID); err != nil {
					return err
				}
				if err := s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, fromModelType); err != nil {
					return err
				}
				return s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, toModelType)
			},
			RefreshCache: func() error {
				_, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
				return err
			},
			Compensate: func(d interfaces.ProjectDBInterface) error {
				return d.DeleteRelationDocuments(cache.Ctx, project.ID, fwdConn, revConn)
			},
		})
		if err != nil {
			return nil, err
		}
		connections = append(connections, toConnectionInfo)
		return connections, nil
	})
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

	project := cache.Project
	// if schema not found then create
	if project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	// Safe extraction: model_name is required and must be non-empty string
	modelNameVal := p.Args["model_name"]
	if modelNameVal == nil {
		return nil, errors.New("model_name is required")
	}
	modelName, ok := modelNameVal.(string)
	if !ok || modelName == "" {
		return nil, errors.New("model_name must be a non-empty string")
	}
	var canonErr error
	modelName, canonErr = utility.CanonicalizeModelName(modelName)
	if canonErr != nil {
		return nil, canonErr
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
	param.ProjectSchemaModels = project.Schema.Models

	// Safe extraction: status defaults to "published" when nil or invalid
	if val, ok := p.Args["status"].(string); ok && val != "" {
		param.DocPublishStatus = val
	} else {
		param.DocPublishStatus = "published"
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
	var docExists bool

	if val, ok := p.Args["_id"]; ok && val != nil {
		if id, ok := val.(string); ok && id != "" {
			param.DocumentID = id
		}
	}

	if param.DocumentID != "" {

		var isSinglePageData bool
		if val, ok := p.Args["single_page_data"].(bool); ok {
			isSinglePageData = val
		}
		param.ResolveParams = &p
		param.Model = modelType
		param.SinglePageData = isSinglePageData

		param.SkipPagination = true
		param.SkipSort = true

		if modelType.SinglePage { // overwrite the input if the model itself is single page
			param.SinglePageData = true
		}

		raw, lookupErr := driver.GetSingleRawDocumentFromProject(cache.Ctx, param)
		existing, found, err := existingDocumentFromLookup(raw, lookupErr)
		if err != nil {
			return nil, err
		}
		if found {
			// got the doc but doc doesn't belong to specific model
			if existing.Type != modelName {
				return nil, fmt.Errorf("document does not belongs to %s", modelName)
			}
			doc = existing
			docExists = true
		}
		// Not found is not fatal here: insert below reuses the caller supplied id so
		// cross-environment copies (apito sync content) keep document id parity.
	}

	if docExists {

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
			if val, ok := input["payload"].(map[string]interface{}); ok && val != nil {
				inputPayload = val
			}
			if inputPayload == nil {
				inputPayload = make(map[string]interface{})
			}

			// local support (safe: nil yields ok=false)
			local := "en"
			if val, ok := input["local"].(string); ok && val != "" {
				local = val
			}

			//#todo need image param validation
			// upsert has no delta update support for now
			modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(cache.Ctx, param, local, modelType.Fields, inputPayload, doc.Data, false)
			if err != nil {
				return nil, err
			}
			doc.Data = modifiedPayload

			// replacing the doc might case the local field to disappear. don't replace the old doc
			// fixed it later !!
			err = driver.UpdateDocumentOfProject(cache.Ctx, param, doc, forceUpdate)
			if err != nil {
				return nil, err
			}
		}

	} else {

		//#todo replace these operation with transaction

		uid := utility.NewID()
		if param.DocumentID != "" {
			uid = param.DocumentID
		}

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
			if val, ok := input["payload"].(map[string]interface{}); ok && val != nil {
				inputPayload = val
			}
			// local support (safe: nil yields ok=false)
			if val, ok := input["local"].(string); ok && val != "" {
				local = val
			}
		}
		if inputPayload == nil {
			inputPayload = make(map[string]interface{})
		}

		//#todo need image param validation
		// upsert has no delta update support for now
		modifiedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(cache.Ctx, param, local, modelType.Fields, inputPayload, make(map[string]interface{}), false)
		if err != nil {
			return nil, err
		}
		doc.Data = modifiedPayload

		//_, err = s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(p.Context, param.ProjectId, modelName, doc)
		_, err = driver.AddDocumentToProject(cache.Ctx, param, doc)
		if err != nil {
			return nil, err
		}
		if s.Cfg.PostDocumentInsertHook != nil {
			if err := s.Cfg.PostDocumentInsertHook(cache.Ctx, param, doc.ID); err != nil {
				return nil, err
			}
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

	project := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		var err error
		modelName, err = utility.CanonicalizeModelName(val)
		if err != nil {
			return nil, err
		}
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
	if val, ok := p.Args["_id"]; ok && val != nil {
		if id, ok := val.(string); ok && id != "" {
			docId = id
		}
	}
	if docId == "" {
		return nil, errors.New("_id is required for duplicate")
	}

	param.DocumentID = docId
	param.ResolveParams = &p
	param.Model = modelType

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	exists, err := driver.GetSingleProjectDocument(cache.Ctx, param)
	if err != nil {
		return nil, err
	}

	if exists != nil {
		exists.Key = utility.NewID()
		exists.ID = exists.Key
		exists.Meta.CreatedAt = utility.GetCurrentTime()
		exists.Meta.UpdatedAt = utility.GetCurrentTime()

		_, err = driver.AddDocumentToProject(cache.Ctx, param, exists)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("doc not found to duplicate")
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
		var cErr error
		modelName, cErr = utility.CanonicalizeModelName(val)
		if cErr != nil {
			return nil, cErr
		}
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
	if val, ok := p.Args["_id"]; ok && val != nil {
		if id, ok := val.(string); ok && id != "" {
			docId = id
		}
	}
	if docId == "" {
		return nil, errors.New("_id is required for delete")
	}

	param.DocumentID = docId
	param.Model = modelType

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	param.DocPublishStatus = "all"
	// GetSingleProjectDocument reads ResolveParams.Args["local"]; delete callers must set this.
	param.ResolveParams = &p

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
