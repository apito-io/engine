package services

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/golang-jwt/jwt"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

const (
	tokenDuration = 72
	expireOffset  = 3600
)

type JWTService struct {
	Cfg        *models.Config
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	kvService  interfaces.KeyValueServiceInterface
}

func GetJWTServiceWithNoSqlDb(cfg *models.Config) *JWTService {
	return &JWTService{
		Cfg:        cfg,
		PrivateKey: getPrivateKey(cfg.PrivateKeyPath),
		PublicKey:  getPublicKey(cfg.PublicKeyPath),
	}
}

func GetJWTServiceWithRedis(cfg *models.Config, kvService interfaces.KeyValueServiceInterface) *JWTService {
	return &JWTService{
		Cfg:        cfg,
		PrivateKey: getPrivateKey(cfg.PrivateKeyPath),
		PublicKey:  getPublicKey(cfg.PublicKeyPath),
		kvService:  kvService,
	}
}

func GetJWTService(cfg *models.Config) (*JWTService, error) {
	return &JWTService{
		Cfg:        cfg,
		PrivateKey: getPrivateKey(cfg.PrivateKeyPath),
		PublicKey:  getPublicKey(cfg.PublicKeyPath),
	}, nil
}

func (s *JWTService) Login(ctx context.Context, req *models.LoginRequest) (*models.JWTTokens, error) {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) Signup(ctx context.Context, req *models.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) ConfirmSignup(ctx context.Context, req *models.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) ForgetPasswordRequest(ctx context.Context, req *models.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) ConfirmForgetPassword(ctx context.Context, req *models.RegisterRequest) error {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) ChangePassword(ctx context.Context, token, old, new string) error {
	//TODO implement me
	panic("implement me")
}

func (s *JWTService) Logout(ctx context.Context, token string) error {
	//TODO implement me
	panic("implement me")
}

// no use at this time
func (t *JWTService) ExchangeAndRefreshToken(ctx context.Context, user *models.SystemUser) (*models.JWTTokens, error) {
	tokenString := strings.TrimSpace(user.RefreshToken)
	tokenObj, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New(fmt.Sprintf("Unexpected signing method"))
		}
		return t.PublicKey, nil
	})
	if err != nil {
		return nil, err
	}

	var payload interface{}
	if claims, ok := tokenObj.Claims.(jwt.MapClaims); ok && tokenObj.Valid {
		if use, ok := claims["token_use"].(string); ok && use != "refresh" {
			return nil, errors.New("invalid token, invalid usages")
		}

		if val, ok := claims["payload"].(string); ok && val != "" {

			err := json.Unmarshal([]byte(val), &payload)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("payload is missing from token. invalid token")
		}
	}

	fmt.Println(payload)

	return nil, nil
}

func (s *JWTService) VerifyIDToken(ctx context.Context, token string) (*models.TokenClaims, error) {
	tokenString := strings.TrimSpace(token)
	tokenObj, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New(fmt.Sprintf("Unexpected signing method"))
		}
		return s.PublicKey, nil
	})
	if err != nil {
		return nil, err
	}

	// Check the validity of the token
	if _, err := s.Validate(tokenObj.Valid, tokenString); err != nil {
		return nil, err
	}

	/*	// verify claims
		// verify audience claim
		if !tokenObj.Claims.(jwt.MapClaims).VerifyAudience(s.Cfg.AuthClientID, false) {
			return errors.New("audience is invalid")
		}*/

	// verify expire time
	if !tokenObj.Claims.(jwt.MapClaims).VerifyExpiresAt(time.Now().Unix(), true) {
		return nil, errors.New("token expired")
	}

	/*// verify issuer
	if !tokenObj.Claims.(jwt.MapClaims).VerifyIssuer(t.Iss, true) {
		return errors.New("iss is invalid")
	}*/

	var _jti string

	var tokenClaims models.TokenClaims
	if claims, ok := tokenObj.Claims.(jwt.MapClaims); ok && tokenObj.Valid {

		if jti, ok := claims["jti"].(string); ok {
			_jti = jti
		}

		// user id is set by access token not id token
		if user, ok := claims["account"].(string); ok {
			tokenClaims.UserID = user
			//ctx.Set("user", user)
		} else {
			return nil, errors.New("invalid token, without user")
		}

		// rest is set using id token
		if project, ok := claims["project_id"].(string); ok {
			tokenClaims.ProjectID = project
			//ctx.Set("project", project)
		}

		if role, ok := claims["project_role"].(string); ok {
			tokenClaims.Role = role
			//ctx.Set("role", role)
		}

		if paymentDueDate, ok := claims["payment_due_date"].(string); ok {
			tokenClaims.PaymentDueDate = paymentDueDate
			//ctx.Set("role", role)
		}

		if val, ok := claims["project_access"].(string); ok {
			access := strings.Split(val, ",")
			tokenClaims.AccessPermissions = access
			//ctx.Set("project_access", access)
		}

		if email, ok := claims["email"].(string); ok {
			tokenClaims.Email = email
			//ctx.Set("email", email)
		}

		if val, ok := claims["read_only"].(string); ok {
			readOnly, _ := strconv.ParseBool(val)
			tokenClaims.IsReadOnly = readOnly
			//ctx.Set("read_only", readOnly)
		}
	}

	// check if token is blacklisted
	_id, err := s.GetTokenSession(ctx, tokenClaims.Email)
	if err != nil {
		return nil, err
	}

	if _id != _jti {
		return nil, ae.LOGIN_CONFICT
	}

	return &tokenClaims, nil
}

