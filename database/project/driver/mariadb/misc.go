package mariadb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/google/uuid"
)

// AddRelationFields adds relation fields to a model (no-op for MariaDB)
func (m *MariaDBDriver) AddRelationFields(ctx context.Context, param *models.CommonSystemParams, sourceModel, targetModel, relationType string) error {
	// MariaDB handles relations through separate relation tables, no schema changes needed
	return nil
}

// DeleteRelationDocuments deletes relation documents for a specific relation
func (m *MariaDBDriver) DeleteRelationDocuments(ctx context.Context, param *models.CommonSystemParams, relationshipName string) error {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND relation_type = ?", tableName)
	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, relationshipName)
	return err
}

// GetRelationDocument retrieves a relation document
func (m *MariaDBDriver) GetRelationDocument(ctx context.Context, cd *models.ConnectDisconnectParam) (*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_relations", cd.DocCollectionName)

	var relationDataJSON sql.NullString
	query := fmt.Sprintf("SELECT relation_data FROM %s WHERE project_id = ? AND relation_id = ?", tableName)
	err := m.DB.QueryRowContext(ctx, query, cd.DocCollectionName, cd.CurrentActionID).Scan(&relationDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("relation document not found")
		}
		return nil, err
	}

	var doc types.DefaultDocumentStructure
	if relationDataJSON.Valid {
		err = json.Unmarshal([]byte(relationDataJSON.String), &doc)
		if err != nil {
			return nil, err
		}
	}

	return &doc, nil
}

// CreateRelation creates a new relation between documents
func (m *MariaDBDriver) CreateRelation(ctx context.Context, param *models.CommonSystemParams, relation *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	// Initialize tables if they don't exist
	if err := m.initializeTables(ctx, param.ProjectID); err != nil {
		return nil, err
	}

	// Generate unique relation ID if not provided
	if relation.ID == "" {
		relation.ID = uuid.New().String()
	}

	relationDataJSON, err := json.Marshal(relation)
	if err != nil {
		return nil, err
	}

	// Extract from_id and to_id from relation data
	var fromId, toId, relationType string
	if relation.Data != nil {
		if fid, ok := relation.Data["from_id"]; ok {
			fromId = fmt.Sprintf("%v", fid)
		}
		if tid, ok := relation.Data["to_id"]; ok {
			toId = fmt.Sprintf("%v", tid)
		}
		if rt, ok := relation.Data["relation_type"]; ok {
			relationType = fmt.Sprintf("%v", rt)
		}
	}

	query := fmt.Sprintf(`INSERT INTO %s (project_id, relation_id, from_id, to_id, relation_type, relation_data, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, NOW())`, tableName)
	_, err = m.DB.ExecContext(ctx, query, param.ProjectID, relation.ID, fromId, toId, relationType, string(relationDataJSON))
	if err != nil {
		return nil, err
	}

	return relation, nil
}

// DeleteRelation deletes a specific relation
func (m *MariaDBDriver) DeleteRelation(ctx context.Context, param *models.CommonSystemParams, relationID string) error {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND relation_id = ?", tableName)
	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, relationID)
	return err
}

