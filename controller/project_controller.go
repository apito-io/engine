package controller

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
)

func contains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

func coerceConfigPort(v interface{}) string {
	switch p := v.(type) {
	case string:
		return p
	case float64:
		return strconv.FormatInt(int64(p), 10)
	case int:
		return strconv.Itoa(p)
	case int64:
		return strconv.FormatInt(p, 10)
	default:
		return fmt.Sprint(v)
	}
}

func dbConfigString(m map[string]interface{}, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// ApplyDBConfigFromMap merges db_config map values into driver credentials (open-core fields only).
func ApplyDBConfigFromMap(d *models.DriverCredentials, dbConfig map[string]interface{}) {
	if s := dbConfigString(dbConfig, "host"); s != "" {
		d.Host = s
	}
	if val, ok := dbConfig["port"]; ok && val != nil {
		d.Port = coerceConfigPort(val)
	}
	if s := dbConfigString(dbConfig, "user"); s != "" {
		d.User = s
	} else if s := dbConfigString(dbConfig, "username"); s != "" {
		d.User = s
	}
	if s := dbConfigString(dbConfig, "password"); s != "" {
		d.Password = s
	}
	if s := dbConfigString(dbConfig, "database"); s != "" {
		d.Database = s
	}
	if s := dbConfigString(dbConfig, "file"); s != "" {
		d.File = s
	}
	if s := dbConfigString(dbConfig, "ssl_mode"); s != "" {
		d.SSLMode = s
	}
}

func (a *AuthController) ProjectCreation(c echo.Context) error {

	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	userID := c.Get("user")
	if userID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the request",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userID.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	// query projects
	projects, err := a.graphQLServer.SystemDriver.FindUserProjectsWithRoles(ctx, userID.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	var createdProject int32
	for _, p := range projects {
		if models.IsCollaboratorMembershipRole(p.Role) {
			continue
		}
		createdProject++
	}

	// prepare the project skeleton
	var _projectID string
	if val, ok := req["id"]; ok && val != nil {
		_projectID = val.(string)
	}

	var projectName string
	if val, ok := req["name"]; ok && val != nil {
		projectName = val.(string)
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Project name is required",
			Code:    http.StatusBadRequest,
		})
	}

	projectID := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(_projectID, " ", "_")))
	project := &models.Project{
		XKey:         projectID,
		ID:           projectID,
		Name:         projectName,
		// generate a project secret key and add default role
		Roles: map[string]*models.Role{ // default role
			"admin": {
				SystemGenerated: true,
				IsAdmin:         true,
			},
		},
		Settings: &models.ProjectSettings{
			Locals: []string{"en"},
		},
		ProjectSecretKey: utility.RandomStringGenerator(25),
	}

	// optional
	if val, ok := req["description"]; ok && val != nil {
		project.Description = val.(string)
	}

	var dbConfig map[string]interface{}
	if val, ok := req["db_config"]; ok && val != nil && len(val.(map[string]interface{})) > 0 {
		dbConfig = val.(map[string]interface{})
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Database configuration is required",
			Code:    http.StatusBadRequest,
		})
	}

	project.Driver = &models.DriverCredentials{}

	var rawDBType string
	if val, ok := req["database_type"]; ok && val != nil {
		rawDBType = strings.ToLower(val.(string))
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Database type is required",
			Code:    http.StatusBadRequest,
		})
	}

	switch rawDBType {
	default:
		project.Driver.Engine = rawDBType
		ApplyDBConfigFromMap(project.Driver, dbConfig)
	}

	if project.Driver == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Database driver could not be built from database_type and db_config",
			Code:    http.StatusBadRequest,
		})
	}

	// inject driver credential to connection manager
	project.Driver.ProjectID = projectID // this is a must for connection manager
	a.graphQLServer.GraphQLExecutor.SetProjectDriverCredential(ctx, project.Driver)

	// Create the collection/table per database engine.
	ctx = context.WithValue(ctx, "project_id", projectID)
	projectDriver, err := a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}
	addColParam := &models.CommonSystemParams{
		ProjectID: projectID,
		UserID:    user.ID,
	}
	if a.Cfg != nil && a.Cfg.InitProjectBaseHook != nil {
		err = a.Cfg.InitProjectBaseHook(ctx, projectDriver, addColParam)
	} else {
		err = projectDriver.InitProjectBase(ctx, addColParam, nil)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	_project, err := a.graphQLServer.SystemDriver.CreateProject(ctx, userID.(string), project)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	a.graphQLServer.EmitProjectLifecycle(ctx, user.ID, _project.ID, _project.Name, models.SystemEventProjectCreated)

	if user.CurrentProjectID == "" {
		user.CurrentProjectID = _project.ID
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if _project.ProjectTemplate != "" {
		// transfer the project
		projectDriver, err := a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		err = projectDriver.TransferProject(ctx, userID.(string), _project.ProjectTemplate, _project.ID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		/*err = a.graphQLServer.S3.TransferBucket(projectDetails.FromExample, projectId)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}*/
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: _project,
		//Token: token,
		Code: http.StatusOK,
	})
}

func (a *AuthController) DemoProjectSwitch(c echo.Context) error {

	var req *models.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "What are you switching?",
			Code:    http.StatusBadRequest,
		})
	}

	validDemoProject := []string{"quantum_ecommerce_ddlj4", "jira_clone_o5t3r", "apito_website"}
	if !contains(validDemoProject, req.ID) {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid Request",
			Code:    http.StatusBadRequest,
		})
	}

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.CurrentProjectID = req.ID
	user.ReadOnlyProject = true

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	// refresh the token
	tokens, err := utility.NewRefreshTokenAuthenticator(a.Cfg, user.RefreshToken)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "enter_project_time", fmt.Sprintf("%v", time.Now().UTC().UnixNano()/1e6), false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "project_id", req.ID, false, false))

	return c.JSON(http.StatusOK, &models.HttpResponse{
		//Token:  token,
		Code: http.StatusOK,
	})
}