func (s *JWTService) VerifyAccessToken(ctx context.Context, token string) error {
	tokenString := strings.TrimSpace(token)
	tokenObj, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New(fmt.Sprintf("Unexpected signing method"))
		}
		return s.PublicKey, nil
	})
	if err != nil {
		return err
	}

	// Check the validity of the token
	if _, err := s.Validate(tokenObj.Valid, tokenString); err != nil {
		return err
	}

	/*	// verify claims
		// verify audience claim
		if !tokenObj.Claims.(jwt.MapClaims).VerifyAudience(s.Cfg.AuthClientID, false) {
			return errors.New("audience is invalid")
		}*/

	// verify expire time
	if !tokenObj.Claims.(jwt.MapClaims).VerifyExpiresAt(time.Now().Unix(), true) {
		return errors.New("token expired")
	}

	/*// verify issuer
	if !tokenObj.Claims.(jwt.MapClaims).VerifyIssuer(t.Iss, true) {
		return errors.New("iss is invalid")
	}*/

	if claims, ok := tokenObj.Claims.(jwt.MapClaims); ok && tokenObj.Valid {
		if use, ok := claims["token_use"].(string); ok && use != "access" {
			return errors.New("invalid token, invalid usages")
		}
	}

	return nil
}

func (s *JWTService) GenerateLoginToken(ctx context.Context, projectWithRoles *models.ProjectWithRoles) (*models.JWTTokens, error) {

	idToken, err := s.GenerateLoginIDToken(ctx, projectWithRoles)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.GenerateLoginAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateLoginRefreshToken(projectWithRoles)
	if err != nil {
		return nil, err
	}

	return &models.JWTTokens{
		AccessToken:  *accessToken,
		IDToken:      *idToken,
		RefreshToken: *refreshToken,
	}, nil
}

