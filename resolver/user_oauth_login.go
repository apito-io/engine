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
)

// ResolveUserForOAuthLogin finds or creates an app user for non-Google OAuth providers.
// Lookup is by provider+oauth_sub, then optional verified-email link.
func (svc *ProjectUserService) ResolveUserForOAuthLogin(
	provider models.OAuthProviderID,
	oauthSub, email string,
	emailVerified bool,
	subLookupTenantID, emailLookupTenantID string,
	createFn func() (*models.User, error),
) (*models.User, error) {
	if svc == nil {
		return nil, errors.New("project user service required")
	}
	prov := models.OAuthProviderID(strings.ToLower(strings.TrimSpace(string(provider))))
	if prov == "" || prov == models.OAuthProviderGoogle {
		return nil, errors.New("oauth provider required")
	}
	sub := strings.TrimSpace(oauthSub)
	if sub == "" {
		return nil, fmt.Errorf("%s token missing subject", prov)
	}

	candidates, err := svc.ListByOAuthSubWithFallback(subLookupTenantID, string(prov), sub)
	if err != nil {
		return nil, err
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("multiple users matched this %s subject", prov)
	}
	if len(candidates) == 1 && candidates[0] != nil {
		return candidates[0], nil
	}

	emailLower := NormalizeUserEmail(email)
	if emailLower == "" {
		if createFn == nil {
			return nil, fmt.Errorf("%s profile missing email", prov)
		}
		if !emailVerified {
			// Allow create without email when provider did not return one (e.g. X).
			return createFn()
		}
		return createFn()
	}

	if !emailVerified {
		return nil, fmt.Errorf("%s email not verified", prov)
	}

	byEmail, err := svc.ListByEmailWithFallback(emailLookupTenantID, emailLower)
	if err != nil {
		return nil, err
	}
	switch len(byEmail) {
	case 0:
		if createFn == nil {
			return nil, fmt.Errorf("%s profile missing email", prov)
		}
		return createFn()
	case 1:
		existing := byEmail[0]
		if existing == nil {
			return nil, errors.New("user not found")
		}
		if existing.Status != models.UserStatusActive {
			return nil, errors.New("user is not active")
		}
		existingSub := strings.TrimSpace(existing.OAuthSub)
		existingProv := strings.ToLower(strings.TrimSpace(existing.Provider))
		if existingSub != "" && existingProv == string(prov) && existingSub != sub {
			return nil, fmt.Errorf("%s account already linked to another user", prov)
		}
		existing.OAuthSub = sub
		if existingProv == "" || existingProv == models.UserProviderLocal || existingProv == string(prov) {
			existing.Provider = UserProviderString(prov)
		}
		linkTenant := svc.linkTenantIDForUser(existing.ID, emailLookupTenantID)
		if err := svc.UpdateUserRecord(existing, linkTenant); err != nil {
			return nil, fmt.Errorf("link %s account: %w", prov, err)
		}
		refreshed, err := svc.GetUserWithFallback(existing.ID, linkTenant)
		if err != nil {
			return nil, err
		}
		if refreshed == nil {
			return existing, nil
		}
		return refreshed, nil
	default:
		return nil, errors.New("multiple users matched this email")
	}
}

func (s *GraphQLServer) completeOAuthProviderLogin(
	ctx context.Context,
	cache *models.ApplicationCache,
	svc *ProjectUserService,
	identity *OAuthIdentity,
) (map[string]interface{}, error) {
	if identity == nil {
		return nil, errors.New("oauth identity required")
	}
	provider := identity.Provider
	sub := strings.TrimSpace(identity.Subject)
	email := NormalizeUserEmail(identity.Email)
	user, err := svc.ResolveUserForOAuthLogin(provider, sub, email, identity.EmailVerified, "", "", func() (*models.User, error) {
		uid := utility.NewID()
		newUser := &models.User{
			ID:        uid,
			ProjectID: cache.Project.ID,
			Username:  internalUserUsername(uid),
			Email:     email,
			Role:      ResolveNewUserRole(cache.Project, ""),
			Provider:  UserProviderString(provider),
			OAuthSub:  sub,
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

func (s *GraphQLServer) loginWithOAuthCode(
	ctx context.Context,
	cache *models.ApplicationCache,
	authProject *models.Project,
	svc *ProjectUserService,
	provider models.OAuthProviderID,
	code, state string,
) (map[string]interface{}, error) {
	if !models.OAuthAuthEffective(authProject, provider) {
		return nil, fmt.Errorf("%s authentication is disabled or not configured for this project", provider)
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return nil, fmt.Errorf("code and state are required for %s login", provider)
	}
	identity, err := ExchangeOAuthAuthorizationCode(ctx, provider, authProject, cache.Project.ID, code, state)
	if err != nil {
		return nil, err
	}
	return s.completeOAuthProviderLogin(ctx, cache, svc, identity)
}

// OAuthStateResolverFn returns HMAC-signed OAuth state for a provider.
// googleOAuthState is a thin alias (provider defaults to google).
func (s *GraphQLServer) OAuthStateResolverFn(p graphql.ResolveParams) (interface{}, error) {
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
	providerRaw := strings.TrimSpace(getArgString(p.Args, "provider"))
	if providerRaw == "" {
		providerRaw = "google"
	}
	provider, ok := models.ParseOAuthProviderID(providerRaw)
	if !ok {
		return nil, errors.New("unsupported oauth provider")
	}
	authProject := cache.Project
	ctx := cache.Ctx
	if ctx == nil {
		ctx = p.Context
	}
	if s.SystemDriver != nil {
		if fresh, ferr := s.SystemDriver.GetProject(ctx, cache.Project.ID); ferr == nil && fresh != nil {
			authProject = fresh
		}
	}
	if !models.OAuthCodeExchangeReady(authProject, provider) {
		return nil, fmt.Errorf("%s oauth code flow is not configured for this project (client id, client secret, and redirect URI required)", provider)
	}
	cred := models.OAuthCredentials(authProject, provider)
	state, err := models.SignOAuthState(cred.ClientSecret, cache.Project.ID, cred.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("sign oauth state: %w", err)
	}
	return map[string]interface{}{"state": state}, nil
}

// GoogleOAuthStateResolverFn keeps the legacy query as an alias for oauthState(provider: google).
func (s *GraphQLServer) GoogleOAuthStateResolverFn(p graphql.ResolveParams) (interface{}, error) {
	if p.Args == nil {
		p.Args = map[string]interface{}{}
	}
	if _, ok := p.Args["provider"]; !ok {
		p.Args["provider"] = "google"
	}
	return s.OAuthStateResolverFn(p)
}
