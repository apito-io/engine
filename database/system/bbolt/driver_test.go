package bbolt

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestProBBoltSystemDriver_Basic(t *testing.T) {
	// Create a temporary database for testing
	tmpDb := "test_apito_system.db"
	defer os.Remove(tmpDb)

	// Create driver credentials
	driverCred := &models.DriverCredentials{
		Database: tmpDb,
	}

	// Create the BBolt driver
	driver, err := GetSystemBBoltDriver(nil, driverCred, nil)
	if err != nil {
		t.Fatalf("Failed to create BBolt driver: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()

	// Test creating a system user
	user := &models.SystemUser{
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
	}

	createdUser, err := driver.CreateSystemUser(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create system user: %v", err)
	}

	if createdUser.ID == "" {
		t.Error("Created user should have an ID")
	}

	if createdUser.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", createdUser.Email)
	}

	// Test retrieving the user
	retrievedUser, err := driver.GetSystemUser(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("Failed to get system user: %v", err)
	}

	if retrievedUser.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", retrievedUser.Email)
	}

	// Test getting user by email
	userByEmail, err := driver.GetSystemUserByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("Failed to get system user by email: %v", err)
	}

	if userByEmail.ID != createdUser.ID {
		t.Errorf("Expected user ID '%s', got '%s'", createdUser.ID, userByEmail.ID)
	}

	// Test creating a project
	project := &models.Project{
		Name:        "Test Project",
		Description: "A test project",
	}

	createdProject, err := driver.CreateProject(ctx, createdUser.ID, project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	if createdProject.ID == "" {
		t.Error("Created project should have an ID")
	}

	if createdProject.Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", createdProject.Name)
	}

	// Test retrieving the project
	retrievedProject, err := driver.GetProject(ctx, createdProject.ID)
	if err != nil {
		t.Fatalf("Failed to get project: %v", err)
	}

	if retrievedProject.Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got '%s'", retrievedProject.Name)
	}

	// Test searching projects
	searchParams := &models.CommonSystemParams{
		ProjectID: createdProject.ID,
	}

	searchResponse, err := driver.SearchProjects(ctx, searchParams)
	if err != nil {
		t.Fatalf("Failed to search projects: %v", err)
	}

	if len(searchResponse.Results) == 0 {
		t.Error("Expected to find the created project in search results")
	}

	// Test creating an audit log
	auditLog := &models.AuditLogs{
		UserID:    createdUser.ID,
		ProjectID: createdProject.ID,
		Activity:  "test_activity",
	}

	err = driver.SaveAuditLog(ctx, auditLog)
	if err != nil {
		t.Fatalf("Failed to save audit log: %v", err)
	}

	// Test searching audit logs
	auditSearchParams := &models.CommonSystemParams{
		ProjectID: createdProject.ID,
		UserID:    createdUser.ID,
	}

	auditResponse, err := driver.SearchAuditLogs(ctx, auditSearchParams)
	if err != nil {
		t.Fatalf("Failed to search audit logs: %v", err)
	}

	if len(auditResponse.Results) == 0 {
		t.Error("Expected to find the created audit log in search results")
	}
}

func TestProBBoltSystemDriver_Migration(t *testing.T) {
	// Create a temporary database for testing
	tmpDb := "test_apito_migration.db"
	defer os.Remove(tmpDb)

	// Create driver credentials
	driverCred := &models.DriverCredentials{
		Database: tmpDb,
	}

	// Create the BBolt driver
	driver, err := GetSystemBBoltDriver(nil, driverCred, nil)
	if err != nil {
		t.Fatalf("Failed to create BBolt driver: %v", err)
	}
	defer driver.Close()

	ctx := context.Background()

	// Test migration
	err = driver.RunMigration(ctx)
	if err != nil {
		t.Fatalf("Failed to run migration: %v", err)
	}
}

func TestDirectoryCreation(t *testing.T) {
	// Test with nested directory path similar to ~/.apito/db/apito_system.db
	tmpDir := "/tmp/test_apito_nested"
	dbPath := filepath.Join(tmpDir, "db", "apito_system.db")

	// Cleanup
	defer os.RemoveAll(tmpDir)

	// Create driver credentials with nested path
	driverCred := &models.DriverCredentials{
		Database: dbPath,
	}

	// This should create the nested directories and open the database
	driver, err := GetSystemBBoltDriver(nil, driverCred, nil)
	if err != nil {
		t.Fatalf("Failed to create BBolt driver with nested path: %v", err)
	}
	defer driver.Close()

	// Verify that the directory was created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Verify that the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	t.Logf("Successfully created database at: %s", dbPath)
}

func TestTildeExpansion(t *testing.T) {
	// Test with tilde path similar to ~/.apito/db/apito_system.db
	dbPath := "~/.apito-test/db/apito_system.db"

	// Get home directory for cleanup
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	testDir := filepath.Join(homeDir, ".apito-test")

	// Cleanup
	defer os.RemoveAll(testDir)

	// Create driver credentials with tilde path
	driverCred := &models.DriverCredentials{
		Database: dbPath,
	}

	// This should expand the tilde and create the directories
	driver, err := GetSystemBBoltDriver(nil, driverCred, nil)
	if err != nil {
		t.Fatalf("Failed to create BBolt driver with tilde path: %v", err)
	}
	defer driver.Close()

	// Verify that the expanded directory was created
	expectedPath := filepath.Join(homeDir, ".apito-test", "db", "apito_system.db")
	if _, err := os.Stat(filepath.Dir(expectedPath)); os.IsNotExist(err) {
		t.Error("Expanded directory was not created")
	}

	// Verify that the database file was created
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Database file was not created at expanded path")
	}

	t.Logf("Successfully created database with tilde expansion at: %s", expectedPath)
}
