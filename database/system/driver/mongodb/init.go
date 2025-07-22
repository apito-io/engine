package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SystemMongoDriver struct {
	Client           *mongo.Client
	Database         *mongo.Database
	DriverCredential *models.DriverCredentials
}

// GetMongoDriver creates a new MongoDB system driver instance
func GetMongoDriver(driverCredentials *models.DriverCredentials) (*SystemMongoDriver, error) {
	// Create MongoDB connection string
	var connectionURI string
	if driverCredentials.Port == "" {
		// MongoDB Atlas or SRV connection (cloud)
		if driverCredentials.User != "" && driverCredentials.Password != "" {
			connectionURI = fmt.Sprintf("mongodb+srv://%s:%s@%s/%s?retryWrites=true&w=majority&authSource=admin",
				driverCredentials.User,
				driverCredentials.Password,
				driverCredentials.Host,
				driverCredentials.Database)
		} else {
			// No authentication
			connectionURI = fmt.Sprintf("mongodb+srv://%s/%s?retryWrites=true&w=majority",
				driverCredentials.Host,
				driverCredentials.Database)
		}
	} else {
		// Standard MongoDB connection (local/self-hosted)
		if driverCredentials.User != "" && driverCredentials.Password != "" {
			connectionURI = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?retryWrites=true&w=majority&authSource=admin",
				driverCredentials.User,
				driverCredentials.Password,
				driverCredentials.Host,
				driverCredentials.Port,
				driverCredentials.Database)
		} else {
			// No authentication (local development)
			connectionURI = fmt.Sprintf("mongodb://%s:%s/%s?retryWrites=true&w=majority",
				driverCredentials.Host,
				driverCredentials.Port,
				driverCredentials.Database)
		}
	}
	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create client options and connect to MongoDB
	opts := options.Client().ApplyURI(connectionURI)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB connection failed: %w", err)
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		client.Disconnect(ctx) // Clean up on ping failure
		return nil, fmt.Errorf("MongoDB ping failed (check credentials and network): %w", err)
	}

	// Get database
	database := client.Database(driverCredentials.Database)

	return &SystemMongoDriver{
		Client:           client,
		Database:         database,
		DriverCredential: driverCredentials,
	}, nil
}

func (m *SystemMongoDriver) Ping() error {
	return m.Client.Ping(context.Background(), nil)
}

// GetTeams retrieves teams for a given user using MongoDB
func (m *SystemMongoDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	userTeamsCollection := m.Database.Collection("user_teams")

	// Find team IDs for the user
	cursor, err := userTeamsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userTeams []map[string]interface{}
	if err = cursor.All(ctx, &userTeams); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var teamIds []string
	for _, ut := range userTeams {
		if teamId, ok := ut["team_id"].(string); ok {
			teamIds = append(teamIds, teamId)
		}
	}

	// Get teams
	teamsCollection := m.Database.Collection("teams")
	teamCursor, err := teamsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": teamIds}})
	if err != nil {
		return nil, err
	}
	defer teamCursor.Close(ctx)

	var teams []*models.Team
	for teamCursor.Next(ctx) {
		var team models.Team
		if err := teamCursor.Decode(&team); err != nil {
			return nil, err
		}
		teams = append(teams, &team)
	}

	return teams, nil
}

