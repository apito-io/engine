// Package bbolt provides a BBolt database driver implementation for the Apito system.
// It uses the ApitoBolt SDK (github.com/apito-io/apitoBolt) to provide a MongoDB-like
// interface on top of BBolt for system-level operations like user management,
// project management, team management, webhooks, audit logs, and more.
package bbolt

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	apitobolt "github.com/apito-io/apitoBolt"
	oci "github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type ProBBoltSystemDriver struct {
	DB          *apitobolt.Store
	CacheDriver oci.CacheDBInterface
	dbPath      string
}

// MigrationStatus represents the migration state in the database
type MigrationStatus struct {
	ID        string `json:"id"`
	XKey      string `json:"_key"`
	Completed bool   `json:"completed"`
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func GetSystemBBoltDriver(cfg *models.Config, driverCred *models.DriverCredentials, cacheDriver oci.CacheDBInterface) (*ProBBoltSystemDriver, error) {
	dbPath := filepath.Join(cfg.DefaultDatabaseDir, cfg.SystemDBName)

	// Expand path (handles ~ and converts to absolute path)
	var err error
	dbPath, err = utility.ExpandPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand database path %s: %v", driverCred.Database, err)
	}

	// Create the directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %v", dbDir, err)
	}

	log.Printf("Opening System BBolt database at: %s", dbPath)

	// Open the BBolt database using ApitoBolt
	store, err := apitobolt.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open BBolt database at %s: %v", dbPath, err)
	}

	driver := &ProBBoltSystemDriver{
		DB:          store,
		CacheDriver: cacheDriver,
		dbPath:      dbPath,
	}

	// Run initial migration to create collections/buckets
	if err := driver.RunMigration(context.Background()); err != nil {
		return nil, err
	}

	return driver, nil
}

func (d *ProBBoltSystemDriver) Close() error {
	return d.DB.Close()
}

// generatePassword generates a random password string
func generatePassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	const passwordLength = 16

	password := make([]byte, passwordLength)
	for i := range password {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password[i] = charset[num.Int64()]
	}

	return string(password)
}

// printCredentials prints the admin credentials in a bordered format
func printCredentials(email, password string) {
	border := "╔══════════════════════════════════════════════════════════════════════╗"
	empty := "║                                                                      ║"
	footer := "╚══════════════════════════════════════════════════════════════════════╝"

	log.Println(border)
	log.Println(empty)
	log.Printf("║                        DEFAULT ADMIN CREDENTIALS                     ║")
	log.Println(empty)
	log.Printf("║  Email:    %-54s ║", email)
	log.Printf("║  Password: %-54s ║", password)
	log.Println(empty)
	log.Printf("║  Please login with these credentials and change the password!       ║")
	log.Println(empty)
	log.Println(footer)
}

// checkMigrationStatus checks if migration has been completed
func (d *ProBBoltSystemDriver) checkMigrationStatus(ctx context.Context) (bool, error) {
	// First ensure the migrations collection exists
	migrationsCollection := d.DB.Collection("migrations")
	if err := migrationsCollection.Init(); err != nil {
		return false, err
	}

	// Try to find existing migration status
	var migrationStatus MigrationStatus
	err := migrationsCollection.FindByID("system_migration_v1", &migrationStatus)
	if err != nil {
		// Migration status doesn't exist, create it with completed = false
		migrationStatus = MigrationStatus{
			ID:        "system_migration_v1",
			XKey:      "system_migration_v1",
			Completed: false,
			Version:   "1.0.0",
			CreatedAt: utility.GetCurrentTime(),
			UpdatedAt: utility.GetCurrentTime(),
		}

		_, err = migrationsCollection.Save(&migrationStatus)
		if err != nil {
			return false, err
		}

		return false, nil
	}

	return migrationStatus.Completed, nil
}

// markMigrationCompleted marks the migration as completed
func (d *ProBBoltSystemDriver) markMigrationCompleted(ctx context.Context) error {
	migrationsCollection := d.DB.Collection("migrations")

	migrationStatus := MigrationStatus{
		ID:        "system_migration_v1",
		XKey:      "system_migration_v1",
		Completed: true,
		Version:   "1.0.0",
		CreatedAt: utility.GetCurrentTime(),
		UpdatedAt: utility.GetCurrentTime(),
	}

	return migrationsCollection.Update(&migrationStatus)
}

