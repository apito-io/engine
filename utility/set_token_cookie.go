package utility

import (
	"github.com/apito-io/engine/models"
	"net/http"
	"strconv"
)

func SetTokenCookie(cfg *models.Config, name, token string, httpOnly bool, expire bool) *http.Cookie {
	tokenTTL, _ := strconv.Atoi(cfg.TokenTTL)
	// send back the token
	cookie := &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		Domain:   "localhost",
		HttpOnly: httpOnly,
	}
	if expire {
		cookie.MaxAge = 0
	} else {
		cookie.MaxAge = tokenTTL * 60
	}
	switch cfg.Environment {
	case "production":
		cookie.Secure = true
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteStrictMode
	case "develop":
		cookie.Secure = true
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteNoneMode
	case "local":
		cookie.Secure = false
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteLaxMode
	default:
		cookie.Secure = false
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteLaxMode
	}
	return cookie
}
