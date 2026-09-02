package services

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"golang.org/x/crypto/bcrypt"
)

const passwordResetTTL = 15 * time.Minute
const passwordResetKVPrefix = "pwreset:"

type systemUserEmailStore interface {
	GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error)
	UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error
}

type LocalAuthService struct {
	Cfg          *models.Config
	TokenService *JWTService
	SystemDriver systemUserEmailStore
	KV           interfaces.KeyValueServiceInterface
	Mailer       EmailSender
}

func NewLocalAuthService(cfg *models.Config, tokenService *JWTService, kv interfaces.KeyValueServiceInterface, mailer EmailSender) (*LocalAuthService, error) {
	return &LocalAuthService{
		Cfg:          cfg,
		TokenService: tokenService,
		KV:           kv,
		Mailer:       mailer,
	}, nil
}

func (l *LocalAuthService) SetSystemDriver(db systemUserEmailStore) {
	if l != nil {
		l.SystemDriver = db
	}
}

func passwordResetKVKey(email string) string {
	return passwordResetKVPrefix + strings.ToLower(strings.TrimSpace(email))
}

func sixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (l *LocalAuthService) Login(ctx context.Context, req *models.LoginRequest, user *models.SystemUser, projectWithRoles *models.ProjectWithRoles) (*models.JWTTokens, error) {

	// check the existing and the new password are same
	if err := bcrypt.CompareHashAndPassword([]byte(user.Secret), []byte(req.Secret)); err != nil {
		return nil, errors.New("wrong password provided")
	}

	tokens, err := l.TokenService.GenerateLoginToken(ctx, &models.ProjectWithRoles{
		User: user,
	})
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (l *LocalAuthService) Signup(ctx context.Context, registerRequest *models.RegisterRequest) (*models.SystemUser, error) {

	if registerRequest.User == nil {
		return nil, errors.New("user is required")
	}

	user := registerRequest.User
	user.IsActive = true
	user.CreatedAt = utility.GetCurrentTime()
	user.UpdatedAt = utility.GetCurrentTime()
	user.ReadOnlyProject = false

	if user.TempPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(user.TempPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Secret = string(hash)
	}

	return user, nil
}

func (l *LocalAuthService) ConfirmSignup(ctx context.Context, req *models.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (l *LocalAuthService) ForgetPasswordRequest(ctx context.Context, req *models.RegisterRequest) error {
	if req == nil || req.User == nil {
		return nil
	}
	email := strings.TrimSpace(req.User.Email)
	if email == "" {
		return nil
	}
	if l == nil || l.SystemDriver == nil || l.KV == nil || l.Mailer == nil {
		log.Println("email: password reset skipped, auth deps not ready")
		return nil
	}
	user, err := l.SystemDriver.GetSystemUserByEmail(ctx, email)
	if err != nil || user == nil || strings.TrimSpace(user.ID) == "" {
		return nil
	}
	code, err := sixDigitCode()
	if err != nil {
		log.Printf("email: reset code: %v", err)
		return nil
	}
	if err := l.KV.SetValue(ctx, passwordResetKVKey(email), code, passwordResetTTL); err != nil {
		log.Printf("email: store reset code: %v", err)
		return nil
	}
	appURL := ""
	if l.Cfg != nil {
		appURL = l.Cfg.CORSOrigin
	}
	mail := &models.EmailSendRequest{
		AppURL:           appURL,
		Recipients:       []string{user.Email},
		VerificationCode: code,
	}
	ComposePasswordReset(mail)
	if err := l.Mailer.Send(ctx, mail); err != nil {
		log.Printf("email: send reset: %v", err)
	}
	return nil
}

func (l *LocalAuthService) ConfirmForgetPassword(ctx context.Context, req *models.RegisterRequest) error {
	if ctx == nil {
		ctx = context.Background()
	}
	const invalid = "invalid verification code"
	if l == nil || l.SystemDriver == nil || l.KV == nil {
		return errors.New(invalid)
	}
	if req == nil || req.User == nil {
		return errors.New(invalid)
	}
	email := strings.TrimSpace(req.User.Email)
	code := strings.TrimSpace(req.VerificationCode)
	secret := req.User.Secret
	if email == "" || code == "" || secret == "" {
		return errors.New(invalid)
	}
	if len(secret) < models.MinPasswordLength {
		return fmt.Errorf("new password length must be greater than or equal to %v", models.MinPasswordLength)
	}
	stored, err := l.KV.GetValue(ctx, passwordResetKVKey(email))
	if err != nil || stored == "" {
		return errors.New(invalid)
	}
	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		return errors.New(invalid)
	}
	user, err := l.SystemDriver.GetSystemUserByEmail(ctx, email)
	if err != nil || user == nil {
		return errors.New(invalid)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Secret = string(hash)
	user.TempPassword = ""
	if err := l.SystemDriver.UpdateSystemUser(ctx, user, true); err != nil {
		return err
	}
	_ = l.KV.DelValue(ctx, passwordResetKVKey(email))
	return nil
}

func (l *LocalAuthService) ChangePassword(ctx context.Context, user *models.SystemUser, old, new string) (*models.SystemUser, error) {

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

func (l *LocalAuthService) Logout(ctx context.Context, token string) error {
	return nil
}

func (l *LocalAuthService) VerifyIDToken(ctx context.Context, token string) (*models.TokenClaims, error) {
	return l.TokenService.VerifyIDToken(ctx, token)
}

func (l *LocalAuthService) VerifyAccessToken(ctx context.Context, token string) error {
	return l.TokenService.VerifyAccessToken(ctx, token)
}

func (l *LocalAuthService) ExchangeAndRefreshToken(ctx context.Context, projectWithRoles *models.ProjectWithRoles) (*models.JWTTokens, error) {
	tokens, err := l.TokenService.GenerateLoginToken(ctx, projectWithRoles)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}
