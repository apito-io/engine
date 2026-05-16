package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
)

// TestCustomTokenBasicOperations tests basic encoding/decoding functionality
func TestCustomTokenBasicOperations(t *testing.T) {
	// Generate a test key
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Create custom token instance
	token, err := NewCustomToken(key)
	if err != nil {
		t.Fatalf("Failed to create custom token: %v", err)
	}

	// Test payload
	testPayload := "user123|project456|sync_token|1758672000"

	// Encode token
	encoded, err := token.EncodeToString(testPayload)
	if err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}

	if encoded == "" {
		t.Fatal("Encoded token should not be empty")
	}

	// Decode token
	decoded, err := token.DecodeToString(encoded)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	if decoded != testPayload {
		t.Errorf("Decoded payload mismatch. Expected: %s, Got: %s", testPayload, decoded)
	}
}

// TestCustomTokenWithTTL tests TTL validation
func TestCustomTokenWithTTL(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	token, err := NewCustomToken(key)
	if err != nil {
		t.Fatalf("Failed to create custom token: %v", err)
	}

	testPayload := "user123|project456|sync_token|1758672000"

	// Encode token
	encoded, err := token.EncodeToString(testPayload)
	if err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}

	// Test with valid TTL (1 hour)
	decoded, err := token.DecodeToStringWithTTL(encoded, 3600)
	if err != nil {
		t.Fatalf("Failed to decode token with TTL: %v", err)
	}

	if decoded != testPayload {
		t.Errorf("Decoded payload mismatch. Expected: %s, Got: %s", testPayload, decoded)
	}

	// Test with very short TTL (should fail)
	_, err = token.DecodeToStringWithTTL(encoded, 1)
	if err == nil {
		t.Error("Expected TTL validation to fail with short TTL")
	}
}

// TestCustomTokenGetInfo tests token metadata extraction
func TestCustomTokenGetInfo(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	token, err := NewCustomToken(key)
	if err != nil {
		t.Fatalf("Failed to create custom token: %v", err)
	}

	testPayload := "user123|project456|sync_token|1758672000"

	// Encode token
	encoded, err := token.EncodeToString(testPayload)
	if err != nil {
		t.Fatalf("Failed to encode token: %v", err)
	}

	// Get token info
	info, err := token.GetTokenInfo(encoded)
	if err != nil {
		t.Fatalf("Failed to get token info: %v", err)
	}

	if info.Version != TokenVersion {
		t.Errorf("Expected version %d, got %d", TokenVersion, info.Version)
	}

	if info.Size == 0 {
		t.Error("Token size should not be zero")
	}

	// Check that timestamp is recent (within last minute)
	now := time.Now().Unix()
	if info.Timestamp > now || info.Timestamp < now-60 {
		t.Errorf("Token timestamp seems invalid: %d (current: %d)", info.Timestamp, now)
	}
}

// TestCustomTokenInvalidInput tests error handling
func TestCustomTokenInvalidInput(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	token, err := NewCustomToken(key)
	if err != nil {
		t.Fatalf("Failed to create custom token: %v", err)
	}

	// Test invalid base64
	_, err = token.DecodeToString("invalid-base64!")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}

	// Test empty token
	_, err = token.DecodeToString("")
	if err == nil {
		t.Error("Expected error for empty token")
	}

	// Test token too short
	_, err = token.DecodeToString("dGVzdA==") // "test" in base64
	if err == nil {
		t.Error("Expected error for token too short")
	}
}

// TestCustomTokenKeyGeneration tests key generation functions
func TestCustomTokenKeyGeneration(t *testing.T) {
	// Test GenerateKey
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key1))
	}

	// Test KeyFromString
	testString := "test-secret-string"
	key2 := KeyFromString(testString)
	if len(key2) != 32 {
		t.Errorf("Expected key length 32, got %d", len(key2))
	}

	// Same string should produce same key
	key3 := KeyFromString(testString)
	if string(key2) != string(key3) {
		t.Error("Same string should produce same key")
	}

	// Different strings should produce different keys
	key4 := KeyFromString("different-string")
	if string(key2) == string(key4) {
		t.Error("Different strings should produce different keys")
	}
}

// TestCustomTokenInvalidKey tests invalid key handling
func TestCustomTokenInvalidKey(t *testing.T) {
	// Test with wrong key length
	invalidKey := []byte("short")
	_, err := NewCustomToken(invalidKey)
	if err == nil {
		t.Error("Expected error for invalid key length")
	}

	// Test with nil key
	_, err = NewCustomToken(nil)
	if err == nil {
		t.Error("Expected error for nil key")
	}
}

