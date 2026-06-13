package models

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSignAndVerifyGoogleOAuthState(t *testing.T) {
	secret := "test-client-secret"
	pid := "proj-1"
	redirect := "https://app.example.com/oauth/callback"

	state, err := SignGoogleOAuthState(secret, pid, redirect)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(state) == "" || !strings.Contains(state, ".") {
		t.Fatalf("unexpected state format: %q", state)
	}
	if err := VerifyGoogleOAuthState(secret, pid, redirect, state); err != nil {
		t.Fatalf("verify same redirect: %v", err)
	}
	if err := VerifyGoogleOAuthState(secret, pid, "https://other.example.com/cb", state); err == nil {
		t.Fatal("expected redirect mismatch error")
	}
	if err := VerifyGoogleOAuthState(secret, "other-project", redirect, state); err == nil {
		t.Fatal("expected project mismatch error")
	}
	if err := VerifyGoogleOAuthState("wrong", pid, redirect, state); err == nil {
		t.Fatal("expected bad signature")
	}
}

func TestVerifyGoogleOAuthStateExpired(t *testing.T) {
	secret := "sec"
	pid := "proj"
	rd := "https://a.example/cb"
	p := googleOAuthPayload{
		ProjectID: pid,
		Exp:       time.Now().UTC().Add(-time.Hour).Unix(),
		RH:        HashGoogleOAuthRedirectURI(rd),
		N:         "abcd",
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	box := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(box))
	sig := hex.EncodeToString(mac.Sum(nil))
	state := box + "." + sig
	verr := VerifyGoogleOAuthState(secret, pid, rd, state)
	if verr == nil {
		t.Fatal("want expired oauth state error")
	}
	if !strings.Contains(strings.ToLower(verr.Error()), "expired") {
		t.Fatalf("expected expired message, got %v", verr)
	}
}