// createDefaultAdminUser creates the default admin user
func (d *ProBBoltSystemDriver) createDefaultAdminUser(ctx context.Context) error {
	// Check if admin user already exists
	usersCollection := d.DB.Collection("users")
	var existingUser models.SystemUser
	err := usersCollection.FindOne("email", "admin@apito.io", &existingUser)
	if err == nil && existingUser.Email == "admin@apito.io" {
		// Admin user already exists, skip creation
		log.Println("Default admin user already exists, skipping creation...")
		return nil
	}

	// Create the admin user
	adminUser := &models.SystemUser{
		XKey:                      uuid.New().String(),
		FirstName:                 "System",
		LastName:                  "Administrator",
		Email:                     "admin@apito.io",
		Username:                  "admin",
		//Secret:                    password, // In production, this should be hashed
		IsAdmin:                   true,
		AdministrativePermissions: []string{"all"},
		CreatedAt:                 utility.GetCurrentTime(),
		UpdatedAt:                 utility.GetCurrentTime(),
	}

	// Generate random password
	password := generatePassword()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	adminUser.Secret = string(hash)
	adminUser.ID = adminUser.XKey

	// Save the admin user
	_, err = usersCollection.Save(adminUser)
	if err != nil {
		return fmt.Errorf("failed to create default admin user: %v", err)
	}

	// Print credentials in bordered format
	printCredentials("admin@apito.io", password)

	return nil
}

// RunMigration creates the necessary collections/buckets for the system with migration tracking
func (d *ProBBoltSystemDriver) RunMigration(ctx context.Context) error {
	// Check if migration has already been completed
	migrationCompleted, err := d.checkMigrationStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %v", err)
	}

	if migrationCompleted {
		log.Println("System migration already completed, skipping...")
		return nil
	}

	log.Println("Starting system database migration...")

	// List of collections that need to be created
	collections := []string{
		"users",
		"projects",
		"organizations",
		"teams",
		"webhooks",
		"audit_logs",
		"token_blacklist",
		"usages",
		"subscriptions",
		"invoices",
		"migrations",
	}

	// Create collections and ensure indexes
	for _, collectionName := range collections {
		log.Printf("Creating collection: %s", collectionName)
		collection := d.DB.Collection(collectionName)

		// Initialize the collection (creates bucket if it doesn't exist)
		if err := collection.Init(); err != nil {
			return fmt.Errorf("failed to initialize collection %s: %v", collectionName, err)
		}

		// Create indexes based on collection type
		switch collectionName {
		case "users":
			// Create index on email for fast lookups
			if err := collection.EnsureIndex("email", true); err != nil {
				return fmt.Errorf("failed to create email index on users: %v", err)
			}
			log.Println("  - Created unique index on email field")
		case "projects":
			// Create index on organization id for project queries
			if err := collection.EnsureIndex("organization_id", false); err != nil {
				return fmt.Errorf("failed to create organization_id index on projects: %v", err)
			}
			log.Println("  - Created index on organization_id field")
		case "webhooks":
			// Create index on project_id for webhook queries
			if err := collection.EnsureIndex("project_id", false); err != nil {
				return fmt.Errorf("failed to create project_id index on webhooks: %v", err)
			}
			log.Println("  - Created index on project_id field")
		case "audit_logs":
			// Create index on project_id and user_id for audit log queries
			if err := collection.EnsureIndex("project_id", false); err != nil {
				return fmt.Errorf("failed to create project_id index on audit_logs: %v", err)
			}
			if err := collection.EnsureIndex("user_id", false); err != nil {
				return fmt.Errorf("failed to create user_id index on audit_logs: %v", err)
			}
			log.Println("  - Created indexes on project_id and user_id fields")
		case "usages":
			// Create index on project_id for usage queries
			if err := collection.EnsureIndex("project_id", false); err != nil {
				return fmt.Errorf("failed to create project_id index on usages: %v", err)
			}
			log.Println("  - Created index on project_id field")
		case "subscriptions":
			// Create index on user_id for subscription queries
			if err := collection.EnsureIndex("user_id", false); err != nil {
				return fmt.Errorf("failed to create user_id index on subscriptions: %v", err)
			}
			log.Println("  - Created index on user_id field")
		}
	}

	// Create default admin user
	log.Println("Creating default admin user...")
	if err := d.createDefaultAdminUser(ctx); err != nil {
		return fmt.Errorf("failed to create default admin user: %v", err)
	}

	// Mark migration as completed
	if err := d.markMigrationCompleted(ctx); err != nil {
		return fmt.Errorf("failed to mark migration as completed: %v", err)
	}

	log.Println("System database migration completed successfully!")
	return nil
}