func (s *JWTService) GenerateLoginIDToken(ctx context.Context, projectWithRoles *models.ProjectWithRoles) (*string, error) {

	if projectWithRoles.User == nil {
		return nil, errors.New("user is required")
	}

	user := projectWithRoles.User

	token := jwt.New(jwt.SigningMethodRS512)

	_id := s.GenerateRandomString(32)
	_exp := time.Now().Add(time.Hour * 24).Unix()
	claims := jwt.MapClaims{
		"jti":       _id,
		"exp":       _exp,
		"iat":       time.Now().Unix(),
		"token_use": "id",
	}

	// add to blacklist
	_, err := s.GetTokenSession(ctx, user.Email)
	if err != nil {
		return nil, err
	}

	// invalidate previous session
	/*if _session != "" {
		err := s.invalidateTokenSession(ctx, user.Email)
		if err != nil {
			return nil, err
		}
	}*/

	expirationDuration := time.Until(time.Unix(_exp, 0))
	// store new session
	err = s.StoreTokenSession(ctx, user.Email, _id, expirationDuration)
	if err != nil {
		return nil, err
	}

	claims["user"] = user.ID

	if user.TempPassword != "" {
		claims["temp_password"] = "true"
	}

	if user.CurrentProjectID != "" {
		claims["project_id"] = user.CurrentProjectID
	}

	if user.IsPaymentDue {
		claims["payment_due"] = "true"
	}

	claims["email"] = user.Email
	claims["account"] = user.ID

	// by default everyone is admin in their account
	claims["project_role"] = "admin"
	claims["project_access"] = strings.Join(models.GlobalPermissions, ",")

	if user.ReadOnlyProject {
		claims["project_role"] = "demo"
		claims["read_only"] = "true"
	}

	// overwrite if project exists
	if projectWithRoles.Project != nil {

		claims["project_type"] = projectWithRoles.Project.ProjectType

		claims["project_id"] = projectWithRoles.Project.ID
		claims["project_role"] = projectWithRoles.Role

		if projectWithRoles.Permissions == nil {
			claims["project_access"] = strings.Join(models.GlobalPermissions, ",")
		} else {
			claims["project_access"] = strings.Join(projectWithRoles.Permissions, ",")
		}

		// inject tenant related information
		if projectWithRoles.Project.Settings != nil && projectWithRoles.Project.TenantModelName != "" {
			claims["tenant_model"] = projectWithRoles.Project.TenantModelName
		}
	}

	token.Claims = claims

	if s.PrivateKey == nil {
		return nil, errors.New("private key is required")
	}

	tokenString, err := token.SignedString(s.PrivateKey)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}
	return &tokenString, nil
}

func (s *JWTService) GenerateLoginRefreshToken(projectWithRoles *models.ProjectWithRoles) (*string, error) {

	if projectWithRoles.User == nil {
		return nil, errors.New("user is required")
	}

	user := projectWithRoles.User

	token := jwt.New(jwt.SigningMethodRS512)

	claims := jwt.MapClaims{
		"jti":       s.GenerateRandomString(32),
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
		"iat":       time.Now().Unix(),
		"token_use": "refresh",
	}

	_payload := map[string]interface{}{}

	_payload["user"] = user.ID
	if user.CurrentProjectID != "" {
		_payload["project_id"] = user.CurrentProjectID
	}
	_payload["email"] = user.Email
	_payload["account"] = user.ID

	_payload["project_role"] = "admin"
	_payload["project_access"] = strings.Join(models.GlobalPermissions, ",")

	if user.ReadOnlyProject {
		_payload["project_role"] = "demo"
		_payload["read_only"] = "true"
	}

	if projectWithRoles.Project != nil {

		_payload["project_id"] = projectWithRoles.Project.ID
		_payload["project_role"] = projectWithRoles.Role

		if projectWithRoles.Permissions == nil {
			_payload["project_access"] = strings.Join(models.GlobalPermissions, ",")
		} else {
			_payload["project_access"] = strings.Join(projectWithRoles.Permissions, ",")
		}
	}

	_payloadByte, err := json.Marshal(_payload)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}

	claims["payload"] = string(_payloadByte)

	token.Claims = claims

	tokenString, err := token.SignedString(s.PrivateKey)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}
	return &tokenString, nil
}

func (s *JWTService) GenerateLoginAccessToken(ctx context.Context) (*string, error) {
	token := jwt.New(jwt.SigningMethodRS512)

	_tokenID := s.GenerateRandomString(32)

	claims := jwt.MapClaims{
		"jti":       _tokenID,
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
		"iat":       time.Now().Unix(),
		"token_use": "access",
	}

	token.Claims = claims

	tokenString, err := token.SignedString(s.PrivateKey)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}

	return &tokenString, nil
}

func (s *JWTService) invalidateTokenSession(ctx context.Context, email string) error {
	key := fmt.Sprintf("apito_session:%s", email)
	return s.kvService.DelValue(ctx, key)
}

func (s *JWTService) StoreTokenSession(ctx context.Context, email string, session string, ttl time.Duration) error {
	key := fmt.Sprintf("apito_session:%s", email)
	return s.kvService.SetValue(ctx, key, session, ttl)
}

