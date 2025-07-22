package couchbase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/couchbase/gocb/v2"
	"github.com/google/uuid"
)

type CouchbaseDriver struct {
	Cluster          *gocb.Cluster
	Bucket           *gocb.Bucket
	Collection       *gocb.Collection
	DriverCredential *models.DriverCredentials
}

// GetCouchbaseDriver creates a new Couchbase system driver instance
func GetCouchbaseDriver(driverCredentials *models.DriverCredentials) (*CouchbaseDriver, error) {
	// Connect to Couchbase cluster
	cluster, err := gocb.Connect(
		fmt.Sprintf("couchbase://%s:%s", driverCredentials.Host, driverCredentials.Port),
		gocb.ClusterOptions{
			Authenticator: gocb.PasswordAuthenticator{
				Username: driverCredentials.User,
				Password: driverCredentials.Password,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Couchbase: %v", err)
	}

	// Wait for connection to be ready
	err = cluster.WaitUntilReady(10*time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("cluster not ready: %v", err)
	}

	// Open bucket
	bucket := cluster.Bucket(driverCredentials.Database)
	err = bucket.WaitUntilReady(5*time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("bucket not ready: %v", err)
	}

	// Get default collection
	collection := bucket.DefaultCollection()

	return &CouchbaseDriver{
		Cluster:          cluster,
		Bucket:           bucket,
		Collection:       collection,
		DriverCredential: driverCredentials,
	}, nil
}


// GetTeams retrieves teams for a given user using Couchbase
func (c *CouchbaseDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	query := `SELECT team_data FROM user_teams WHERE user_id = $1`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var teams []*models.Team
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if teamDataStr, ok := row["team_data"].(string); ok {
			var team models.Team
			if err := json.Unmarshal([]byte(teamDataStr), &team); err == nil {
				teams = append(teams, &team)
			}
		}
	}

	return teams, nil
}

// GetTeamsMembers retrieves team members for a project using Couchbase
func (c *CouchbaseDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	query := `SELECT user_data FROM project_teams WHERE project_id = $1`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{projectId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var users []*models.SystemUser
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if userDataStr, ok := row["user_data"].(string); ok {
			var user models.SystemUser
			if err := json.Unmarshal([]byte(userDataStr), &user); err == nil {
				users = append(users, &user)
			}
		}
	}

	return users, nil
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions using Couchbase
func (c *CouchbaseDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	query := `SELECT project_data, role, permissions FROM user_projects WHERE user_id = $1`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var projectWithRoles []*models.ProjectWithRoles

	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		projectDataStr, _ := row["project_data"].(string)
		role, _ := row["role"].(string)
		permissionsStr, _ := row["permissions"].(string)

		var project models.Project
		if err := json.Unmarshal([]byte(projectDataStr), &project); err != nil {
			continue
		}

		var permissions []string
		if err := json.Unmarshal([]byte(permissionsStr), &permissions); err != nil {
			permissions = []string{} // Default to empty permissions
		}

		// Get user data
		user, err := c.GetSystemUser(ctx, userId)
		if err != nil {
			continue
		}

		projectWithRoles = append(projectWithRoles, &models.ProjectWithRoles{
			User:        user,
			Project:     &project,
			Role:        role,
			Permissions: permissions,
		})
	}

	return projectWithRoles, nil
}

// SearchResource searches for resources based on common system parameters using Couchbase
func (c *CouchbaseDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// Generic search implementation - can be extended based on specific needs
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization using Couchbase
func (c *CouchbaseDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	query := `SELECT user_id FROM user_organizations WHERE organization_id = $1 AND role = $2 LIMIT 1`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{orgId, "admin"},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return nil, fmt.Errorf("organization admin not found")
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	userId, _ := row["user_id"].(string)
	return c.GetSystemUser(ctx, userId)
}

// SaveAuditLog saves an audit log entry using Couchbase
func (c *CouchbaseDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	auditLogJson, err := json.Marshal(auditLog)
	if err != nil {
		return err
	}

	doc := map[string]interface{}{
		"id":         auditLog.ID,
		"user_id":    auditLog.UserID,
		"project_id": auditLog.ProjectID,
		"audit_data": string(auditLogJson),
		"created_at": time.Now().Format(time.RFC3339),
		"doc_type":   "audit_log",
	}

	_, err = c.Collection.Upsert("audit_log::"+auditLog.ID, doc, nil)
	return err
}

// SearchAuditLogs searches for audit logs based on common system parameters using Couchbase
func (c *CouchbaseDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	query := fmt.Sprintf("SELECT audit_data FROM `%s` WHERE doc_type = \"audit_log\"", c.Bucket.Name())
	args := []interface{}{}

	if param.UserID != "" && param.ProjectID != "" {
		query += ` AND user_id = $1 AND project_id = $2`
		args = append(args, param.UserID, param.ProjectID)
	} else if param.UserID != "" {
		query += ` AND user_id = $1`
		args = append(args, param.UserID)
	} else if param.ProjectID != "" {
		query += ` AND project_id = $1`
		args = append(args, param.ProjectID)
	}

	query += ` ORDER BY created_at DESC LIMIT 100`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: args,
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var logs []*models.AuditLogs
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if auditDataStr, ok := row["audit_data"].(string); ok {
			var log models.AuditLogs
			if err := json.Unmarshal([]byte(auditDataStr), &log); err == nil {
				logs = append(logs, &log)
			}
		}
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations retrieves organizations for a given user using Couchbase
func (c *CouchbaseDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	query := `SELECT organization_data FROM user_organizations WHERE user_id = $1`

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var organizations []*models.Organization
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if orgDataStr, ok := row["organization_data"].(string); ok {
			var org models.Organization
			if err := json.Unmarshal([]byte(orgDataStr), &org); err == nil {
				organizations = append(organizations, &org)
			}
		}
	}

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}

// RunMigration runs the database migrations for Couchbase (creates necessary indexes)
func (c *CouchbaseDriver) RunMigration(ctx context.Context) error {
	// Create primary index
	_, err := c.Cluster.Query(fmt.Sprintf("CREATE PRIMARY INDEX ON `%s`", c.Bucket.Name()), nil)
	if err != nil && !isIndexExistsError(err) {
		return err
	}

	// Create indexes for better query performance
	indexes := []string{
		fmt.Sprintf("CREATE INDEX idx_doc_type ON `%s`(doc_type)", c.Bucket.Name()),
		fmt.Sprintf("CREATE INDEX idx_user_id ON `%s`(user_id) WHERE doc_type IN [\"user_project\", \"user_organization\", \"user_team\", \"audit_log\"]", c.Bucket.Name()),
		fmt.Sprintf("CREATE INDEX idx_project_id ON `%s`(project_id) WHERE doc_type IN [\"user_project\", \"project_team\", \"webhook\", \"audit_log\"]", c.Bucket.Name()),
		fmt.Sprintf("CREATE INDEX idx_email ON `%s`(email) WHERE doc_type = \"user\"", c.Bucket.Name()),
		fmt.Sprintf("CREATE INDEX idx_name ON `%s`(name) WHERE doc_type IN [\"project\", \"organization\", \"team\"]", c.Bucket.Name()),
		fmt.Sprintf("CREATE INDEX idx_created_at ON `%s`(created_at) WHERE doc_type = \"audit_log\"", c.Bucket.Name()),
	}

	for _, index := range indexes {
		_, err := c.Cluster.Query(index, nil)
		if err != nil && !isIndexExistsError(err) {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	return nil
}

// isIndexExistsError checks if the error is related to index already existing
func isIndexExistsError(err error) bool {
	return err != nil && (fmt.Sprintf("%v", err) == "index already exists" ||
		fmt.Sprintf("%v", err) == "Index already exists")
}

