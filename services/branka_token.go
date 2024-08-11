package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/hako/branca"
	"github.com/labstack/echo/v4"
)

const letterBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz01234567890"

// const numberBytes = "1234567890"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandomStringGenerator(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

type BrankaToken struct {
	Token  *branca.Branca
	driver interfaces.SystemDBInterface
}

func GetBrankaToken(cfg *models.Config, db interfaces.SystemDBInterface) *BrankaToken {
	return &BrankaToken{
		Token:  branca.NewBranca(KEY),
		driver: db,
	}
}

func (t *BrankaToken) GenerateProjectToken(userId string, projectId string, ttl uint32) (*string, error) {
	number := RandomStringGenerator(12)
	// Encode String to Branca Token.
	tokenPayload := fmt.Sprintf("%s|%s|%s|%s", userId, projectId, number)
	token, err := t.Token.EncodeToString(tokenPayload)
	if err != nil {
		return nil, err
	}

	t.Token.SetTTL(ttl) // Uncomment this to set an expiration (or ttl) of the token (in seconds).
	return &token, nil
}

func (t *BrankaToken) Validate(ctx context.Context, token string) (*models.TokenClaims, error) {
	// Decode Branca Token.
	m, err := t.Token.DecodeToString(token)
	if err != nil {
		return nil, err
	}
	message := strings.Split(m, "|")

	tokenUniqueId := message[3]

	err = t.driver.CheckTokenBlacklisted(ctx, tokenUniqueId)
	if err != nil {
		return nil, err
	}

	return &models.TokenClaims{
		UserId:        message[1],
		ProjectId:     message[2],
		TokenUniqueId: tokenUniqueId,
	}, nil
}

func (t *BrankaToken) VerifyBlankaApiToken(c echo.Context, token string) (*models.TokenClaims, error) {

	ctx := c.Request().Context()

	tokenObj, err := t.Validate(ctx, token)
	if err != nil {
		return nil, err
	}

	if tokenObj.UserId != "" {
		c.Set("user", tokenObj.UserId)
	} else {
		return nil, errors.New("invalid token, without user")
	}

	if tokenObj.ProjectId != "" {
		c.Set("project", tokenObj.ProjectId)
	} else {
		return nil, errors.New("invalid token, without project")
	}

	return tokenObj, nil
}
