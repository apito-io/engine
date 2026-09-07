package resolver

import (
	"context"
	"errors"
	"fmt"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/executor"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

func (s *GraphQLServer) DataLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	// graph-gophers/dataloader requires len(results) == len(keys) for every batch outcome.
	handleError := func(err error) []*dataloader.Result {
		out := make([]*dataloader.Result, len(keys))
		for i := range keys {
			out[i] = &dataloader.Result{Error: err}
		}
		return out
	}

	cache, ok := utility.LegacyApplicationCache(ctx)
	if !ok {
		return handleError(errors.New("graphql context: application cache missing"))
	}
	relationMeta, ok := utility.LegacyRelationMeta(ctx)
	if !ok {
		return handleError(errors.New("graphql context: relation_meta missing"))
	}

	param := s.NewParam(cache.Param)

	var allKeys []string
	for _, key := range keys {
		allKeys = append(allKeys, key.String())
	}

	var resolveParams *graphql.ResolveParams
	if val, ok := relationMeta["resolveParam"].(*graphql.ResolveParams); ok {
		resolveParams = val
	}
	var selectionSet *ast.SelectionSet
	if val, ok := relationMeta["selectionSet"].(*ast.SelectionSet); ok {
		selectionSet = val
	}
	var connectionModel models.ConnectionType
	if val, ok := relationMeta["connection"].(*models.ConnectionType); ok {
		connectionModel = *val
	}
	parentModel, _ := relationMeta["parentModel"].(string)

	paramSource := resolveParams.Source.(*types.DefaultDocumentStructure)
	parentID := ""
	parentType := parentModel
	if paramSource != nil {
		parentID = paramSource.ID
		if paramSource.Type != "" {
			parentType = paramSource.Type
		}
	}
	if parentID == "" {
		out := make([]*dataloader.Result, len(keys))
		for i := range keys {
			out[i] = &dataloader.Result{Data: nil}
		}
		return out
	}

	// Target schema model id comes from the parent model's connection (e.g. "employee"), not from the
	// GraphQL field name. When KnownAs is set, the field may be "waiter"/"chef" while ModelType.Name is
	// still "employee" — matching only by field name yields ModelTypeNotFound or the wrong model.
	var modelType *models.ModelType
	if connectionModel.Model != "" {
		targetKey := utility.SingularResourceName(connectionModel.Model)
		for _, _model := range cache.Project.Schema.Models {
			if _model.Name == connectionModel.Model || utility.ModelIDMatchesGraphQLField(_model.Name, targetKey) {
				modelType = _model
				break
			}
		}
	}
	if modelType == nil {
		model := utility.SingularResourceName(resolveParams.Info.FieldName)
		for _, _model := range cache.Project.Schema.Models {
			if utility.ModelIDMatchesGraphQLField(_model.Name, model) {
				modelType = _model
				break
			}
		}
	}

	if modelType == nil {
		return handleError(ae.ModelTypeNotFound)
	}

	if err := utility.AuthorizeModelRead(cache.Param.Role, modelType.Name); err != nil {
		return handleError(err)
	}

	connection := map[string]interface{}{
		"to_model":        parentType,     // issue
		"model":           modelType.Name, // comment
		"relation_type":   relationMeta["relation_type"].(string),
		"known_as":        connectionModel.KnownAs,
		"connection_type": connectionModel.Type,
	}

	param.Model = modelType
	param.KnownAs = connectionModel.KnownAs
	param.ResolveParams = resolveParams // overwrite the parent

	param.QuerySelectionSets = selectionSet
	param.DocumentIDs = allKeys

	if modelType.SinglePage {
		param.SinglePageData = true
	}

	var parentModelType *models.ModelType
	for _, m := range cache.Project.Schema.Models {
		if m != nil && (m.Name == parentType || utility.ModelIDMatchesGraphQLField(m.Name, parentType)) {
			parentModelType = m
			break
		}
	}
	relCtx := publicProjectDBContext(cache, ctx)
	if models.RelationLoaderUsesBaseDB(param, parentModelType, modelType) {
		relCtx = models.WithBaseProjectDB(relCtx)
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(relCtx)
	if err != nil {
		return handleError(err)
	}

	// Dataloader's request ctx is often already cancelled; Scan must not use it.
	_results, err := driver.RelationshipDataLoader(context.Background(), param, connection)
	if err != nil {
		return handleError(err)
	}

	if models.ParamPerTenantSeparateDB(param) && models.ModelIsCommon(param.Model) {
		if typed, ok := _results.([]*dataloader.Result); ok {
			var ids []string
			for _, r := range typed {
				if r == nil || r.Data == nil {
					continue
				}
				switch d := r.Data.(type) {
				case *types.DefaultDocumentStructure:
					if d != nil && d.ID != "" {
						ids = append(ids, d.ID)
					}
				case []*types.DefaultDocumentStructure:
					for _, doc := range d {
						if doc != nil && doc.ID != "" {
							ids = append(ids, doc.ID)
						}
					}
				}
			}
			exec, ok := s.GraphQLExecutor.(*executor.GraphQLExecutor)
			if !ok || exec == nil {
				return handleError(fmt.Errorf("dataloader: common-model hydration requires GraphQLExecutor"))
			}
			byID, hydErr := exec.LoadCommonDocsByIDs(relCtx, param, ids)
			if hydErr != nil {
				return handleError(hydErr)
			}
			for _, r := range typed {
				if r == nil || r.Data == nil {
					continue
				}
				switch d := r.Data.(type) {
				case *types.DefaultDocumentStructure:
					src := byID[d.ID]
					if src == nil {
						r.Data = nil
						continue
					}
					d.Data = src.Data
					d.Meta = src.Meta
					d.Type = src.Type
				case []*types.DefaultDocumentStructure:
					var kept []*types.DefaultDocumentStructure
					for _, doc := range d {
						if doc == nil {
							continue
						}
						src := byID[doc.ID]
						if src == nil {
							continue
						}
						doc.Data = src.Data
						doc.Meta = src.Meta
						doc.Type = src.Type
						kept = append(kept, doc)
					}
					r.Data = kept
				}
			}
			_results = typed
		}
	}

	results, ok := _results.([]*dataloader.Result)
	if !ok {
		return handleError(fmt.Errorf("dataloader: driver returned %T, expected []*dataloader.Result", _results))
	}
	if len(results) != len(keys) {
		return handleError(fmt.Errorf("dataloader: driver returned %d results for %d keys (model %q)", len(results), len(keys), modelType.Name))
	}

	return results
}
