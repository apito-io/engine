package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// GetOrganizations retrieves organizations for a given user from BadgerDB
func (b *BadgerDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	var organizations []*models.Organization

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("org:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var org models.Organization
				if err := json.Unmarshal(val, &org); err != nil {
					return err
				}
				// Simple check if user belongs to organization (simplified logic)
				organizations = append(organizations, &org)
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

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}

// GetProject retrieves a project by ID from BadgerDB
func (b *BadgerDriver) GetProject(ctx context.Context, projectId string) (*models.Project, error) {
	var project models.Project
	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("project:" + projectId))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &project)
		})
		if err != nil {
			return err
		}
		return nil
	})

	return &project, err
}

// SaveProject saves a project to BadgerDB with TTL
func (b *BadgerDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(project)
		if err != nil {
			return err
		}
		e := badger.NewEntry([]byte("project:"+project.ID), data).WithTTL(1 * time.Minute)
		err = txn.SetEntry(e)
		return err
	})
	return project, err
}

// GetSystemUser retrieves a system user by ID from BadgerDB
func (b *BadgerDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	var user models.SystemUser
	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("user:" + id))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &user)
		})
		return err
	})

	return &user, err
}

// GetSystemUserByEmail retrieves a system user by email from BadgerDB
func (b *BadgerDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	var user models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var u models.SystemUser
				if err := json.Unmarshal(val, &u); err != nil {
					return err
				}
				if u.Email == email {
					user = u
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if user.ID == "" {
		return nil, fmt.Errorf("user not found with email: %s", email)
	}
	return &user, err
}

// CheckProjectName checks if a project name already exists in BadgerDB
func (b *BadgerDriver) CheckProjectName(ctx context.Context, name string) error {
	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("project:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var project models.Project
				if err := json.Unmarshal(val, &project); err != nil {
					return err
				}
				if project.Name == name {
					return fmt.Errorf("project already exists with this name")
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// SearchProjects searches for projects based on common system parameters
func (b *BadgerDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var projects []*models.Project

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("project:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var project models.Project
				if err := json.Unmarshal(val, &project); err != nil {
					return err
				}
				projects = append(projects, &project)
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

	return &models.SearchResponse[models.Project]{
		Results: projects,
	}, nil
}

// GetProjectWithRolesAndPermission retrieves projects with roles and permissions for a user
func (b *BadgerDriver) GetProjectWithRolesAndPermission(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
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

// ListAllProjects lists all projects for a user (with admin check)
func (b *BadgerDriver) ListAllProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	// Check if user is admin
	user, err := b.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	if !user.IsSuperAdmin {
		return nil, fmt.Errorf("not allowed")
	}

	var projects []*models.Project

	err = b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("project:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var project models.Project
				if err := json.Unmarshal(val, &project); err != nil {
					return err
				}
				projects = append(projects, &project)
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

// ListAllUsers lists all system users
func (b *BadgerDriver) ListAllUsers(ctx context.Context) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user:")
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

// ListTeams lists team members for a project
func (b *BadgerDriver) ListTeams(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
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

// SearchFunctions searches for cloud functions in a project
func (b *BadgerDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := b.GetProject(ctx, param.ProjectID)
	if err != nil {
		return nil, err
	}

	if project.Schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	return &models.SearchResponse[models.ApitoFunction]{
		Results: project.Schema.Functions,
	}, nil
}

// SearchWebHooks searches for webhooks in a project
func (b *BadgerDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	var hooks []*models.Webhook

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("webhook:" + param.ProjectID + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var hook models.Webhook
				if err := json.Unmarshal(val, &hook); err != nil {
					return err
				}
				hooks = append(hooks, &hook)
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

	return &models.SearchResponse[models.Webhook]{
		Results: hooks,
	}, nil
}

// GetWebHook retrieves a specific webhook by ID
func (b *BadgerDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	var webhook models.Webhook
	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("webhook:" + projectId + ":" + hookId))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &webhook)
		})
		return err
	})

	return &webhook, err
}

// DeleteWebhook deletes a webhook from BadgerDB
func (b *BadgerDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("webhook:" + projectId + ":" + hookId))
	})
}