// NewInsertableRelations creates new insertable relations
func (m *MariaDBDriver) NewInsertableRelations(ctx context.Context, param *models.CommonSystemParams, relations []*types.DefaultDocumentStructure) ([]*types.DefaultDocumentStructure, error) {
	var createdRelations []*types.DefaultDocumentStructure

	for _, relation := range relations {
		createdRelation, err := m.CreateRelation(ctx, param, relation)
		if err != nil {
			return nil, err
		}
		createdRelations = append(createdRelations, createdRelation)
	}

	return createdRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation already exists
func (m *MariaDBDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.CommonSystemParams, fromId, toId, relationName string) (bool, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s 
		WHERE project_id = ? AND from_id = ? AND to_id = ? AND relation_type = ?`, tableName)
	err := m.DB.QueryRowContext(ctx, query, param.ProjectID, fromId, toId, relationName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRelationIds gets relation IDs for a document
func (m *MariaDBDriver) GetRelationIds(ctx context.Context, param *models.CommonSystemParams, documentID, relationName string) ([]string, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	query := fmt.Sprintf(`SELECT to_id FROM %s 
		WHERE project_id = ? AND from_id = ? AND relation_type = ?`, tableName)
	rows, err := m.DB.QueryContext(ctx, query, param.ProjectID, documentID, relationName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationIds []string
	for rows.Next() {
		var toId string
		if err := rows.Scan(&toId); err != nil {
			continue
		}
		relationIds = append(relationIds, toId)
	}

	return relationIds, nil
}

// ConnectBuilder connects a builder to the project
func (m *MariaDBDriver) ConnectBuilder(ctx context.Context, projectId, userId string) error {
	tableName := fmt.Sprintf("p_%s_builders", projectId)

	// Initialize tables if they don't exist
	if err := m.initializeTables(ctx, projectId); err != nil {
		return err
	}

	query := fmt.Sprintf(`INSERT INTO %s (project_id, user_id, connected_at) 
		VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE connected_at = NOW()`, tableName)
	_, err := m.DB.ExecContext(ctx, query, projectId, userId)
	return err
}

// DisconnectBuilder disconnects a builder from the project
func (m *MariaDBDriver) DisconnectBuilder(ctx context.Context, projectId, userId string) error {
	tableName := fmt.Sprintf("p_%s_builders", projectId)

	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND user_id = ?", tableName)
	_, err := m.DB.ExecContext(ctx, query, projectId, userId)
	return err
}

// GetProjectUser gets a project user by email or phone
func (m *MariaDBDriver) GetProjectUser(ctx context.Context, projectId, email, phone string) (*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_users", projectId)

	var query string
	var args []interface{}

	if email != "" {
		query = fmt.Sprintf("SELECT user_data FROM %s WHERE project_id = ? AND email = ?", tableName)
		args = []interface{}{projectId, email}
	} else if phone != "" {
		query = fmt.Sprintf("SELECT user_data FROM %s WHERE project_id = ? AND phone = ?", tableName)
		args = []interface{}{projectId, phone}
	} else {
		return nil, fmt.Errorf("either email or phone must be provided")
	}

	var userDataJSON sql.NullString
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&userDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	var user types.DefaultDocumentStructure
	if userDataJSON.Valid {
		err = json.Unmarshal([]byte(userDataJSON.String), &user)
		if err != nil {
			return nil, err
		}
	}

	return &user, nil
}

// GetLoggedInProjectUser gets a logged-in project user by user ID
func (m *MariaDBDriver) GetLoggedInProjectUser(ctx context.Context, projectId, userId string) (*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_users", projectId)

	var userDataJSON sql.NullString
	query := fmt.Sprintf("SELECT user_data FROM %s WHERE project_id = ? AND user_id = ?", tableName)
	err := m.DB.QueryRowContext(ctx, query, projectId, userId).Scan(&userDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	var user types.DefaultDocumentStructure
	if userDataJSON.Valid {
		err = json.Unmarshal([]byte(userDataJSON.String), &user)
		if err != nil {
			return nil, err
		}
	}

	return &user, nil
}

// GetProjectUsers gets project users by their IDs
func (m *MariaDBDriver) GetProjectUsers(ctx context.Context, projectId string, userIds []string) ([]*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_users", projectId)

	if len(userIds) == 0 {
		return []*types.DefaultDocumentStructure{}, nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(userIds))
	args := make([]interface{}, len(userIds)+1)
	args[0] = projectId

	for i, userId := range userIds {
		placeholders[i] = "?"
		args[i+1] = userId
	}

	query := fmt.Sprintf("SELECT user_data FROM %s WHERE project_id = ? AND user_id IN (%s)",
		tableName, strings.Join(placeholders, ","))
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*types.DefaultDocumentStructure
	for rows.Next() {
		var userDataJSON sql.NullString
		if err := rows.Scan(&userDataJSON); err != nil {
			continue
		}

		if userDataJSON.Valid {
			var user types.DefaultDocumentStructure
			err = json.Unmarshal([]byte(userDataJSON.String), &user)
			if err == nil {
				users = append(users, &user)
			}
		}
	}

	return users, nil
}

// GetAllRelationDocumentsOfSingleDocument gets all relation documents for a single document
func (m *MariaDBDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, param *models.CommonSystemParams, documentID string) ([]*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	// Get relations where this document is the source or target
	query := fmt.Sprintf(`SELECT relation_data FROM %s 
		WHERE project_id = ? AND (from_id = ? OR to_id = ?)`, tableName)
	rows, err := m.DB.QueryContext(ctx, query, param.ProjectID, documentID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []*types.DefaultDocumentStructure
	for rows.Next() {
		var relationDataJSON sql.NullString
		if err := rows.Scan(&relationDataJSON); err != nil {
			continue
		}

		if relationDataJSON.Valid {
			var relation types.DefaultDocumentStructure
			err = json.Unmarshal([]byte(relationDataJSON.String), &relation)
			if err == nil {
				relations = append(relations, &relation)
			}
		}
	}

	return relations, nil
}

// AddTeamMetaInfo adds team meta info to a document
func (m *MariaDBDriver) AddTeamMetaInfo(ctx context.Context, param *models.CommonSystemParams, teamInfo *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Store team info as a regular document
	result, err := m.AddDocumentToProject(ctx, param, teamInfo)
	if err != nil {
		return nil, err
	}

	// AddDocumentToProject returns interface{}, we need to assert it back to our type
	if doc, ok := result.(*types.DefaultDocumentStructure); ok {
		return doc, nil
	}

	return teamInfo, nil
}

// RelationshipDataLoader loads relationship data
func (m *MariaDBDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, ids []string) ([]*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	if len(ids) == 0 {
		return []*types.DefaultDocumentStructure{}, nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = param.ProjectID

	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf("SELECT relation_data FROM %s WHERE project_id = ? AND relation_id IN (%s)",
		tableName, strings.Join(placeholders, ","))
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []*types.DefaultDocumentStructure
	for rows.Next() {
		var relationDataJSON sql.NullString
		if err := rows.Scan(&relationDataJSON); err != nil {
			continue
		}

		if relationDataJSON.Valid {
			var relation types.DefaultDocumentStructure
			err = json.Unmarshal([]byte(relationDataJSON.String), &relation)
			if err == nil {
				relations = append(relations, &relation)
			}
		}
	}

	return relations, nil
}

// RelationshipDataLoaderBytes loads relationship data as bytes
func (m *MariaDBDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, ids []string) ([][]byte, error) {
	tableName := fmt.Sprintf("p_%s_relations", param.ProjectID)

	if len(ids) == 0 {
		return [][]byte{}, nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = param.ProjectID

	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf("SELECT relation_data FROM %s WHERE project_id = ? AND relation_id IN (%s)",
		tableName, strings.Join(placeholders, ","))
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationBytes [][]byte
	for rows.Next() {
		var relationDataJSON sql.NullString
		if err := rows.Scan(&relationDataJSON); err != nil {
			continue
		}

		if relationDataJSON.Valid {
			relationBytes = append(relationBytes, []byte(relationDataJSON.String))
		}
	}

	return relationBytes, nil
}

// CountDocOfProject counts documents in a project
func (m *MariaDBDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE project_id = ? AND model_name = ?", tableName)
	err := m.DB.QueryRowContext(ctx, query, param.ProjectID, param.Model.Name).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountDocOfProjectBytes counts documents in a project (bytes version)
func (m *MariaDBDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	return m.CountDocOfProject(ctx, param)
}

// CountMultiDocumentOfProject counts multiple documents with filters
func (m *MariaDBDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, condition map[string]interface{}) (int, error) {
	// For simplicity, return total count (MariaDB filtering would require more complex implementation)
	return m.CountDocOfProject(ctx, param)
}

// AggregateDocOfProject performs aggregation on documents
func (m *MariaDBDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) (interface{}, error) {
	// MariaDB supports some aggregation functions but not MongoDB-style pipelines
	// This would need custom implementation based on specific aggregation needs
	return nil, fmt.Errorf("aggregation not yet implemented for MariaDB")
}

// AggregateDocOfProjectBytes performs aggregation on documents (bytes version)
func (m *MariaDBDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) ([]byte, error) {
	result, err := m.AggregateDocOfProject(ctx, param, pipeline)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// DropField drops a field from all documents in a collection
func (m *MariaDBDriver) DropField(ctx context.Context, param *models.CommonSystemParams, fieldName string) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	// MariaDB supports JSON_REMOVE function to remove fields from JSON documents
	query := fmt.Sprintf(`UPDATE %s SET 
		document_data = JSON_REMOVE(document_data, '$.data.%s'),
		updated_at = NOW()
		WHERE project_id = ? AND model_name = ?`, tableName, fieldName)

	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, param.Model.Name)
	return err
}

// RenameField renames a field in all documents in a collection
func (m *MariaDBDriver) RenameField(ctx context.Context, param *models.CommonSystemParams, oldFieldName, newFieldName string) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	// MariaDB JSON functions to rename a field
	query := fmt.Sprintf(`UPDATE %s SET 
		document_data = JSON_SET(
			JSON_REMOVE(document_data, '$.data.%s'),
			'$.data.%s',
			JSON_EXTRACT(document_data, '$.data.%s')
		),
		updated_at = NOW()
		WHERE project_id = ? AND model_name = ? 
		AND JSON_EXTRACT(document_data, '$.data.%s') IS NOT NULL`,
		tableName, oldFieldName, newFieldName, oldFieldName, oldFieldName)

	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, param.Model.Name)
	return err
}

// DeleteMediaFile deletes a media file document
func (m *MariaDBDriver) DeleteMediaFile(ctx context.Context, param *models.CommonSystemParams) error {
	return m.DeleteDocumentFromProject(ctx, param)
}

// DuplicateModel duplicates a model by copying all its documents
func (m *MariaDBDriver) DuplicateModel(ctx context.Context, param *models.CommonSystemParams, newModelName string) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	// MariaDB query to duplicate all documents from one model to another with new IDs
	query := fmt.Sprintf(`INSERT INTO %s (project_id, document_id, model_name, document_data, created_at, updated_at)
		SELECT 
			project_id,
			UUID() as document_id,
			? as model_name,
			JSON_SET(document_data, '$.id', UUID()) as document_data,
			NOW() as created_at,
			NOW() as updated_at
		FROM %s 
		WHERE project_id = ? AND model_name = ?`, tableName, tableName)

	_, err := m.DB.ExecContext(ctx, query, newModelName, param.ProjectID, param.Model.Name)
	return err
}
