package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// OAuthIdentity is the normalized profile after a provider code exchange.
type OAuthIdentity struct {
	Provider      models.OAuthProviderID
	Subject       string
	Email         string
	EmailVerified bool
}

func oauth2ConfigForProvider(provider models.OAuthProviderID, cred models.OAuthProviderCredentials) (*oauth2.Config, error) {
	switch provider {
	case models.OAuthProviderFacebook:
		return &oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			RedirectURL:  cred.RedirectURI,
			Scopes:       []string{"email", "public_profile"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.facebook.com/v19.0/dialog/oauth",
				TokenURL: "https://graph.facebook.com/v19.0/oauth/access_token",
			},
		}, nil
	case models.OAuthProviderGithub:
		return &oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			RedirectURL:  cred.RedirectURI,
			Scopes:       []string{"read:user", "user:email"},
			Endpoint:     github.Endpoint,
		}, nil
	case models.OAuthProviderX:
		return &oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			RedirectURL:  cred.RedirectURI,
			Scopes:       []string{"users.read", "tweet.read", "offline.access"},
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://twitter.com/i/oauth2/authorize",
				TokenURL:  "https://api.twitter.com/2/oauth2/token",
				AuthStyle: oauth2.AuthStyleInHeader,
			},
		}, nil
	case models.OAuthProviderLinkedin:
		return &oauth2.Config{
			ClientID:     cred.ClientID,
			ClientSecret: cred.ClientSecret,
			RedirectURL:  cred.RedirectURI,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.linkedin.com/oauth/v2/authorization",
				TokenURL: "https://www.linkedin.com/oauth/v2/accessToken",
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported oauth provider %q", provider)
	}
}

// ExchangeOAuthAuthorizationCode exchanges code+state for a provider identity.
func ExchangeOAuthAuthorizationCode(
	ctx context.Context,
	provider models.OAuthProviderID,
	project *models.Project,
	projectID, code, state string,
) (*OAuthIdentity, error) {
	if !models.OAuthCodeExchangeReady(project, provider) {
		return nil, fmt.Errorf("%s oauth code flow is not configured for this project (client id, client secret, and redirect URI all are required)", provider)
	}
	cred := models.OAuthCredentials(project, provider)
	if err := models.VerifyOAuthState(cred.ClientSecret, projectID, cred.RedirectURI, state); err != nil {
		return nil, fmt.Errorf("invalid oauth state: %w", err)
	}
	conf, err := oauth2ConfigForProvider(provider, cred)
	if err != nil {
		return nil, err
	}
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%s token exchange failed: %w", provider, err)
	}
	if !tok.Valid() {
		return nil, fmt.Errorf("%s token exchange returned invalid token", provider)
	}
	client := conf.Client(ctx, tok)
	client.Timeout = 20 * time.Second
	switch provider {
	case models.OAuthProviderFacebook:
		return fetchFacebookIdentity(ctx, client)
	case models.OAuthProviderGithub:
		return fetchGithubIdentity(ctx, client)
	case models.OAuthProviderX:
		return fetchXIdentity(ctx, client)
	case models.OAuthProviderLinkedin:
		return fetchLinkedinIdentity(ctx, client)
	default:
		return nil, fmt.Errorf("unsupported oauth provider %q", provider)
	}
}

func readJSONBody(resp *http.Response, dest interface{}) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}
	return nil
}

func fetchFacebookIdentity(ctx context.Context, client *http.Client) (*OAuthIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.facebook.com/me?fields=id,email,name", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	if err := readJSONBody(resp, &payload); err != nil {
		return nil, fmt.Errorf("facebook profile: %w", err)
	}
	sub := strings.TrimSpace(payload.ID)
	if sub == "" {
		return nil, errors.New("facebook profile missing id")
	}
	email := NormalizeUserEmail(payload.Email)
	return &OAuthIdentity{
		Provider:      models.OAuthProviderFacebook,
		Subject:       sub,
		Email:         email,
		EmailVerified: email != "",
	}, nil
}

func fetchGithubIdentity(ctx context.Context, client *http.Client) (*OAuthIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var user struct {
		ID    json.Number `json:"id"`
		Email string      `json:"email"`
		Login string      `json:"login"`
	}
	if err := readJSONBody(resp, &user); err != nil {
		return nil, fmt.Errorf("github profile: %w", err)
	}
	sub := strings.TrimSpace(user.ID.String())
	if sub == "" {
		return nil, errors.New("github profile missing id")
	}
	email := NormalizeUserEmail(user.Email)
	verified := false
	if email == "" {
		email, verified = fetchGithubPrimaryEmail(ctx, client)
	} else {
		verified = true
	}
	return &OAuthIdentity{
		Provider:      models.OAuthProviderGithub,
		Subject:       sub,
		Email:         email,
		EmailVerified: verified && email != "",
	}, nil
}

func fetchGithubPrimaryEmail(ctx context.Context, client *http.Client) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	var rows []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := readJSONBody(resp, &rows); err != nil {
		return "", false
	}
	var fallback string
	for _, row := range rows {
		em := NormalizeUserEmail(row.Email)
		if em == "" || !row.Verified {
			continue
		}
		if row.Primary {
			return em, true
		}
		if fallback == "" {
			fallback = em
		}
	}
	return fallback, fallback != ""
}

func fetchXIdentity(ctx context.Context, client *http.Client) (*OAuthIdentity, error) {
	u, _ := url.Parse("https://api.twitter.com/2/users/me")
	q := u.Query()
	q.Set("user.fields", "id,username,name,confirmed_email")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			ID             string `json:"id"`
			ConfirmedEmail string `json:"confirmed_email"`
		} `json:"data"`
	}
	if err := readJSONBody(resp, &payload); err != nil {
		return nil, fmt.Errorf("x profile: %w", err)
	}
	sub := strings.TrimSpace(payload.Data.ID)
	if sub == "" {
		return nil, errors.New("x profile missing id")
	}
	email := NormalizeUserEmail(payload.Data.ConfirmedEmail)
	return &OAuthIdentity{
		Provider:      models.OAuthProviderX,
		Subject:       sub,
		Email:         email,
		EmailVerified: email != "",
	}, nil
}

func fetchLinkedinIdentity(ctx context.Context, client *http.Client) (*OAuthIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.linkedin.com/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := readJSONBody(resp, &payload); err != nil {
		return nil, fmt.Errorf("linkedin profile: %w", err)
	}
	sub := strings.TrimSpace(payload.Sub)
	if sub == "" {
		return nil, errors.New("linkedin profile missing sub")
	}
	email := NormalizeUserEmail(payload.Email)
	return &OAuthIdentity{
		Provider:      models.OAuthProviderLinkedin,
		Subject:       sub,
		Email:         email,
		EmailVerified: payload.EmailVerified && email != "",
	}, nil
}

// UserProviderString maps OAuth provider id to User.Provider constant.
func UserProviderString(provider models.OAuthProviderID) string {
	switch provider {
	case models.OAuthProviderFacebook:
		return models.UserProviderFacebook
	case models.OAuthProviderGithub:
		return models.UserProviderGithub
	case models.OAuthProviderX:
		return models.UserProviderX
	case models.OAuthProviderLinkedin:
		return models.UserProviderLinkedin
	case models.OAuthProviderGoogle:
		return models.UserProviderGoogle
	default:
		return string(provider)
	}
}
