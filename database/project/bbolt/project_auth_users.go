package bbolt

import (
	"context"

	"github.com/apito-io/engine/database/project/projectauthusers"
	"github.com/apito-io/engine/models"
)

func (b *BBoltDriver) EnsureUsersTable(ctx context.Context) error {
	return projectauthusers.EnsureUsersTable(ctx)
}

func (b *BBoltDriver) CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	return projectauthusers.CreateProjectAuthUser(ctx, user)
}

func (b *BBoltDriver) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	return projectauthusers.GetProjectAuthUser(ctx, userID)
}

func (b *BBoltDriver) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	return projectauthusers.GetProjectAuthUserByUsername(ctx, username)
}

func (b *BBoltDriver) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByEmail(ctx, tenantID, email)
}

func (b *BBoltDriver) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByPhone(ctx, tenantID, phone)
}

func (b *BBoltDriver) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	return projectauthusers.ListProjectAuthUsersByGoogleSub(ctx, tenantID, googleSub)
}

func (b *BBoltDriver) SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	return projectauthusers.SearchProjectAuthUsers(ctx, tenantID, limit, offset)
}

func (b *BBoltDriver) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	return projectauthusers.CountProjectAuthUsersByRole(ctx, tenantID)
}

func (b *BBoltDriver) UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	return projectauthusers.UpdateProjectAuthUser(ctx, user)
}

func (b *BBoltDriver) DeleteProjectAuthUser(ctx context.Context, userID string) error {
	return projectauthusers.DeleteProjectAuthUser(ctx, userID)
}
