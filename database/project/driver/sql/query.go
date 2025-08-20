package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/tailor-inc/graphql"
	_ "github.com/uptrace/bun"
)

func (S *SQLDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	query, err := RootConnectionResolverQueryBuilder(param)
	if err != nil {
		return nil, err
	}

	_, err = S.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total": 0,
	}, nil
}

func (S *SQLDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	query, err := RootConnectionResolverQueryBuilder(param)
	if err != nil {
		return nil, err
	}

	_, err = S.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return []byte{}, nil
}

func (S *SQLDriver) AddAuthAddOns(ctx context.Context, project *models.Project, auth map[string]interface{}) error {
	panic("add auth addons not implemented")
}

func (S *SQLDriver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	var err error
	for _, param := range param.ConDisParam {
		for _, id := range param.ActionIDs {
			switch param.ConnectionType {
			case "forward":
				tableName := utility.MultipleResourceName(param.ForwardConnectionType.Model)
				switch param.BackwardConnectionType.Relation {
				case "has_one":
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, param.BackwardConnectionType.Model): param.ForwardConnectionID,
					}
					_, err = S.ORM.NewUpdate().Table(tableName).Where("id = ?", id).Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				case "has_many":
					tableName = fmt.Sprintf(`%s_%s`, utility.MultipleResourceName(param.BackwardConnectionType.Model), tableName)
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, param.BackwardConnectionType.Model): param.ForwardConnectionID,
						fmt.Sprintf(`%s_id`, param.ForwardConnectionType.Model):  id,
					}
					_, err = S.ORM.NewInsert().Table(tableName).Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				}
				break
			case "backward":
				tableName := utility.MultipleResourceName(param.ForwardConnectionType.Model)
				switch param.ForwardConnectionType.Relation {
				case "has_one":
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, param.BackwardConnectionType.Model): param.ForwardConnectionID,
					}
					_, err = S.ORM.NewUpdate().Table(tableName).Where("id = ?", id).Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				case "has_many":
					tableName = fmt.Sprintf(`%s_%s`, utility.MultipleResourceName(param.BackwardConnectionType.Model), tableName)
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, param.BackwardConnectionType.Model): param.ForwardConnectionID,
						fmt.Sprintf(`%s_id`, param.ForwardConnectionType.Model):  id,
					}
					_, err = S.ORM.NewInsert().Table(tableName).Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				}
				break
			}
		}
	}
	return nil
}

func (S *SQLDriver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	return nil
}

func (S *SQLDriver) CheckProjectExists(ctx context.Context, projectId string) (bool, error) {

	var result int64

	switch S.DriverCredential.Engine {
	case _const.MySQLDriver:
		count, err := S.ORM.NewSelect().Table("information_schema.SCHEMATA").
			Where("SCHEMA_NAME = ?", projectId).Count(ctx)
		if err != nil {
			return false, err
		}
		result = int64(count)
		if result == 1 {
			return true, nil
		}
	case _const.PostgreSQLDriver:
		count, err := S.ORM.NewSelect().Table("pg_database").
			Where("datname = ?", projectId).Count(ctx)
		if err != nil {
			return false, err
		}
		result = int64(count)
		if result == 1 {
			return true, nil
		}
	}
	return false, nil
}

/* deprecated
func (S *SQLDriver) GetAllPreviewDocumentsByModel(param models.CommonSystemParams) ([]*models.PreviewMode, error) {
	query, err := RootResolverQueryBuilder(param, true)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	err = S.ORM.NewRaw(*query).Scan(ctx, &results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*models.PreviewMode{}, nil
		} else {
			return nil, err
		}
	}

	var docs []*models.PreviewMode
	for _, res := range results {
		doc := &models.PreviewMode{}
		if val, ok := res["id"].([]byte); ok {
			doc.Id = string(val)
		}
		if val, ok := res["title"].(string); ok {
			doc.Title = val
		}
		if val, ok := res["status"].(string); ok {
			doc.Status = val
		} else {
			doc.Status = "draft" // default
		}
		if val, ok := res["icon"].(string); ok {
			doc.Icon = val
		}
		// filter doc title
		title := strip.StripTags(doc.Title)
		if len(title) > 35 {
			doc.Title = title[0:35] + "..."
		} else {
			doc.Title = title
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
*/

