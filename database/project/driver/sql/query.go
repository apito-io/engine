package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
	"github.com/uptrace/bun"
)

func (S *SQLDriver) maybeEnsureRelationDDL(ctx context.Context, param *models.CommonSystemParams) error {
	if S == nil || param == nil || param.ResolveParams == nil {
		return nil
	}
	conn, ok := param.ResolveParams.Args["connection"].(map[string]interface{})
	if !ok || len(conn) == 0 {
		return nil
	}
	if len(param.ProjectSchemaModels) == 0 {
		return nil
	}
	return S.EnsureRelationArtifactsFromSchema(ctx, param.ProjectSchemaModels)
}

func (S *SQLDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	// Must use RootResolverQueryBuilder(..., true): RootConnectionResolverQueryBuilder emits Arango AQL (FOR/FILTER/...), not SQL.
	if n, ok, err := S.tryCountFromRowCountTable(ctx, param); ok {
		return int(n), err
	}
	if err := S.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(S.Conf, param, true)
	if err != nil {
		return nil, err
	}

	var result int64
	var scanErr error
	if len(args) > 0 {
		scanErr = S.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		scanErr = S.ORM.NewRaw(query).Scan(ctx, &result)
	}
	if scanErr != nil {
		return nil, fmt.Errorf("failed to execute SQL:\n%w", scanErr)
	}

	return int(result), nil
}

func (S *SQLDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	n, err := S.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{"total": n})
}

func (S *SQLDriver) AddAuthAddOns(ctx context.Context, project *models.Project, auth map[string]interface{}) error {
	panic("add auth addons not implemented")
}

// pivotManyManyInsertRow builds column names for a pivot INSERT. ForwardConnectionID is always the
// document being updated; relatedID is the other endpoint (ConnectDisconnectParamBuilder).
func pivotManyManyInsertRow(cdp *models.ConnectDisconnectParam, relatedID string) map[string]interface{} {
	if cdp.ForwardConnectionModelType != nil && cdp.BackwardConnectionModelType != nil {
		return map[string]interface{}{
			fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.ForwardConnectionModelType.Name)): cdp.ForwardConnectionID,
			fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.BackwardConnectionModelType.Name)):  relatedID,
		}
	}
	return map[string]interface{}{
		fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.BackwardConnectionType.Model)): cdp.ForwardConnectionID,
		fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.ForwardConnectionType.Model)):  relatedID,
	}
}

