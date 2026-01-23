package resolver

import (
	"context"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

func (s *GraphQLServer) DataLoaderFn(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {

	handleError := func(err error) []*dataloader.Result {
		var results []*dataloader.Result
		var result dataloader.Result
		result.Error = err
		results = append(results, &result)
		return results
	}

	var (
		v            = ctx.Value
		cache        = v("cache").(*models.ApplicationCache)
		relationMeta = v("relation_meta").(map[string]interface{})
	)

	param := s.NewParam(cache.Param)

	var queryIds []string
	for _, key := range keys {
		queryIds = append(queryIds, key.String())
	}

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

	paramSource := resolveParams.Source.(*types.DefaultDocumentStructure)

	model := utility.SingularResourceName(resolveParams.Info.FieldName)

	var modelType *models.ModelType
	for _, _model := range cache.Project.Schema.Models {
		if _model.Name == model {
			modelType = _model
		}
	}

	if modelType == nil {
		return handleError(ae.ModelTypeNotFound)
	}

	connection := map[string]interface{}{
		"to_model":        paramSource.Type, // issue
		"model":           modelType.Name,   // comment
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

	// do not use old passed ctx, instead use the new one, because dataloader is a concurrent process
	// the old ctx might be already cancelled
	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return handleError(err)
	}

	_results, err := driver.RelationshipDataLoader(context.Background(), param, connection)
	if err != nil {
		return handleError(err)
	}

	results := _results.([]*dataloader.Result)

	return results
}
