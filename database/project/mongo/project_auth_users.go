package mongo

import (
	"context"

	"github.com/apito-io/engine/database/project/projectauthusers"
	"github.com/apito-io/engine/models"
)

func (m *MongoDriver) EnsureUsersTable(ctx context.Context) error {
	return projectauthusers.EnsureUsersTable(ctx)
}

func (m *MongoDriver) CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	return projectauthusers.CreateProjectAuthUser(ctx, user)
}

func (m *MongoDriver) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	return projectauthusers.GetProjectAuthUser(ctx, userID)
}

func (m *MongoDriver) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	return projectauthusers.GetProjectAuthUserByUsername(ctx, username)
}

func (m *MongoDriver) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByEmail(ctx, tenantID, email)
}

func (m *MongoDriver) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByPhone(ctx, tenantID, phone)
}

func (m *MongoDriver) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByGoogleSub(ctx, tenantID, googleSub)
}

func (m *MongoDriver) SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	return projectauthusers.SearchProjectAuthUsers(ctx, tenantID, limit, offset)
}

func (m *MongoDriver) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	return projectauthusers.CountProjectAuthUsersByRole(ctx, tenantID)
}

func (m *MongoDriver) UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	return projectauthusers.UpdateProjectAuthUser(ctx, user)
}

func (m *MongoDriver) DeleteProjectAuthUser(ctx context.Context, userID string) error {
	return projectauthusers.DeleteProjectAuthUser(ctx, userID)
}
