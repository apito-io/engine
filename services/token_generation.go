package services

import (
	"fmt"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/tomogoma/go-api-guard"
	typederrs "github.com/tomogoma/go-typed-errors"
	"strings"
)

type APIKeyMock struct {
	Val []byte
}

func (k APIKeyMock) Value() []byte {
	return k.Val
}

type DBMock struct {
	typederrs.NotFoundErrCheck

	ExpInsAPIKErr        error
	ExpAPIKBUsrIDVal     api.Key
	ExpAPIKsBUsrIDValErr error
	RecInsAPIKUsrID      string
}

func (db *DBMock) APIKeyByUserIDVal(userID string, key []byte) (api.Key, error) {
	if db.ExpAPIKsBUsrIDValErr != nil {
		return nil, db.ExpAPIKsBUsrIDValErr
	}
	if db.ExpAPIKBUsrIDVal == nil {
		return nil, typederrs.NewNotFound("not found")
	}
	return db.ExpAPIKBUsrIDVal, db.ExpAPIKsBUsrIDValErr
}

func (db *DBMock) InsertAPIKey(userID string, key []byte) (api.Key, error) {
	if db.ExpInsAPIKErr != nil {
		return nil, db.ExpInsAPIKErr
	}
	db.RecInsAPIKUsrID = userID
	return APIKeyMock{Val: key}, db.ExpInsAPIKErr
}

type KeyGenMock struct {
	ExpSRBsErr error
	ExpSRBs    []byte
}

func (kg *KeyGenMock) SecureRandomBytes(length int) ([]byte, error) {
	if kg.ExpSRBsErr != nil {
		return nil, kg.ExpSRBsErr
	}
	return kg.ExpSRBs, kg.ExpSRBsErr
}

type TokenService struct {
	Guard *api.Guard
}

func GetTokenService(cfg *models.Config) *TokenService {

	db := &DBMock{}
	// mocking key generation to demonstrate resulting API key
	keyGen := &KeyGenMock{ExpSRBs: []byte(cfg.BrankaKey)}

	g, _ := api.NewGuard(
		db,
		api.WithKeyGenerator(keyGen), // This is optional
	)

	return &TokenService{
		Guard: g,
	}
}

func GetTokenServiceWithRedis(cfg *models.Config) *TokenService {

	db := &DBMock{}
	// mocking key generation to demonstrate resulting API key
	keyGen := &KeyGenMock{ExpSRBs: []byte(cfg.BrankaKey)}

	g, _ := api.NewGuard(
		db,
		api.WithKeyGenerator(keyGen), // This is optional
	)

	return &TokenService{
		Guard: g,
	}
}

func (t *TokenService) GenerateProjectToken(user *models.SystemUser, projectId string) (string, error) {

	/*var account *protobuff.UserProject
	for _, a := range user.Accounts {
		if a.ProjectId == projectId {
			account = a
		}
	}

	if account == nil {
		return "", errors.New("Project Not Found")
	}

	claim := fmt.Sprintf("user=%s,role=%s,project=%s,t=%d", user.Id, account.Roles[0], account.ProjectId, user.TokenRefreshVal)
	// GenerateIdToken API key
	APIKey, err := t.Guard.NewAPIKey(claim)
	if err != nil {
		return "", err
	}
	// Output:
	// bXktdW5pcXVlLXVzZXItaWQ=.an-api-key
	return strings.Split(string(APIKey.Value()), "=.")[0], nil*/
	return "", nil
}

func (t *TokenService) Validate(cfg *models.Config, token string) (*models.TokenClaims, error) {
	token = fmt.Sprintf("%s=.%s", token, cfg.BrankaKey)
	// Validate API Key
	userID, _ := t.Guard.APIKeyValid([]byte(token))
	claims := strings.Split(userID, ",")
	if len(claims) == 0 {
		return nil, ae.InvalidToken
	}
	if len(claims) == 2 { // login token
		return &models.TokenClaims{
			UserID: strings.Split(claims[0], "=")[1],
		}, nil
	} else {
		return &models.TokenClaims{
			UserID:    strings.Split(claims[0], "=")[1],
			Role:      strings.Split(claims[1], "=")[1],
			ProjectID: strings.Split(claims[2], "=")[1],
		}, nil
	}

}
