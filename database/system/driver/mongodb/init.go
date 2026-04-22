package mongodb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// atlasBootstrapTimeout is for first SRV/DNS/TLS/server selection; 10s is often too short for Atlas.
const atlasBootstrapTimeout = 45 * time.Second

type SystemMongoDriver struct {
	Client           *mongo.Client
	Database         *mongo.Database
	DriverCredential *models.DriverCredentials
}

// buildSystemMongoURI builds a MongoDB connection URI. Empty port implies mongodb+srv (Atlas / SRV).
func buildSystemMongoURI(c *models.DriverCredentials) (string, bool, error) {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		return "", false, fmt.Errorf("MongoDB host is required (SYSTEM_DB_HOST)")
	}
	db := strings.Trim(strings.TrimSpace(c.Database), "/")
	port := strings.TrimSpace(c.Port)

	if port == "" {
		u := &url.URL{Scheme: "mongodb+srv", Host: host}
		if strings.TrimSpace(c.User) != "" || strings.TrimSpace(c.Password) != "" {
			u.User = url.UserPassword(c.User, c.Password)
		}
		if db != "" {
			u.Path = "/" + db
		}
		q := url.Values{}
		q.Set("retryWrites", "true")
		q.Set("w", "majority")
		if u.User != nil {
			// Atlas DB users are typically defined on the admin auth DB.
			q.Set("authSource", "admin")
		}
		if db != "" {
			q.Set("appName", db)
		}
		u.RawQuery = q.Encode()
		return u.String(), true, nil
	}

	u := &url.URL{
		Scheme: "mongodb",
		Host:   net.JoinHostPort(host, port),
	}
	if strings.TrimSpace(c.User) != "" || strings.TrimSpace(c.Password) != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	if db != "" {
		u.Path = "/" + db
	}
	q := url.Values{}
	q.Set("retryWrites", "true")
	q.Set("w", "majority")
	if u.User != nil {
		q.Set("authSource", "admin")
	}
	u.RawQuery = q.Encode()
	return u.String(), false, nil
}

// GetSystemMongoDriver creates a new MongoDB system driver instance
func GetSystemMongoDriver(driverCredentials *models.DriverCredentials) (*SystemMongoDriver, error) {
	connectionURI, useSRV, err := buildSystemMongoURI(driverCredentials)
	if err != nil {
		return nil, err
	}
	bootstrapTimeout := 20 * time.Second
	if useSRV {
		bootstrapTimeout = atlasBootstrapTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	opts := options.Client().ApplyURI(connectionURI)
	if useSRV {
		opts.SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1))
		// Ping uses ctx deadline; align driver server selection so context is not the only bottleneck.
		opts.SetServerSelectionTimeout(bootstrapTimeout - 2*time.Second)
		opts.SetConnectTimeout(30 * time.Second)
	}
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

// mongoIsNamespaceExists reports whether err is MongoDB error 48 (NamespaceExists).
func mongoIsNamespaceExists(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return ce.HasErrorCode(48)
	}
	return false
}

// mongoIsIndexConflict reports index-already-exists style errors (codes 85/86) or equivalent wording.
func mongoIsIndexConflict(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		if ce.HasErrorCode(85) || ce.HasErrorCode(86) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") && strings.Contains(msg, "index")
}

func mongoEnsureCollection(ctx context.Context, db *mongo.Database, name string) error {
	err := db.CreateCollection(ctx, name)
	if err == nil || mongoIsNamespaceExists(err) {
		return nil
	}
	return err
}

func mongoCreateIndex(ctx context.Context, coll *mongo.Collection, model mongo.IndexModel) error {
	_, err := coll.Indexes().CreateOne(ctx, model)
	if err == nil || mongoIsIndexConflict(err) {
		return nil
	}
	return err
}

func mongoEnsureIndexes(ctx context.Context, coll *mongo.Collection, collName string) error {
	switch collName {
	case "users":
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		})
	case "projects":
		if err := mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "name", Value: 1}}}); err != nil {
			return err
		}
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "organization_id", Value: 1}}})
	case "user_projects":
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "project_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		})
	case "webhooks":
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "project_id", Value: 1}}})
	case "audit_logs":
		if err := mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "created_at", Value: -1}}}); err != nil {
			return err
		}
		if err := mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "project_id", Value: 1}}}); err != nil {
			return err
		}
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "user_id", Value: 1}}})
	case "usages":
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "project_id", Value: 1}}})
	case "subscriptions":
		return mongoCreateIndex(ctx, coll, mongo.IndexModel{Keys: bson.D{{Key: "user_id", Value: 1}}})
	default:
		return nil
	}
}

// RunMigration ensures collections and indexes exist. Idempotent (safe on every startup).
// Plain Mongo collections; names like user_project_edges mirror common Atlas layouts, not Arango graph semantics.
func (m *SystemMongoDriver) RunMigration(ctx context.Context) error {
	collections := []string{
		"audit_logs",
		"early_access",
		"invoices",
		"migrations",
		"organization_projects",
		"organization_teams",
		"organizations",
		"paddle",
		"project_teams",
		"projects",
		"raw_data",
		"subscriptions",
		"team_projects",
		"teams",
		"token_blacklist",
		"udbhabon_control_center",
		"usages",
		"user_metadata",
		"user_organizations",
		"user_project_edges",
		"user_projects",
		"user_teams",
		"users",
		"webhooks",
	}

	for _, collName := range collections {
		if err := mongoEnsureCollection(ctx, m.Database, collName); err != nil {
			return fmt.Errorf("create collection %q: %w", collName, err)
		}
		collection := m.Database.Collection(collName)
		if err := mongoEnsureIndexes(ctx, collection, collName); err != nil {
			return fmt.Errorf("indexes for %q: %w", collName, err)
		}
	}

	return nil
}