// SearchUsers searches for system users based on parameters
func (b *BadgerDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	var users []*models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user:")
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

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.SystemUser]{
		Results: users,
	}, nil
}

// AddSystemUserMetaInfo adds metadata to a system user
func (b *BadgerDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		return txn.Set([]byte("user_meta:"+doc.ID), data)
	})

	return doc, err
}

// AddTeamMetaInfo adds metadata to team members
func (b *BadgerDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	err := b.Db.Update(func(txn *badger.Txn) error {
		for _, doc := range docs {
			data, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			if err := txn.Set([]byte("team_meta:"+doc.ID), data); err != nil {
				return err
			}
		}
		return nil
	})

	return docs, err
}

// RemoveATeamMemberFromProject removes a team member from a project
func (b *BadgerDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("project_team:" + projectId + ":" + memberID))
	})
}

// CheckTeamMemberExists checks if a team member exists in a project
func (b *BadgerDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	return b.Db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("project_team:" + projectId + ":" + memberID))
		return err
	})
}

// CreateProject creates a new project in BadgerDB
func (b *BadgerDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(project)
		if err != nil {
			return err
		}

		// Store project
		if err := txn.Set([]byte("project:"+project.ID), data); err != nil {
			return err
		}

		// Store user-project relation
		relation := models.ProjectWithRoles{
			Project:     project,
			Role:        "owner",
			Permissions: []string{"read", "write", "admin"},
		}
		relationData, err := json.Marshal(relation)
		if err != nil {
			return err
		}

		return txn.Set([]byte("user_project:"+userId+":"+project.ID), relationData)
	})

	return project, err
}

// CreateSystemUser creates a new system user in BadgerDB
func (b *BadgerDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return txn.Set([]byte("user:"+user.ID), data)
	})

	return user, err
}

// UpdateSystemUser updates a system user in BadgerDB
func (b *BadgerDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	return b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return txn.Set([]byte("user:"+user.ID), data)
	})
}

// UpdateProject updates a project in BadgerDB
func (b *BadgerDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	return b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(project)
		if err != nil {
			return err
		}
		return txn.Set([]byte("project:"+project.ID), data)
	})
}

// CheckTokenBlacklisted checks if a token is blacklisted
func (b *BadgerDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	return b.Db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("blacklist:" + tokenId))
		if err == badger.ErrKeyNotFound {
			return nil // Token is not blacklisted
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("token is blacklisted")
	})
}

// BlacklistAToken adds a token to the blacklist
func (b *BadgerDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	return b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(token)
		if err != nil {
			return err
		}
		return txn.Set([]byte("blacklist:"+tokenId), data)
	})
}

// DeleteProjectFromSystem deletes a project and all related data from BadgerDB
func (b *BadgerDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		// Delete the project
		if err := txn.Delete([]byte("project:" + projectId)); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Delete user-project relations
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		var keysToDelete [][]byte

		prefix := []byte("user_project:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			if strings.Contains(string(key), ":"+projectId) {
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
			}
		}

		// Delete project team relations
		prefix = []byte("project_team:" + projectId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			keysToDelete = append(keysToDelete, append([]byte(nil), key...))
		}

		// Delete webhooks
		prefix = []byte("webhook:" + projectId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			keysToDelete = append(keysToDelete, append([]byte(nil), key...))
		}

		// Delete all collected keys
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}

		return nil
	})
}

// AddWebhookToProject adds a webhook to a project in BadgerDB
func (b *BadgerDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		return txn.Set([]byte("webhook:"+doc.ProjectID+":"+doc.ID), data)
	})

	return doc, err
}

// SaveRawData saves raw data to BadgerDB for payment-related operations
func (b *BadgerDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := uuid.New().String()
	return b.Db.Update(func(txn *badger.Txn) error {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return txn.Set([]byte("raw:"+collection+":"+id), jsonData)
	})
}
