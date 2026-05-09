// Package services provides optimized token services with compression and stateless scope management
package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
)

// Scope constants - stateless mapping
const (
	// System scopes
	ScopeSystemAPIRead  = 1
	ScopeSystemAPIWrite = 2
	ScopeSystemAdmin    = 3

	// Project scopes
	ScopeProjectRead  = 10
	ScopeProjectWrite = 11
	ScopeProjectAdmin = 12

	// Plugin scopes
	ScopePluginRead  = 20
	ScopePluginWrite = 21
	ScopePluginAdmin = 22

	// User scopes
	ScopeUserRead  = 30
	ScopeUserWrite = 31
	ScopeUserAdmin = 32

	// Analytics scopes
	ScopeAnalyticsRead  = 40
	ScopeAnalyticsWrite = 41

	// Sync scopes
	ScopeSyncRead  = 50
	ScopeSyncWrite = 51
	ScopeSyncAdmin = 52
)

// ScopeNameToNumber maps scope names to their numeric representations
var ScopeNameToNumber = map[string]int{
	"system_api_read":  ScopeSystemAPIRead,
	"system_api_write": ScopeSystemAPIWrite,
	"system_admin":     ScopeSystemAdmin,
	"project_read":     ScopeProjectRead,
	"project_write":    ScopeProjectWrite,
	"project_admin":    ScopeProjectAdmin,
	"plugin_read":      ScopePluginRead,
	"plugin_write":     ScopePluginWrite,
	"plugin_admin":     ScopePluginAdmin,
	"user_read":        ScopeUserRead,
	"user_write":       ScopeUserWrite,
	"user_admin":       ScopeUserAdmin,
	"analytics_read":   ScopeAnalyticsRead,
	"analytics_write":  ScopeAnalyticsWrite,
	"sync_read":        ScopeSyncRead,
	"sync_write":       ScopeSyncWrite,
	"sync_admin":       ScopeSyncAdmin,
}

// ScopeNumberToName maps numeric scope representations back to their names
var ScopeNumberToName = map[int]string{
	ScopeSystemAPIRead:  "system_api_read",
	ScopeSystemAPIWrite: "system_api_write",
	ScopeSystemAdmin:    "system_admin",
	ScopeProjectRead:    "project_read",
	ScopeProjectWrite:   "project_write",
	ScopeProjectAdmin:   "project_admin",
	ScopePluginRead:     "plugin_read",
	ScopePluginWrite:    "plugin_write",
	ScopePluginAdmin:    "plugin_admin",
	ScopeUserRead:       "user_read",
	ScopeUserWrite:      "user_write",
	ScopeUserAdmin:      "user_admin",
	ScopeAnalyticsRead:  "analytics_read",
	ScopeAnalyticsWrite: "analytics_write",
	ScopeSyncRead:       "sync_read",
	ScopeSyncWrite:      "sync_write",
	ScopeSyncAdmin:      "sync_admin",
}

// CompactSyncPayload - optimized payload structure
type CompactSyncPayload struct {
	PIDs []string `json:"p"` // project_ids (shortened)
	SCPs []int    `json:"s"` // scopes as numbers (shortened)
}

// Custom token implementation constants
const (
	// Token version - for future compatibility
	TokenVersion = 1

	// Header size: version(1) + timestamp(8) + nonce(12) = 21 bytes
	HeaderSize = 21

	// Nonce size for AES-GCM
	NonceSize = 12

	// HMAC size
	HMACSize = 32

	// Minimum token size
	MinTokenSize = HeaderSize + NonceSize + HMACSize + 16 // 16 is minimum AES block size
)

// CustomToken represents our proprietary token encoder/decoder
type CustomToken struct {
	key []byte // 32-byte key for AES-256 and HMAC
}

// TokenHeader represents the token header structure
type TokenHeader struct {
	Version   uint8  // Token version (1 byte)
	Timestamp int64  // Unix timestamp (8 bytes)
	Nonce     []byte // Random nonce (12 bytes)
}

