package models

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const googleOAuthStateTTL = 15 * time.Minute

type googleOAuthPayload struct {
	ProjectID string `json:"p"`
	Exp       int64  `json:"e"`
	RH        string `json:"rh"`
	N         string `json:"n"`
}

// HashGoogleOAuthRedirectURI returns lowercase hex SHA-256 for binding state to redirect.
func HashGoogleOAuthRedirectURI(raw string) string {
	u := strings.TrimSpace(raw)
	sum := sha256.Sum256([]byte(u))
	return hex.EncodeToString(sum[:])
}

// SignGoogleOAuthState builds a signed OAuth state bound to project and redirect (HMAC keyed by Google client secret).
func SignGoogleOAuthState(clientSecret, projectID, redirectURI string) (string, error) {
	if strings.TrimSpace(clientSecret) == "" {
		return "", errors.New("client secret required for oauth state")
	}
	if strings.TrimSpace(projectID) == "" {
		return "", errors.New("project id required for oauth state")
	}
	rd := strings.TrimSpace(redirectURI)
	if rd == "" {
		return "", errors.New("redirect uri required for oauth state")
	}
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", err
	}
	payload := googleOAuthPayload{
		ProjectID: strings.TrimSpace(projectID),
		Exp:       time.Now().UTC().Add(googleOAuthStateTTL).Unix(),
		RH:        HashGoogleOAuthRedirectURI(rd),
		N:         hex.EncodeToString(nonceBytes),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	box := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(clientSecret))
	_, _ = mac.Write([]byte(box))
	sig := hex.EncodeToString(mac.Sum(nil))
	return box + "." + sig, nil
}

// VerifyGoogleOAuthState checks timing, project binding, redirect binding, and HMAC.
func VerifyGoogleOAuthState(clientSecret, projectID string, redirectURI string, state string) error {
	if strings.TrimSpace(clientSecret) == "" {
		return errors.New("oauth state verification failed")
	}
	state = strings.TrimSpace(state)
	part0, part1, ok := strings.Cut(state, ".")
	if !ok || part0 == "" || part1 == "" {
		return errors.New("invalid oauth state format")
	}
	mac := hmac.New(sha256.New, []byte(clientSecret))
	_, _ = mac.Write([]byte(part0))
	wantSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(part1))), []byte(strings.ToLower(wantSig))) {
		return errors.New("invalid oauth state signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(part0)
	if err != nil {
		return errors.New("invalid oauth state payload")
	}
	var p googleOAuthPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return errors.New("invalid oauth state payload")
	}
	if strings.TrimSpace(p.ProjectID) != strings.TrimSpace(projectID) {
		return errors.New("oauth state project mismatch")
	}
	wantRH := HashGoogleOAuthRedirectURI(redirectURI)
	if !strings.EqualFold(strings.TrimSpace(p.RH), wantRH) {
		return errors.New("oauth state redirect mismatch")
	}
	if p.Exp < time.Now().UTC().Unix() {
		return errors.New("oauth state expired")
	}
	return nil
}

// ValidateOAuthRedirectURIForPersist returns an error if the URI is unset or malformed for storage.
func ValidateOAuthRedirectURIForPersist(s, fieldName string) error {
	if fieldName == "" {
		fieldName = "oauth_redirect_uri"
	}
	u := strings.TrimSpace(s)
	if u == "" {
		return nil
	}
	if len(u) > 4096 {
		return fmt.Errorf("%s is too long", fieldName)
	}
	schemeSep := strings.Index(u, "://")
	if schemeSep <= 0 {
		return fmt.Errorf("%s must be an absolute URL with scheme", fieldName)
	}
	scheme := strings.ToLower(u[:schemeSep])
	switch scheme {
	case "https":
		return nil
	case "http":
		hostStart := schemeSep + 3
		rest := strings.ToLower(strings.TrimPrefix(u[hostStart:], "//"))
		if strings.HasPrefix(rest, "localhost") || strings.HasPrefix(rest, "127.0.0.1") {
			return nil
		}
		return fmt.Errorf("http redirect uri is only allowed for localhost development")
	default:
		return fmt.Errorf("%s scheme must be https (or http for localhost)", fieldName)
	}
}

// ValidateGoogleOAuthRedirectURIForPersist returns an error if the URI is unset or malformed for storage.
func ValidateGoogleOAuthRedirectURIForPersist(s string) error {
	return ValidateOAuthRedirectURIForPersist(s, "google_oauth_redirect_uri")
}

// SignOAuthState is an alias for SignGoogleOAuthState (HMAC keyed by provider client secret).
func SignOAuthState(clientSecret, projectID, redirectURI string) (string, error) {
	return SignGoogleOAuthState(clientSecret, projectID, redirectURI)
}

// VerifyOAuthState is an alias for VerifyGoogleOAuthState.
func VerifyOAuthState(clientSecret, projectID string, redirectURI string, state string) error {
	return VerifyGoogleOAuthState(clientSecret, projectID, redirectURI, state)
}

// Deprecated: use SignGoogleOAuthState.
func SignTenantGoogleOAuthState(clientSecret, projectID, redirectURI string) (string, error) {
	return SignGoogleOAuthState(clientSecret, projectID, redirectURI)
}

// Deprecated: use VerifyGoogleOAuthState.
func VerifyTenantGoogleOAuthState(clientSecret, projectID string, redirectURI string, state string) error {
	return VerifyGoogleOAuthState(clientSecret, projectID, redirectURI, state)
}
