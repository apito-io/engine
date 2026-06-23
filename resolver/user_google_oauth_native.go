//go:build !cloudflare

package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
)

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

// GoogleOAuthStateResolverFn returns an HMAC-signed OAuth state bound to the project and configured redirect URI.
func (s *GraphQLServer) GoogleOAuthStateResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if hooks := s.projectUserHooks(); hooks != nil {
		if res, stop, err := s.runProjectUserHook(hooks.GoogleOAuthState, p); stop {
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
	if err := requireProjectArg(p.Args, cache.Project.ID); err != nil {
		return nil, err
	}
	authProject := cache.Project
	if !models.GoogleOAuthCodeExchangeReady(authProject) {
		return nil, errors.New("google oauth code flow is not configured for this project (client id, client secret, and redirect URI required)")
	}
	secret := models.GoogleOAuthClientSecret(authProject)
	redirectURI := models.GoogleOAuthRedirectURI(authProject)
	state, err := models.SignGoogleOAuthState(secret, cache.Project.ID, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("sign oauth state: %w", err)
	}
	return map[string]interface{}{
		"state": state,
	}, nil
}

func (s *GraphQLServer) loginUserGoogleOAuth(
	ctx context.Context,
	cache *models.ApplicationCache,
	svc *ProjectUserService,
	authProject *models.Project,
	args map[string]interface{},
) (interface{}, error) {
	if !models.GoogleAuthEffective(authProject) {
		return nil, errors.New("google authentication is disabled or not configured for this project")
	}
	if !models.GoogleOAuthCodeExchangeReady(authProject) {
		return nil, errors.New("google oauth code flow is not configured for this project (client id, client secret, and redirect URI all are required)")
	}
	code := strings.TrimSpace(getArgString(args, "code"))
	oauthState := strings.TrimSpace(getArgString(args, "state"))
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
}

func (s *GraphQLServer) loginUserGoogleIDToken(
	ctx context.Context,
	cache *models.ApplicationCache,
	svc *ProjectUserService,
	authProject *models.Project,
	args map[string]interface{},
) (interface{}, error) {
	if !models.GoogleAuthEffective(authProject) {
		return nil, errors.New("google authentication is disabled or not configured for this project")
	}
	googleClientID := strings.TrimSpace(models.GoogleOAuthClientID(authProject))
	if googleClientID == "" {
		return nil, errors.New("google client id is not configured for this project")
	}
	idTok := strings.TrimSpace(getArgString(args, "id_token"))
	if idTok == "" {
		return nil, errors.New("id_token is required for google_id_token login")
	}
	payload, err := idtoken.Validate(ctx, idTok, googleClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid google id_token: %w", err)
	}
	return s.completeGeneralGoogleLogin(ctx, cache, svc, payload)
}
