package utility

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/apito-io/engine/models"
	oidc "github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// googleOIDCIssuer is Google's OpenID Connect issuer (discovery URL base).
// oidc.NewProvider expects this URL, not the OAuth client_id.
// See https://accounts.google.com/.well-known/openid-configuration
const googleOIDCIssuer = "https://accounts.google.com"

type Authenticator struct {
	Provider *oidc.Provider
	Config   oauth2.Config
	Ctx      context.Context
}

func NewAuthenticator(cfg *models.Config) (*Authenticator, error) {
	ctx := context.Background()

	provider, err := oidc.NewProvider(ctx, googleOIDCIssuer)
	if err != nil {
		log.Printf("failed to get provider: %v", err)
		return nil, err
	}

	conf := oauth2.Config{
		ClientID:     cfg.GoogleOauthClientID,
		ClientSecret: cfg.GoogleOauthClientSecret,
		RedirectURL:  cfg.GoogleOauthRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &Authenticator{
		Provider: provider,
		Config:   conf,
		Ctx:      ctx,
	}, nil
}

func NewRefreshTokenAuthenticator(cfg *models.Config, token string) (*models.JWTTokens, error) {

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, googleOIDCIssuer)
	if err != nil {
		return nil, err
	}

	payload := strings.NewReader(fmt.Sprintf("grant_type=refresh_token&client_id=%s&client_secret=%s&refresh_token=%s", cfg.GoogleOauthClientID, cfg.GoogleOauthClientSecret, token))

	req, err := http.NewRequest("POST", provider.Endpoint().TokenURL, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	if val, ok := resp["error"].(string); ok {
		return nil, errors.New(val)
	}

	return &models.JWTTokens{
		AccessToken: resp["access_token"].(string),
		IDToken:     resp["id_token"].(string),
	}, nil
}
