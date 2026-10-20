package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

type commonModelShadowReducer interface {
	ReduceModelTableToShadow(ctx context.Context, modelName string) error
}

type commonModelColumnEnsurer interface {
	EnsureModelUserFieldColumns(ctx context.Context, model *models.ModelType) error
	CreateModelTable(ctx context.Context, model *models.ModelType, ifNotExists bool) error
}

type distinctColumnLister interface {
	ListDistinctNonEmptyColumn(ctx context.Context, table, column string) ([]string, error)
}

type shadowIDWriter interface {
	InsertShadowIDs(ctx context.Context, table string, ids []string) (int, error)
}

// CommonModelReconcileResult is returned by reconcileCommonModels.
type CommonModelReconcileResult struct {
	Promoted       int `json:"promoted"`
	ShadowsFixed   int `json:"shadows_fixed"`
	OrphansRemoved int `json:"orphans_removed"`
	TablesReduced  int `json:"tables_reduced"`
}

func (s *GraphQLServer) guardCommonModelToggle(cache *models.ApplicationCache, project *models.Project, modelName string, args map[string]interface{}) error {
	if cache == nil || project == nil || project.Schema == nil {
		return nil
	}
	want, ok := args["is_common_model"].(bool)
	if !ok {
		return nil
	}
	var model *models.ModelType
	for _, m := range project.Schema.Models {
		if m != nil && m.Name == modelName {
			model = m
			break
		}
	}
	if model == nil || models.ModelIsCommon(model) == want {
		return nil
	}
	strategy, _ := args["common_model_migration"].(string)
	hasRows, err := s.modelHasDocuments(cache.Ctx, project, cache.Param, model)
	if err != nil {
		return err
	}
	return validateCommonModelToggle(want, strategy, hasRows, model.Name)
}

func (s *GraphQLServer) modelHasDocuments(ctx context.Context, project *models.Project, parent *models.CommonSystemParams, model *models.ModelType) (bool, error) {
	param := s.NewParam(parent)
	param.Model = model
	param.ProjectID = project.ID
	param.ResolveParams = &graphql.ResolveParams{Args: map[string]interface{}{"limit": 1, "page": 1}}
	try := func(c context.Context, d interfaces.ProjectDBInterface) bool {
		if d == nil {
			return false
		}
		docs, err := d.QueryMultiDocumentOfProject(c, param)
		return err == nil && len(docs) > 0
	}
	base, err := s.getSchemaBaseProjectDriver(ctx)
	if err == nil && try(models.WithBaseProjectDB(ctx), base) {
		return true, nil
	}
	found := false
	if s.Cfg != nil && s.Cfg.SchemaIterateHook != nil {
		_ = s.Cfg.SchemaIterateHook(ctx, project, func(tctx context.Context, driver interface{}) error {
			if found {
				return nil
			}
			td, ok := driver.(interfaces.ProjectDBInterface)
			if ok && try(tctx, td) {
				found = true
			}
			return nil
		})
	}
	return found, nil
}

func (s *GraphQLServer) ReconcileCommonModelsResolverFn(p graphql.ResolveParams) (interface{}, error) {
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
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	modelName, _ := p.Args["model_name"].(string)
	result, err := s.reconcileCommonModels(cache.Ctx, cache.Project, cache.Param, strings.TrimSpace(modelName))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"promoted":        result.Promoted,
		"shadows_fixed":   result.ShadowsFixed,
		"orphans_removed": result.OrphansRemoved,
		"tables_reduced":  result.TablesReduced,
	}, nil
}

// ReconcileCommonModels runs promotion, missing-shadow repair, and orphan GC.
func (s *GraphQLServer) ReconcileCommonModels(ctx context.Context, project *models.Project, parentParam *models.CommonSystemParams, onlyModel string) (*CommonModelReconcileResult, error) {
	return s.reconcileCommonModels(ctx, project, parentParam, onlyModel)
}

