package sysusers

import (
	"context"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
)

func CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	_ = ctx
	_ = user
	return nil, ae.ErrProjectUsersUnsupported
}

func GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	_ = ctx
	_ = projectID
	_ = userID
	return nil, ae.ErrProjectUsersUnsupported
}

func UpdateUser(ctx context.Context, user *models.User) error {
	_ = ctx
	_ = user
	return ae.ErrProjectUsersUnsupported
}

func DeleteUser(ctx context.Context, projectID, userID string) error {
	_ = ctx
	_ = projectID
	_ = userID
	return ae.ErrProjectUsersUnsupported
}

func SearchProjectUsers(ctx context.Context, projectID string, limit, offset int) ([]*models.User, int, error) {
	_ = ctx
	_ = projectID
	_ = limit
	_ = offset
	return nil, 0, ae.ErrProjectUsersUnsupported
}

func CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	_ = ctx
	_ = projectID
	return map[string]int{}, ae.ErrProjectUsersUnsupported
}

func GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	_ = ctx
	_ = projectID
	_ = username
	return nil, ae.ErrProjectUsersUnsupported
}

func ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	_ = ctx
	_ = projectID
	_ = email
	return nil, ae.ErrProjectUsersUnsupported
}

func ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	_ = ctx
	_ = projectID
	_ = phone
	return nil, ae.ErrProjectUsersUnsupported
}

func ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	_ = ctx
	_ = projectID
	_ = googleSub
	return nil, ae.ErrProjectUsersUnsupported
}
