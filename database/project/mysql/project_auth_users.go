package mysql

import (
	"context"

	"github.com/apito-io/engine/database/project/projectauthusers"
	"github.com/apito-io/engine/models"
)

func (d *Driver) authUsersStore() *projectauthusers.SQLStore {
	if d == nil {
		return nil
	}
	return &projectauthusers.SQLStore{DB: d.ORM}
}

func (d *Driver) EnsureUsersTable(ctx context.Context) error {
	return d.authUsersStore().EnsureUsersTable(ctx)
}

func (d *Driver) CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	return d.authUsersStore().CreateProjectAuthUser(ctx, user)
}

func (d *Driver) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	return d.authUsersStore().GetProjectAuthUser(ctx, userID)
}

func (d *Driver) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	return d.authUsersStore().GetProjectAuthUserByUsername(ctx, username)
}

func (d *Driver) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	return d.authUsersStore().ListProjectAuthUsersByEmail(ctx, tenantID, email)
}

func (d *Driver) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	return d.authUsersStore().ListProjectAuthUsersByPhone(ctx, tenantID, phone)
}

func (d *Driver) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	return d.authUsersStore().ListProjectAuthUsersByGoogleSub(ctx, tenantID, googleSub)
}

func (d *Driver) SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	return d.authUsersStore().SearchProjectAuthUsers(ctx, tenantID, limit, offset)
}

func (d *Driver) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	return d.authUsersStore().CountProjectAuthUsersByRole(ctx, tenantID)
}

func (d *Driver) UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	return d.authUsersStore().UpdateProjectAuthUser(ctx, user)
}

func (d *Driver) DeleteProjectAuthUser(ctx context.Context, userID string) error {
	return d.authUsersStore().DeleteProjectAuthUser(ctx, userID)
}
