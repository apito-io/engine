package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// GetProjects retrieves multiple projects by their IDs from BadgerDB
func (b *BadgerDriver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	var projects []*models.Project

	err := b.Db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte("project:" + key))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue // Skip missing projects
				}
				return err
			}

			err = item.Value(func(val []byte) error {
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

// AddATeamMemberToProject adds a team member to a project in BadgerDB
func (b *BadgerDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// Get the user to add
	user, err := b.GetSystemUser(ctx, req.UserID)
	if err != nil {
		return err
	}

	return b.Db.Update(func(txn *badger.Txn) error {
		// Store team member in project
		userData, err := json.Marshal(user)
		if err != nil {
			return err
		}

		if err := txn.Set([]byte("project_team:"+req.ProjectID+":"+req.UserID), userData); err != nil {
			return err
		}

		// Store user-project relation with role
		relation := models.ProjectWithRoles{
			User:        user,
			Role:        req.Role,
			Permissions: req.Permissions,
		}
		relationData, err := json.Marshal(relation)
		if err != nil {
			return err
		}

		return txn.Set([]byte("user_project:"+req.UserID+":"+req.ProjectID), relationData)
	})
}

// GetSystemUsers retrieves multiple system users by their IDs from BadgerDB
func (b *BadgerDriver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := b.Db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte("user:" + key))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue // Skip missing users
				}
				return err
			}

			err = item.Value(func(val []byte) error {
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

// FindUserOrganizations retrieves all organizations for a given user from BadgerDB
func (b *BadgerDriver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	var organizations []*models.Organization

	err := b.Db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("user_org:" + userId + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var org models.Organization
				if err := json.Unmarshal(val, &org); err != nil {
					return err
				}
				organizations = append(organizations, &org)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return organizations, err
}

// CreateOrganization creates a new organization in BadgerDB
func (b *BadgerDriver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(org)
		if err != nil {
			return err
		}
		return txn.Set([]byte("org:"+org.ID), data)
	})

	return org, err
}

// AssignTeamToOrganization assigns a team to an organization in BadgerDB
func (b *BadgerDriver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		// Create organization-team relation
		relationKey := fmt.Sprintf("org_team:%s:%s", orgId, teamId)
		relationData := map[string]interface{}{
			"organization_id": orgId,
			"team_id":         teamId,
			"assigned_by":     userId,
			"assigned_at":     time.Now().Format(time.RFC3339),
		}

		data, err := json.Marshal(relationData)
		if err != nil {
			return err
		}

		return txn.Set([]byte(relationKey), data)
	})
}

// RemoveATeamFromOrganization removes a team from an organization in BadgerDB
func (b *BadgerDriver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		relationKey := fmt.Sprintf("org_team:%s:%s", orgId, teamId)
		return txn.Delete([]byte(relationKey))
	})
}

// AssignProjectToOrganization assigns a project to an organization in BadgerDB
func (b *BadgerDriver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		// Create organization-project relation
		relationKey := fmt.Sprintf("org_project:%s:%s", orgId, projectId)
		relationData := map[string]interface{}{
			"organization_id": orgId,
			"project_id":      projectId,
			"assigned_by":     userId,
			"assigned_at":     time.Now().Format(time.RFC3339),
		}

		data, err := json.Marshal(relationData)
		if err != nil {
			return err
		}

		return txn.Set([]byte(relationKey), data)
	})
}

// RemoveProjectFromOrganization removes a project from an organization in BadgerDB
func (b *BadgerDriver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	return b.Db.Update(func(txn *badger.Txn) error {
		relationKey := fmt.Sprintf("org_project:%s:%s", orgId, projectId)
		return txn.Delete([]byte(relationKey))
	})
}

// GetProjectTeams retrieves team information for a project from BadgerDB
func (b *BadgerDriver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	var team models.Team

	err := b.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("project_main_team:" + projectId))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &team)
		})
		return err
	})

	if err != nil {
		return nil, err
	}

	return &team, nil
}

// FindUserTeams retrieves all teams for a given user from BadgerDB
func (b *BadgerDriver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
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

// CreateTeam creates a new team in BadgerDB
func (b *BadgerDriver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	err := b.Db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(team)
		if err != nil {
			return err
		}

		// Store the team
		if err := txn.Set([]byte("team:"+team.ID), data); err != nil {
			return err
		}

		// Store user-team relations for each user
		for _, user := range team.Users {
			relationKey := fmt.Sprintf("user_team:%s:%s", user.ID, team.ID)
			if err := txn.Set([]byte(relationKey), data); err != nil {
				return err
			}
		}

		return nil
	})

	return team, err
}
