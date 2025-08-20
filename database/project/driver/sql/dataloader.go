package sql

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
)

type FieldClassification struct {
	MultilineFields []string
	DoubleFields    []string
	PictureField    []string
	GalleryField    []string
	ListFields      []string
	RepeatedFields  map[string][]*models.FieldInfo
}

func (S *SQLDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	// query relations and find all docs
	query, _, err := BuildCombinedRelationQuery("--removed", "--removed", param)
	if err != nil {
		return nil, err
	}

	queryResults := []map[string]interface{}{}
	err = S.ORM.NewRaw(*query).Scan(ctx, &queryResults)
	if err != nil {
		return nil, err
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	finalResults := make(map[string][]*types.DefaultDocumentStructure)
	// format query results to finalResults
	for _, res := range queryResults {
		if val, ok := res["sys_key"].([]byte); ok {
			key := string(val)
			if docs, ok := finalResults[key]; ok {
				doc, err := CommonDocTransformation(param.Model, "en", res, classification)
				if err != nil {
					fmt.Println(err.Error())
				}
				docs = append(docs, doc)
				finalResults[key] = docs
			} else {
				doc, err := CommonDocTransformation(param.Model, "en", res, classification)
				if err != nil {
					fmt.Println(err.Error())
				}
				finalResults[key] = []*types.DefaultDocumentStructure{doc}
			}
		}
	}

	keys := param.DocumentIDs

	var results []*dataloader.Result
	switch connection["relation_type"] {
	case "has_many":
		// prepare the result
		for _, id := range keys {
			result := dataloader.Result{
				Data:  finalResults[id],
				Error: nil,
			}
			results = append(results, &result)
		}
		break
	case "has_one":
		// prepare the result
		for _, id := range keys {
			if len(finalResults[id]) > 0 {
				results = append(results, &dataloader.Result{
					Data:  finalResults[id][0], // because it has only one
					Error: nil,
				})
			} else {
				results = append(results, &dataloader.Result{
					Data:  nil, // because it has only one
					Error: nil,
				})
			}
		}
		break
	}

	return []byte{}, nil
}

func (S *SQLDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	// query relations and find all docs
	query, _, err := BuildCombinedRelationQuery("--removed", "--removed", param)
	if err != nil {
		return nil, err
	}

	queryResults := []map[string]interface{}{}
	err = S.ORM.NewRaw(*query).Scan(ctx, &queryResults)
	if err != nil {
		return nil, err
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	finalResults := make(map[string][]*types.DefaultDocumentStructure)
	// format query results to finalResults
	for _, res := range queryResults {
		if val, ok := res["sys_key"].([]byte); ok {
			key := string(val)
			if docs, ok := finalResults[key]; ok {
				doc, err := CommonDocTransformation(param.Model, "en", res, classification)
				if err != nil {
					fmt.Println(err.Error())
				}
				docs = append(docs, doc)
				finalResults[key] = docs
			} else {
				doc, err := CommonDocTransformation(param.Model, "en", res, classification)
				if err != nil {
					fmt.Println(err.Error())
				}
				finalResults[key] = []*types.DefaultDocumentStructure{doc}
			}
		}
	}

	keys := param.DocumentIDs

	var results []*dataloader.Result
	switch connection["relation_type"] {
	case "has_many":
		// prepare the result
		for _, id := range keys {
			result := dataloader.Result{
				Data:  finalResults[id],
				Error: nil,
			}
			results = append(results, &result)
		}
		break
	case "has_one":
		// prepare the result
		for _, id := range keys {
			if len(finalResults[id]) > 0 {
				results = append(results, &dataloader.Result{
					Data:  finalResults[id][0], // because it has only one
					Error: nil,
				})
			} else {
				results = append(results, &dataloader.Result{
					Data:  nil, // because it has only one
					Error: nil,
				})
			}
		}
		break
	}

	return results, nil
}
