package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/relationmigrate"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var relationTransitionPreviewType = graphql.NewObject(graphql.ObjectConfig{
	Name: "RelationTransitionPreview",
	Fields: graphql.Fields{
		"plan_hash":                &graphql.Field{Type: graphql.String},
		"kind":                     &graphql.Field{Type: graphql.String},
		"safety":                   &graphql.Field{Type: graphql.String},
		"reversibility":            &graphql.Field{Type: graphql.String},
		"blocked":                  &graphql.Field{Type: graphql.Boolean},
		"backup_required":          &graphql.Field{Type: graphql.Boolean},
		"source_storage":           &graphql.Field{Type: graphql.String},
		"target_storage":           &graphql.Field{Type: graphql.String},
		"required_acknowledgement": &graphql.Field{Type: graphql.String},
		"preview_json":             &graphql.Field{Type: graphql.String},
		"disclaimers":              &graphql.Field{Type: graphql.NewList(graphql.String)},
		"warnings":                 &graphql.Field{Type: graphql.NewList(graphql.String)},
		"blockers":                 &graphql.Field{Type: graphql.NewList(graphql.String)},
	},
})

var relationTransitionStageType = graphql.NewObject(graphql.ObjectConfig{
	Name: "RelationTransitionStageResult",
	Fields: graphql.Fields{
		"staged":    &graphql.Field{Type: graphql.Boolean},
		"message":   &graphql.Field{Type: graphql.String},
		"plan_hash": &graphql.Field{Type: graphql.String},
	},
})

func relationTransitionArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"from_model":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"to_model":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"known_as":             &graphql.ArgumentConfig{Type: graphql.String},
		"from_relation":        &graphql.ArgumentConfig{Type: graphql.String},
		"to_relation":          &graphql.ArgumentConfig{Type: graphql.String},
		"target_from_relation": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"target_to_relation":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"schema_version":       &graphql.ArgumentConfig{Type: graphql.String},
	}
}

func (s *GraphQLServer) CollectRelationTransitionPreview(ctx context.Context, project *models.Project, req relationmigrate.Request) (relationmigrate.Preview, error) {
	var snaps []relationmigrate.DatabaseSnapshot
	source, target := "", ""
	add := func(callCtx context.Context, driver interfaces.ProjectDBInterface) error {
		if driver == nil {
			return nil
		}
		preview, err := driver.PreviewRelationCardinalityMigration(callCtx, &req)
		if err != nil {
			return err
		}
		if preview == nil {
			return nil
		}
		source, target = preview.SourceStorage, preview.TargetStorage
		snaps = append(snaps, preview.Databases...)
		return nil
	}
	base, skip, err := s.getSchemaBaseProjectDriverIfNeeded(ctx, project, nil)
	if err != nil {
		return relationmigrate.Preview{}, err
	}
	if !skip {
		if err := add(ctx, base); err != nil {
			return relationmigrate.Preview{}, err
		}
	}
	if s != nil && s.Cfg != nil && s.Cfg.SchemaIterateHook != nil {
		if err := s.Cfg.SchemaIterateHook(ctx, project, func(tenantCtx context.Context, driver interface{}) error {
			typed, _ := driver.(interfaces.ProjectDBInterface)
			return add(tenantCtx, typed)
		}); err != nil {
			return relationmigrate.Preview{}, err
		}
	}
	if len(snaps) == 0 {
		snaps = append(snaps, relationmigrate.DatabaseSnapshot{ScopeKey: "none", Blocked: true, Capability: relationmigrate.CapabilityUnsupported, Error: "no project database available"})
	}
	return relationmigrate.BuildPreview(req, source, target, snaps), nil
}

func (s *GraphQLServer) cachedSchemaProject(p graphql.ResolveParams, capability string) (*models.ApplicationCache, *models.Project, error) {
	router, _ := p.Context.Value("router").(echo.Context)
	if router == nil {
		return nil, nil, fmt.Errorf("request context required")
	}
	if err := requireAccessCapability(router, capability); err != nil {
		return nil, nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, nil, err
	}
	if cache == nil || cache.Project == nil {
		return nil, nil, fmt.Errorf("project is required")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, nil, err
	}
	return cache, cache.Project, nil
}

