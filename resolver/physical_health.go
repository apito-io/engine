package resolver

import (
	"fmt"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	schemasvc "github.com/apito-io/engine/services/schema"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

func modelPhysicalHealthObject() *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: "ModelPhysicalHealth",
		Fields: graphql.Fields{
			"model_name": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"table_exists": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"physical_columns": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			"expected_columns": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			"missing_columns": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			"extra_columns": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			"is_common_model": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"warnings": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
		},
	})
}

var modelPhysicalHealthGQL = modelPhysicalHealthObject()

func healthToMap(h schemasvc.ModelPhysicalHealth) map[string]interface{} {
	phys := h.PhysicalColumns
	if phys == nil {
		phys = []string{}
	}
	exp := h.ExpectedColumns
	if exp == nil {
		exp = []string{}
	}
	miss := h.MissingColumns
	if miss == nil {
		miss = []string{}
	}
	extra := h.ExtraColumns
	if extra == nil {
		extra = []string{}
	}
	warn := h.Warnings
	if warn == nil {
		warn = []string{}
	}
	return map[string]interface{}{
		"model_name":        h.ModelName,
		"table_exists":      h.TableExists,
		"physical_columns":  phys,
		"expected_columns":  exp,
		"missing_columns":   miss,
		"extra_columns":     extra,
		"is_common_model":   h.IsCommonModel,
		"warnings":          warn,
	}
}

func (s *GraphQLServer) inspectModelPhysicalHealth(
	cache *models.ApplicationCache,
	model *models.ModelType,
) (schemasvc.ModelPhysicalHealth, error) {
	if model == nil {
		return schemasvc.ModelPhysicalHealth{}, fmt.Errorf("nil model")
	}
	driver, err := s.getSchemaBaseProjectDriver(cache.Ctx)
	if err != nil {
		return schemasvc.ModelPhysicalHealth{}, err
	}

	param := s.NewParam(cache.Param)
	param.Model = model
	param.ProjectID = cache.Project.ID

	exists, err := driver.CheckTableOrCollectionExists(cache.Ctx, param)
	if err != nil {
		return schemasvc.ModelPhysicalHealth{}, err
	}

	tableName := utility.PhysicalSQLTableName(model.Name)
	var cols []string
	if exists {
		lister := physicalColumnLister(driver)
		if lister == nil {
			return schemasvc.ModelPhysicalHealth{}, fmt.Errorf(
				"physical column inspection not supported for this project database driver",
			)
		}
		cols, err = lister.ListTableColumnNames(cache.Ctx, tableName)
		if err != nil {
			return schemasvc.ModelPhysicalHealth{}, err
		}
	}
	return schemasvc.BuildModelPhysicalHealth(model, exists, cols), nil
}

// physicalColumnLister unwraps PeelProjectDB wrappers (e.g. slow-query) then
// asserts PhysicalTableColumnLister.
func physicalColumnLister(driver interfaces.ProjectDBInterface) interfaces.PhysicalTableColumnLister {
	cur := driver
	for i := 0; i < 8 && cur != nil; i++ {
		if lister, ok := cur.(interfaces.PhysicalTableColumnLister); ok && lister != nil {
			return lister
		}
		type peelable interface {
			PeelProjectDB() interfaces.ProjectDBInterface
		}
		peel, ok := cur.(peelable)
		if !ok || peel == nil {
			return nil
		}
		next := peel.PeelProjectDB()
		if next == nil || next == cur {
			return nil
		}
		cur = next
	}
	return nil
}

func (s *GraphQLServer) findProjectModel(project *models.Project, rawName string) (*models.ModelType, error) {
	if project == nil || project.Schema == nil {
		return nil, ae.ModelTypeNotFound
	}
	rawName = strings.TrimSpace(rawName)
	if rawName == "" {
		return nil, ae.ModelTypeNotFound
	}
	singular := utility.SingularResourceName(rawName)
	for _, model := range project.Schema.Models {
		if model == nil {
			continue
		}
		if utility.ModelIDMatchesGraphQLField(model.Name, singular) || model.Name == rawName {
			return model, nil
		}
	}
	return nil, ae.ModelTypeNotFound
}

// ModelPhysicalHealthResolverFn compares logical schema fields to physical columns (read-only).
func (s *GraphQLServer) ModelPhysicalHealthResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaRead); err != nil {
		return nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	rawName, _ := p.Args["model_name"].(string)
	model, err := s.findProjectModel(cache.Project, rawName)
	if err != nil {
		return nil, err
	}
	h, err := s.inspectModelPhysicalHealth(cache, model)
	if err != nil {
		return nil, err
	}
	return healthToMap(h), nil
}

// ProjectPhysicalHealthResolverFn inspects one or more models on the base project DB (read-only).
func (s *GraphQLServer) ProjectPhysicalHealthResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaRead); err != nil {
		return nil, err
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil || cache.Project.Schema == nil {
		return []interface{}{}, nil
	}

	var filter []string
	if raw, ok := p.Args["model_names"].([]interface{}); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				filter = append(filter, strings.TrimSpace(s))
			}
		}
	}

	modelsList := cache.Project.Schema.Models
	if len(filter) > 0 {
		var selected []*models.ModelType
		seen := map[string]struct{}{}
		for _, name := range filter {
			m, err := s.findProjectModel(cache.Project, name)
			if err != nil {
				return nil, err
			}
			key := strings.ToLower(m.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			selected = append(selected, m)
		}
		modelsList = selected
	}

	const maxModels = 200
	if len(modelsList) > maxModels {
		modelsList = modelsList[:maxModels]
	}

	out := make([]interface{}, 0, len(modelsList))
	for _, model := range modelsList {
		if model == nil {
			continue
		}
		h, err := s.inspectModelPhysicalHealth(cache, model)
		if err != nil {
			return nil, fmt.Errorf("model %q: %w", model.Name, err)
		}
		out = append(out, healthToMap(h))
	}
	return out, nil
}
