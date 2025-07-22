package firestore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4"
	"github.com/apito-io/engine/models"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type FirestoreDriver struct {
	Client           *firestore.Client
	DriverCredential *models.DriverCredentials
}

// GetFirestoreDriver creates a new Firestore system driver instance
func GetFirestoreDriver(driverCredentials *models.DriverCredentials) (*FirestoreDriver, error) {
	ctx := context.Background()

	// Configure Firebase app
	config := &firebase.Config{
		ProjectID: driverCredentials.Database, // Use Database field as project ID
	}

	var app *firebase.App
	var err error

	// If Host field contains service account key file path
	if driverCredentials.Host != "" {
		opt := option.WithCredentialsFile(driverCredentials.Host)
		app, err = firebase.NewApp(ctx, config, opt)
	} else {
		// Use default credentials (for cloud environments)
		app, err = firebase.NewApp(ctx, config)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %v", err)
	}

	// Create Firestore client
	client, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %v", err)
	}

	return &FirestoreDriver{
		Client:           client,
		DriverCredential: driverCredentials,
	}, nil
}


// GetTeams retrieves teams for a given user using Firestore
func (f *FirestoreDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	iter := f.Client.Collection("user_teams").Where("user_id", "==", userId).Documents(ctx)
	defer iter.Stop()

	var teams []*models.Team
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if teamDataStr, ok := data["team_data"].(string); ok {
			var team models.Team
			if err := json.Unmarshal([]byte(teamDataStr), &team); err == nil {
				teams = append(teams, &team)
			}
		}
	}

	return teams, nil
}

// GetTeamsMembers retrieves team members for a project using Firestore
func (f *FirestoreDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	iter := f.Client.Collection("project_teams").Where("project_id", "==", projectId).Documents(ctx)
	defer iter.Stop()

	var users []*models.SystemUser
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if userDataStr, ok := data["user_data"].(string); ok {
			var user models.SystemUser
			if err := json.Unmarshal([]byte(userDataStr), &user); err == nil {
				users = append(users, &user)
			}
		}
	}

	return users, nil
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions using Firestore
func (f *FirestoreDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	iter := f.Client.Collection("user_projects").Where("user_id", "==", userId).Documents(ctx)
	defer iter.Stop()

	var projectWithRoles []*models.ProjectWithRoles

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		projectId, _ := data["project_id"].(string)
		role, _ := data["role"].(string)

		// Get the project
		project, err := f.GetProject(ctx, projectId)
		if err != nil {
			continue
		}

		// Get the user
		user, err := f.GetSystemUser(ctx, userId)
		if err != nil {
			continue
		}

		var permissions []string
		if permissionsData, ok := data["permissions"].([]interface{}); ok {
			for _, perm := range permissionsData {
				if p, ok := perm.(string); ok {
					permissions = append(permissions, p)
				}
			}
		}

		projectWithRoles = append(projectWithRoles, &models.ProjectWithRoles{
			User:        user,
			Project:     project,
			Role:        role,
			Permissions: permissions,
		})
	}

	return projectWithRoles, nil
}

// SearchResource searches for resources based on common system parameters using Firestore
func (f *FirestoreDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// Generic search implementation - can be extended based on specific needs
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization using Firestore
func (f *FirestoreDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	iter := f.Client.Collection("user_organizations").Where("organization_id", "==", orgId).Where("role", "==", "admin").Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("organization admin not found")
	}
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := doc.DataTo(&data); err != nil {
		return nil, err
	}

	userId, _ := data["user_id"].(string)
	return f.GetSystemUser(ctx, userId)
}

// SaveAuditLog saves an audit log entry using Firestore
func (f *FirestoreDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	auditLogJson, err := json.Marshal(auditLog)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"id":         auditLog.ID,
		"user_id":    auditLog.UserID,
		"project_id": auditLog.ProjectID,
		"audit_data": string(auditLogJson),
		"created_at": time.Now(),
	}

	_, err = f.Client.Collection("audit_logs").Doc(auditLog.ID).Set(ctx, data)
	return err
}

// SearchAuditLogs searches for audit logs based on common system parameters using Firestore
func (f *FirestoreDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	query := f.Client.Collection("audit_logs")

	if param.UserID != "" {
		query = query.Where("user_id", "==", param.UserID)
	}
	if param.ProjectID != "" {
		query = query.Where("project_id", "==", param.ProjectID)
	}

	iter := query.OrderBy("created_at", firestore.Desc).Limit(100).Documents(ctx)
	defer iter.Stop()

	var logs []*models.AuditLogs
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if auditDataStr, ok := data["audit_data"].(string); ok {
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

// GetOrganizations retrieves organizations for a given user using Firestore
func (f *FirestoreDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	iter := f.Client.Collection("user_organizations").Where("user_id", "==", userId).Documents(ctx)
	defer iter.Stop()

	var organizations []*models.Organization
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if orgDataStr, ok := data["organization_data"].(string); ok {
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

// RunMigration runs the database migrations for Firestore (creates necessary indexes)
func (f *FirestoreDriver) RunMigration(ctx context.Context) error {
	// Firestore doesn't require explicit schema creation like SQL databases
	// But we can create some initial collections and composite indexes

	collections := []string{
		"users", "projects", "organizations", "teams", "webhooks",
		"user_projects", "user_organizations", "user_teams",
		"project_teams", "team_projects", "organization_teams",
		"organization_projects", "token_blacklist", "audit_logs",
		"user_metadata", "team_metadata", "raw_data",
	}

	// Initialize collections by creating a temporary document and then deleting it
	for _, collName := range collections {
		tempDoc := f.Client.Collection(collName).NewDoc()
		_, err := tempDoc.Set(ctx, map[string]interface{}{
			"_temp":      true,
			"created_at": time.Now(),
		})
		if err != nil {
			return err
		}

		// Delete the temporary document
		_, err = tempDoc.Delete(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

