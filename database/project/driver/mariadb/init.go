package mariadb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/apito-io/engine/models"
	_ "github.com/go-sql-driver/mysql"
)

type MariaDBDriver struct {
	DB               *sql.DB
	DriverCredential *models.DriverCredentials
}

// GetMariaDBDriver creates a new MariaDB project driver instance
func GetMariaDBDriver(driverCredentials *models.DriverCredentials) (*MariaDBDriver, error) {
	// Build MariaDB connection string (MySQL-compatible)
	// Format: user:password@tcp(host:port)/database?parseTime=true
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		driverCredentials.User,
		driverCredentials.Password,
		driverCredentials.Host,
		driverCredentials.Port,
		driverCredentials.Database)

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MariaDB: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MariaDB database: %v", err)
	}

	return &MariaDBDriver{
		DB:               db,
		DriverCredential: driverCredentials,
	}, nil
}

// Close closes the MariaDB database connection
func (m *MariaDBDriver) Close() error {
	if m.DB != nil {
		return m.DB.Close()
	}
	return nil
}

// DeleteProject deletes a project and all related data
func (m *MariaDBDriver) DeleteProject(ctx context.Context, projectID string) error {
	// Drop all tables related to this project
	tableNames := []string{
		fmt.Sprintf("p_%s_documents", projectID),
		fmt.Sprintf("p_%s_relations", projectID),
		fmt.Sprintf("p_%s_revisions", projectID),
		fmt.Sprintf("p_%s_builders", projectID),
		fmt.Sprintf("p_%s_users", projectID),
		fmt.Sprintf("p_%s_models", projectID),
	}

	for _, tableName := range tableNames {
		_, err := m.DB.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName))
		if err != nil {
			return fmt.Errorf("failed to drop table %s: %v", tableName, err)
		}
	}

	return nil
}

// TransferProject transfers a project from one user to another
func (m *MariaDBDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	// Update project ownership in all relevant tables
	tables := []string{
		fmt.Sprintf("p_%s_documents", userId),
		fmt.Sprintf("p_%s_users", userId),
	}

	for _, tableName := range tables {
		query := fmt.Sprintf("UPDATE `%s` SET owner_id = ? WHERE owner_id = ? AND project_id = ?", tableName)
		_, err := m.DB.ExecContext(ctx, query, to, from, userId)
		if err != nil {
			return err
		}
	}

	return nil
}

// initializeTables creates the necessary MariaDB tables for project operations
func (m *MariaDBDriver) initializeTables(ctx context.Context, projectID string) error {
	tables := []struct {
		name string
		ddl  string
	}{
		// Project documents table
		{
			name: fmt.Sprintf("p_%s_documents", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_documents (
				project_id VARCHAR(255) NOT NULL,
				document_id VARCHAR(255) NOT NULL,
				model_name VARCHAR(255) NOT NULL,
				document_data JSON NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, document_id),
				INDEX idx_model_name (model_name),
				INDEX idx_created_at (created_at)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
		// Project relations table
		{
			name: fmt.Sprintf("p_%s_relations", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_relations (
				project_id VARCHAR(255) NOT NULL,
				relation_id VARCHAR(255) NOT NULL,
				from_id VARCHAR(255) NOT NULL,
				to_id VARCHAR(255) NOT NULL,
				relation_type VARCHAR(255),
				relation_data JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, relation_id),
				INDEX idx_from_id (from_id),
				INDEX idx_to_id (to_id),
				INDEX idx_relation_type (relation_type)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
		// Project revisions table
		{
			name: fmt.Sprintf("p_%s_revisions", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_revisions (
				document_id VARCHAR(255) NOT NULL,
				revision_id VARCHAR(255) NOT NULL,
				project_id VARCHAR(255) NOT NULL,
				revision_data JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (document_id, revision_id),
				INDEX idx_project_id (project_id),
				INDEX idx_created_at (created_at)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
		// Project builders table
		{
			name: fmt.Sprintf("p_%s_builders", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_builders (
				project_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, user_id)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
		// Project users table
		{
			name: fmt.Sprintf("p_%s_users", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_users (
				project_id VARCHAR(255) NOT NULL,
				user_id VARCHAR(255) NOT NULL,
				email VARCHAR(255),
				phone VARCHAR(255),
				user_data JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, user_id),
				INDEX idx_email (email),
				INDEX idx_phone (phone)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
		// Project models metadata table
		{
			name: fmt.Sprintf("p_%s_models", projectID),
			ddl: fmt.Sprintf(`CREATE TABLE IF NOT EXISTS p_%s_models (
				project_id VARCHAR(255) NOT NULL,
				model_name VARCHAR(255) NOT NULL,
				model_data JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				PRIMARY KEY (project_id, model_name)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, projectID),
		},
	}

	for _, table := range tables {
		_, err := m.DB.ExecContext(ctx, table.ddl)
		if err != nil {
			return fmt.Errorf("failed to create table %s: %v", table.name, err)
		}
	}

	return nil
}