func relationRequestFromArgs(args map[string]interface{}) (relationmigrate.Request, error) {
	req := relationmigrate.Request{
		FromModel:          strings.TrimSpace(fmt.Sprint(args["from_model"])),
		ToModel:            strings.TrimSpace(fmt.Sprint(args["to_model"])),
		KnownAs:            strings.TrimSpace(stringOrEmpty(args["known_as"])),
		FromRelation:       strings.TrimSpace(stringOrEmpty(args["from_relation"])),
		ToRelation:         strings.TrimSpace(stringOrEmpty(args["to_relation"])),
		TargetFromRelation: strings.TrimSpace(stringOrEmpty(args["target_from_relation"])),
		TargetToRelation:   strings.TrimSpace(stringOrEmpty(args["target_to_relation"])),
		ConflictPolicy:     relationmigrate.ConflictPolicyAbort,
		SchemaVersion:      intArg(args["schema_version"]),
	}
	if req.FromModel == "" || req.FromModel == "<nil>" || req.ToModel == "" || req.TargetFromRelation == "" || req.TargetToRelation == "" {
		return req, fmt.Errorf("from_model, to_model, target_from_relation, and target_to_relation are required")
	}
	return req, nil
}

func stringOrEmpty(v interface{}) string {
	s, _ := v.(string)
	return s
}

