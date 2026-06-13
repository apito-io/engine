package bbolt

import (
	"context"

	"github.com/apito-io/engine/database/system/sysusers"
	"github.com/apito-io/engine/models"
)

func (d *ProBBoltSystemDriver) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	return sysusers.CreateUser(ctx, user)
}

func (d *ProBBoltSystemDriver) GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	return sysusers.GetUser(ctx, projectID, userID)
}

func (d *ProBBoltSystemDriver) UpdateUser(ctx context.Context, user *models.User) error {
	return sysusers.UpdateUser(ctx, user)
}

func (d *ProBBoltSystemDriver) DeleteUser(ctx context.Context, projectID, userID string) error {
	return sysusers.DeleteUser(ctx, projectID, userID)
}

func (d *ProBBoltSystemDriver) SearchProjectUsers(ctx context.Context, projectID string, limit, offset int) ([]*models.User, int, error) {
	return sysusers.SearchProjectUsers(ctx, projectID, limit, offset)
}

func (d *ProBBoltSystemDriver) GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	return sysusers.GetUserByUsername(ctx, projectID, username)
}

func (d *ProBBoltSystemDriver) ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	return sysusers.ListUsersByEmail(ctx, projectID, email)
}

func (d *ProBBoltSystemDriver) ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	return sysusers.ListUsersByPhone(ctx, projectID, phone)
}

func (d *ProBBoltSystemDriver) ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	return sysusers.ListUsersByGoogleSub(ctx, projectID, googleSub)
}

func (d *ProBBoltSystemDriver) CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	return sysusers.CountProjectUsersByRole(ctx, projectID)
}
