package services

import (
	"context"
	"encoding/base32"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/oklog/ulid/v2"
)

func testBrankaConfig() *models.Config {
	return &models.Config{BrankaKey: strings.Repeat("k", 32)}
}

func TestTokenPrefixAndFingerprint(t *testing.T) {
	token := "ak_" + strings.Repeat("A", 40)
	prefix := TokenPrefix(token)
	if len(prefix) != projectTokenPrefixDisplayLen {
		t.Fatalf("prefix len=%d want %d (%q)", len(prefix), projectTokenPrefixDisplayLen, prefix)
	}
	if !strings.HasPrefix(token, prefix) {
		t.Fatalf("prefix %q not prefix of token", prefix)
	}
	fp := TokenFingerprint(token)
	if len(fp) != 64 {
		t.Fatalf("fingerprint len=%d want 64", len(fp))
	}
	if TokenFingerprint(token) != fp {
		t.Fatal("fingerprint not stable")
	}
	if TokenFingerprint(token+"x") == fp {
		t.Fatal("fingerprint collision")
	}
}

func TestGenerateKeyV2LongRoleRoundTrip(t *testing.T) {
	mgr, err := NewProjectKeyManagerNoDB(testBrankaConfig())
	if err != nil {
		t.Fatal(err)
	}
	role := "authenticated_app_user_role" // >16 chars — fails on v1 fixed field
	userID := ulid.Make().String()
	claims := &models.TokenClaims{
		Role:      role,
		UserID:    userID,
		ProjectID: "proj_test",
		ExpireAt:  time.Now().Add(time.Hour).Unix(),
		TokenType: "api",
	}
	key, err := mgr.GenerateKey(claims)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if !strings.HasPrefix(key, "ak_") {
		t.Fatalf("missing ak_ prefix: %s", key)
	}
	if claims.TokenUniqueID == "" || len(claims.TokenUniqueID) != 12 {
		t.Fatalf("TokenUniqueID not set: %q", claims.TokenUniqueID)
	}

	got, err := mgr.Validate(context.Background(), key, true)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.Role != role {
		t.Fatalf("role=%q want %q", got.Role, role)
	}
	if got.UserID != userID {
		t.Fatalf("user=%q want %q", got.UserID, userID)
	}
	if got.ProjectID != "proj_test" {
		t.Fatalf("project=%q", got.ProjectID)
	}
	if got.TokenUniqueID != claims.TokenUniqueID {
		t.Fatalf("token id mismatch")
	}
}

func TestGenerateKeyRejectsRoleTooLong(t *testing.T) {
	mgr, err := NewProjectKeyManagerNoDB(testBrankaConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = mgr.GenerateKey(&models.TokenClaims{
		Role:      strings.Repeat("r", 65),
		UserID:    ulid.Make().String(),
		ProjectID: "p",
		ExpireAt:  time.Now().Add(time.Hour).Unix(),
	})
	if err == nil {
		t.Fatal("expected role too long error")
	}
}

func TestValidateV1LegacyStillWorks(t *testing.T) {
	mgr, err := NewProjectKeyManagerNoDB(testBrankaConfig())
	if err != nil {
		t.Fatal(err)
	}
	userID := ulid.Make().String()
	claims := &models.TokenClaims{
		Role:          "public",
		UserID:        userID,
		ProjectID:     "p1",
		TokenUniqueID: "abcdefghijkl",
		TokenType:     "api",
		ExpireAt:      time.Now().Add(time.Hour).Unix(),
	}
	userWire, err := userIDToWireBytes(claims.UserID)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 0, 128)
	data = append(data, stringToFixedBytes(claims.Role, 16)...)
	data = append(data, userWire...)
	data = append(data, byte(len(claims.ProjectID)))
	data = append(data, []byte(claims.ProjectID)...)
	data = append(data, stringToFixedBytes(claims.TokenUniqueID, 12)...)
	data = append(data, stringToFixedBytes(claims.TokenType, 8)...)
	exp := make([]byte, 8)
	binary.LittleEndian.PutUint64(exp, uint64(claims.ExpireAt))
	data = append(data, exp...)
	data = append(data, encodeScopesAsBytes(nil)...)

	nonce := make([]byte, mgr.gcm.NonceSize())
	encrypted := mgr.gcm.Seal(nonce, nonce, data, nil)
	key := "ak_" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(encrypted)

	got, err := mgr.Validate(context.Background(), key, true)
	if err != nil {
		t.Fatalf("v1 Validate: %v", err)
	}
	if got.Role != "public" {
		t.Fatalf("v1 role=%q", got.Role)
	}
	if got.TokenUniqueID != claims.TokenUniqueID {
		t.Fatalf("v1 uid=%q", got.TokenUniqueID)
	}
}
