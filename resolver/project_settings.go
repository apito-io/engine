package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/schemas/objects"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var (
	projectAuthenticationSettingsObject  *graphql.Object
	updateProjectAuthenticationInputType *graphql.InputObject
	projectAuthenticationSettingsPayload *graphql.Object
	projectStorageSettingsObject         *graphql.Object
	updateProjectStorageInputType        *graphql.InputObject
	projectStorageSettingsPayload        *graphql.Object
)

func init() {
	projectAuthenticationSettingsObject = graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectAuthenticationSettings",
		Fields: graphql.Fields{
			"enable_general_auth":           &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"enable_google_auth":            &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"general_authentication_method": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"google_client_id":              &graphql.Field{Type: graphql.String},
			"google_oauth_redirect_uri":     &graphql.Field{Type: graphql.String},
			"has_google_client_secret":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"default_registration_role":     &graphql.Field{Type: graphql.String},
		},
	})

	updateProjectAuthenticationInputType = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UpdateProjectAuthenticationInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"enable_general_auth":           &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"enable_google_auth":            &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"general_authentication_method": &graphql.InputObjectFieldConfig{Type: graphql.String},
			"google_client_id":              &graphql.InputObjectFieldConfig{Type: graphql.String},
			"google_client_secret":          &graphql.InputObjectFieldConfig{Type: graphql.String},
			"google_oauth_redirect_uri":     &graphql.InputObjectFieldConfig{Type: graphql.String},
			"default_registration_role":     &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})

	projectAuthenticationSettingsPayload = graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectAuthenticationSettingsPayload",
		Fields: graphql.Fields{
			"authentication_settings": &graphql.Field{
				Type: graphql.NewNonNull(projectAuthenticationSettingsObject),
			},
		},
	})

	projectStorageSettingsObject = graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectStorageSettings",
		Fields: graphql.Fields{
			"use_free_cloud_storage":  &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"endpoint":              &graphql.Field{Type: graphql.String},
			"region":                &graphql.Field{Type: graphql.String},
			"bucket":                &graphql.Field{Type: graphql.String},
			"access_key_id":         &graphql.Field{Type: graphql.String},
			"has_secret_access_key": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"public_base_url":       &graphql.Field{Type: graphql.String},
			"force_path_style":      &graphql.Field{Type: graphql.Boolean},
		},
	})

	updateProjectStorageInputType = graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "UpdateProjectStorageInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"use_free_cloud_storage": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
			"endpoint":               &graphql.InputObjectFieldConfig{Type: graphql.String},
			"region":                 &graphql.InputObjectFieldConfig{Type: graphql.String},
			"bucket":                 &graphql.InputObjectFieldConfig{Type: graphql.String},
			"access_key_id":          &graphql.InputObjectFieldConfig{Type: graphql.String},
			"secret_access_key":      &graphql.InputObjectFieldConfig{Type: graphql.String},
			"public_base_url":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"force_path_style":       &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		},
	})

	projectStorageSettingsPayload = graphql.NewObject(graphql.ObjectConfig{
		Name: "ProjectStorageSettingsPayload",
		Fields: graphql.Fields{
			"storage_settings": &graphql.Field{
				Type: graphql.NewNonNull(projectStorageSettingsObject),
			},
		},
	})
}

func projectAuthenticationSettingsSnapshot(project *models.Project) map[string]interface{} {
	enableGeneral := true
	if project != nil && project.AuthenticationSettings != nil && project.AuthenticationSettings.EnableGeneralAuth != nil {
		enableGeneral = *project.AuthenticationSettings.EnableGeneralAuth
	}
	enableGoogle := false
	if project != nil && project.AuthenticationSettings != nil && project.AuthenticationSettings.EnableGoogleAuth != nil {
		enableGoogle = *project.AuthenticationSettings.EnableGoogleAuth
	}
	method := models.GeneralIdentifierMethod(project)
	gcid := strings.TrimSpace(models.GoogleOAuthClientID(project))
	return map[string]interface{}{
		"enable_general_auth":           enableGeneral,
		"enable_google_auth":            enableGoogle,
		"general_authentication_method": method,
		"google_client_id":              settingsNullIfEmpty(gcid),
		"google_oauth_redirect_uri":     settingsNullIfEmpty(strings.TrimSpace(models.GoogleOAuthRedirectURI(project))),
		"has_google_client_secret":      models.HasGoogleClientSecretConfigured(project),
		"default_registration_role":     settingsNullIfEmpty(models.DefaultRegistrationRoleConfigured(project)),
	}
}

