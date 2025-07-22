package mariadb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/google/uuid"
)

// CheckCollectionExists checks if a collection exists in the project
func (m *MariaDBDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var tableName string
	if isRelationCollection {
		tableName = fmt.Sprintf("p_%s_relations", param.ProjectID)
	} else {
		tableName = fmt.Sprintf("p_%s_documents", param.ProjectID)
	}

	var count int
	query := "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ? AND table_schema = DATABASE()"
	err := m.DB.QueryRowContext(ctx, query, tableName).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 && !isRelationCollection {
		// Also check if documents exist for this model
		var docCount int
		docQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE project_id = ? AND model_name = ?", tableName)
		err = m.DB.QueryRowContext(ctx, docQuery, param.ProjectID, param.Model.Name).Scan(&docCount)
		if err != nil {
			return false, err
		}
		return docCount > 0, nil
	}

	return count > 0, nil
}

// AddCollection adds a new collection to the project
func (m *MariaDBDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams) error {
	// Initialize tables for this project if they don't exist
	if err := m.initializeTables(ctx, param.ProjectID); err != nil {
		return err
	}

	// Store model metadata
	modelsTable := fmt.Sprintf("p_%s_models", param.ProjectID)
	modelDataJSON, err := json.Marshal(param.Model)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`INSERT INTO %s (project_id, model_name, model_data, created_at) 
		VALUES (?, ?, ?, NOW()) ON DUPLICATE KEY UPDATE 
		model_data = VALUES(model_data), updated_at = NOW()`, modelsTable)
	_, err = m.DB.ExecContext(ctx, query, param.ProjectID, param.Model.Name, string(modelDataJSON))
	return err
}

// AddModel adds a new model to the project
func (m *MariaDBDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	// Initialize tables for this project if they don't exist
	if err := m.initializeTables(ctx, project.ID); err != nil {
		return nil, err
	}

	modelsTable := fmt.Sprintf("p_%s_models", project.ID)
	modelDataJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`INSERT INTO %s (project_id, model_name, model_data, created_at) 
		VALUES (?, ?, ?, NOW())`, modelsTable)
	_, err = m.DB.ExecContext(ctx, query, project.ID, model.Name, string(modelDataJSON))
	if err != nil {
		return nil, err
	}

	// Update project schema
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			ProjectID: project.ID,
			Models:    []*models.ModelType{model},
		}
	} else {
		project.Schema.Models = append(project.Schema.Models, model)
	}

	return project.Schema, nil
}

// AddFieldToModel adds a new field to an existing model in the project
func (m *MariaDBDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, repeatedGroupIdentifier string) (*models.ModelType, error) {
	modelsTable := fmt.Sprintf("p_%s_models", param.ProjectID)

	// Get current model data
	var modelDataJSON string
	query := fmt.Sprintf("SELECT model_data FROM %s WHERE project_id = ? AND model_name = ?", modelsTable)
	err := m.DB.QueryRowContext(ctx, query, param.ProjectID, param.Model.Name).Scan(&modelDataJSON)
	if err != nil {
		return nil, err
	}

	// Parse the model
	var model models.ModelType
	if err := json.Unmarshal([]byte(modelDataJSON), &model); err != nil {
		return nil, err
	}

	// Add the new field
	model.Fields = append(model.Fields, param.FieldInfo)

	// Update the model
	updatedModelDataJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	updateQuery := fmt.Sprintf(`UPDATE %s SET model_data = ?, updated_at = NOW() 
		WHERE project_id = ? AND model_name = ?`, modelsTable)
	_, err = m.DB.ExecContext(ctx, updateQuery, string(updatedModelDataJSON), param.ProjectID, param.Model.Name)
	if err != nil {
		return nil, err
	}

	return &model, nil
}

// RenameModel renames a model in the project
func (m *MariaDBDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	modelsTable := fmt.Sprintf("p_%s_models", project.ID)
	documentsTable := fmt.Sprintf("p_%s_documents", project.ID)

	// Start transaction
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update model metadata
	updateModelQuery := fmt.Sprintf(`UPDATE %s SET model_name = ?, updated_at = NOW() 
		WHERE project_id = ? AND model_name = ?`, modelsTable)
	_, err = tx.ExecContext(ctx, updateModelQuery, newName, project.ID, modelName)
	if err != nil {
		return err
	}

	// Update all documents that use this model
	updateDocsQuery := fmt.Sprintf(`UPDATE %s SET model_name = ? 
		WHERE project_id = ? AND model_name = ?`, documentsTable)
	_, err = tx.ExecContext(ctx, updateDocsQuery, newName, project.ID, modelName)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ConvertModel converts a model in the project (no-op for MariaDB)
func (m *MariaDBDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// MariaDB doesn't require model conversion
	return nil
}

// DropModel drops a model from the project
func (m *MariaDBDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	modelsTable := fmt.Sprintf("p_%s_models", project.ID)
	documentsTable := fmt.Sprintf("p_%s_documents", project.ID)

	// Start transaction
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete all documents of this model
	deleteDocsQuery := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND model_name = ?", documentsTable)
	_, err = tx.ExecContext(ctx, deleteDocsQuery, project.ID, modelName)
	if err != nil {
		return err
	}

	// Delete model metadata
	deleteModelQuery := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND model_name = ?", modelsTable)
	_, err = tx.ExecContext(ctx, deleteModelQuery, project.ID, modelName)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CreateIndex creates an index for a model in the project
func (m *MariaDBDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, repeatedGroupIdentifier string) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)
	indexName := fmt.Sprintf("idx_%s_%s_%s", param.ProjectID, param.Model.Name, fieldName)

	// MariaDB supports JSON indexing with generated columns
	// Create a generated column for the JSON field and index it
	alterQuery := fmt.Sprintf(`ALTER TABLE %s 
		ADD COLUMN IF NOT EXISTS %s_gen VARCHAR(255) AS (JSON_UNQUOTE(JSON_EXTRACT(document_data, '$.data.%s'))) VIRTUAL,
		ADD INDEX IF NOT EXISTS %s (%s_gen)`,
		tableName, fieldName, fieldName, indexName, fieldName)

	_, err := m.DB.ExecContext(ctx, alterQuery)
	return err
}

