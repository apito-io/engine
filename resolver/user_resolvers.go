package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

// RequireProjectArg ensures project_id arg matches the authenticated project.
func RequireProjectArg(args map[string]interface{}, expectedProjectID string) error {
	return requireProjectArg(args, expectedProjectID)
}

func requireProjectArg(args map[string]interface{}, expectedProjectID string) error {
	argProjectID := strings.TrimSpace(getArgString(args, "project_id"))
	if argProjectID == "" {
		return errors.New("project_id is required")
	}
	if argProjectID != strings.TrimSpace(expectedProjectID) {
		return errors.New("project_id does not match authenticated project")
	}
	return nil
}

func (s *GraphQLServer) mintProjectUserToken(cache *models.ApplicationCache, role, userID string) (string, error) {
	if s.ProjectKeyManager == nil {
		return "", errors.New("project key manager not initialized")
	}
	exp := time.Now().UTC().AddDate(1, 0, 0)
	exp = time.Date(exp.Year(), exp.Month(), exp.Day(), 23, 59, 59, 0, time.UTC)
	tokenType := "user"
	scopes := []string{"project:" + cache.Project.ID}
	if s.Cfg != nil && s.Cfg.ProjectUserAPITokenHook != nil {
		tokenType, scopes = s.Cfg.ProjectUserAPITokenHook(cache, userID, role)
	}
	return s.ProjectKeyManager.GenerateKey(&models.TokenClaims{
		Role:          role,
		UserID:        userID,
		ProjectID:     cache.Project.ID,
		TokenType:     tokenType,
		IsProjectUser: true,
		ExpireAt:      exp.Unix(),
		Scopes:        scopes,
	})
}

func (s *GraphQLServer) completeGeneralGoogleLogin(
	ctx context.Context,
	cache *models.ApplicationCache,
	svc *ProjectUserService,
	payload *idtoken.Payload,
) (map[string]interface{}, error) {
	sub := strings.TrimSpace(payload.Subject)
	email := GoogleEmailFromPayload(payload)
	user, err := svc.ResolveUserForGoogleLogin(sub, email, GoogleEmailVerified(payload), "", "", func() (*models.User, error) {
		uid := utility.NewID()
		newUser := &models.User{
			ID:        uid,
			ProjectID: cache.Project.ID,
			Username:  internalUserUsername(uid),
			Email:     email,
			Role:      ResolveNewUserRole(cache.Project, ""),
			Provider:  models.UserProviderGoogle,
			GoogleSub: sub,
			Status:    models.UserStatusActive,
		}
		return svc.CreateUserRecord(newUser, "")
	})
	if err != nil {
		return nil, err
	}
	if user.Status != models.UserStatusActive {
		return nil, errors.New("user is not active")
	}
	token, err := s.mintProjectUserToken(cache, user.Role, user.ID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"token": token,
		"user":  appUserToMap(user),
	}, nil
}