func projectStorageSettingsSnapshot(project *models.Project) map[string]interface{} {
	useFree := models.UseFreeCloudStorageEffective(project)
	out := map[string]interface{}{
		"use_free_cloud_storage":  useFree,
		"has_secret_access_key": models.HasSecretAccessKeyConfigured(project),
	}
	if project == nil || project.StorageSettings == nil || useFree {
		return out
	}
	st := project.StorageSettings
	out["endpoint"] = settingsNullIfEmpty(st.Endpoint)
	out["region"] = settingsNullIfEmpty(st.Region)
	out["bucket"] = settingsNullIfEmpty(st.Bucket)
	out["access_key_id"] = settingsNullIfEmpty(st.AccessKeyID)
	out["public_base_url"] = settingsNullIfEmpty(st.PublicBaseURL)
	if st.ForcePathStyle != nil {
		out["force_path_style"] = *st.ForcePathStyle
	}
	return out
}

func settingsNullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (s *GraphQLServer) loadActiveProjectForSettings(p graphql.ResolveParams) (*models.Project, context.Context, error) {
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, nil, err
	}
	if cache.Project == nil {
		return nil, nil, errors.New("no project loaded in cache")
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	if s.SystemDriver == nil {
		return nil, nil, errors.New("system driver not available")
	}
	project, err := s.SystemDriver.GetProject(ctx, cache.Project.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load project: %w", err)
	}
	return project, ctx, nil
}

// UpdateProjectAuthenticationSettingsResolverFn persists authentication settings for the active project.
func (s *GraphQLServer) UpdateProjectAuthenticationSettingsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	project, ctx, err := s.loadActiveProjectForSettings(p)
	if err != nil {
		return nil, err
	}
	raw, ok := p.Args["input"]
	if !ok || raw == nil {
		return nil, errors.New("input is required")
	}
	input, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("input: expected object")
	}
	nextAuth, err := models.ApplyUpdateProjectAuthenticationInput(project, input)
	if err != nil {
		return nil, err
	}
	if err := s.SystemDriver.SaveProjectAuthenticationSettings(ctx, project.ID, nextAuth); err != nil {
		return nil, err
	}
	updated, err := s.SystemDriver.GetProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"authentication_settings": projectAuthenticationSettingsSnapshot(updated),
	}, nil
}

// UpdateProjectStorageSettingsResolverFn persists storage settings for the active project.
func (s *GraphQLServer) UpdateProjectStorageSettingsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	project, ctx, err := s.loadActiveProjectForSettings(p)
	if err != nil {
		return nil, err
	}
	raw, ok := p.Args["input"]
	if !ok || raw == nil {
		return nil, errors.New("input is required")
	}
	input, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("input: expected object")
	}
	nextStorage, err := models.ApplyUpdateProjectStorageInput(project, input, models.HasSecretAccessKeyConfigured(project))
	if err != nil {
		return nil, err
	}
	if err := s.SystemDriver.SaveProjectStorageSettings(ctx, project.ID, nextStorage); err != nil {
		return nil, err
	}
	updated, err := s.SystemDriver.GetProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	if _, err := s.refreshProjectAndReCache(ctx, project.ID); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"storage_settings": projectStorageSettingsSnapshot(updated),
	}, nil
}

// StoragePluginInitEnvs loads project storage settings and builds plugin Init env vars.
func (s *GraphQLServer) StoragePluginInitEnvs(ctx context.Context, projectID string) ([]*protobuff.EnvVariable, error) {
	if s.SystemDriver == nil {
		return nil, errors.New("system driver not available")
	}
	project, err := s.SystemDriver.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return models.BuildStoragePluginEnvVars(project, s.Cfg)
}

func registerProjectSettingsGraphQLFields(objs *objects.ObjectModels) {
	if objs == nil || objs.ProjectDetailsObject == nil {
		return
	}
	objs.ProjectDetailsObject.AddFieldConfig("authentication_settings", &graphql.Field{
		Type: projectAuthenticationSettingsObject,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			switch src := p.Source.(type) {
			case *models.Project:
				return projectAuthenticationSettingsSnapshot(src), nil
			default:
				return nil, nil
			}
		},
	})
	objs.ProjectDetailsObject.AddFieldConfig("storage_settings", &graphql.Field{
		Type: projectStorageSettingsObject,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			switch src := p.Source.(type) {
			case *models.Project:
				return projectStorageSettingsSnapshot(src), nil
			default:
				return nil, nil
			}
		},
	})
}
