package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/apito-io/engine/models"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

type BadgerDriver struct {
	Db *badger.DB
}

// GetTeams retrieves teams for a given user from BadgerDB
func (b *BadgerDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	var teams []*models.Team

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user_team:" + userId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var team models.Team
				if err := json.Unmarshal(val, &team); err != nil {
					return err
				}
				teams = append(teams, &team)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return teams, err
}

// GetTeamsMembers retrieves team members for a project from BadgerDB
func (b *BadgerDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("project_team:" + projectId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var user models.SystemUser
				if err := json.Unmarshal(val, &user); err != nil {
					return err
				}
				users = append(users, &user)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return users, err
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions from BadgerDB
func (b *BadgerDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	var projectWithRoles []*models.ProjectWithRoles

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user_project:" + userId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var pwr models.ProjectWithRoles
				if err := json.Unmarshal(val, &pwr); err != nil {
					return err
				}
				projectWithRoles = append(projectWithRoles, &pwr)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return projectWithRoles, err
}

// FindUserProjects retrieves all projects for a given user from BadgerDB
func (b *BadgerDriver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	var projects []*models.Project

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user_project:" + userId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var pwr models.ProjectWithRoles
				if err := json.Unmarshal(val, &pwr); err != nil {
					return err
				}
				if pwr.Project != nil {
					projects = append(projects, pwr.Project)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return projects, err
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions
func (b *BadgerDriver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	var projectWithRoles models.ProjectWithRoles
	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("user_project:" + userId + ":" + projectId))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &projectWithRoles)
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return &projectWithRoles, nil
}

// SearchResource searches for resources based on common system parameters
func (b *BadgerDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	var resources []*any

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// For simplicity, search across all data (in a real implementation, you'd filter by type)
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var resource any
				if err := json.Unmarshal(val, &resource); err != nil {
					return err
				}
				resources = append(resources, &resource)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[any]{
		Results: resources,
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization from BadgerDB
func (b *BadgerDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	var admin models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("org_admin:" + orgId))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &admin)
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return &admin, nil
}

// SaveAuditLog saves an audit log entry to BadgerDB
func (b *BadgerDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	return b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(auditLog)
		if err != nil {
			return err
		}
		return txn.Set([]byte("audit:"+auditLog.ID), data)
	})
}

// SearchAuditLogs searches for audit logs based on common system parameters
func (b *BadgerDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	var logs []*models.AuditLogs

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("audit:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var log models.AuditLogs
				if err := json.Unmarshal(val, &log); err != nil {
					return err
				}
				logs = append(logs, &log)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// RunMigration runs the database migrations for BadgerDB (creates necessary prefixes/structure)
func (b *BadgerDriver) RunMigration(ctx context.Context) error {
	// BadgerDB doesn't require explicit schema creation like SQL databases
	// We can create some initial index entries or verify database structure
	return b.Db.Update(func(txn *badger.Txn) error {
		// Create a migration marker to indicate the database has been initialized
		migrationData := map[string]interface{}{
			"version":    "1.0.0",
			"created_at": "2024-01-01T00:00:00Z",
			"status":     "completed",
		}

		data, err := json.Marshal(migrationData)
		if err != nil {
			return err
		}

		return txn.Set([]byte("migration:init"), data)
	})
}

// GetSystemBadgerDriver creates a new BadgerDB system driver instance
func GetSystemBadgerDriver(cfg *models.DriverCredentials) (*BadgerDriver, error) {
	// Open the Badger database located in the /tmp/badger directory.
	// It will be created if it doesn't exist.
	db, err := badger.Open(badger.DefaultOptions("./db/system/badger"))
	if err != nil {
		log.Fatal(err)
	}
	//defer db.Close()

	return &BadgerDriver{Db: db}, nil
}