// GetTeamsMembers retrieves team members for a project using MongoDB
func (m *SystemMongoDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	projectTeamsCollection := m.Database.Collection("project_teams")

	// Find user IDs for the project
	cursor, err := projectTeamsCollection.Find(ctx, bson.M{"project_id": projectId})
	if err != nil {
		return nil, err
	}

	var projectTeams []map[string]interface{}
	if err = cursor.All(ctx, &projectTeams); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var userIds []string
	for _, pt := range projectTeams {
		if userId, ok := pt["user_id"].(string); ok {
			userIds = append(userIds, userId)
		}
	}

	// Get users
	usersCollection := m.Database.Collection("users")
	userCursor, err := usersCollection.Find(ctx, bson.M{"_id": bson.M{"$in": userIds}})
	if err != nil {
		return nil, err
	}
	defer userCursor.Close(ctx)

	var users []*models.SystemUser
	for userCursor.Next(ctx) {
		var user models.SystemUser
		if err := userCursor.Decode(&user); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions using MongoDB
func (m *SystemMongoDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	userProjectsCollection := m.Database.Collection("user_projects")

	// Find user-project relations
	cursor, err := userProjectsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userProjects []map[string]interface{}
	if err = cursor.All(ctx, &userProjects); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var projectWithRoles []*models.ProjectWithRoles

	for _, up := range userProjects {
		projectId, _ := up["project_id"].(string)
		role, _ := up["role"].(string)
		permissions, _ := up["permissions"].([]interface{})

		// Get the project
		project, err := m.GetProject(ctx, projectId)
		if err != nil {
			continue // Skip if project not found
		}

		// Get the user
		user, err := m.GetSystemUser(ctx, userId)
		if err != nil {
			continue // Skip if user not found
		}

		var permList []string
		for _, perm := range permissions {
			if p, ok := perm.(string); ok {
				permList = append(permList, p)
			}
		}

		projectWithRoles = append(projectWithRoles, &models.ProjectWithRoles{
			User:        user,
			Project:     project,
			Role:        role,
			Permissions: permList,
		})
	}

	return projectWithRoles, nil
}

// SearchResource searches for resources based on common system parameters using MongoDB
func (m *SystemMongoDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// This is a generic search function - implementation depends on specific resource type
	// For now, return empty results
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization using MongoDB
func (m *SystemMongoDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	userOrganizationsCollection := m.Database.Collection("user_organizations")

	var userOrg map[string]interface{}
	err := userOrganizationsCollection.FindOne(ctx, bson.M{
		"organization_id": orgId,
		"role":            "admin",
	}).Decode(&userOrg)
	if err != nil {
		return nil, err
	}

	userId, ok := userOrg["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("user ID not found")
	}

	return m.GetSystemUser(ctx, userId)
}

// SaveAuditLog saves an audit log entry using MongoDB
func (m *SystemMongoDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	collection := m.Database.Collection("audit_logs")
	_, err := collection.InsertOne(ctx, auditLog)

	return err
}

// SearchAuditLogs searches for audit logs based on common system parameters using MongoDB
func (m *SystemMongoDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	collection := m.Database.Collection("audit_logs")

	filter := bson.M{}

	if param.UserID != "" {
		filter["user_id"] = param.UserID
	}
	if param.ProjectID != "" {
		filter["project_id"] = param.ProjectID
	}

	opts := options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(100)
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*models.AuditLogs
	for cursor.Next(ctx) {
		var log models.AuditLogs
		if err := cursor.Decode(&log); err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations retrieves organizations for a given user using MongoDB
func (m *SystemMongoDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	userOrganizationsCollection := m.Database.Collection("user_organizations")

	// Find organization IDs for the user
	cursor, err := userOrganizationsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userOrgs []map[string]interface{}
	if err = cursor.All(ctx, &userOrgs); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var orgIds []string
	for _, uo := range userOrgs {
		if orgId, ok := uo["organization_id"].(string); ok {
			orgIds = append(orgIds, orgId)
		}
	}

	// Get organizations
	organizationsCollection := m.Database.Collection("organizations")
	orgCursor, err := organizationsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": orgIds}})
	if err != nil {
		return nil, err
	}
	defer orgCursor.Close(ctx)

	var organizations []*models.Organization
	for orgCursor.Next(ctx) {
		var org models.Organization
		if err := orgCursor.Decode(&org); err != nil {
			return nil, err
		}
		organizations = append(organizations, &org)
	}

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}

// RunMigration runs the database migrations for MongoDB (creates necessary collections and indexes)
func (m *SystemMongoDriver) RunMigration(ctx context.Context) error {
	// Create collections with proper indexes
	collections := []string{
		"users", "projects", "organizations", "teams", "webhooks",
		"user_projects", "user_organizations", "user_teams",
		"project_teams", "team_projects", "organization_teams",
		"organization_projects", "token_blacklist", "audit_logs",
		"user_metadata", "team_metadata", "raw_data",
	}

	for _, collName := range collections {
		// Create collection if it doesn't exist
		err := m.Database.CreateCollection(ctx, collName)
		if err != nil {
			// Collection might already exist, which is fine
			if !mongo.IsDuplicateKeyError(err) && err.Error() != "Collection already exists." {
				return err
			}
		}

		collection := m.Database.Collection(collName)

		// Create indexes based on collection type
		switch collName {
		case "users":
			_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.M{"email": 1},
				Options: options.Index().SetUnique(true),
			})
		case "projects":
			_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.M{"name": 1},
			})
		case "user_projects":
			_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys:    bson.M{"user_id": 1, "project_id": 1},
				Options: options.Index().SetUnique(true),
			})
		case "webhooks":
			_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.M{"project_id": 1},
			})
		case "audit_logs":
			_, err = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
				Keys: bson.M{"created_at": -1},
			})
		}

		if err != nil {
			return err
		}
	}

	return nil
}
