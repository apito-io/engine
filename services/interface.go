package services

import (
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/models"
	"golang.org/x/net/context"
)

type AuthServiceInterface interface {
	Login(ctx context.Context, req *protobuff.LoginRequest, user *protobuff.SystemUser) (*models.JWTTokens, error)

	Signup(ctx context.Context, req *protobuff.RegisterRequest) (*protobuff.SystemUser, error)
	ConfirmSignup(ctx context.Context, req *protobuff.RegisterRequest) error

	ForgetPasswordRequest(ctx context.Context, req *protobuff.RegisterRequest) error
	ConfirmForgetPassword(ctx context.Context, req *protobuff.RegisterRequest) error

	ChangePassword(ctx context.Context, user *protobuff.SystemUser, old, new string) (*protobuff.SystemUser, error)

	Logout(ctx context.Context, token string) error

	VerifyIDToken(ctx context.Context, token string) (*models.TokenClaims, error)
	VerifyAccessToken(ctx context.Context, token string) error

	ExchangeAndRefreshToken(ctx context.Context, user *protobuff.SystemUser) (*models.JWTTokens, error)
}