// SearchUsersResolverFn lists project end-users for the authenticated project.
func (s *GraphQLServer) SearchUsersResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.SearchUsers, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	if err := requireProjectArg(p.Args, cache.Project.ID); err != nil {
		return nil, err
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	limit := getArgInt(p.Args, "limit", 50)
	if limit <= 0 {
		limit = 50
	}
	offset := getArgInt(p.Args, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	searchQ := strings.TrimSpace(getArgString(p.Args, "q"))
	rows, count, err := svc.SearchWithFallback("", searchQ, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		if r != nil {
			out = append(out, appUserToMap(r))
		}
	}
	return map[string]interface{}{"users": out, "count": count}, nil
}

// LoginUserResolverFn handles general (password) or Google OAuth code flow and returns a project-scoped API token.
func (s *GraphQLServer) LoginUserResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.LoginUser, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	if err := requireProjectArg(p.Args, cache.Project.ID); err != nil {
		return nil, err
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	authProject := cache.Project
	// Prefer DB-backed project for auth settings — ProjectCache can lag after
	// updateProjectAuthenticationSettings until cache is rewritten.
	if s.SystemDriver != nil {
		if fresh, ferr := s.SystemDriver.GetProject(ctx, cache.Project.ID); ferr == nil && fresh != nil {
			authProject = fresh
		}
	}
	authMethod := strings.ToLower(strings.TrimSpace(getArgString(p.Args, "auth_method")))
	if authMethod == "" {
		authMethod = "general"
	}

	switch authMethod {
	case "google":
		if !models.GoogleAuthEffective(authProject) {
			return nil, errors.New("google authentication is disabled or not configured for this project")
		}
		if !models.GoogleOAuthCodeExchangeReady(authProject) {
			return nil, errors.New("google oauth code flow is not configured for this project (client id, client secret, and redirect URI all are required)")
		}
		code := strings.TrimSpace(getArgString(p.Args, "code"))
		oauthState := strings.TrimSpace(getArgString(p.Args, "state"))
		if code == "" || oauthState == "" {
			return nil, errors.New("code and state are required for google login")
		}
		secret := models.GoogleOAuthClientSecret(authProject)
		redirectURI := models.GoogleOAuthRedirectURI(authProject)
		googleClientID := strings.TrimSpace(models.GoogleOAuthClientID(authProject))
		if err := models.VerifyGoogleOAuthState(secret, cache.Project.ID, redirectURI, oauthState); err != nil {
			return nil, fmt.Errorf("invalid oauth state: %w", err)
		}
		oauthConf := &oauth2.Config{
			ClientID:     googleClientID,
			ClientSecret: secret,
			RedirectURL:  redirectURI,
			Scopes: []string{
				"openid",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}
		tok, err := oauthConf.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("google token exchange failed: %w", err)
		}
		idTokRaw, _ := tok.Extra("id_token").(string)
		if strings.TrimSpace(idTokRaw) == "" {
			return nil, errors.New("google token response missing id_token; ensure authorize request used scope openid")
		}
		payload, err := idtoken.Validate(ctx, idTokRaw, googleClientID)
		if err != nil {
			return nil, fmt.Errorf("invalid google id_token: %w", err)
		}
		return s.completeGeneralGoogleLogin(ctx, cache, svc, payload)

	case "google_id_token":
		if !models.GoogleAuthEffective(authProject) {
			return nil, errors.New("google authentication is disabled or not configured for this project")
		}
		googleClientID := strings.TrimSpace(models.GoogleOAuthClientID(authProject))
		if googleClientID == "" {
			return nil, errors.New("google client id is not configured for this project")
		}
		idTok := strings.TrimSpace(getArgString(p.Args, "id_token"))
		if idTok == "" {
			return nil, errors.New("id_token is required for google_id_token login")
		}
		payload, err := idtoken.Validate(ctx, idTok, googleClientID)
		if err != nil {
			return nil, fmt.Errorf("invalid google id_token: %w", err)
		}
		return s.completeGeneralGoogleLogin(ctx, cache, svc, payload)

	case "facebook", "github", "x", "linkedin":
		provider, ok := models.ParseOAuthProviderID(authMethod)
		if !ok {
			return nil, errors.New("unsupported auth_method")
		}
		return s.loginWithOAuthCode(ctx, cache, authProject, svc, provider,
			getArgString(p.Args, "code"), getArgString(p.Args, "state"))

	case "general":
		if !models.GeneralAuthEffective(authProject) {
			return nil, errors.New("general authentication is disabled for this project")
		}
		pw := getArgString(p.Args, "password")
		if strings.TrimSpace(pw) == "" {
			return nil, errors.New("password is required")
		}
		var appCandidates []*models.User
		switch models.GeneralIdentifierMethod(authProject) {
		case "phone":
			phone := strings.TrimSpace(getArgString(p.Args, "phone"))
			if phone == "" {
				return nil, errors.New("phone is required")
			}
			appCandidates, err = svc.ListByPhoneWithFallback("", phone)
		default:
			email := strings.TrimSpace(strings.ToLower(getArgString(p.Args, "email")))
			if email == "" {
				return nil, errors.New("email is required")
			}
			appCandidates, err = svc.ListByEmailWithFallback("", email)
		}
		if err != nil {
			return nil, err
		}
		if len(appCandidates) == 0 {
			return nil, errors.New("invalid credentials")
		}
		var user *models.User
		for _, u := range appCandidates {
			if u == nil || u.Status != models.UserStatusActive || strings.TrimSpace(u.Secret) == "" {
				continue
			}
			if bcrypt.CompareHashAndPassword([]byte(u.Secret), []byte(pw)) == nil {
				if user != nil {
					return nil, errors.New("multiple users matched this login")
				}
				user = u
			}
		}
		if user == nil {
			return nil, errors.New("invalid credentials")
		}
		token, err := s.mintProjectUserToken(cache, user.Role, user.ID)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"token": token, "user": appUserToMap(user)}, nil

	default:
		return nil, errors.New("unsupported auth_method")
	}
}