// TestBrankaTokenOptimized tests the optimized token service
func TestBrankaTokenOptimized(t *testing.T) {
	// Create a mock config
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}

	// Create optimized token service
	service := GetBrankaTokenOptimized(cfg, nil)

	if service == nil {
		t.Fatal("Failed to create optimized token service")
	}

	if service.Token == nil {
		t.Fatal("Token should not be nil")
	}
}

// TestGenerateSyncTokenOptimized tests optimized sync token generation
func TestGenerateSyncTokenOptimized(t *testing.T) {
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}

	service := GetBrankaTokenOptimized(cfg, nil)
	ctx := context.Background()

	// Test data
	userID := "user123"
	projectIDs := []string{"proj1", "proj2"}
	scopes := []string{"system_api_read", "project_write", "plugin_admin"}
	tokenType := "sync_token"
	expireAt := time.Now().Unix() + 3600

	// Generate token
	token, err := service.GenerateSyncTokenOptimized(ctx, userID, projectIDs, scopes, tokenType, expireAt)
	if err != nil {
		t.Fatalf("Failed to generate sync token: %v", err)
	}

	if token == nil || *token == "" {
		t.Fatal("Generated token should not be empty")
	}

	// Validate token
	claims, err := service.ValidateSyncTokenOptimized(ctx, *token)
	if err != nil {
		t.Fatalf("Failed to validate sync token: %v", err)
	}

	// Verify claims
	if claims.UserID != userID {
		t.Errorf("UserID mismatch. Expected: %s, Got: %s", userID, claims.UserID)
	}

	if len(claims.ProjectIDs) != len(projectIDs) {
		t.Errorf("ProjectIDs length mismatch. Expected: %d, Got: %d", len(projectIDs), len(claims.ProjectIDs))
	}

	if len(claims.Scopes) != len(scopes) {
		t.Errorf("Scopes length mismatch. Expected: %d, Got: %d", len(scopes), len(claims.Scopes))
	}

	if claims.TokenType != tokenType {
		t.Errorf("TokenType mismatch. Expected: %s, Got: %s", tokenType, claims.TokenType)
	}

	if claims.ExpireAt != expireAt {
		t.Errorf("ExpireAt mismatch. Expected: %d, Got: %d", expireAt, claims.ExpireAt)
	}
}

// TestValidateSyncTokenOptimizedMcpPrefix ensures mcp- prefix strips like cli- for the same inner payload.
func TestValidateSyncTokenOptimizedMcpPrefix(t *testing.T) {
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}
	service := GetBrankaTokenOptimized(cfg, nil)
	ctx := context.Background()
	userID := "user-mcp"
	projectIDs := []string{"projA"}
	scopes := []string{"system_api_read"}
	tokenType := "mcp_token"
	expireAt := time.Now().Unix() + 7200

	raw, err := service.GenerateSyncTokenOptimized(ctx, userID, projectIDs, scopes, tokenType, expireAt)
	if err != nil || raw == nil {
		t.Fatalf("generate: %v", err)
	}
	prefixed := "mcp-" + *raw
	claims, err := service.ValidateSyncTokenOptimized(ctx, prefixed)
	if err != nil {
		t.Fatalf("validate mcp-: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID got %q want %q", claims.UserID, userID)
	}
	if claims.TokenType != tokenType {
		t.Errorf("TokenType got %q want %q", claims.TokenType, tokenType)
	}
}

