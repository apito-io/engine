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
	if cfg.Environment == "production" {
		cookie.Secure = true
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteStrictMode
	} else if cfg.Environment == "develop" {
		cookie.Secure = true
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteNoneMode
	} else if cfg.Environment == "local" {
		cookie.Secure = false
		cookie.Domain = cfg.CookieDomain
		cookie.SameSite = http.SameSiteLaxMode
	}
	return cookie
}