// CreateUserResolverFn creates a project end-user with a bcrypt password.
func (s *GraphQLServer) CreateUserResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.CreateUser, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	if err := requireProjectArg(p.Args, cache.Project.ID); err != nil {
		return nil, err
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	pw := getArgString(p.Args, "password")
	if pw == "" {
		return nil, errors.New("password is required")
	}
	emailLower := NormalizeUserEmail(getArgString(p.Args, "email"))
	phoneNorm := models.NormalizeUserPhoneKey(strings.TrimSpace(getArgString(p.Args, "phone")))
	if phoneNorm == "" {
		return nil, errors.New("phone is required")
	}
	uid := utility.NewID()
	username, err := svc.ResolveCreateUsername(uid, getArgString(p.Args, "username"), "")
	if err != nil {
		return nil, err
	}
	if err := svc.assertUserPhoneUnique("", phoneNorm, ""); err != nil {
		return nil, err
	}
	if err := svc.assertUserEmailUnique("", emailLower, ""); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	role := ResolveNewUserRole(cache.Project, getArgString(p.Args, "role"))
	user := &models.User{
		ID:        uid,
		ProjectID: cache.Project.ID,
		Username:  username,
		Email:     emailLower,
		Phone:     phoneNorm,
		Secret:    string(hash),
		Role:      role,
		Provider:  models.UserProviderLocal,
		Status:    models.UserStatusActive,
	}
	created, err := svc.CreateUserRecord(user, "")
	if err != nil {
		return nil, err
	}
	return appUserToMap(created), nil
}

// UpdateUserResolverFn updates an existing project end-user.
func (s *GraphQLServer) UpdateUserResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.UpdateUser, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	uid := strings.TrimSpace(getArgString(p.Args, "user_id"))
	if uid == "" {
		return nil, errors.New("user_id is required")
	}
	appUser, err := svc.GetUserWithFallback(uid, "")
	if err != nil {
		return nil, err
	}
	if appUser == nil {
		return nil, errors.New("user not found")
	}
	if _, has := p.Args["phone"]; has {
		appUser.Phone = models.NormalizeUserPhoneKey(strings.TrimSpace(getArgString(p.Args, "phone")))
		if err := svc.assertUserPhoneUnique("", appUser.Phone, appUser.ID); err != nil {
			return nil, err
		}
	}
	if _, has := p.Args["email"]; has {
		appUser.Email = NormalizeUserEmail(getArgString(p.Args, "email"))
		if err := svc.assertUserEmailUnique("", appUser.Email, appUser.ID); err != nil {
			return nil, err
		}
	}
	if _, has := p.Args["username"]; has {
		newName := NormalizeUserUsernameArg(getArgString(p.Args, "username"))
		if newName != "" && newName != appUser.Username {
			ex, err := svc.GetUserByUsernameWithFallback(newName, "")
			if err != nil {
				return nil, err
			}
			if ex != nil && ex.ID != appUser.ID {
				return nil, errors.New("username already exists")
			}
			appUser.Username = newName
		}
	}
	if v := strings.TrimSpace(getArgString(p.Args, "role")); v != "" {
		appUser.Role = v
	}
	if err := svc.UpdateUserRecord(appUser, ""); err != nil {
		return nil, err
	}
	refreshed, err := svc.GetUserWithFallback(uid, "")
	if err != nil {
		return nil, err
	}
	return appUserToMap(refreshed), nil
}

// DeleteUserResolverFn removes a project end-user.
func (s *GraphQLServer) DeleteUserResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.DeleteUser, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	uid := strings.TrimSpace(getArgString(p.Args, "user_id"))
	if uid == "" {
		return nil, errors.New("user_id is required")
	}
	if err := svc.DeleteUserRecord(uid); err != nil {
		return nil, err
	}
	return true, nil
}

// ResetUserPasswordResolverFn hard-resets a project end-user password (admin only).
func (s *GraphQLServer) ResetUserPasswordResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.ResetUserPassword, p); stop {
			return res, err
		}
	}
	router, ok := p.Context.Value("router").(echo.Context)
	if !ok {
		return nil, errors.New("router context missing")
	}
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if cache.Project == nil {
		return nil, errors.New("no project loaded in cache")
	}
	if err := requireProjectAdmin(cache); err != nil {
		return nil, err
	}
	pw := getArgString(p.Args, "password")
	if strings.TrimSpace(pw) == "" {
		return nil, errors.New("password is required")
	}
	uid := strings.TrimSpace(getArgString(p.Args, "user_id"))
	if uid == "" {
		return nil, errors.New("user_id is required")
	}
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	svc, err := s.ProjectUserService(cache, ctx)
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user, err := svc.GetUserWithFallback(uid, "")
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	user.Secret = string(hash)
	if err := svc.UpdateUserRecord(user, ""); err != nil {
		return nil, err
	}
	return true, nil
}