// DropIndex drops an index from a model in the project
func (m *MariaDBDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	dropQuery := fmt.Sprintf("ALTER TABLE %s DROP INDEX IF EXISTS %s", tableName, indexName)
	_, err := m.DB.ExecContext(ctx, dropQuery)
	return err
}

// GetSingleProjectDocument retrieves a single document by its ID
func (m *MariaDBDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	var documentDataJSON string
	query := fmt.Sprintf("SELECT document_data FROM %s WHERE project_id = ? AND document_id = ?", tableName)
	err := m.DB.QueryRowContext(ctx, query, param.ProjectID, param.DocumentID).Scan(&documentDataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found")
		}
		return nil, err
	}

	var doc types.DefaultDocumentStructure
	err = json.Unmarshal([]byte(documentDataJSON), &doc)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

// GetSingleProjectDocumentBytes retrieves a single document as bytes
func (m *MariaDBDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	doc, err := m.GetSingleProjectDocument(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(doc)
}

// GetSingleProjectDocumentRevisions retrieves document revisions
func (m *MariaDBDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_revisions", param.ProjectID)

	query := fmt.Sprintf(`SELECT revision_data FROM %s 
		WHERE document_id = ? ORDER BY created_at DESC`, tableName)
	rows, err := m.DB.QueryContext(ctx, query, param.DocumentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*types.DefaultDocumentStructure
	for rows.Next() {
		var revisionDataJSON sql.NullString
		if err := rows.Scan(&revisionDataJSON); err != nil {
			continue
		}

		if revisionDataJSON.Valid {
			var revision types.DefaultDocumentStructure
			if err := json.Unmarshal([]byte(revisionDataJSON.String), &revision); err != nil {
				continue
			}
			revisions = append(revisions, &revision)
		}
	}

	return revisions, nil
}

// GetSingleRawDocumentFromProject retrieves a raw document
func (m *MariaDBDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return m.GetSingleProjectDocument(ctx, param)
}

// QueryMultiDocumentOfProject retrieves multiple documents from a project
func (m *MariaDBDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	query := fmt.Sprintf(`SELECT document_data FROM %s 
		WHERE project_id = ? AND model_name = ? 
		ORDER BY created_at DESC`, tableName)
	rows, err := m.DB.QueryContext(ctx, query, param.ProjectID, param.Model.Name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documents []*types.DefaultDocumentStructure
	for rows.Next() {
		var documentDataJSON string
		if err := rows.Scan(&documentDataJSON); err != nil {
			continue
		}

		var doc types.DefaultDocumentStructure
		if err := json.Unmarshal([]byte(documentDataJSON), &doc); err != nil {
			continue
		}

		documents = append(documents, &doc)
	}

	return documents, nil
}

// QueryMultiDocumentOfProjectBytes retrieves multiple documents as bytes
func (m *MariaDBDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	docs, err := m.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(docs)
}

// AddDocumentToProject adds a new document to the project
func (m *MariaDBDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	// Initialize tables if they don't exist
	if err := m.initializeTables(ctx, param.ProjectID); err != nil {
		return nil, err
	}

	// Generate ID if not provided
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.CreatedAt = time.Now().Format(time.RFC3339)
	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentDataJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`INSERT INTO %s (project_id, document_id, model_name, document_data, created_at, updated_at) 
		VALUES (?, ?, ?, ?, NOW(), NOW())`, tableName)
	_, err = m.DB.ExecContext(ctx, query, param.ProjectID, doc.ID, param.Model.Name, string(documentDataJSON))
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocumentOfProject updates a particular document in the project
func (m *MariaDBDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentDataJSON, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`UPDATE %s SET document_data = ?, updated_at = NOW() 
		WHERE project_id = ? AND document_id = ?`, tableName)
	_, err = m.DB.ExecContext(ctx, query, string(documentDataJSON), param.ProjectID, doc.ID)

	return err
}

// DeleteDocumentFromProject deletes a document from the project
func (m *MariaDBDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND document_id = ?", tableName)
	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, param.DocumentID)

	return err
}

// DeleteDocumentsFromProject deletes multiple documents from the project
func (m *MariaDBDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := fmt.Sprintf("p_%s_documents", param.ProjectID)

	if len(param.DocumentIDs) == 0 {
		return nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(param.DocumentIDs))
	args := make([]interface{}, len(param.DocumentIDs)+1)
	args[0] = param.ProjectID

	for i, docID := range param.DocumentIDs {
		placeholders[i] = "?"
		args[i+1] = docID
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND document_id IN (%s)",
		tableName, fmt.Sprintf("%s", placeholders))
	_, err := m.DB.ExecContext(ctx, query, args...)

	return err
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project
func (m *MariaDBDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	relationsTable := fmt.Sprintf("p_%s_relations", param.ProjectID)

	// Delete relations where this document is the source or target
	query := fmt.Sprintf("DELETE FROM %s WHERE project_id = ? AND (from_id = ? OR to_id = ?)", relationsTable)
	_, err := m.DB.ExecContext(ctx, query, param.ProjectID, param.DocumentID, param.DocumentID)

	return err
}
