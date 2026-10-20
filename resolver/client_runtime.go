package resolver

import (
	"fmt"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/types"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var (
	platformClientRuntimeObject *graphql.Object
	projectClientRuntimeObject  *graphql.Object
	updateClientRuntimeInput    *graphql.InputObject
)

func init() {
	platformClientRuntimeObject = graphql.NewObject(graphql.ObjectConfig{
		Name: "PlatformClientRuntime",
		Fields: graphql.Fields{
			"platform":          &graphql.Field{Type: graphql.String},
			"min_version_name":  &graphql.Field{Type: graphql.String},
			"min_build_number":  &graphql.Field{Type: graphql.Int},
			"force_update":      &graphql.Field{Type: graphql.Boolean},
			"store_url":         &graphql.Field{Type: graphql.String},
			"message_en":        &graphql.Field{Type: graphql.String},
			"message_bn":        &graphql.Field{Type: graphql.String},
			"maintenance":       &graphql.Field{Type: graphql.Boolean},
			"maintenance_hours": &graphql.Field{Type: graphql.Int},
			"extras":            &graphql.Field{Type: scaler.ScalarJSON},
		},
	})
	projectClientRuntimeObject = graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectClientRuntime",
		Fields: graphql.Fields{
			"android": &graphql.Field{Type: platformClientRuntimeObject},
			"ios":     &graphql.Field{Type: platformClientRuntimeObject},
			"web":     &graphql.Field{Type: platformClientRuntimeObject},
		},
	})

	platformInput := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UpdatePlatformClientRuntimeInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"min_version_name":  &graphql.InputObjectFieldConfig{Type: graphql.String},
			"min_build_number":  &graphql.InputObjectFieldConfig{Type: graphql.Int},
			"force_update":      &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"store_url":         &graphql.InputObjectFieldConfig{Type: graphql.String},
			"message_en":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"message_bn":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"maintenance":       &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"maintenance_hours": &graphql.InputObjectFieldConfig{Type: graphql.Int},
			"extras":            &graphql.InputObjectFieldConfig{Type: scaler.ScalarJSON},
		},
	})
	updateClientRuntimeInput = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UpdateClientRuntimeInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"android": &graphql.InputObjectFieldConfig{Type: platformInput},
			"ios":     &graphql.InputObjectFieldConfig{Type: platformInput},
			"web":     &graphql.InputObjectFieldConfig{Type: platformInput},
		},
	})
}

func (s *GraphQLServer) ClientRuntimeResolverFn(p graphql.ResolveParams) (interface{}, error) {
	router := p.Context.Value("router").(echo.Context)
	if err := requireAccessCapability(router, authz.CapSettingsRead); err != nil {
		return nil, err
	}
	platform, _ := p.Args["platform"].(string)
	normalized, ok := models.NormalizeClientRuntimePlatform(platform)
	if !ok {
		return nil, fmt.Errorf("platform must be android, ios, or web")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	var runtime *models.ClientRuntime
	if cache.Project != nil && cache.Project.Settings != nil {
		runtime = cache.Project.Settings.ClientRuntime
	}
	return models.PlatformRuntimePayload(runtime, normalized), nil
}

func clientRuntimeSettingsResolve(p graphql.ResolveParams) (interface{}, error) {
	var settings *models.ProjectSettings
	switch src := p.Source.(type) {
	case *models.ProjectSettings:
		settings = src
	case models.ProjectSettings:
		settings = &src
	}
	var runtime *models.ClientRuntime
	if settings != nil {
		runtime = settings.ClientRuntime
	}
	return models.ClientRuntimeSnapshot(runtime), nil
}

// importClientRuntimeIfEmpty copies legacy common-model rows into project settings
// the first time an admin loads General. The public query never does this.
func (s *GraphQLServer) importClientRuntimeIfEmpty(cache *models.ApplicationCache) {
	if s == nil || cache == nil || cache.Project == nil {
		return
	}
	project := cache.Project
	if project.Settings != nil && !project.Settings.ClientRuntime.IsEmpty() {
		return
	}
	imported := models.ImportClientRuntimeFromRows(
		s.clientRuntimeRows(cache, "app_release_policy"),
		s.clientRuntimeRows(cache, "app_config"),
	)
	if imported == nil {
		return
	}
	if project.Settings == nil {
		project.Settings = &models.ProjectSettings{
			Locals:                  []string{"en"},
			IdleTenantRetentionDays: models.MinIdleTenantRetentionDays,
		}
	}
	project.Settings.ProjectID = project.ID
	project.Settings.ClientRuntime = imported
	if s.SystemDriver == nil {
		return
	}
	if err := s.SystemDriver.UpdateProject(cache.Ctx, project, true); err != nil {
		fmt.Printf("client runtime import: %v\n", err)
		return
	}
	if _, err := s.refreshProjectAndReCache(cache.Ctx, project.ID); err != nil {
		fmt.Printf("client runtime import cache: %v\n", err)
	}
}

func (s *GraphQLServer) clientRuntimeRows(cache *models.ApplicationCache, modelName string) []map[string]interface{} {
	docs := s.queryClientRuntimeModel(cache, modelName)
	rows := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || len(doc.Data) == 0 {
			continue
		}
		rows = append(rows, doc.Data)
	}
	return rows
}

func (s *GraphQLServer) queryClientRuntimeModel(cache *models.ApplicationCache, modelName string) []*types.DefaultDocumentStructure {
	if s == nil || s.GraphQLExecutor == nil || cache == nil || cache.Project == nil || cache.Project.Schema == nil || cache.Param == nil {
		return nil
	}
	var model *models.ModelType
	for _, candidate := range cache.Project.Schema.Models {
		if candidate != nil && candidate.Name == modelName {
			model = candidate
			break
		}
	}
	if model == nil {
		return nil
	}
	param := s.NewParam(cache.Param)
	param.Model = model
	param.ProjectID = cache.Project.ID
	param.ProjectSchemaModels = cache.Project.Schema.Models
	param.ResolveParams = &graphql.ResolveParams{Args: map[string]interface{}{"limit": 50, "page": 1}}
	baseCtx := models.WithBaseProjectDB(cache.Ctx)
	driver, err := s.GraphQLExecutor.GetProjectDriver(baseCtx)
	if err != nil {
		return nil
	}
	docs, err := driver.QueryMultiDocumentOfProject(baseCtx, param)
	if err != nil {
		return nil
	}
	return docs
}
