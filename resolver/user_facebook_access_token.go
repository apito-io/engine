package resolver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
)

// facebookGraphClient is the Graph API client for native Facebook access-token login.
var facebookGraphClient = &http.Client{Timeout: 20 * time.Second}

// FacebookTokenLoginReady reports whether native Facebook access-token login can run.
// The redirect URI is not required; the app secret stays on the server.
func FacebookTokenLoginReady(project *models.Project) error {
	if !models.OAuthAuthEffective(project, models.OAuthProviderFacebook) {
		return errors.New("facebook authentication is disabled or not configured for this project")
	}
	cred := models.OAuthCredentials(project, models.OAuthProviderFacebook)
	if cred.ClientSecret == "" {
		return errors.New("facebook client secret is not configured for this project")
	}
	return nil
}

// ResolveFacebookAccessToken checks a user access token with Graph debug_token and /me.
func ResolveFacebookAccessToken(ctx context.Context, project *models.Project, userToken string) (*OAuthIdentity, error) {
	if err := FacebookTokenLoginReady(project); err != nil {
		return nil, err
	}
	cred := models.OAuthCredentials(project, models.OAuthProviderFacebook)
	return resolveFacebookAccessToken(ctx, facebookGraphClient, "https://graph.facebook.com", cred.ClientID, cred.ClientSecret, userToken, time.Now())
}

func resolveFacebookAccessToken(ctx context.Context, client *http.Client, graphBase, appID, appSecret, userToken string, now time.Time) (*OAuthIdentity, error) {
	if client == nil {
		return nil, errors.New("facebook graph client is required")
	}
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)
	userToken = strings.TrimSpace(userToken)
	if appID == "" || appSecret == "" {
		return nil, errors.New("facebook client id and client secret are required")
	}
	if userToken == "" {
		return nil, errors.New("access_token is required for facebook_access_token login")
	}

	debugURL := strings.TrimRight(graphBase, "/") + "/debug_token?" + url.Values{
		"input_token":  {userToken},
		"access_token": {appID + "|" + appSecret},
	}.Encode()
	debugReq, err := http.NewRequestWithContext(ctx, http.MethodGet, debugURL, nil)
	if err != nil {
		return nil, err
	}
	debugResp, err := client.Do(debugReq)
	if err != nil {
		return nil, fmt.Errorf("facebook debug_token: %w", err)
	}
	var debug struct {
		Data struct {
			AppID     string `json:"app_id"`
			IsValid   bool   `json:"is_valid"`
			ExpiresAt int64  `json:"expires_at"`
			Error     *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"data"`
	}
	if err := readJSONBody(debugResp, &debug); err != nil {
		return nil, fmt.Errorf("facebook debug_token: %w", err)
	}
	if debug.Data.Error != nil && strings.TrimSpace(debug.Data.Error.Message) != "" && !debug.Data.IsValid {
		return nil, fmt.Errorf("facebook access token rejected: %s", strings.TrimSpace(debug.Data.Error.Message))
	}
	if !debug.Data.IsValid {
		return nil, errors.New("facebook access token is not valid")
	}
	if strings.TrimSpace(debug.Data.AppID) != appID {
		return nil, errors.New("facebook access token was issued for a different app")
	}
	if debug.Data.ExpiresAt > 0 && !time.Unix(debug.Data.ExpiresAt, 0).After(now) {
		return nil, errors.New("facebook access token is expired")
	}

	meURL := strings.TrimRight(graphBase, "/") + "/me?" + url.Values{
		"fields":       {"id,name,email,picture"},
		"access_token": {userToken},
	}.Encode()
	meReq, err := http.NewRequestWithContext(ctx, http.MethodGet, meURL, nil)
	if err != nil {
		return nil, err
	}
	meResp, err := client.Do(meReq)
	if err != nil {
		return nil, fmt.Errorf("facebook profile: %w", err)
	}
	var profile struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := readJSONBody(meResp, &profile); err != nil {
		return nil, fmt.Errorf("facebook profile: %w", err)
	}
	subject := strings.TrimSpace(profile.ID)
	if subject == "" {
		return nil, errors.New("facebook profile missing id")
	}
	email := NormalizeUserEmail(profile.Email)
	return &OAuthIdentity{
		Provider:      models.OAuthProviderFacebook,
		Subject:       subject,
		Email:         email,
		EmailVerified: email != "",
		Name:          strings.TrimSpace(profile.Name),
		PhotoURL:      strings.TrimSpace(profile.Picture.Data.URL),
	}, nil
}