// TestValidateSyncTokenOptimizedInvalidToken tests invalid token validation
func TestValidateSyncTokenOptimizedInvalidToken(t *testing.T) {
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}

	service := GetBrankaTokenOptimized(cfg, nil)
	ctx := context.Background()

	// Test invalid token
	_, err := service.ValidateSyncTokenOptimized(ctx, "invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}

	// Test empty token
	_, err = service.ValidateSyncTokenOptimized(ctx, "")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

// TestScopeMappings tests scope name/number mappings
func TestScopeMappings(t *testing.T) {
	// Test scope name to number mapping
	testScopes := []string{"system_api_read", "project_write", "plugin_admin"}
	expectedNumbers := []int{ScopeSystemAPIRead, ScopeProjectWrite, ScopePluginAdmin}

	numbers := GetScopeNumbers(testScopes)
	if len(numbers) != len(expectedNumbers) {
		t.Errorf("Expected %d numbers, got %d", len(expectedNumbers), len(numbers))
	}

	for i, expected := range expectedNumbers {
		if numbers[i] != expected {
			t.Errorf("Expected number %d, got %d", expected, numbers[i])
		}
	}

	// Test scope number to name mapping
	names := GetScopeNames(expectedNumbers)
	if len(names) != len(testScopes) {
		t.Errorf("Expected %d names, got %d", len(testScopes), len(names))
	}

	for i, expected := range testScopes {
		if names[i] != expected {
			t.Errorf("Expected name %s, got %s", expected, names[i])
		}
	}
}

// TestCompactSyncPayload tests the compact payload structure
func TestCompactSyncPayload(t *testing.T) {
	// Create compact payload
	payload := CompactSyncPayload{
		PIDs: []string{"proj1", "proj2"},
		SCPs: []int{ScopeSystemAPIRead, ScopeProjectWrite},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Unmarshal back
	var decodedPayload CompactSyncPayload
	err = json.Unmarshal(jsonData, &decodedPayload)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Verify data integrity
	if len(decodedPayload.PIDs) != len(payload.PIDs) {
		t.Errorf("PIDs length mismatch. Expected: %d, Got: %d", len(payload.PIDs), len(decodedPayload.PIDs))
	}

	if len(decodedPayload.SCPs) != len(payload.SCPs) {
		t.Errorf("SCPs length mismatch. Expected: %d, Got: %d", len(payload.SCPs), len(decodedPayload.SCPs))
	}
}

// TestCompressionAndEncoding tests compression and encoding functions
func TestCompressionAndEncoding(t *testing.T) {
	// Test data
	originalData := []byte(`{"project_ids":["proj1","proj2"],"scopes":["system_api_read","project_write"]}`)

	// Compress and encode
	compressed, err := compressAndEncode(originalData)
	if err != nil {
		t.Fatalf("Failed to compress and encode: %v", err)
	}

	if compressed == "" {
		t.Fatal("Compressed data should not be empty")
	}

	// Decompress and decode
	decompressed, err := decompressAndDecode(compressed)
	if err != nil {
		t.Fatalf("Failed to decompress and decode: %v", err)
	}

	// Verify data integrity
	if string(decompressed) != string(originalData) {
		t.Errorf("Data integrity check failed. Expected: %s, Got: %s", string(originalData), string(decompressed))
	}

	// Verify compression ratio
	if len(compressed) >= len(originalData) {
		t.Logf("Compression ratio: %d -> %d bytes", len(originalData), len(compressed))
	}
}

// TestTokenExpiration tests token expiration handling
func TestTokenExpiration(t *testing.T) {
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}

	service := GetBrankaTokenOptimized(cfg, nil)
	ctx := context.Background()

	// Create expired token (expired 1 hour ago)
	expiredAt := time.Now().Unix() - 3600
	token, err := service.GenerateSyncTokenOptimized(ctx, "user123", []string{"proj1"}, []string{"system_api_read"}, "sync_token", expiredAt)
	if err != nil {
		t.Fatalf("Failed to generate expired token: %v", err)
	}

	// Validate expired token
	_, err = service.ValidateSyncTokenOptimized(ctx, *token)
	if err == nil {
		t.Error("Expected error for expired token")
	}

	// Check error message contains expiration info
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// BenchmarkCustomTokenEncode benchmarks token encoding performance
func BenchmarkCustomTokenEncode(b *testing.B) {
	key, _ := GenerateKey()
	token, _ := NewCustomToken(key)
	payload := "user123|project456|sync_token|1758672000"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := token.EncodeToString(payload)
		if err != nil {
			b.Fatalf("Failed to encode token: %v", err)
		}
	}
}

// BenchmarkCustomTokenDecode benchmarks token decoding performance
func BenchmarkCustomTokenDecode(b *testing.B) {
	key, _ := GenerateKey()
	token, _ := NewCustomToken(key)
	payload := "user123|project456|sync_token|1758672000"
	encoded, _ := token.EncodeToString(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := token.DecodeToString(encoded)
		if err != nil {
			b.Fatalf("Failed to decode token: %v", err)
		}
	}
}

// BenchmarkSyncTokenGeneration benchmarks optimized sync token generation
func BenchmarkSyncTokenGeneration(b *testing.B) {
	cfg := &models.Config{
		BrankaKey: "test-branka-key",
	}

	service := GetBrankaTokenOptimized(cfg, nil)
	ctx := context.Background()

	userID := "user123"
	projectIDs := []string{"proj1", "proj2"}
	scopes := []string{"system_api_read", "project_write", "plugin_admin"}
	tokenType := "sync_token"
	expireAt := time.Now().Unix() + 3600

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateSyncTokenOptimized(ctx, userID, projectIDs, scopes, tokenType, expireAt)
		if err != nil {
			b.Fatalf("Failed to generate sync token: %v", err)
		}
	}
}
