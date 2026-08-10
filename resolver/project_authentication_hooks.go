package resolver

import (
	"context"
	"sync"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

// ProjectAuthenticationSettingsFieldsHook lets the host extend ProjectAuthenticationSettings
// output fields. Open-core does not name host fields.
type ProjectAuthenticationSettingsFieldsHook func(obj *graphql.Object)

// UpdateProjectAuthenticationInputFieldsHook lets the host extend UpdateProjectAuthenticationInput.
// Open-core does not name host fields.
type UpdateProjectAuthenticationInputFieldsHook func(input *graphql.InputObject)

// ProjectAuthenticationSettingsSnapshotHook lets the host enrich the authentication_settings
// snapshot map after open-core fills base fields.
type ProjectAuthenticationSettingsSnapshotHook func(project *models.Project, snap map[string]interface{})

// AfterUpdateProjectAuthenticationSettingsHook runs after open-core persists base
// authentication_settings. Host may persist extra input keys (open-core ignores unknown keys
// in ApplyUpdateProjectAuthenticationInput).
type AfterUpdateProjectAuthenticationSettingsHook func(ctx context.Context, projectID string, input map[string]interface{}) error

var projectAuthenticationSettingsSchemaHooksOnce sync.Once

func (s *GraphQLServer) applyProjectAuthenticationSettingsSchemaHooks() {
	if s == nil {
		return
	}
	projectAuthenticationSettingsSchemaHooksOnce.Do(func() {
		if s.Cfg == nil {
			return
		}
		if s.Cfg.ProjectAuthenticationSettingsFieldsHook != nil {
			if hook, ok := s.Cfg.ProjectAuthenticationSettingsFieldsHook.(ProjectAuthenticationSettingsFieldsHook); ok && hook != nil {
				hook(projectAuthenticationSettingsObject)
			}
		}
		if s.Cfg.UpdateProjectAuthenticationInputFieldsHook != nil {
			if hook, ok := s.Cfg.UpdateProjectAuthenticationInputFieldsHook.(UpdateProjectAuthenticationInputFieldsHook); ok && hook != nil {
				hook(updateProjectAuthenticationInputType)
			}
		}
	})
}

func (s *GraphQLServer) enrichAuthenticationSettingsSnapshot(project *models.Project, snap map[string]interface{}) map[string]interface{} {
	if snap == nil {
		snap = map[string]interface{}{}
	}
	if s == nil || s.Cfg == nil || s.Cfg.ProjectAuthenticationSettingsSnapshotHook == nil {
		return snap
	}
	hook, ok := s.Cfg.ProjectAuthenticationSettingsSnapshotHook.(ProjectAuthenticationSettingsSnapshotHook)
	if !ok || hook == nil {
		return snap
	}
	hook(project, snap)
	return snap
}

func (s *GraphQLServer) runAfterUpdateProjectAuthenticationSettings(ctx context.Context, projectID string, input map[string]interface{}) error {
	if s == nil || s.Cfg == nil || s.Cfg.AfterUpdateProjectAuthenticationSettingsHook == nil {
		return nil
	}
	hook, ok := s.Cfg.AfterUpdateProjectAuthenticationSettingsHook.(AfterUpdateProjectAuthenticationSettingsHook)
	if !ok || hook == nil {
		return nil
	}
	return hook(ctx, projectID, input)
}

// AuthenticationSettingsSnapshot builds the base+host snapshot for GraphQL.
func (s *GraphQLServer) AuthenticationSettingsSnapshot(project *models.Project) map[string]interface{} {
	s.applyProjectAuthenticationSettingsSchemaHooks()
	snap := projectAuthenticationSettingsSnapshot(project)
	return s.enrichAuthenticationSettingsSnapshot(project, snap)
}
