package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
)

func TestResolveFacebookAccessTokenValid(t *testing.T) {
	var sawAppToken bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/debug_token":
			if r.URL.Query().Get("access_token") == "2304966806993060|test-secret" &&
				r.URL.Query().Get("input_token") == "user-token" {
				sawAppToken = true
			}
			_, _ = w.Write([]byte(`{"data":{"app_id":"2304966806993060","is_valid":true,"expires_at":2000000000,"user_id":"fb-1"}}`))
		case "/me":
			_, _ = w.Write([]byte(`{"id":"fb-1","name":"Fahim Test","email":"fahim@example.com","picture":{"data":{"url":"https://example.com/p.jpg"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	now := time.Unix(1_700_000_000, 0)
	identity, err := resolveFacebookAccessToken(context.Background(), srv.Client(), srv.URL, "2304966806993060", "test-secret", "user-token", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sawAppToken {
		t.Fatal("debug_token did not receive the app token")
	}
	if identity.Provider != models.OAuthProviderFacebook || identity.Subject != "fb-1" {
		t.Fatalf("identity %#v", identity)
	}
	if identity.Email != "fahim@example.com" || !identity.EmailVerified {
		t.Fatalf("email %#v", identity)
	}
	if identity.Name != "Fahim Test" || identity.PhotoURL != "https://example.com/p.jpg" {
		t.Fatalf("profile %#v", identity)
	}
}

func TestResolveFacebookAccessTokenWrongApp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"app_id":"999","is_valid":true,"expires_at":2000000000}}`))
	}))
	defer srv.Close()

	_, err := resolveFacebookAccessToken(context.Background(), srv.Client(), srv.URL, "2304966806993060", "test-secret", "user-token", time.Unix(1_700_000_000, 0))
	if err == nil || !strings.Contains(err.Error(), "different app") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveFacebookAccessTokenExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"app_id":"2304966806993060","is_valid":true,"expires_at":100}}`))
	}))
	defer srv.Close()

	_, err := resolveFacebookAccessToken(context.Background(), srv.Client(), srv.URL, "2304966806993060", "test-secret", "user-token", time.Unix(1_700_000_000, 0))
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveFacebookAccessTokenMissingEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/debug_token" {
			_, _ = w.Write([]byte(`{"data":{"app_id":"2304966806993060","is_valid":true,"expires_at":0}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"fb-phone","name":"No Email"}`))
	}))
	defer srv.Close()

	identity, err := resolveFacebookAccessToken(context.Background(), srv.Client(), srv.URL, "2304966806993060", "test-secret", "user-token", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "fb-phone" || identity.Email != "" || identity.EmailVerified {
		t.Fatalf("identity %#v", identity)
	}
}

func TestResolveUserForOAuthLoginCreatesWithoutEmail(t *testing.T) {
	store := &oauthMemStore{}
	svc := &ProjectUserService{ctx: context.Background(), projectID: "p1", store: store}
	user, err := svc.ResolveUserForOAuthLogin(models.OAuthProviderFacebook, "fb-phone", "", false, "", "", func() (*models.User, error) {
		return &models.User{
			ID: "created", Provider: models.UserProviderFacebook, OAuthSub: "fb-phone",
			Status: models.UserStatusActive, Role: "vendor",
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "created" || user.Email != "" {
		t.Fatalf("user %#v", user)
	}
}

func TestResolveUserForOAuthLoginLinksGoogleEmail(t *testing.T) {
	store := &oauthMemStore{
		byEmail: map[string]*models.ProjectAuthUser{
			"shared@example.com": {
				ID: "google-user", Provider: "google", Email: "shared@example.com",
				Status: models.UserStatusActive, Role: "vendor",
			},
		},
	}
	svc := &ProjectUserService{ctx: context.Background(), projectID: "p1", store: store}
	user, err := svc.ResolveUserForOAuthLogin(models.OAuthProviderFacebook, "fb-9", "shared@example.com", true, "", "", func() (*models.User, error) {
		t.Fatal("should link the Google user, not create another")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "google-user" || user.OAuthSub != "fb-9" {
		t.Fatalf("user %#v", user)
	}
}