func intArg(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func (s *GraphQLServer) PreviewSchemaRelationTransitionResolverFn(p graphql.ResolveParams) (interface{}, error) {
	cache, project, err := s.cachedSchemaProject(p, CapSchemaRead)
	if err != nil {
		return nil, err
	}
	req, err := relationRequestFromArgs(p.Args)
	if err != nil {
		return nil, err
	}
	preview, err := s.CollectRelationTransitionPreview(cache.Ctx, project, req)
	if err != nil {
		return nil, err
	}
	return relationPreviewMap(preview), nil
}

func relationPreviewMap(preview relationmigrate.Preview) map[string]interface{} {
	raw, _ := json.Marshal(preview)
	return map[string]interface{}{
		"plan_hash":                preview.PlanHash,
		"kind":                     preview.TransitionKind,
		"safety":                   preview.SafetyClass,
		"reversibility":            preview.Reversibility,
		"blocked":                  preview.Blocked,
		"backup_required":          preview.BackupRequired,
		"source_storage":           preview.SourceStorage,
		"target_storage":           preview.TargetStorage,
		"required_acknowledgement": preview.RequiredAcknowledgement,
		"disclaimers":              nonNilStrings(preview.Disclaimers),
		"warnings":                 nonNilStrings(preview.Warnings),
		"blockers":                 nonNilStrings(preview.Blockers),
		"preview_json":             string(raw),
	}
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func (s *GraphQLServer) StageRelationTransitionResolverFn(p graphql.ResolveParams) (interface{}, error) {
	cache, project, err := s.cachedSchemaProject(p, CapSchemaWrite)
	if err != nil {
		return nil, err
	}
	req, err := relationRequestFromArgs(p.Args)
	if err != nil {
		return nil, err
	}
	preview, err := s.CollectRelationTransitionPreview(cache.Ctx, project, req)
	if err != nil {
		return nil, err
	}
	if preview.Blocked {
		return nil, fmt.Errorf("relation transition is blocked: %s", strings.Join(preview.Blockers, "; "))
	}
	if !relationmigrate.SamePlan(stringOrEmpty(p.Args["plan_hash"]), preview) {
		return nil, fmt.Errorf("plan_hash is stale; preview the transition again")
	}
	ack := stringOrEmpty(p.Args["acknowledgement"])
	if strings.TrimSpace(preview.RequiredAcknowledgement) != "" && ack != preview.RequiredAcknowledgement {
		return nil, fmt.Errorf("acknowledgement must exactly match %q", preview.RequiredAcknowledgement)
	}
	args := map[string]interface{}{
		"from": req.FromModel, "to": req.ToModel, "known_as": req.KnownAs,
		"forward_connection_type": req.TargetFromRelation, "reverse_connection_type": req.TargetToRelation,
		"target_from_relation": req.TargetFromRelation, "target_to_relation": req.TargetToRelation,
		"plan_hash": preview.PlanHash, "conflict_policy": relationmigrate.ConflictPolicyAbort,
		"databases": preview.Databases, "safety": preview.SafetyClass, "reversibility": preview.Reversibility,
	}
	_, err = s.tryStageSchemaMutation(cache, project, models.SchemaOpTypeUpdateConnection, args, func() (interface{}, error) {
		fromModel, toModel := findSchemaModel(project, req.FromModel), findSchemaModel(project, req.ToModel)
		driver, skip, err := s.getSchemaBaseProjectDriverIfNeeded(cache.Ctx, project, fromModel)
		if err != nil {
			return nil, err
		}
		if err := s.runSchemaChange(cache.Ctx, schemasvc.RunInput{
			Ctx: cache.Ctx, Project: project,
			OperationType: models.SchemaOpTypeUpdateConnection, Request: args,
			BaseDriver: driver, SkipBaseDDL: skip, PhysicalDDLRequired: true,
			ApplyDDL: func(ctx context.Context, d interfaces.ProjectDBInterface) error {
				ctx = relationmigrate.WithExpectedSnapshots(ctx, preview.Databases)
				_, err := d.MigrateRelationCardinality(ctx, &req)
				return err
			},
			PersistSystem: func() error {
				applyTargetRelation(project.Schema, req)
				if err := s.SystemDriver.TouchProjectUpdatedAt(cache.Ctx, project.ID); err != nil {
					return err
				}
				if fromModel != nil {
					if err := s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, fromModel); err != nil {
						return err
					}
				}
				if toModel != nil && toModel != fromModel {
					return s.SystemDriver.UpsertModelType(cache.Ctx, project.ID, toModel)
				}
				return nil
			},
			RefreshCache: func() error {
				_, err := s.refreshProjectAndReCache(cache.Ctx, project.ID)
				return err
			},
		}); err != nil {
			return nil, err
		}
		return map[string]interface{}{"staged": true, "message": "Applied", "plan_hash": preview.PlanHash}, nil
	})
	if err != nil {
		return nil, err
	}
	message := "Staged, not applied"
	if !s.schemaVersioningActive(cache.Ctx) {
		message = "Applied"
	}
	return map[string]interface{}{"staged": true, "message": message, "plan_hash": preview.PlanHash}, nil
}

func findSchemaModel(project *models.Project, name string) *models.ModelType {
	if project == nil || project.Schema == nil {
		return nil
	}
	for _, model := range project.Schema.Models {
		if model != nil && model.Name == name {
			return model
		}
	}
	return nil
}

func applyTargetRelation(schema *models.ProjectSchema, req relationmigrate.Request) {
	if schema == nil {
		return
	}
	for _, model := range schema.Models {
		if model == nil {
			continue
		}
		for _, conn := range model.Connections {
			if conn == nil || conn.KnownAs != req.KnownAs {
				continue
			}
			if model.Name == req.FromModel && conn.Model == req.ToModel {
				conn.Relation = req.TargetFromRelation
			}
			if model.Name == req.ToModel && conn.Model == req.FromModel {
				conn.Relation = req.TargetToRelation
			}
		}
	}
}

func rejectStorageChangingConnectionUpsert(project *models.Project, from, to, known, forward, reverse string) error {
	if project == nil || project.Schema == nil || forward == "" || reverse == "" {
		return nil
	}
	var currentForward, currentReverse string
	for _, model := range project.Schema.Models {
		if model == nil {
			continue
		}
		for _, conn := range model.Connections {
			if conn == nil || conn.KnownAs != known {
				continue
			}
			if model.Name == from && conn.Model == to {
				currentForward = conn.Relation
			}
			if model.Name == to && conn.Model == from {
				currentReverse = conn.Relation
			}
		}
	}
	if currentForward == "" || currentReverse == "" {
		return nil
	}
	if relationmigrate.StorageChanging(currentForward, currentReverse, forward, reverse) {
		return fmt.Errorf("changing relation storage requires previewSchemaRelationTransition and stageRelationTransition")
	}
	return nil
}