func (a *AuthController) ProjectNameCheck(c echo.Context) error {

	var req *models.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project Name is required",
			Code:    http.StatusBadRequest,
		})
	}

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	// first check proejct name is available in the sytem db or not 
	err := a.graphQLServer.SystemDriver.CheckProjectName(ctx, req.Name)
	if err != nil {
		if errors.Is(err, ae.ErrProjectNameTaken) {
			return c.JSON(http.StatusConflict, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusConflict,
			})
		}
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		//Token:  token,
		Body: req.Name,
		Code: http.StatusOK,
	})
}

func (a *AuthController) ProjectList(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	projects, err := a.graphQLServer.SystemDriver.FindUserProjectsWithRoles(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	var result []*models.Project
	for _, _p := range projects {
		p := _p.Project
		_project := &models.Project{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
		}
		if p.Driver != nil {
			_project.Driver = &models.DriverCredentials{Engine: p.Driver.Engine}
		}
		result = append(result, _project)
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: result,
		Code: http.StatusOK,
	})
}

func (a *AuthController) CSVTempGen(c echo.Context) error {

	var req *models.CSVTemplateGenerator
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	projectID := c.Get("project")
	if projectID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project id is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	_project, err := a.graphQLServer.SystemDriver.GetProject(ctx, projectID.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	var model *models.ModelType
	for _, m := range _project.Schema.Models {
		if m.Name == req.ModelName {
			model = m
			break
		}
	}

	if model == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Model not found",
			Code:    http.StatusBadRequest,
		})
	}

	// loop through each field and generate the csv template
	var csvTemplate []string
	for _, f := range model.Fields {
		csvTemplate = append(csvTemplate, f.Identifier)
	}

	// write the csv template to the file and return the downloadable file
	fileName := fmt.Sprintf("%s_template.csv", req.ModelName)
	file, err := os.Create(fileName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "Failed to create CSV file",
			Code:    http.StatusInternalServerError,
		})
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write(csvTemplate); err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "Failed to write CSV file",
			Code:    http.StatusInternalServerError,
		})
	}

	return c.Attachment(fileName, fileName)
}

