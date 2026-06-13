package projectauthusers

import (
	"context"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
)

func EnsureUsersTable(ctx context.Context) error {
	_ = ctx
	return ae.ErrProjectAuthUsersUnsupported
}

func CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	_ = ctx
	_ = user
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	_ = ctx
	_ = userID
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	_ = ctx
	_ = username
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	_ = ctx
	_ = tenantID
	_ = email
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	_ = ctx
	_ = tenantID
	_ = phone
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	_ = ctx
	_ = tenantID
	_ = googleSub
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	_ = ctx
	_ = tenantID
	_ = limit
	_ = offset
	return nil, 0, ae.ErrProjectAuthUsersUnsupported
}

func CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	_ = ctx
	_ = tenantID
	return nil, ae.ErrProjectAuthUsersUnsupported
}

func UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	_ = ctx
	_ = user
	return ae.ErrProjectAuthUsersUnsupported
}

func DeleteProjectAuthUser(ctx context.Context, userID string) error {
	_ = ctx
	_ = userID
	return ae.ErrProjectAuthUsersUnsupported
}
