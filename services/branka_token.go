package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/hako/branca"
	"github.com/labstack/echo/v4"
)

type BrankaToken struct {
	Token  *branca.Branca
	driver interfaces.ApitoSystemDB
}

func GetBrankaToken(cfg *models.Config, db interfaces.ApitoSystemDB) *BrankaToken {
	return &BrankaToken{
		Token:  branca.NewBranca(cfg.BrankaKey),
		driver: db,
	}
}

func (t *BrankaToken) GenerateProjectToken(claims *models.TokenClaims, ttl uint32) (*string, error) {
	number := utility.RandomStringGenerator(12)
	// Encode String to Branca Token.
	_payloadValues := []string{
		claims.Role,
		claims.UserID,
		claims.ProjectID,
		number,
	}
	if claims.TokenType != "" {
		_payloadValues = append(_payloadValues, claims.TokenType)
	}

	tokenPayload := strings.Join(_payloadValues, "|")
	token, err := t.Token.EncodeToString(tokenPayload)
	if err != nil {
		return nil, err
	}

	t.Token.SetTTL(ttl) // Uncomment this to set an expiration (or ttl) of the token (in seconds).
	return &token, nil
}

func (t *BrankaToken) GenerateSyncToken(ctx context.Context, userID, payload interface{}, tokenType string, expireAt int64) (*string, error) {
	
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	number := utility.RandomStringGenerator(18)
	// Encode String to Branca Token.
	// userID|randomNumber|tokenType|expireAt
	tokenPayload := fmt.Sprintf("%s|%s|%s|%s|%d", userID, jsonPayload, number, tokenType, expireAt)
	token, err := t.Token.EncodeToString(tokenPayload)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (t *BrankaToken) Validate(ctx context.Context, token string) (*models.TokenClaims, error) {

	if !strings.HasPrefix(token, "cli-") && !strings.HasPrefix(token, "sdk-") {
		return nil, errors.New("invalid token format")
	}

	var extractedToken string
	if strings.HasPrefix(token, "cli-") {
		extractedToken = token[4:]
	} else if strings.HasPrefix(token, "sdk-") {
		extractedToken = token[5:]
	} else {
		return nil, errors.New("invalid token format")
	}

	// Decode Branca Token.
	// "7d7b9970-6a7d-4026-949b-f953f0d4109a|todo_note_p2a46||Z55EW5FB1C5W5PP310 |api_key|1876759200"
	m, err := t.Token.DecodeToString(extractedToken)
	if err != nil {
		return nil, err
	}
	messages := strings.Split(m, "|")

	// Validate minimum required fields
	if len(messages) < 4 {
		return nil, errors.New("invalid token format")
	}

	var userID, projectID, tokenType, expireAtStr, tokenUniqueID string

	// Detect token format based on structure and content
	if len(messages) == 6 && isNumeric(messages[5]) {
		// GenerateAPIKey format: userID|projectID|_legacy_|randomNumber|tokenType|expireAt
		userID = messages[0]
		projectID = messages[1]
		// messages[2] is a legacy field, ignored
		tokenUniqueID = messages[3]
		tokenType = messages[4]
		expireAtStr = messages[5]
	} else {
		return nil, errors.New("invalid token format")
	}

	// Check if token has expiration and validate it
	if expireAtStr != "" {
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

		// Log time remaining for debugging
		timeLeft := expireAt - currentTime
		fmt.Printf("Token valid - Time remaining: %d seconds (expires at: %s)\n",
			timeLeft, time.Unix(expireAt, 0).Format(time.RFC3339))
	}

	// Check if token is blacklisted (using expireAtStr as unique identifier for API keys)
	if tokenUniqueID != "" {
		err = t.driver.CheckTokenBlacklisted(ctx, tokenUniqueID)
		if err != nil {
			return nil, err
		}
	}

	claims := &models.TokenClaims{
		Role:          "admin", // default role for api key
		UserID:        userID,
		ProjectID:     projectID,
		TokenType:     tokenType,
		TokenUniqueID: tokenUniqueID,
	}

	return claims, nil
}

// Helper function to check if a string represents a numeric value
func isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func (t *BrankaToken) ValidateAndSetContext(c echo.Context, token string) (*models.TokenClaims, error) {

	ctx := c.Request().Context()

	tokenObj, err := t.Validate(ctx, token)
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

	// role is optional for blank token since we are not using it for api token
	if tokenObj.Role != "" {
		c.Set("role", tokenObj.Role)
	}

	return tokenObj, nil
}
