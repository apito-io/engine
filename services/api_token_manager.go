package services

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidKey     = errors.New("invalid key format")
	ErrExpiredKey     = errors.New("key has expired")
	ErrInvalidPayload = errors.New("invalid payload")
)

// APIKeyManager handles API key operations with optimized performance
type APIKeyManager struct {
	cipher cipher.Block
	gcm    cipher.AEAD
	driver interfaces.ApitoSystemDB
}

func NewAPIKeyManager(cfg *models.Config, driver interfaces.ApitoSystemDB) (*APIKeyManager, error) {
	key := []byte(cfg.BrankaKey)
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &APIKeyManager{
		cipher: block,
		gcm:    gcm,
		driver: driver,
	}, nil
}

func NewAPIKeyManagerNoDB(cfg *models.Config) (*APIKeyManager, error) {
	key := []byte(cfg.BrankaKey)
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &APIKeyManager{
		cipher: block,
		gcm:    gcm,
	}, nil
}

func (m *APIKeyManager) GenerateKey(payload *models.TokenClaims) (string, error) {

	if payload.TokenUniqueID == "" {
		payload.TokenUniqueID = utility.RandomStringGenerator(12)
	}

	data := make([]byte, 0, 256) // Initial capacity, will grow if needed

	// Role (fixed 16 bytes)
	data = append(data, stringToFixedBytes(payload.Role, 16)...)

	// UserId (UUID - 16 bytes)
	data = append(data, uuidToBytes(payload.UserID)...)

	// ProjectId (variable length)
	projectIdBytes := []byte(payload.ProjectID)
	if len(projectIdBytes) > 255 {
		return "", errors.New("ProjectId too long")
	}
	data = append(data, byte(len(projectIdBytes)))
	data = append(data, projectIdBytes...)

	// TokenUniqueId (12 characters)
	if len(payload.TokenUniqueID) != 12 {
		return "", errors.New("TokenUniqueID must be 12 characters")
	}
	data = append(data, stringToFixedBytes(payload.TokenUniqueID, 12)...)

	// TenantID (UUID - 16 bytes)
	data = append(data, uuidToBytes(payload.TenantID)...)

	// TokenType (fixed 8 bytes)
	data = append(data, stringToFixedBytes(payload.TokenType, 8)...)

	// ExpireAt (8 bytes)
	expBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(expBytes, uint64(payload.ExpireAt.Unix()))
	data = append(data, expBytes...)

	// Scopes
	scopesData := encodeScopesAsBytes(payload.Scopes)
	data = append(data, scopesData...)

	// Generate nonce
	nonce := make([]byte, m.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt
	encrypted := m.gcm.Seal(nonce, nonce, data, nil)

	// Encode to base32 for shorter length
	return fmt.Sprintf("ak_%s", base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(encrypted)), nil
}

func (m *APIKeyManager) Validate(ctx context.Context, key string, skipDBCheck bool) (*models.TokenClaims, error) {
	if len(key) < 3 || key[:3] != "ak_" {
		return nil, ErrInvalidKey
	}

	// Decode base32
	encrypted, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(key[3:])
	if err != nil {
		return nil, ErrInvalidKey
	}

	// Extract nonce and decrypt
	if len(encrypted) < m.gcm.NonceSize() {
		return nil, ErrInvalidKey
	}
	nonce := encrypted[:m.gcm.NonceSize()]
	data, err := m.gcm.Open(nil, nonce, encrypted[m.gcm.NonceSize():], nil)
	if err != nil {
		return nil, ErrInvalidKey
	}

	payload := &models.TokenClaims{}
	offset := 0

	// Role (16 bytes)
	if offset+16 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.Role = string(bytes.TrimRight(data[offset:offset+16], "\x00"))
	offset += 16

	// UserId (16 bytes)
	if offset+16 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.UserID = bytesToUUID(data[offset : offset+16])
	offset += 16

	// ProjectId (variable length)
	if offset >= len(data) {
		return nil, ErrInvalidPayload
	}
	projectIdLen := int(data[offset])
	offset++
	if offset+projectIdLen > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.ProjectID = string(data[offset : offset+projectIdLen])
	offset += projectIdLen

	// TokenUniqueId (12 bytes)
	if offset+12 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.TokenUniqueID = string(data[offset : offset+12])
	offset += 12

	if !skipDBCheck {
		err = m.driver.CheckTokenBlacklisted(ctx, payload.TokenUniqueID)
		if err != nil {
			return nil, err
		}
	}

	// TenantID (16 bytes)
	if offset+16 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.TenantID = bytesToUUID(data[offset : offset+16])
	offset += 16

	// TokenType (8 bytes)
	if offset+8 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.TokenType = string(bytes.TrimRight(data[offset:offset+8], "\x00"))
	offset += 8

	// ExpireAt (8 bytes)
	if offset+8 > len(data) {
		return nil, ErrInvalidPayload
	}
	payload.ExpireAt = time.Unix(int64(binary.LittleEndian.Uint64(data[offset:offset+8])), 0)
	offset += 8

	// Scopes (remaining data)
	scopes, err := decodeScopesFromBytes(data[offset:])
	if err != nil {
		return nil, err
	}
	payload.Scopes = scopes

	// Check expiration
	if time.Now().After(payload.ExpireAt) {
		return nil, ErrExpiredKey
	}

	return payload, nil
}

func (m *APIKeyManager) ValidateAndSetContext(c echo.Context, token string) (*models.TokenClaims, error) {

	ctx := c.Request().Context()

	tokenObj, err := m.Validate(ctx, token, true)
	if err != nil {
		return nil, err
	}

	// set token in context
	c.Set("token", token)

	if tokenObj.UserID != "" {
		c.Set("user", tokenObj.UserID)
	} else {
		return nil, errors.New("invalid token, without user")
	}

	if tokenObj.ProjectID != "" {
		c.Set("project", tokenObj.ProjectID)
	} else {
		return nil, errors.New("invalid token, without project")
	}

	if tokenObj.Role != "" {
		c.Set("role", tokenObj.Role)
	} else {
		return nil, errors.New("invalid token, without role")
	}

	if tokenObj.TenantID != "" {
		c.Set("tenant", tokenObj.TenantID)
	}

	return tokenObj, nil
}

func (m *APIKeyManager) GenerateTenantToken(ctx context.Context, tenantID, token string) (string, error) {

	claims, err := m.Validate(ctx, token, true)
	if err != nil {
		return "", err
	}

	var projectUserID string
	if val, ok := ctx.Value("project_user_id").(string); ok && val != "" {
		projectUserID = val
	}

	// overwrite the project user id
	claims.UserID = projectUserID

	claims.TenantID = tenantID
	//claims.TenantID = "2a7cd0c3-d263-4d25-8b6a-263692879fc6"
	claims.TokenType = "tenant"
	claims.ExpireAt = time.Now().Add(time.Hour * 24 * 30) // 30 days
	return m.GenerateKey(claims)
}

func stringToFixedBytes(s string, size int) []byte {
	b := make([]byte, size)
	copy(b, s)
	return b
}

func uuidToBytes(s string) []byte {
	u, err := uuid.Parse(s)
	if err != nil {
		return make([]byte, 16)
	}
	return u[:]
}

func bytesToUUID(b []byte) string {
	u, err := uuid.FromBytes(b)
	if err != nil {
		return ""
	}
	return u.String()
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func encodeScopesAsBytes(scopes []string) []byte {
	var result []byte
	result = append(result, byte(len(scopes)))
	for _, scope := range scopes {
		result = append(result, byte(len(scope)))
		result = append(result, []byte(scope)...)
	}
	return result
}

func decodeScopesFromBytes(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, ErrInvalidPayload
	}
	count := int(data[0])
	scopes := make([]string, 0, count)
	offset := 1
	for i := 0; i < count; i++ {
		if offset >= len(data) {
			return nil, ErrInvalidPayload
		}
		length := int(data[offset])
		offset++
		if offset+length > len(data) {
			return nil, ErrInvalidPayload
		}
		scopes = append(scopes, string(data[offset:offset+length]))
		offset += length
	}
	return scopes, nil
}