func (S *SQLDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := SelectBuilder("y", local, param.Model, false)

	tableName := utility.SingularResourceName(param.Model.Name)
	result := map[string]interface{}{}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = '%s'", strings.Join(returnType, ", "), tableName, param.DocumentID)
	err := S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []byte{}, nil
		} else {
			return nil, err
		}
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	doc, err := CommonDocTransformation(param.Model, local, result, classification)
	if err != nil {
		return nil, err
	}

	doc.Type = param.Model.Name
	return nil, nil
}

func (S *SQLDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := SelectBuilder("y", local, param.Model, false)

	tableName := utility.SingularResourceName(param.Model.Name)
	result := map[string]interface{}{}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = '%s'", strings.Join(returnType, ", "), tableName, param.DocumentID)
	err := S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.DefaultDocumentStructure{}, nil
		} else {
			return nil, err
		}
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	doc, err := CommonDocTransformation(param.Model, local, result, classification)
	if err != nil {
		return nil, err
	}

	doc.Type = param.Model.Name
	return doc, nil
}

func (S *SQLDriver) BuildFieldClassification(_fields []*models.FieldInfo) *FieldClassification {
	classification := FieldClassification{}
	for _, f := range _fields {
		if f.FieldType == _const.MultilineField {
			classification.MultilineFields = append(classification.MultilineFields, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && f.Validation.IsGallery {
			classification.GalleryField = append(classification.GalleryField, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && !f.Validation.IsGallery {
			classification.PictureField = append(classification.PictureField, f.Identifier)
		} else if f.FieldType == _const.NumberField && f.InputType == _const.DoubleInput {
			classification.DoubleFields = append(classification.DoubleFields, f.Identifier)
		} else if f.FieldType == _const.ListField && f.Validation != nil && (len(f.Validation.FixedListElements) == 0 || f.Validation.IsMultiChoice) {
			classification.ListFields = append(classification.ListFields, f.Identifier)
		} else if f.FieldType == _const.RepeatedField {
			classification.RepeatedFields = map[string][]*models.FieldInfo{
				f.Identifier: f.SubFieldInfo,
			}
		}
	}
	return &classification
}

func (S *SQLDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	if param.SinglePageData {
		return &types.DefaultDocumentStructure{
			Key:  param.DocumentID,
			ID:   param.DocumentID,
			Type: param.Model.Name,
			Data: map[string]interface{}{},
			Meta: &types.MetaField{
				LastModifiedBy: &types.SystemUser{},
			},
		}, nil
	}

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := `
	 x.*, y.created_at AS sys_created_at, y.updated_at AS sys_updated_at, 
		y.created_by AS sys_created_by, y.updated_by AS sys_updated_by, 
		y.status as sys_status
	`

	tableName := utility.SingularResourceName(param.Model.Name)
	result := map[string]interface{}{}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = '%s'", returnType, tableName, param.DocumentID)
	err := S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.DefaultDocumentStructure{}, nil
		} else {
			return nil, err
		}
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	doc, err := CommonDocTransformation(param.Model, local, result, classification)
	if err != nil {
		return nil, err
	}

	doc.Type = param.Model.Name
	return doc, nil
}

// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID.
func (s *SQLDriver) GetAllRelationDocumentsOfSingleDocument_AUTOGEN(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	var relations []interface{}

	// Get all table names to find relation tables
	var tables []string
	err := s.ORM.NewRaw("SHOW TABLES").Scan(ctx, &tables)
	if err != nil {
		return nil, err
	}

	// Look for relation tables and query them
	for _, table := range tables {
		if strings.Contains(table, "_") && !strings.Contains(table, "meta") && !strings.Contains(table, "media") {
			// This looks like a relation table
			parts := strings.Split(table, "_")
			if len(parts) == 2 {
				// Query for relations where this document is the source
				var results []map[string]interface{}
				query1 := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ?", table, parts[0])
				s.ORM.NewRaw(query1, from).Scan(ctx, &results)

				for _, result := range results {
					relations = append(relations, result)
				}

				// Query for relations where this document is the target
				query2 := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ?", table, parts[1])
				s.ORM.NewRaw(query2, from).Scan(ctx, &results)

				for _, result := range results {
					relations = append(relations, result)
				}
			}
		}
	}

	return relations, nil
}

func (S *SQLDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	// query relations and find all docs
	arg.DocumentIDs = []string{arg.DocumentID}
	arg.OnlyReturnCount = true
	query, relationType, err := BuildCombinedRelationQuery("", from, arg)
	if err != nil {
		return nil, err
	}

	switch *relationType {
	case "has_many":
		var result []map[string]interface{}
		err = S.ORM.NewRaw(*query).Scan(ctx, &result)
		if err != nil {
			return nil, err
		}
		var docs []*models.PreviewMode
		for _, res := range result {
			doc := models.PreviewMode{}
			if val, ok := res["id"].([]byte); ok {
				id := string(val)
				doc.ID = id
			} else {
				// if no id then return nil
				return []*models.PreviewMode{}, nil
			}
			if val, ok := res["title"].(string); ok {
				doc.Title = val
			}
			if val, ok := res["icon"].(string); ok {
				doc.Icon = val
			}
			if val, ok := res["status"].(string); ok {
				doc.Status = val
			}
			docs = append(docs, &doc)
		}
		return docs, nil
	case "has_one":
		result := map[string]interface{}{}
		err = S.ORM.NewRaw(*query).Scan(ctx, &result)
		if err != nil {
			return nil, err
		}
		doc := models.PreviewMode{}
		if val, ok := result["id"].([]byte); ok {
			id := string(val)
			doc.ID = id
		} else {
			// if no id then return nil
			return nil, nil
		}
		if val, ok := result["title"].(string); ok {
			doc.Title = val
		}
		if val, ok := result["icon"].(string); ok {
			doc.Icon = val
		}
		if val, ok := result["status"].(string); ok {
			doc.Status = val
		}
		return doc, nil
	}

	return nil, errors.New("Invalid Relation Type")
}

func (S *SQLDriver) CountMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) (int, error) {
	return 0, nil
}

func (S *SQLDriver) ListMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) ([]*models.FileDetails, error) {
	query, err := RootResolverMediaQueryBuilder(param)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		return nil, err
	}

	var docs []*models.FileDetails
	for _, res := range result {
		doc, err := MediaDocTransformation("media", res)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (f *SQLDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewMode bool) (int, error) {

	query, err := RootResolverQueryBuilder(param, true)
	if err != nil {
		return 0, err
	}

	var result int64
	err = f.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		return 0, err
	}

	return int(result), nil

}

func (S *SQLDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	query, err := RootResolverQueryBuilder(param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // send empty response for table
			return []byte{}, nil
		}
		return nil, err
	}

	classification := S.BuildFieldClassification(param.Model.Fields)

	var docs []*types.DefaultDocumentStructure
	for _, res := range result {
		doc, err := CommonDocTransformation(param.Model, local, res, classification)
		if err != nil {
			return nil, err
		}
		doc.Type = param.Model.Name
		docs = append(docs, doc)
	}

	return []byte{}, nil
}

func (S *SQLDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {

	query, err := RootResolverQueryBuilder(param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = S.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // send empty response for table
			return []*types.DefaultDocumentStructure{}, nil
		}
		return nil, err
	}

	classification := S.BuildFieldClassification(param.Model.Fields)
	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	var docs []*types.DefaultDocumentStructure
	for _, res := range result {
		doc, err := CommonDocTransformation(param.Model, local, res, classification)
		if err != nil {
			return nil, err
		}
		doc.Type = param.Model.Name
		docs = append(docs, doc)
	}

	return docs, nil
}
