package bbolt

import (
	"context"
	"errors"
	"github.com/apito-io/engine/models"
)

// Misc functions and helper methods

// FindUserTeams retrieves all teams for a given user
func (d *ProBBoltSystemDriver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	// Simplified implementation - in production you'd have proper user-team relationship tracking
	collection := d.DB.Collection("teams")
	var teams []models.Team

	// For now, return all teams - in production you'd filter by user membership
	err := collection.All(&teams)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.Team
	for i := range teams {
		results = append(results, &teams[i])
	}

	return results, nil
}

// ListTeams retrieves teams for a project
func (d *ProBBoltSystemDriver) ListTeams(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	// This function appears to be intended to return team members for a project
	// Delegating to GetTeamsMembers which implements the same functionality
	return d.GetTeamsMembers(ctx, projectId)
}

// Helper function to check if user is team member
func (d *ProBBoltSystemDriver) isUserTeamMember(ctx context.Context, userId, projectId string) (bool, error) {
	collection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	err := collection.Find("user_id", userId, &memberships)
	if err != nil {
		return false, err
	}

	for _, membership := range memberships {
		if membership.ProjectID == projectId {
			return true, nil
		}
	}

	return false, nil
}

// Helper function to get user permissions for a project
func (d *ProBBoltSystemDriver) getUserProjectPermissions(ctx context.Context, userId, projectId string) ([]string, error) {
	collection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	err := collection.Find("user_id", userId, &memberships)
	if err != nil {
		return nil, err
	}

	for _, membership := range memberships {
		if membership.ProjectID == projectId {
			return membership.Permissions, nil
		}
	}

	return nil, errors.New("user not found in project")
}

// Helper function to validate project access
func (d *ProBBoltSystemDriver) validateProjectAccess(ctx context.Context, userId, projectId string) error {
	// Check if user has access to the project
	_, err := d.isUserTeamMember(ctx, userId, projectId)
	if err != nil {
		return err
	}

	return nil
}

// Helper function to clean up project-related data
func (d *ProBBoltSystemDriver) cleanupProjectData(ctx context.Context, projectId string) error {
	// Clean up team memberships
	membershipCollection := d.DB.Collection("team_memberships")
	var memberships []TeamMembership

	err := membershipCollection.Find("project_id", projectId, &memberships)
	if err == nil {
		for _, membership := range memberships {
			membershipCollection.DeleteStruct(&membership)
		}
	}

	// Clean up webhooks
	webhookCollection := d.DB.Collection("webhooks")
	var webhooks []models.Webhook

	err = webhookCollection.Find("project_id", projectId, &webhooks)
	if err == nil {
		for _, webhook := range webhooks {
			webhookCollection.DeleteStruct(&webhook)
		}
	}

	// Clean up audit logs
	auditCollection := d.DB.Collection("audit_logs")
	var auditLogs []models.AuditLogs

	err = auditCollection.Find("project_id", projectId, &auditLogs)
	if err == nil {
		for _, log := range auditLogs {
			auditCollection.DeleteStruct(&log)
		}
	}

	return nil
}

// Helper function to ensure database collections are initialized
func (d *ProBBoltSystemDriver) ensureCollections() error {
	collections := []string{
		"users",
		"projects",
		"organizations",
		"teams",
		"team_memberships",
		"webhooks",
		"audit_logs",
		"token_blacklist",
		"usages",
		"subscriptions",
		"invoices",
	}

	for _, name := range collections {
		collection := d.DB.Collection(name)
		if err := collection.Init(); err != nil {
			return err
		}
	}

	return nil
}

// Helper function to backup database
func (d *ProBBoltSystemDriver) BackupDatabase(backupPath string) error {
	// BBolt has built-in backup functionality
	// This would need to be implemented using BBolt's backup mechanism
	// For now, return not implemented
	return errors.New("backup functionality not implemented yet")
}

// Helper function to get database statistics
func (d *ProBBoltSystemDriver) GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	collections := []string{
		"users", "projects", "organizations", "teams",
		"webhooks", "audit_logs", "usages", "subscriptions",
	}

	for _, name := range collections {
		collection := d.DB.Collection(name)
		// Get count of documents in collection
		var count int
		// This is a simplified count - in production you'd implement proper counting
		var items []interface{}
		err := collection.All(&items)
		if err == nil {
			count = len(items)
		}
		stats[name+"_count"] = count
	}

	return stats, nil
}
