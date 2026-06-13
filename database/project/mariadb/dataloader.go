package mariadb

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
)

// relationBatchRowParentKey reads the parent id column produced by BuildCombinedRelationQuery.
func relationBatchRowParentKey(row map[string]interface{}) (string, bool) {
	if row == nil {
		return "", false
	}
	for _, col := range []string{"sys_key", "key"} {
		v, ok := row[col]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case []byte:
			if len(t) > 0 {
				return string(t), true
			}
		case string:
			if t != "" {
				return t, true
			}
		}
	}
	return "", false
}

// relationParentModel returns the parent (anchor) model name for BuildCombinedRelationQuery.
// Public and executor dataloaders set connection["to_model"] to the source document type (e.g. person)
// and connection["model"] to the related schema model being loaded (e.g. task).
func relationParentModel(connection map[string]interface{}) (string, error) {
	if connection == nil {
		return "", fmt.Errorf("RelationshipDataLoader: connection is nil")
	}
	v, ok := connection["to_model"].(string)
	if !ok || v == "" {
		return "", fmt.Errorf("RelationshipDataLoader: connection missing to_model (parent model)")
	}
	return utility.PhysicalSQLTableName(v), nil
}

// withMergedConnectionArgs copies ResolveParams.Args and overwrites relation geometry keys from
// connection so BuildCombinedRelationQuery sees from_model/to_model/relation_type even when the
// parent list resolver left non-empty Args (pagination, filters, etc.).
func withMergedConnectionArgs(param *models.CommonSystemParams, connection map[string]interface{}) (restore func()) {
	if param == nil || param.ResolveParams == nil {
		return func() {}
	}
	orig := param.ResolveParams.Args
	merged := make(map[string]interface{}, len(orig)+8)
	for k, v := range orig {
		merged[k] = v
	}
	if connection != nil {
		if v, ok := connection["to_model"].(string); ok && v != "" {
			merged["from_model"] = utility.PhysicalSQLTableName(v)
		}
		if v, ok := connection["model"].(string); ok && v != "" {
			merged["to_model"] = utility.PhysicalSQLTableName(v)
		}
		for _, k := range []string{"relation_type", "connection_type", "known_as"} {
			if v, ok := connection[k]; ok {
				merged[k] = v
			}
		}
		if v, ok := connection["_id"]; ok {
			merged["_id"] = v
		}
	}
	param.ResolveParams.Args = merged
	return func() { param.ResolveParams.Args = orig }
}

type FieldClassification struct {
	MultilineFields []string
	PictureField    []string
	GalleryField    []string
	ListFields      []string
	RepeatedFields  map[string][]*models.FieldInfo
	ObjectField     []string
	BooleanFields   []string // this is needed because in turso and sqlite boolean is stored as integer
	DateFields      []string
}

func (d *Driver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	parentModel, err := relationParentModel(connection)
	if err != nil {
		return nil, err
	}
	relType, _ := connection["relation_type"].(string)
	restoreArgs := withMergedConnectionArgs(param, connection)
	defer restoreArgs()

	query, relArgs, _, err := BuildCombinedRelationQuery(d.Conf, relType, parentModel, param)
	if err != nil {
		return nil, err
	}

	queryResults := []map[string]interface{}{}
	if len(relArgs) > 0 {
		err = d.ORM.NewRaw(query, relArgs...).Scan(ctx, &queryResults)
	} else {
		err = d.ORM.NewRaw(query).Scan(ctx, &queryResults)
	}
	if err != nil {
		return nil, err
	}

	classification := d.BuildFieldClassification(param.Model.Fields)

	finalResults := make(map[string][]*types.DefaultDocumentStructure)
	for _, res := range queryResults {
		key, ok := relationBatchRowParentKey(res)
		if !ok {
			continue
		}
		doc, err := CommonDocTransformation(param.Model, "en", res, classification)
		if err != nil || doc == nil {
			continue
		}
		finalResults[key] = append(finalResults[key], doc)
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

func (d *Driver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	parentModel, err := relationParentModel(connection)
	if err != nil {
		return nil, err
	}
	relType, _ := connection["relation_type"].(string)
	restoreArgs := withMergedConnectionArgs(param, connection)
	defer restoreArgs()

	query, relArgs, _, err := BuildCombinedRelationQuery(d.Conf, relType, parentModel, param)
	if err != nil {
		return nil, err
	}

	queryResults := []map[string]interface{}{}
	if len(relArgs) > 0 {
		err = d.ORM.NewRaw(query, relArgs...).Scan(ctx, &queryResults)
	} else {
		err = d.ORM.NewRaw(query).Scan(ctx, &queryResults)
	}
	if err != nil {
		return nil, err
	}

	classification := d.BuildFieldClassification(param.Model.Fields)

	finalResults := make(map[string][]*types.DefaultDocumentStructure)
	for _, res := range queryResults {
		key, ok := relationBatchRowParentKey(res)
		if !ok {
			continue
		}
		doc, err := CommonDocTransformation(param.Model, "en", res, classification)
		if err != nil || doc == nil {
			continue
		}
		finalResults[key] = append(finalResults[key], doc)
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
