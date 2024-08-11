package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/engine/models"
	"golang.org/x/crypto/bcrypt"
)

type LocalAuthService struct {
	TokenService *JWTService
}

func NewLocalAuthService(cfg *models.Config, tokenService *JWTService) (*LocalAuthService, error) {
	return &LocalAuthService{
		TokenService: tokenService,
	}, nil
}

func (l LocalAuthService) Login(ctx context.Context, req *protobuff.LoginRequest, user *protobuff.SystemUser) (*models.JWTTokens, error) {
	// check the existing and the new password are same
	if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(req.Secret)); err != nil {
		return nil, errors.New("wrong password provided")
	}

	tokens, err := l.TokenService.GenerateLoginToken(ctx, user)
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (l LocalAuthService) Signup(ctx context.Context, registerRequest *protobuff.RegisterRequest) (*protobuff.SystemUser, error) {

	if registerRequest.User == nil {
		return nil, errors.New("user is required")
	}

	user := &protobuff.SystemUser{
		FirstName: registerRequest.User.FirstName,
		Email:     registerRequest.User.Email,
	}

	// check the existing and the new password are same
	/*if err := bcrypt.CompareHashAndPassword([]byte(teacherProfile.Secret), []byte(req.NewPassword)); err == nil {
		return nil, errors.New("new and existing password are same")
	}*/

	if registerRequest.User.Secret != "" {
		// generate new hashed password and update teacher's secret
		hash, err := bcrypt.GenerateFromPassword([]byte(registerRequest.User.Secret), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Secret = string(hash)
	}

	return user, nil
}

func (l LocalAuthService) ConfirmSignup(ctx context.Context, req *protobuff.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (l LocalAuthService) ForgetPasswordRequest(ctx context.Context, req *protobuff.RegisterRequest) error {
	//TODO implement me
	return errors.New("if you forget your password then please contact your admin")
}

func (l LocalAuthService) ConfirmForgetPassword(ctx context.Context, req *protobuff.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (l LocalAuthService) ChangePassword(ctx context.Context, user *protobuff.SystemUser, old, new string) (*protobuff.SystemUser, error) {

	// check old and new password are empty or not
	if old == "" || new == "" {
		return nil, errors.New("old or new password fields are empty")
	}

	// check old password is matching with existing
	if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(old)); err != nil {
		return nil, errors.New("old password is not correct")
	}

	// check new password length(allowed min length: 6)
	if len(new) < models.MinPasswordLength {
		return nil, errors.New(fmt.Sprintf("new password length must be greater than or equal to %v", models.MinPasswordLength))
	}

	// check the existing and the new password are same
	if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(new)); err == nil {
		return nil, errors.New("new and existing password are same")
	}

	// generate new hashed password and update teacher's secret
	hash, err := bcrypt.GenerateFromPassword([]byte(new), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user.Secret = string(hash)

	return user, nil
}

func (l LocalAuthService) Logout(ctx context.Context, token string) error {
	return nil
}

func (l LocalAuthService) VerifyIDToken(ctx context.Context, token string) (*models.TokenClaims, error) {
	return l.TokenService.VerifyIDToken(ctx, token)
}

func (l LocalAuthService) VerifyAccessToken(ctx context.Context, token string) error {
	return l.TokenService.VerifyAccessToken(ctx, token)
}

func (l LocalAuthService) ExchangeAndRefreshToken(ctx context.Context, user *protobuff.SystemUser) (*models.JWTTokens, error) {
	tokens, err := l.TokenService.GenerateLoginToken(ctx, user)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}
