package services

import (
	"github.com/apito-io/engine/models"
	"golang.org/x/net/context"
)

type AuthServiceInterface interface {
	Login(ctx context.Context, req *models.LoginRequest, user *models.SystemUser, projectWithRoles *models.ProjectWithRoles) (*models.JWTTokens, error)

	Signup(ctx context.Context, req *models.RegisterRequest) (*models.SystemUser, error)
	ConfirmSignup(ctx context.Context, req *models.RegisterRequest) error

	ForgetPasswordRequest(ctx context.Context, req *models.RegisterRequest) error
	ConfirmForgetPassword(ctx context.Context, req *models.RegisterRequest) error

	ChangePassword(ctx context.Context, user *models.SystemUser, old, new string) (*models.SystemUser, error)

	Logout(ctx context.Context, token string) error

	VerifyIDToken(ctx context.Context, token string) (*models.TokenClaims, error)
	VerifyAccessToken(ctx context.Context, token string) error

	ExchangeAndRefreshToken(ctx context.Context, projectWithRoles *models.ProjectWithRoles) (*models.JWTTokens, error)
}