func (a *AuthController) GetProfile(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "you need to be logged in to access this route",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	//subs, err :=  a.gqlServer.SystemDriver.ListSubscription("38c6bef1-a693-41e5-ad9b-a4e6a7e6dc24")
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""
	user.RefreshToken = ""
	user.AccessToken = ""

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body: user,
		Code: http.StatusOK,
	})
}

func (a *AuthController) UpdateProfile(c echo.Context) error {

	userId := c.Get("user")
	if userId == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "you need to be logged in to access this route",
			Code:    http.StatusBadRequest,
		})
	}

	var req *models.SystemUser
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	//subs, err :=  a.gqlServer.SystemDriver.ListSubscription("38c6bef1-a693-41e5-ad9b-a4e6a7e6dc24")
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userId.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}

	if req.LastName != "" {
		user.LastName = req.LastName
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Body:    user,
		Message: "Profile Updated",
		Code:    http.StatusOK,
	})
}

func (a *AuthController) ProjectDelete(c echo.Context) error {

	var req *models.ProjectCreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	_project, err := a.graphQLServer.SystemDriver.GetProject(ctx, req.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.HttpResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
	}

	ctx = context.WithValue(ctx, "project_id", _project.ID)
	err = a.graphQLServer.GraphQLExecutor.Init(ctx, &models.InitParams{ProjectDB: _project.Driver})
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.HttpResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
	}

	if req.ID == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project id is missing",
			Code:    http.StatusBadRequest,
		})
	}

	userID := c.Get("user")
	if userID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userID.(string))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	err = a.graphQLServer.SystemDriver.DeleteProjectFromSystem(ctx, req.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if req.ID == user.CurrentProjectID {
		user.CurrentProjectID = ""
	}

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if user.CurrentProjectID == "" {
		tokens, err := a.graphQLServer.JWTTokenService.GenerateLoginToken(ctx, &models.ProjectWithRoles{User: user})
		if err != nil && strings.TrimSpace(user.RefreshToken) != "" {
			oauthTokens, oauthErr := utility.NewRefreshTokenAuthenticator(a.Cfg, user.RefreshToken)
			if oauthErr != nil {
				return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
					Message: captureInternalServerError(err).Error(),
					Code:    http.StatusInternalServerError,
				})
			}
			tokens = &models.JWTTokens{
				IDToken:      oauthTokens.IDToken,
				AccessToken:  oauthTokens.AccessToken,
				RefreshToken: oauthTokens.RefreshToken,
			}
			err = nil
		}
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
		if strings.TrimSpace(tokens.AccessToken) != "" {
			http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))
		}
		return c.JSON(http.StatusOK, &models.HttpResponse{
			Code: http.StatusOK,
		})
	}

	go func() {
		projectDriver, err := a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx)
		if err != nil {
			utility.CaptureInternalServerError(err, map[string]interface{}{
				"func": "GetProjectDriver",
				"req":  req,
			})
			return
		}

		err = projectDriver.DeleteProject(ctx, req.ID)
		if err != nil {
			utility.CaptureInternalServerError(err, map[string]interface{}{
				"func": "DeleteProject",
				"req":  req,
			})
		}
		/* delete files from s3
		err = a.graphQLServer.S3.DeleteFilesV2(req.ID)
		if err != nil {
			utility.CaptureInternalServerError(err, map[string]interface{}{
				"func": "DeleteFilesFromS3",
				"req":  req,
			})
		}*/
	}()

	return c.JSON(http.StatusOK, &models.HttpResponse{
		//Token:  token,
		Code: http.StatusOK,
	})
}
