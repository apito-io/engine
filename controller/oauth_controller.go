package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

func (a *authCtrl) GoogleLogin(c echo.Context) error {
	// Generate random state for security
	state := uuid.New().String()

	// Set state cookie
	cookie := &http.Cookie{
		Name:     "state",
		Value:    state,
		HttpOnly: true,
		Secure:   false, // Set to true for production HTTPS
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   300, // 5 minutes
	}
	c.SetCookie(cookie)

	// Generate OAuth URL
	url := a.googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Return the URL for frontend to redirect
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: url,
		Code:    http.StatusOK,
	})
}

func (a *authCtrl) GithubLogin(c echo.Context) error {
	// Generate random state for security
	state := uuid.New().String()

	// Set state cookie
	cookie := &http.Cookie{
		Name:     "state",
		Value:    state,
		HttpOnly: true,
		Secure:   false, // Set to true for production HTTPS
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   300, // 5 minutes
	}
	c.SetCookie(cookie)

	// Generate OAuth URL
	url := a.githubOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Return the URL for frontend to redirect
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: url,
		Code:    http.StatusOK,
	})
}

func (a *authCtrl) GoogleCallback(c echo.Context) error {
	state := c.QueryParam("state")

	stateString, err := fetchFromCookies(c.Request(), "state")
	if err != nil {
		return c.JSON(http.StatusUnauthorized, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusUnauthorized,
		})
	}

	if state != stateString {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid state parameter",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	code := c.QueryParam("code")
	token, err := a.googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to exchange token: "+err.Error())
	}

	idToken := token.Extra("id_token").(string)
	payload, err := idtoken.Validate(ctx, idToken, a.googleOauthConfig.ClientID)
	if err != nil {
		return c.String(http.StatusUnauthorized, "Failed to validate ID token: "+err.Error())
	}

	// Extract user info from the payload
	payloadClaims := payload.Claims

	var email string
	if val, ok := payloadClaims["email"].(string); ok {
		email = val
	}

	var name string
	if val, ok := payloadClaims["name"].(string); ok {
		name = val
	}

	client := a.googleOauthConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo?alt=json")
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get user info: "+err.Error())
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to decode user info: "+err.Error())
	}

	var avatar string
	if val, ok := userInfo["picture"].(string); ok {
		avatar = val
	}

	/*
		userId := c.Get("user")
		if userId == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! User!",
				Code:    http.StatusBadRequest,
			})
		}

		projectID := c.Get("project_id")
		if projectID == nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: "Nope, Can't Do it..! Project!",
				Code:    http.StatusBadRequest,
			})
		}
	*/

	param := &models.CommonSystemParams{
		//UserId:    userId.(string),
		//ProjectId: projectID.(string),
		SystemCollectionName: "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		IsEntireCollectionSearchRequest: true,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"email": map[string]interface{}{
						"eq": email,
					},
				},
			},
		},
	}

	users, err := a.graphQLServer.SystemDriver.SearchUsers(ctx, param)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	if len(users.Results) == 0 {
		// create user
		registerRequest := &models.RegisterRequest{
			User: &models.SystemUser{
				Avatar:           avatar,
				Email:            email,
				FirstName:        name,
				RegisterProvider: "google",
			},
		}
		user, err := a.graphQLServer.AuthService.Signup(ctx, registerRequest)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusBadRequest,
			})
		}

		user, err = a.graphQLServer.SystemDriver.CreateSystemUser(ctx, user)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
	}

	user := users.Results[0]

	tokens, err := a.graphQLServer.JWTTokenService.GenerateLoginToken(ctx, &models.ProjectWithRoles{User: user})
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	//state := uuid.New().String()
	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "state", state, true))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "email", email, false, false))

	user.RefreshToken = tokens.RefreshToken
	user.AccessToken = tokens.AccessToken
	user.LastLoggedIn = utility.GetCurrentTime()

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""

	// Clear state cookie
	c.SetCookie(&http.Cookie{
		Name:   "state",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})

	// Redirect to frontend with success
	return c.Redirect(http.StatusTemporaryRedirect, a.Cfg.CORSOrigin)
}

func fetchGitHubUserEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("no primary verified email found")
}

func (a *authCtrl) GithubCallback(c echo.Context) error {
	state := c.QueryParam("state")

	stateString, err := fetchFromCookies(c.Request(), "state")
	if err != nil {
		return c.JSON(http.StatusUnauthorized, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusUnauthorized,
		})
	}

	if state != stateString {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "Invalid state parameter",
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()

	code := c.QueryParam("code")
	token, err := a.githubOauthConfig.Exchange(ctx, code)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to exchange token: "+err.Error())
	}

	client := a.githubOauthConfig.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get user info: "+err.Error())
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to decode user info: "+err.Error())
	}

	var name string
	if val, ok := userInfo["name"].(string); ok {
		name = val
	}

	var avatar string
	if val, ok := userInfo["avatar_url"].(map[string]interface{}); ok {
		if _val, ok := val["avatar_url"].(string); ok {
			avatar = _val
		}
	}

	email, err := fetchGitHubUserEmail(client)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch user email: "+err.Error())
	}

	param := &models.CommonSystemParams{
		//UserId:    userId.(string),
		//ProjectId: projectID.(string),
		SystemCollectionName: "users",
		Role: &models.Role{
			ID:      "admin",
			IsAdmin: true,
		},
		IsEntireCollectionSearchRequest: true,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"email": map[string]interface{}{
						"eq": email,
					},
				},
			},
		},
	}

	users, err := a.graphQLServer.SystemDriver.SearchUsers(ctx, param)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	if len(users.Results) == 0 {
		// create user
		registerRequest := &models.RegisterRequest{
			User: &models.SystemUser{
				Avatar:           avatar,
				Email:            email,
				FirstName:        name,
				RegisterProvider: "github",
			},
		}
		user, err := a.graphQLServer.AuthService.Signup(ctx, registerRequest)
		if err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: err.Error(),
				Code:    http.StatusBadRequest,
			})
		}

		user, err = a.graphQLServer.SystemDriver.CreateSystemUser(ctx, user)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
				Message: captureInternalServerError(err).Error(),
				Code:    http.StatusInternalServerError,
			})
		}
	}

	user := users.Results[0]

	tokens, err := a.graphQLServer.JWTTokenService.GenerateLoginToken(ctx, &models.ProjectWithRoles{User: user})
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	//state := uuid.New().String()
	//http.SetCookie(c.Writer, utility.SetTokenCookie(a.Cfg, "state", state, true))

	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "userToken", tokens.IDToken, false, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "accessToken", tokens.AccessToken, true, false))
	http.SetCookie(c.Response(), utility.SetTokenCookie(a.Cfg, "email", email, false, false))

	user.RefreshToken = tokens.RefreshToken
	user.AccessToken = tokens.AccessToken
	user.LastLoggedIn = utility.GetCurrentTime()

	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: captureInternalServerError(err).Error(),
			Code:    http.StatusInternalServerError,
		})
	}

	user.Secret = ""

	// Clear state cookie
	c.SetCookie(&http.Cookie{
		Name:   "state",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})

	// Redirect to frontend with success
	return c.Redirect(http.StatusTemporaryRedirect, a.Cfg.CORSOrigin)
}
