package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	_ "github.com/uptrace/bun"
)

// GetProjectUsers retrieves metadata for multiple users in the project.
func (s *SQLDriver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	results := make(map[string]*types.DefaultDocumentStructure)

	// In SQL, project users might be stored in a specific table
	var users []map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND id IN (?)"
	err := s.ORM.NewRaw(query, projectId, keys).Scan(ctx, &users)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		doc := &types.DefaultDocumentStructure{
			ID:   user["id"].(string),
			Type: "project_user",
			Data: user,
		}
		results[doc.ID] = doc
	}

	return results, nil
}

// GetProjectUser retrieves a user profile by phone, email, and project ID.
func (s *SQLDriver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	var user map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND (phone = ? OR email = ?)"
	err := s.ORM.NewRaw(query, projectId, phone, email).Scan(ctx, &user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	doc := &types.DefaultDocumentStructure{
		ID:   user["id"].(string),
		Type: "project_user",
		Data: user,
	}

	return doc, nil
}

// GetLoggedInProjectUser retrieves the logged-in user profile for the project.
func (s *SQLDriver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	var user map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND user_id = ?"
	err := s.ORM.NewRaw(query, param.ProjectID, param.UserID).Scan(ctx, &user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	doc := &types.DefaultDocumentStructure{
		ID:   user["id"].(string),
		Type: "project_user",
		Data: user,
	}

	return doc, nil
}

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID.
func (s *SQLDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	var revisions []*models.DocumentRevisionHistory

	query := `
		SELECT id, revision_at, status 
		FROM document_revisions 
		WHERE original_doc_id = ? OR id = ?
		ORDER BY revision_at DESC
	`

	var results []map[string]interface{}
	err := s.ORM.NewRaw(query, param.DocumentID, param.DocumentID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		revision := &models.DocumentRevisionHistory{
			ID:         result["id"].(string),
			RevisionAt: result["revision_at"].(string),
			Status:     result["status"].(string),
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

// AggregateDocOfProject aggregates the documents in the project.
func (s *SQLDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	tableName := utility.SingularResourceName(param.Model.Name)

	// Build basic aggregation query
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s` x LEFT JOIN meta y ON y.doc_id = x.id", tableName)

	// Add where conditions if provided
	var conditions []string
	if param.TenantID != "" {
		conditions = append(conditions, "x.tenant_id = ?")
	}

	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok && len(where) > 0 {
			// Add simple where conditions (this is a simplified implementation)
			for k := range where {
				conditions = append(conditions, fmt.Sprintf("x.%s = ?", k))
			}
		}
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	var result map[string]interface{}
	err := s.ORM.NewRaw(query, param.TenantID).Scan(ctx, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// AggregateDocOfProjectBytes aggregates the documents in the project and returns it as bytes.
func (s *SQLDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	data, err := s.AggregateDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// NewInsertableRelations retrieves new insertable relations in the project.
func (s *SQLDriver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	// Find existing relations
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(param.ForwardConnectionType.Model), utility.SingularResourceName(param.BackwardConnectionType.Model))

	var existingIds []string
	query := fmt.Sprintf("SELECT %s_id FROM `%s` WHERE %s_id = ?",
		utility.SingularResourceName(param.BackwardConnectionType.Model),
		tableName,
		utility.SingularResourceName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &existingIds)
	if err != nil {
		return nil, err
	}

	// Filter out existing relations
	existingMap := make(map[string]bool)
	for _, id := range existingIds {
		existingMap[id] = true
	}

	var newRelations []string
	for _, actionID := range param.ActionIDs {
		if !existingMap[actionID] {
			newRelations = append(newRelations, actionID)
		}
	}

	return newRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation exists in the project.
func (s *SQLDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(param.ForwardConnectionType.Model), utility.SingularResourceName(param.BackwardConnectionType.Model))

	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE %s_id = ?",
		tableName,
		utility.SingularResourceName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRelationIds retrieves the IDs of every document related to a document.
func (s *SQLDriver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(param.ForwardConnectionType.Model), utility.SingularResourceName(param.BackwardConnectionType.Model))

	var relationIds []string
	query := fmt.Sprintf("SELECT %s_id FROM `%s` WHERE %s_id = ?",
		utility.SingularResourceName(param.BackwardConnectionType.Model),
		tableName,
		utility.SingularResourceName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &relationIds)
	if err != nil {
		return nil, err
	}

	return relationIds, nil
}

// GetRelationDocument retrieves a relation document by ID.
func (s *SQLDriver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(param.ForwardConnectionType.Model), utility.SingularResourceName(param.BackwardConnectionType.Model))

	var result map[string]interface{}
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ? AND %s_id = ?",
		tableName,
		utility.SingularResourceName(param.ForwardConnectionType.Model),
		utility.SingularResourceName(param.BackwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID, param.ActionIDs[0]).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("relation not found")
		}
		return nil, err
	}

	// Convert to EdgeRelation structure
	relation := &models.EdgeRelation{
		Key:       result["id"].(string),
		From:      param.ForwardConnectionType.Model,
		To:        param.BackwardConnectionType.Model,
		FromID:    param.ForwardConnectionID,
		ToID:      param.ActionIDs[0],
		Relation:  param.ForwardConnectionType.Relation,
		CreatedAt: utility.GetCurrentTime(),
	}

	return relation, nil
}

// CreateRelation creates a relation in the project.
func (s *SQLDriver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(relation.From), utility.SingularResourceName(relation.To))

	data := map[string]interface{}{
		"id": uuid.New().String(),
		utility.SingularResourceName(relation.From) + "_id": relation.FromID,
		utility.SingularResourceName(relation.To) + "_id":   relation.ToID,
		"created_at": relation.CreatedAt,
	}

	if relation.TenantID != "" {
		data["tenant_id"] = relation.TenantID
	}

	_, err := s.ORM.NewInsert().Table(tableName).Model(&data).Exec(ctx)
	return err
}

// DeleteRelation deletes a relation in the project.
func (s *SQLDriver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	tableName := fmt.Sprintf("%s_%s", utility.SingularResourceName(param.ForwardConnectionType.Model), utility.SingularResourceName(param.BackwardConnectionType.Model))

	_, err := s.ORM.NewDelete().Table(tableName).Where("id = ?", id).Exec(ctx)
	return err
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project.
func (s *SQLDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Find all relation tables and delete relations involving this document
	// This is a simplified implementation - in practice you'd want to know all the relations for this model

	// Get all table names
	var tables []string
	err := s.ORM.NewRaw("SHOW TABLES").Scan(ctx, &tables)
	if err != nil {
		return err
	}

	// Look for relation tables (tables with two model names)
	for _, table := range tables {
		if strings.Contains(table, "_") && !strings.Contains(table, "meta") && !strings.Contains(table, "media") {
			// This looks like a relation table
			parts := strings.Split(table, "_")
			if len(parts) == 2 {
				// Try to delete relations where this document is referenced
				query1 := fmt.Sprintf("DELETE FROM `%s` WHERE %s_id = ?", table, parts[0])
				s.ORM.NewRaw(query1, param.DocumentID).Exec(ctx)

				query2 := fmt.Sprintf("DELETE FROM `%s` WHERE %s_id = ?", table, parts[1])
				s.ORM.NewRaw(query2, param.DocumentID).Exec(ctx)
			}
		}
	}

	return nil
}

// DeleteDocumentsFromProject deletes multiple documents from the project.
func (s *SQLDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := utility.SingularResourceName(param.Model.Name)

	_, err := s.ORM.NewDelete().Table(tableName).Where("id IN (?)", param.DocumentIDs).Exec(ctx)
	if err != nil {
		return err
	}

	// Also delete from meta table
	_, err = s.ORM.NewDelete().Table("meta").Where("doc_id IN (?)", param.DocumentIDs).Exec(ctx)
	return err
}

// RenameModel renames a model in the project.
func (s *SQLDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	oldTableName := utility.SingularResourceName(modelName)
	newTableName := utility.SingularResourceName(newName)

	query := fmt.Sprintf("RENAME TABLE `%s` TO `%s`", oldTableName, newTableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return err
	}

	// Update the project schema
	for _, model := range project.Schema.Models {
		if model.Name == modelName {
			model.Name = newName
			break
		}
	}

	return nil
}

// ConvertModel converts a model in the project.
func (s *SQLDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// This could involve creating a new table with converted structure
	// For now, we'll create a backup table
	oldTableName := utility.SingularResourceName(modelName)
	newTableName := fmt.Sprintf("converted_%s", oldTableName)

	query := fmt.Sprintf("CREATE TABLE `%s` AS SELECT * FROM `%s`", newTableName, oldTableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// RenameField renames a field in a model along with its data key.
func (s *SQLDriver) RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error {
	tableName := utility.SingularResourceName(param.Model.Name)
	newFieldName := param.FieldInfo.Identifier

	query := fmt.Sprintf("ALTER TABLE `%s` CHANGE COLUMN `%s` `%s` %s", tableName, oldFieldName, newFieldName, "TEXT")
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// DropModel drops a model from the project.
func (s *SQLDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	tableName := utility.SingularResourceName(modelName)

	query := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return err
	}

	// Remove from project schema
	var updatedModels []*models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name != modelName {
			updatedModels = append(updatedModels, model)
		}
	}
	project.Schema.Models = updatedModels

	return nil
}

// CreateIndex creates an index for a model in the project.
func (s *SQLDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	tableName := utility.SingularResourceName(param.Model.Name)
	indexName := fmt.Sprintf("idx_%s_%s", tableName, fieldName)

	query := fmt.Sprintf("CREATE INDEX `%s` ON `%s` (`%s`)", indexName, tableName, fieldName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// DropIndex drops an index from a model in the project.
func (s *SQLDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	tableName := utility.SingularResourceName(param.Model.Name)

	query := fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// AddTeamMetaInfo adds metadata information for a team in the project.
func (s *SQLDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	// Get additional team metadata for each user
	var userIds []string
	for _, user := range docs {
		userIds = append(userIds, user.ID)
	}

	if len(userIds) == 0 {
		return docs, nil
	}

	// Query team metadata
	var teamData []map[string]interface{}
	query := "SELECT * FROM team_members WHERE user_id IN (?)"
	err := s.ORM.NewRaw(query, userIds).Scan(ctx, &teamData)
	if err != nil {
		return docs, nil // Return original docs if metadata query fails
	}

	// Map team data to users
	teamMap := make(map[string]map[string]interface{})
	for _, data := range teamData {
		if userID, ok := data["user_id"].(string); ok {
			teamMap[userID] = data
		}
	}

	// Enhance user data with team metadata
	for _, user := range docs {
		if teamInfo, exists := teamMap[user.ID]; exists {
			// Add team-specific metadata to user
			if role, ok := teamInfo["team_role"].(string); ok {
				user.ProjectAssignedRole = role
			}
		}
	}

	return docs, nil
}

// DeleteMediaFile deletes a media file from the project.
func (s *SQLDriver) DeleteMediaFile(ctx context.Context, param models.CommonSystemParams) error {
	query := "DELETE FROM media WHERE id = ? AND model = ?"
	_, err := s.ORM.NewRaw(query, param.DocumentID, param.Model.Name).Exec(ctx)
	return err
}

// DuplicateModel duplicates a model in the project.
func (s *SQLDriver) DuplicateModel(ctx context.Context, project *models.Project, modelName, newName string) (*models.ProjectSchema, error) {
	// Find the original model
	var originalModel *models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name == modelName {
			originalModel = model
			break
		}
	}

	if originalModel == nil {
		return nil, errors.New("model not found")
	}

	// Create a copy of the model
	newModel := &models.ModelType{
		Name:   newName,
		Fields: make([]*models.FieldInfo, len(originalModel.Fields)),
	}

	// Deep copy fields
	for i, field := range originalModel.Fields {
		newModel.Fields[i] = &models.FieldInfo{
			Identifier:  field.Identifier,
			Description: field.Description,
			InputType:   field.InputType,
			FieldType:   field.FieldType,
			Validation:  field.Validation,
			Serial:      field.Serial,
			Label:       field.Label,
		}
	}

	// Add to project schema
	project.Schema.Models = append(project.Schema.Models, newModel)

	// Create the database table
	oldTableName := utility.SingularResourceName(modelName)
	newTableName := utility.SingularResourceName(newName)

	query := fmt.Sprintf("CREATE TABLE `%s` LIKE `%s`", newTableName, oldTableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return project.Schema, nil
}