func (s *GraphQLServer) reconcileCommonModels(ctx context.Context, project *models.Project, parentParam *models.CommonSystemParams, onlyModel string) (*CommonModelReconcileResult, error) {
	out := &CommonModelReconcileResult{}
	if project == nil || project.Schema == nil {
		return out, nil
	}
	base, err := s.getSchemaBaseProjectDriver(ctx)
	if err != nil {
		return nil, err
	}
	if parentParam == nil {
		parentParam = &models.CommonSystemParams{ProjectID: project.ID}
	}
	parentParam.ProjectSchemaModels = project.Schema.Models
	if err := base.InitProjectBase(models.WithBaseProjectDB(ctx), parentParam, nil); err != nil {
		return nil, fmt.Errorf("ensure base project storage: %w", err)
	}
	for _, model := range project.Schema.Models {
		if model == nil || !models.ModelIsCommon(model) {
			continue
		}
		if onlyModel != "" && !utility.ModelIDMatchesGraphQLField(model.Name, onlyModel) && !strings.EqualFold(model.Name, onlyModel) {
			continue
		}
		if ensurer, ok := peelAs[commonModelColumnEnsurer](base); ok {
			if err := ensurer.CreateModelTable(ctx, model, true); err != nil {
				return nil, err
			}
			if err := ensurer.EnsureModelUserFieldColumns(ctx, model); err != nil {
				return nil, err
			}
		}
		if s.Cfg == nil || s.Cfg.SchemaIterateHook == nil {
			continue
		}
		param := s.NewParam(parentParam)
		param.Model = model
		param.ProjectID = project.ID
		param.ProjectSchemaModels = project.Schema.Models
		err := s.Cfg.SchemaIterateHook(ctx, project, func(tctx context.Context, driver interface{}) error {
			td, ok := driver.(interfaces.ProjectDBInterface)
			if !ok || td == nil {
				return nil
			}
			promoted, reduced, err := s.promoteTenantCommonRows(tctx, base, td, param, model)
			if err != nil {
				return err
			}
			out.Promoted += promoted
			if reduced {
				out.TablesReduced++
			}
			orphans, shadows, err := s.repairCommonShadows(tctx, base, td, param, model, project.Schema.Models)
			if err != nil {
				return err
			}
			out.OrphansRemoved += orphans
			out.ShadowsFixed += shadows
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *GraphQLServer) promoteTenantCommonRows(ctx context.Context, base, tenant interfaces.ProjectDBInterface, param *models.CommonSystemParams, model *models.ModelType) (int, bool, error) {
	pageSize := commonPromotePageSize()
	io := commonPromoteIO{
		listTenant: func(page, limit int) ([]*types.DefaultDocumentStructure, error) {
			listParam := *param
			listParam.Model = model
			listParam.DocumentIDs = nil
			listParam.ResolveParams = &graphql.ResolveParams{
				Args: map[string]interface{}{
					"limit": limit,
					"page":  page,
				},
			}
			docs, err := tenant.QueryMultiDocumentOfProject(ctx, &listParam)
			if err != nil {
				return nil, nil
			}
			return docs, nil
		},
		getBase: func(id string) (*types.DefaultDocumentStructure, error) {
			baseParam := *param
			baseParam.DocumentID = id
			existing, err := base.GetSingleProjectDocument(models.WithBaseProjectDB(ctx), &baseParam)
			if err != nil {
				return nil, nil
			}
			return existing, nil
		},
		addBase: func(doc *types.DefaultDocumentStructure) error {
			_, err := base.AddDocumentToProject(models.WithBaseProjectDB(ctx), param, doc)
			return err
		},
		baseHas: func(id string) (bool, error) {
			baseParam := *param
			baseParam.DocumentID = id
			existing, _ := base.GetSingleProjectDocument(models.WithBaseProjectDB(ctx), &baseParam)
			return existing != nil && !commonDocIsSoftDeleted(existing), nil
		},
		reduce: func() error {
			reducer, ok := peelAs[commonModelShadowReducer](tenant)
			if !ok {
				return nil
			}
			return reducer.ReduceModelTableToShadow(ctx, model.Name)
		},
	}
	return runCommonModelPromotion(pageSize, io)
}

func (s *GraphQLServer) repairCommonShadows(
	ctx context.Context,
	base, tenant interfaces.ProjectDBInterface,
	param *models.CommonSystemParams,
	model *models.ModelType,
	schemaModels []*models.ModelType,
) (int, int, error) {
	listParam := *param
	listParam.Model = model
	listParam.ResolveParams = &graphql.ResolveParams{
		Args: map[string]interface{}{"limit": commonPromotePageSize(), "page": 1},
	}
	shadowSet := map[string]struct{}{}
	for page := 1; ; page++ {
		listParam.ResolveParams.Args["page"] = page
		listParam.ResolveParams.Args["limit"] = commonPromotePageSize()
		shadows, err := tenant.QueryMultiDocumentOfProject(ctx, &listParam)
		if err != nil || len(shadows) == 0 {
			break
		}
		for _, doc := range shadows {
			if doc != nil && strings.TrimSpace(doc.ID) != "" {
				shadowSet[strings.TrimSpace(doc.ID)] = struct{}{}
			}
		}
		if len(shadows) < commonPromotePageSize() {
			break
		}
	}

	orphans := 0
	for id := range shadowSet {
		baseParam := *param
		baseParam.DocumentID = id
		existing, _ := base.GetSingleProjectDocument(models.WithBaseProjectDB(ctx), &baseParam)
		if existing == nil || commonDocIsSoftDeleted(existing) {
			shadowModel := *model
			shadowModel.Ext = nil
			_ = tenant.DeleteDocumentFromProject(ctx, &models.CommonSystemParams{
				Model:      &shadowModel,
				DocumentID: id,
				ProjectID:  param.ProjectID,
			})
			delete(shadowSet, id)
			orphans++
		}
	}

	refs := map[string]struct{}{}
	if lister, ok := peelAs[distinctColumnLister](tenant); ok {
		for _, src := range commonModelShadowRefSources(schemaModels, model) {
			ids, err := lister.ListDistinctNonEmptyColumn(ctx, src.Table, src.Column)
			if err != nil {
				return orphans, 0, err
			}
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id != "" {
					refs[id] = struct{}{}
				}
			}
		}
	}

	var missing []string
	for id := range refs {
		if _, ok := shadowSet[id]; ok {
			continue
		}
		baseParam := *param
		baseParam.DocumentID = id
		existing, _ := base.GetSingleProjectDocument(models.WithBaseProjectDB(ctx), &baseParam)
		if existing == nil || commonDocIsSoftDeleted(existing) {
			continue
		}
		missing = append(missing, id)
	}
	fixed := 0
	if len(missing) > 0 {
		if writer, ok := peelAs[shadowIDWriter](tenant); ok {
			n, err := writer.InsertShadowIDs(ctx, utility.PhysicalSQLTableName(model.Name), missing)
			if err != nil {
				return orphans, 0, err
			}
			fixed = n
		}
	}
	return orphans, fixed, nil
}

func peelAs[T any](driver interfaces.ProjectDBInterface) (T, bool) {
	var zero T
	cur := driver
	for i := 0; i < 8 && cur != nil; i++ {
		if v, ok := any(cur).(T); ok {
			return v, true
		}
		type peelable interface {
			PeelProjectDB() interfaces.ProjectDBInterface
		}
		p, ok := cur.(peelable)
		if !ok {
			return zero, false
		}
		next := p.PeelProjectDB()
		if next == nil || next == cur {
			return zero, false
		}
		cur = next
	}
	return zero, false
}