func (S *SQLDriver) ConnectBuilder(ctx context.Context, root *models.CommonSystemParams) error {
	var err error
	for _, cdp := range root.ConDisParam {
		for _, id := range cdp.ActionIDs {
			switch cdp.ConnectionType {
			case "forward":
				tableName := utility.SingularResourceName(cdp.ForwardConnectionType.Model)
				switch cdp.BackwardConnectionType.Relation {
				case "has_one":
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.BackwardConnectionType.Model)): cdp.ForwardConnectionID,
					}
					qu := S.ORM.NewUpdate().Table(tableName).Where("id = ?", id)
					qu = applyBunHookWheresUpdate(S.Conf, ctx, root, qu)
					_, err = qu.Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				case "has_many":
					tableName = fmt.Sprintf(`%s_%s`, utility.SingularResourceName(cdp.BackwardConnectionType.Model), tableName)
					row := pivotManyManyInsertRow(cdp, id)
					_, err = S.ORM.NewInsert().Table(tableName).Model(&row).Exec(ctx)
					if err != nil {
						return err
					}
					break
				}
				break
			case "backward":
				tableName := utility.SingularResourceName(cdp.ForwardConnectionType.Model)
				switch cdp.ForwardConnectionType.Relation {
				case "has_one":
					u := map[string]interface{}{
						fmt.Sprintf(`%s_id`, utility.SingularResourceName(cdp.BackwardConnectionType.Model)): cdp.ForwardConnectionID,
					}
					qu := S.ORM.NewUpdate().Table(tableName).Where("id = ?", id)
					qu = applyBunHookWheresUpdate(S.Conf, ctx, root, qu)
					_, err = qu.Model(&u).Exec(ctx)
					if err != nil {
						return err
					}
					break
				case "has_many":
					tableName = fmt.Sprintf(`%s_%s`, utility.SingularResourceName(cdp.BackwardConnectionType.Model), tableName)
					row := pivotManyManyInsertRow(cdp, id)
					_, err = S.ORM.NewInsert().Table(tableName).Model(&row).Exec(ctx)
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
	if param == nil || len(param.ConDisParam) == 0 {
		return nil
	}

	var err error
	for _, cdp := range param.ConDisParam {
		if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
			continue
		}
		for _, id := range cdp.ActionIDs {
			switch cdp.ConnectionType {
			case "forward":
				tableName := utility.SingularResourceName(cdp.ForwardConnectionType.Model)
				switch cdp.BackwardConnectionType.Relation {
				case "has_one":
					fkCol := fmt.Sprintf("%s_id", utility.SingularResourceName(cdp.BackwardConnectionType.Model))
					qu := S.ORM.NewUpdate().Table(tableName).
						Set("? = NULL", bun.Ident(fkCol)).
						Where("id = ?", id).
						Where("? = ?", bun.Ident(fkCol), cdp.ForwardConnectionID)
					qu = applyBunHookWheresUpdate(S.Conf, ctx, param, qu)
					_, err = qu.Exec(ctx)
					if err != nil {
						return err
					}
				case "has_many":
					pivotTable := fmt.Sprintf(`%s_%s`, utility.SingularResourceName(cdp.BackwardConnectionType.Model), tableName)
					row := pivotManyManyInsertRow(cdp, id)
					qd := S.ORM.NewDelete().Table(pivotTable)
					for k, v := range row {
						qd = qd.Where("? = ?", bun.Ident(k), v)
					}
					_, err = qd.Exec(ctx)
					if err != nil {
						return err
					}
				}
			case "backward":
				tableName := utility.SingularResourceName(cdp.ForwardConnectionType.Model)
				switch cdp.ForwardConnectionType.Relation {
				case "has_one":
					fkCol := fmt.Sprintf("%s_id", utility.SingularResourceName(cdp.BackwardConnectionType.Model))
					qu := S.ORM.NewUpdate().Table(tableName).
						Set("? = NULL", bun.Ident(fkCol)).
						Where("id = ?", id).
						Where("? = ?", bun.Ident(fkCol), cdp.ForwardConnectionID)
					qu = applyBunHookWheresUpdate(S.Conf, ctx, param, qu)
					_, err = qu.Exec(ctx)
					if err != nil {
						return err
					}
				case "has_many":
					pivotTable := fmt.Sprintf(`%s_%s`, utility.SingularResourceName(cdp.BackwardConnectionType.Model), tableName)
					row := pivotManyManyInsertRow(cdp, id)
					qd := S.ORM.NewDelete().Table(pivotTable)
					for k, v := range row {
						qd = qd.Where("? = ?", bun.Ident(k), v)
					}
					_, err = qd.Exec(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
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
	doc, err := S.GetSingleProjectDocument(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

func (S *SQLDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := SelectBuilder("y", local, param.Model, false)

	tableName := utility.SingularResourceName(param.Model.Name)
	result := map[string]interface{}{}
	hookWhere, hookArgs, err := singleDocHookWhereSQLAndArgs(S.Conf, ctx, param)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = ?%s", strings.Join(returnType, ", "), tableName, hookWhere)
	args := append([]interface{}{param.DocumentID}, hookArgs...)
	err = S.ORM.NewRaw(query, args...).Scan(ctx, &result)
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
	hookWhere, hookArgs, err := singleDocHookWhereSQLAndArgs(S.Conf, ctx, param)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = ?%s", returnType, tableName, hookWhere)
	args := append([]interface{}{param.DocumentID}, hookArgs...)
	err = S.ORM.NewRaw(query, args...).Scan(ctx, &result)
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
	query, relArgs, relationType, err := BuildCombinedRelationQuery(S.Conf, "", from, arg)
	if err != nil {
		return nil, err
	}

	switch *relationType {
	case "has_many":
		var result []map[string]interface{}
		if len(relArgs) > 0 {
			err = S.ORM.NewRaw(query, relArgs...).Scan(ctx, &result)
		} else {
			err = S.ORM.NewRaw(query).Scan(ctx, &result)
		}
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
		if len(relArgs) > 0 {
			err = S.ORM.NewRaw(query, relArgs...).Scan(ctx, &result)
		} else {
			err = S.ORM.NewRaw(query).Scan(ctx, &result)
		}
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

	if err := f.maybeEnsureRelationDDL(ctx, param); err != nil {
		return 0, err
	}
	query, args, err := RootResolverQueryBuilder(f.Conf, param, true)
	if err != nil {
		return 0, err
	}

	var result int64
	if len(args) > 0 {
		err = f.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = f.ORM.NewRaw(query).Scan(ctx, &result)
	}
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

	if err := S.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(S.Conf, param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if len(args) > 0 {
		err = S.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = S.ORM.NewRaw(query).Scan(ctx, &result)
	}
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

	if err := S.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(S.Conf, param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if len(args) > 0 {
		err = S.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = S.ORM.NewRaw(query).Scan(ctx, &result)
	}
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