// TokenInfo contains metadata about a token
type TokenInfo struct {
	Version   uint8     `json:"version"`
	Timestamp int64     `json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
	Size      int       `json:"size"`
}

// NewCustomToken creates a new token encoder/decoder with the given key
func NewCustomToken(key []byte) (*CustomToken, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be exactly 32 bytes for AES-256")
	}

	return &CustomToken{
		key: key,
	}, nil
}

// EncodeToString encodes a payload string into a proprietary token
func (ct *CustomToken) EncodeToString(payload string) (string, error) {
	// Create header
	header := TokenHeader{
		Version:   TokenVersion,
		Timestamp: time.Now().Unix(),
		Nonce:     make([]byte, NonceSize),
	}

	// Generate random nonce
	if _, err := rand.Read(header.Nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Serialize header
	headerBytes := make([]byte, HeaderSize)
	headerBytes[0] = header.Version
	binary.BigEndian.PutUint64(headerBytes[1:9], uint64(header.Timestamp))
	copy(headerBytes[9:21], header.Nonce)

	// Encrypt payload
	encryptedPayload, err := ct.encrypt([]byte(payload), header.Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt payload: %w", err)
	}

	// Create HMAC over header + encrypted payload
	dataToSign := append(headerBytes, encryptedPayload...)
	hmacValue := ct.createHMAC(dataToSign)

	// Combine: header + encrypted_payload + hmac
	tokenBytes := append(headerBytes, encryptedPayload...)
	tokenBytes = append(tokenBytes, hmacValue...)

	// Encode to base64
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	return token, nil
}

// DecodeToString decodes a proprietary token back to the original payload string
func (ct *CustomToken) DecodeToString(token string) (string, error) {
	// Decode from base64
	tokenBytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("invalid base64 token: %w", err)
	}

	// Validate minimum size
	if len(tokenBytes) < MinTokenSize {
		return "", errors.New("token too short")
	}

	// Extract components
	headerBytes := tokenBytes[:HeaderSize]
	encryptedPayload := tokenBytes[HeaderSize : len(tokenBytes)-HMACSize]
	providedHMAC := tokenBytes[len(tokenBytes)-HMACSize:]

	// Verify HMAC
	dataToVerify := append(headerBytes, encryptedPayload...)
	expectedHMAC := ct.createHMAC(dataToVerify)

	if !ct.verifyHMAC(providedHMAC, expectedHMAC) {
		return "", errors.New("invalid token signature")
	}

	// Parse header
	header, err := ct.parseHeader(headerBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse header: %w", err)
	}

	// Decrypt payload
	payloadBytes, err := ct.decrypt(encryptedPayload, header.Nonce)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt payload: %w", err)
	}

	return string(payloadBytes), nil
}

// DecodeToStringWithTTL decodes a token and validates TTL
func (ct *CustomToken) DecodeToStringWithTTL(token string, ttlSeconds uint32) (string, error) {
	payload, err := ct.DecodeToString(token)
	if err != nil {
		return "", err
	}

	// Extract timestamp from token for TTL validation
	tokenBytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("invalid base64 token: %w", err)
	}

	if len(tokenBytes) < HeaderSize {
		return "", errors.New("token too short")
	}

	headerBytes := tokenBytes[:HeaderSize]
	header, err := ct.parseHeader(headerBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse header: %w", err)
	}

	// Check TTL
	currentTime := time.Now().Unix()
	if currentTime > header.Timestamp+int64(ttlSeconds) {
		return "", fmt.Errorf("token expired (TTL: %d seconds)", ttlSeconds)
	}

	return payload, nil
}

// GetTokenInfo extracts information from a token without decrypting
func (ct *CustomToken) GetTokenInfo(token string) (*TokenInfo, error) {
	tokenBytes, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 token: %w", err)
	}

	if len(tokenBytes) < HeaderSize {
		return nil, errors.New("token too short")
	}

	headerBytes := tokenBytes[:HeaderSize]
	header, err := ct.parseHeader(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	return &TokenInfo{
		Version:   header.Version,
		Timestamp: header.Timestamp,
		CreatedAt: time.Unix(header.Timestamp, 0),
		Size:      len(tokenBytes),
	}, nil
}

// SetTTL is a compatibility method (tokens are stateless, TTL is validated on decode)
func (ct *CustomToken) SetTTL(ttl uint32) {
	// This is a no-op for our stateless implementation
	// TTL validation happens during DecodeToStringWithTTL
}

// encrypt encrypts data using AES-GCM
func (ct *CustomToken) encrypt(data []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(ct.key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nil, nonce, data, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM
func (ct *CustomToken) decrypt(ciphertext []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(ct.key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// createHMAC creates HMAC-SHA256 of the given data
func (ct *CustomToken) createHMAC(data []byte) []byte {
	h := hmac.New(sha256.New, ct.key)
	h.Write(data)
	return h.Sum(nil)
}

// verifyHMAC verifies HMAC using constant-time comparison
func (ct *CustomToken) verifyHMAC(provided, expected []byte) bool {
	return hmac.Equal(provided, expected)
}

// parseHeader parses the token header
func (ct *CustomToken) parseHeader(headerBytes []byte) (*TokenHeader, error) {
	if len(headerBytes) != HeaderSize {
		return nil, errors.New("invalid header size")
	}

	header := &TokenHeader{
		Version:   headerBytes[0],
		Timestamp: int64(binary.BigEndian.Uint64(headerBytes[1:9])),
		Nonce:     make([]byte, NonceSize),
	}

	copy(header.Nonce, headerBytes[9:21])

	// Validate version
	if header.Version != TokenVersion {
		return nil, fmt.Errorf("unsupported token version: %d", header.Version)
	}

	return header, nil
}

// GenerateKey generates a random 32-byte key for token encryption
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// KeyFromString creates a key from a string (using SHA256)
func KeyFromString(s string) []byte {
	hash := sha256.Sum256([]byte(s))
	return hash[:]
}

// BrankaTokenOptimized - optimized token service using our custom token library
type BrankaTokenOptimized struct {
	Token  *CustomToken
	driver interfaces.ApitoSystemDB
}

func GetBrankaTokenOptimized(cfg *models.Config, db interfaces.ApitoSystemDB) *BrankaTokenOptimized {
	// Convert config key to our custom token format
	var key []byte
	if cfg.BrankaKey != "" {
		key = KeyFromString(cfg.BrankaKey)
	} else {
		// Generate a key from a default string if none provided
		key = KeyFromString("apito-default-key-change-in-production")
	}

	customToken, err := NewCustomToken(key)
	if err != nil {
		// Fallback to generated key
		key, _ = GenerateKey()
		customToken, _ = NewCustomToken(key)
	}

	return &BrankaTokenOptimized{
		Token:  customToken,
		driver: db,
	}
}

// compressAndEncode compresses JSON payload and encodes to base64
func compressAndEncode(data []byte) (string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(data); err != nil {
		return "", err
	}

	if err := gz.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// decompressAndDecode decompresses base64 encoded data back to JSON
func decompressAndDecode(encoded string) ([]byte, error) {
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// GenerateSyncTokenOptimized creates an optimized sync token with compressed payload
func (t *BrankaTokenOptimized) GenerateSyncTokenOptimized(ctx context.Context, userID string, projectIDs []string, scopes []string, tokenType string, expireAt int64) (*string, error) {
	// Convert scopes to numbers
	scopeNumbers := make([]int, 0, len(scopes))
	for _, scope := range scopes {
		if num, exists := ScopeNameToNumber[scope]; exists {
			scopeNumbers = append(scopeNumbers, num)
		} else {
			// Unknown scope - log warning but continue
			fmt.Printf("Warning: Unknown scope '%s' in token generation\n", scope)
		}
	}

	// Create compact payload
	payload := CompactSyncPayload{
		PIDs: projectIDs,
		SCPs: scopeNumbers,
	}

	// Marshal to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Compress and encode
	compressedPayload, err := compressAndEncode(jsonPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to compress payload: %w", err)
	}

	// Shorter random string (10 instead of 18)
	number := utility.RandomStringGenerator(10)

	// Optimized format: userID|compressedPayload|number|tokenType|expireAt
	tokenPayload := fmt.Sprintf("%s|%s|%s|%s|%d", userID, compressedPayload, number, tokenType, expireAt)

	token, err := t.Token.EncodeToString(tokenPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode token: %w", err)
	}

	return &token, nil
}

// ValidateSyncTokenOptimized validates an optimized sync token
func (t *BrankaTokenOptimized) ValidateSyncTokenOptimized(ctx context.Context, token string) (*models.TokenClaims, error) {

	extractedToken := token
	if strings.HasPrefix(token, "cli-") || strings.HasPrefix(token, "sdk-") {
		extractedToken = token[4:]
	}

	// Decode Custom Token
	m, err := t.Token.DecodeToString(extractedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	messages := strings.Split(m, "|")

	// Validate format: userID|compressedPayload|number|tokenType|expireAt
	if len(messages) != 5 {
		return nil, errors.New("invalid token format - expected 5 parts")
	}

	userID := messages[0]
	compressedPayload := messages[1]
	number := messages[2]
	tokenType := messages[3]
	expireAtStr := messages[4]

	// Parse expiration
	expireAt, err := strconv.ParseInt(expireAtStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid expiration time format: %v", err)
	}

	// Check if token is expired
	currentTime := time.Now().Unix()
	if currentTime > expireAt {
		timeLeft := expireAt - currentTime
		return nil, fmt.Errorf("token expired %d seconds ago (expired at: %s)",
			-timeLeft, time.Unix(expireAt, 0).Format(time.RFC3339))
	}

	// Decompress and decode payload
	jsonPayload, err := decompressAndDecode(compressedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress payload: %w", err)
	}

	// Unmarshal payload
	var payload CompactSyncPayload
	if err := json.Unmarshal(jsonPayload, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Convert scope numbers back to names
	scopes := make([]string, 0, len(payload.SCPs))
	for _, scopeNum := range payload.SCPs {
		if scopeName, exists := ScopeNumberToName[scopeNum]; exists {
			scopes = append(scopes, scopeName)
		} else {
			// Unknown scope number - log warning but continue
			fmt.Printf("Warning: Unknown scope number '%d' in token validation\n", scopeNum)
		}
	}

	// Create claims
	claims := &models.TokenClaims{
		Role:          "admin",
		UserID:        userID,
		ProjectIDs:    payload.PIDs,
		Scopes:        scopes,
		TokenType:     tokenType,
		TokenUniqueID: number,
		ExpireAt:      expireAt,
	}

	return claims, nil
}

// ValidateSyncTokenAndSetContext validates token and sets context (optimized version)
func (t *BrankaTokenOptimized) ValidateSyncTokenAndSetContext(c echo.Context, token string) (*models.TokenClaims, error) {
	ctx := c.Request().Context()

	claims, err := t.ValidateSyncTokenOptimized(ctx, token)
	if err != nil {
		return nil, err
	}

	// Set token in context
	c.Set("token", token)
	c.Set("user", claims.UserID)
	c.Set("sync_token_claims", claims)

	// Set project IDs if available
	if len(claims.ProjectIDs) > 0 {
		c.Set("project_ids", claims.ProjectIDs)
		c.Set("project", claims.ProjectIDs[0]) // Set first project as primary
	}

	// Set scopes
	c.Set("scopes", claims.Scopes)

	return claims, nil
}

// GetScopeNames returns scope names for given numbers (utility function)
func GetScopeNames(scopeNumbers []int) []string {
	scopes := make([]string, 0, len(scopeNumbers))
	for _, num := range scopeNumbers {
		if name, exists := ScopeNumberToName[num]; exists {
			scopes = append(scopes, name)
		}
	}
	return scopes
}

// GetScopeNumbers returns scope numbers for given names (utility function)
func GetScopeNumbers(scopeNames []string) []int {
	numbers := make([]int, 0, len(scopeNames))
	for _, name := range scopeNames {
		if num, exists := ScopeNameToNumber[name]; exists {
			numbers = append(numbers, num)
		}
	}
	return numbers
}

// GetTokenInfo extracts metadata from a token without decrypting the payload
func (t *BrankaTokenOptimized) GetTokenInfo(token string) (*TokenInfo, error) {
	return t.Token.GetTokenInfo(token)
}
