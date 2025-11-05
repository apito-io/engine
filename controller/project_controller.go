package controller

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
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

func (a *authCtrl) ProjectCreation(c echo.Context) error {

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
		if p.Role == "team" { // if team project then skip
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
		Organization: user.DefaultOrganization,
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
		Teams:            []*models.Team{user.DefaultTeam},
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

	if val, ok := req["database_type"]; ok && val != nil {
		project.Driver.Engine = val.(string)
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Database type is required",
			Code:    http.StatusBadRequest,
		})
	}



	if val, ok := dbConfig["host"]; ok && val != nil {
		project.Driver.Host = val.(string)
	}
	if val, ok := dbConfig["port"]; ok && val != nil {
		project.Driver.Port = val.(string)
	}
	if val, ok := dbConfig["user"]; ok && val != nil {
		project.Driver.User = val.(string)
	}
	if val, ok := dbConfig["password"]; ok && val != nil {
		project.Driver.Password = val.(string)
	}
	if val, ok := dbConfig["database"]; ok && val != nil {
		project.Driver.Database = val.(string)
	}
	if val, ok := dbConfig["file"]; ok && val != nil {
		project.Driver.File = val.(string)
	}

	switch strings.ToLower(req["project_type"].(string)) {
	case "regular", "general":
		project.ProjectType = models.ProjectType_General
	case "saas":
		project.ProjectType = models.ProjectType_SaaS

		var tenantModelName string
		if val, ok := req["tenant_model_name"]; ok && val != nil {
			tenantModelName = val.(string)
		} else {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Tenant model name is required for SaaS project",
				Code:    http.StatusBadRequest,
			})
		}

		// saas is just in trail mode right now for 7 days
		//project.TrialEnds = time.Now().AddDate(0, 0, 7).UTC().Format(time.RFC3339)
		project.TenantModelName = utility.SingularResourceName(tenantModelName)
		project.Schema = &models.ProjectSchema{
			Models: []*models.ModelType{
				{
					Name:          tenantModelName,
					IsTenantModel: true,
					Fields: []*models.FieldInfo{
						{
							Identifier:  "name",
							Description: fmt.Sprintf("%s Name", strings.Title(tenantModelName)),
							FieldType:   "text",
							InputType:   "string",
							Serial:      1,
							Label:       "Name",
							//SystemGenerated: true,
						},
						{
							Identifier:  "logo",
							Description: fmt.Sprintf("%s Logo", strings.Title(tenantModelName)),
							InputType:   "string",
							FieldType:   "media",
							Serial:      2,
							Label:       "Logo",
							//SystemGenerated: true,
						},
					},
				},
			},
		}
	default:
		project.ProjectType = models.ProjectType_General
	}

	if project.Driver == nil {
		project.Driver = &models.DriverCredentials{
			ProjectID: projectID, // this is must be passed from here
			Engine:    a.Cfg.DefaultProjectDatabaseEngine,
			Host:      a.Cfg.DefaultProjectDBHost,
			Port:      a.Cfg.DefaultProjectDBPort,
			User:      a.Cfg.DefaultProjectDBUser,
			Database:  a.Cfg.DefaultProjectDBName,
			Password:  a.Cfg.DefaultProjectDBPassword,
		}
	}

	// inject driver credential to connection manager
	project.Driver.ProjectID = projectID // this is a must for connection manager
	a.graphQLServer.GraphQLExecutor.SetProjectDriverCredential(ctx, project.Driver)

	if project.ProjectType != models.ProjectType_SaaS {
		//inject project_id via context
		ctx = context.WithValue(ctx, "project_id", projectID)
		projectDriver, err := a.graphQLServer.GraphQLExecutor.GetProjectDriver(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
		err = projectDriver.AddCollection(ctx, &models.CommonSystemParams{
			ProjectID:   projectID,
			UserID:      user.ID,
			ProjectType: project.ProjectType,
		}, false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
	}

	_project, err := a.graphQLServer.SystemDriver.CreateProject(ctx, userID.(string), project)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	if user.CurrentProjectID == "" {
		user.CurrentProjectID = _project.ID
	}

	var _teamID string
	// inject default team and organization
	if user.DefaultTeam == nil {
		_teamID = uuid.New().String()
		_team := &models.Team{
			XKey:        _teamID,
			ID:          _teamID,
			Name:        "Default",
			Description: "Default Team",
			CreatedBy:   user.ID,
			Users: []*models.SystemUser{
				{
					ID:      user.ID,
					Role:    "admin",
					IsAdmin: true,
				},
			},
		}

		user.DefaultTeam = &models.Team{
			ID: _teamID,
		}
		_, err = a.graphQLServer.SystemDriver.CreateTeam(ctx, _team)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
	}
	if user.DefaultOrganization == nil {
		_id := uuid.New().String()
		_org := &models.Organization{
			XKey:        _id,
			ID:          _id,
			Name:        "Default",
			Description: "Default Organization",
			Teams: []*models.Team{
				{
					ID: _teamID,
				},
			},
			Users: []*models.SystemUser{
				{
					ID:      user.ID,
					Role:    "admin",
					IsAdmin: true,
				},
			},
		}
		user.DefaultOrganization = &models.Organization{
			ID: _id,
		}
		_, err = a.graphQLServer.SystemDriver.CreateOrganization(ctx, _org)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
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

func (a *authCtrl) DemoProjectSwitch(c echo.Context) error {

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

func (a *authCtrl) ProjectNameCheck(c echo.Context) error {

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

	err := a.graphQLServer.SystemDriver.CheckProjectName(ctx, req.Name)
	if err != nil {
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

func (a *authCtrl) ProjectList(c echo.Context) error {

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
			ProjectType: p.ProjectType,
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

func (a *authCtrl) CSVTempGen(c echo.Context) error {

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

func (a *authCtrl) GetProfile(c echo.Context) error {

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

func (a *authCtrl) UpdateProfile(c echo.Context) error {

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

func (a *authCtrl) ProjectDelete(c echo.Context) error {

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
		// refresh the token
		tokens, err := utility.NewRefreshTokenAuthenticator(a.Cfg, user.RefreshToken)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}

		token := tokens.IDToken
		http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", token, true, false))
		return c.JSON(http.StatusOK, &models.HttpResponse{
			//Token:  token,
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
