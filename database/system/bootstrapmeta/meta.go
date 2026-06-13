// Package bootstrapmeta holds shared constants and helpers for system DB first-run bootstrap.
// Orchestration lives in each ApitoSystemDB driver implementation.
package bootstrapmeta

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

// CreateStarterProjectFn creates the starter project row when it does not exist yet.
type CreateStarterProjectFn func(ctx context.Context, userID string, proj *models.Project) error

const (
	AdminEmail         = "admin@apito.io"
	DefaultAdminPass   = "#ApitoRocks#"
	AdminFirstName     = "Apito"
	AdminLastName      = "Admin"
	StarterProjectID   = "apito_website"
	StarterProjectName = "Apito Website"
	// OSSStarterSQLiteFile is the SQLite filename (under Config.DefaultDatabaseDir) for the bundled
	// starter project when SYSTEM_DB_ENGINE=coredb. Not read from PROJECT_DB_* env.
	OSSStarterSQLiteFile = "apito_starter.sqlite"
	// StarterMongoDatabaseName is the logical Mongo database for the OSS starter project when the
	// system database is MongoDB (uses SYSTEM_DB_* connection fields).
	StarterMongoDatabaseName = "apito_starter_apito_website"
	// StarterPostgresDatabaseName is the physical database for the OSS starter project when the
	// system database engine is PostgreSQL (uses SYSTEM_DB_* connection as admin; InitProjectBase creates the DB).
	StarterPostgresDatabaseName = "apito_starter_apito_website"
	bboltUserNotFound  = "user not found"
)

// HashDefaultAdminPassword returns a bcrypt hash for the default bootstrap password.
func HashDefaultAdminPassword() ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(DefaultAdminPass), bcrypt.DefaultCost)
}

// LogDefaultAdminCredentials prints the bordered login banner (plaintext password).
func LogDefaultAdminCredentials(email, plainPassword string) {
	border := "╔══════════════════════════════════════════════════════════════════════╗"
	empty := "║                                                                      ║"
	footer := "╚══════════════════════════════════════════════════════════════════════╝"

	log.Println(border)
	log.Println(empty)
	log.Printf("║                        DEFAULT ADMIN CREDENTIALS                     ║")
	log.Println(empty)
	log.Printf("║  Email:    %-54s ║", email)
	log.Printf("║  Password: %-54s ║", plainPassword)
	log.Println(empty)
	log.Printf("║  Please login with these credentials and change the password!       ║")
	log.Println(empty)
	log.Println(footer)
}

// IsUserLookupMiss reports whether err means the user document was not found.
func IsUserLookupMiss(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.TrimSpace(err.Error())
	if msg == bboltUserNotFound {
		return true
	}
	// Arango returns "user not found : email@..."; Badger uses "user not found with email: ...".
	return strings.HasPrefix(msg, "user not found")
}