func (s *JWTService) GetTokenSession(ctx context.Context, email string) (string, error) {
	key := fmt.Sprintf("apito_session:%s", email)
	val, err := s.kvService.GetValue(ctx, key)
	if err != nil {
		if err.Error() == "key not found" || err.Error() == "key expired" || errors.Is(err, redis.Nil) {
			return "", nil
		} else {
			return "", err
		}
	}

	return val, err
}

func (s *JWTService) isTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	currentTime := float64(time.Now().Unix())
	expiryTime, err := s.kvService.GetFromSortedSets(ctx, "token_blacklist", token)
	if err != nil {
		return false, err
	}
	return expiryTime > currentTime, nil
}

func (s *JWTService) GenerateIdToken(param *models.CommonSystemParams) (*string, error) {
	token := jwt.New(jwt.SigningMethodRS512)

	token.Claims = jwt.MapClaims{
		"jti":     s.GenerateRandomString(32),
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
		"iat":     time.Now().Unix(),
		"user":    param.UserID,
		"role":    param.Role.ID,
		"project": param.ProjectID,
		"email":   param.Email,
	}

	tokenString, err := token.SignedString(s.PrivateKey)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}
	return &tokenString, nil
}

func (s *JWTService) GenerateRefreshToken(param *models.CommonSystemParams, month int) (*string, error) {
	token := jwt.New(jwt.SigningMethodRS512)

	token.Claims = jwt.MapClaims{
		"jti":     s.GenerateRandomString(32),
		"exp":     time.Now().AddDate(0, month, 0).Unix(),
		"iat":     time.Now().Unix(),
		"user":    param.UserID,
		"role":    param.Role.ID,
		"project": param.ProjectID,
	}

	tokenString, err := token.SignedString(s.PrivateKey)
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
		return nil, err
	}
	return &tokenString, nil
}

/*func (s *JWTService) RefreshToken(tokenObj bool, tokenString string) (string, error) {
	if s.Validate(tokenObj, tokenString) {
		err := s.Invalidate(tokenString)
		if err != nil {

		}
	}
	return "", nil
}*/

func (s *JWTService) Validate(tokenObj bool, tokenString string) (bool, error) {
	if !tokenObj {
		return false, nil
	}
	return true, nil
}

func (s *JWTService) Invalidate(tokenString string) error {
	//return s.kvService.SetValue(tokenString, tokenString, time.Duration(s.getRemainingValidity(validity)))
	panic("implement me")
	//return s.RedisService.SetValue(tokenString, tokenString, time.Microsecond*100) // Expire the token in .1 seconds
}

func (s *JWTService) IsInBlacklist(token string) bool {
	panic("implement me")
	/*redisToken, _ := s.RedisService.GetValue(token, false)
	if redisToken == "" {
		return false
	}
	return true*/
}

func (s *JWTService) GenerateRandomString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

func (s *JWTService) getRemainingValidity(timestamp interface{}) int {
	if validity, ok := timestamp.(float64); ok {
		tm := time.Unix(int64(validity), 0)
		remainer := tm.Sub(time.Now())
		if remainer > 0 {
			return int(remainer.Seconds() + expireOffset)
		}
	}
	return expireOffset
}

func getPrivateKey(path string) *rsa.PrivateKey {
	privateKeyFile, err := ioutil.ReadFile(path)
	if err != nil {
		msg := fmt.Sprintf("Could not read private file from path %s", path)
		fmt.Println(msg)
		return nil
	}
	rsaPri, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyFile)
	if err != nil {
		msg := fmt.Sprintf("Could not uderstand the Private Key file, %s", err.Error())
		fmt.Println(msg)
		return nil
	}
	return rsaPri
}

func getPublicKey(path string) *rsa.PublicKey {
	publicKeyFile, err := ioutil.ReadFile(path)
	if err != nil {
		msg := fmt.Sprintf("Could not read public file from path %s", path)
		fmt.Println(msg)
		return nil
	}
	rsaPub, err := jwt.ParseRSAPublicKeyFromPEM(publicKeyFile)
	if err != nil {
		msg := fmt.Sprintf("Could not uderstand the Public Key file, %s", err.Error())
		fmt.Println(msg)
		return nil
	}
	return rsaPub
}
