package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	goBatch "github.com/RashadAnsari/go-batch/v2"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/telemetry"
	"github.com/apito-io/engine/utility"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type ApitoTokenService struct {
	cfg            *models.Config
	systemDB       interfaces.ApitoSystemDB
	apiKeyManager  *ProjectKeyManager
	syncKeyManager *BrankaTokenOptimized
	blankaService  *BrankaToken
	authService    AuthServiceInterface
	Batch          *goBatch.Batch[models.ProjectApiTracking]
	// Removed dbWriteLock - channels are thread-safe
	ctx    context.Context
	cancel context.CancelFunc
}

func getPrimaryProjectIDFromClaims(claims *models.TokenClaims) string {
	if claims == nil {
		return ""
	}
	if claims.ProjectID != "" {
		return claims.ProjectID
	}
	if len(claims.ProjectIDs) > 0 {
		return claims.ProjectIDs[0]
	}
	return ""
}

func (t *ApitoTokenService) runPostTokenValidateHook(ctx echo.Context, claims *models.TokenClaims) {
	if t == nil || t.cfg == nil || claims == nil || t.cfg.PostTokenValidateHook == nil {
		return
	}
	start := time.Now()
	t.cfg.PostTokenValidateHook(ctx, claims)
	telemetry.RecordSessionValidate(ctx.Request().Context(), t.cfg, "ok", time.Since(start))
}

func NewApitoTokenService(cfg *models.Config, auth AuthServiceInterface, driver interfaces.ApitoSystemDB) (*ApitoTokenService, error) {

	ctx, cancel := context.WithCancel(context.Background())

	batch := goBatch.New[models.ProjectApiTracking](
		goBatch.WithSize(100),
		goBatch.WithMaxWait(5*time.Second),
		goBatch.WithContext(ctx),
	)

	apiKeyManager, err := NewProjectKeyManager(cfg, driver)
	if err != nil {
		cancel()
		return nil, err
	}

	syncKeyManager := GetBrankaTokenOptimized(cfg, driver)

	service := &ApitoTokenService{
		cfg:            cfg,
		systemDB:       driver,
		blankaService:  GetBrankaToken(cfg, driver),
		apiKeyManager:  apiKeyManager,
		syncKeyManager: syncKeyManager,
		authService:    auth,
		Batch:          batch,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start batch processing in a goroutine
	go service.batchProcessor()

	return service, nil
}

// batchProcessor runs in a separate goroutine to consume batch data
func (t *ApitoTokenService) batchProcessor() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🚨 [BATCH-PROCESSOR] Panic recovered: %v\n", r)
		}
		fmt.Println("🔍 [BATCH-PROCESSOR] Batch processor stopped")
	}()

	fmt.Println("🔍 [BATCH-PROCESSOR] Starting batch processor")

	for {
		select {
		case <-t.ctx.Done():
			fmt.Println("🔍 [BATCH-PROCESSOR] Context cancelled, stopping batch processor")
			return
		case data, ok := <-t.Batch.Output:
			if !ok {
				fmt.Println("🔍 [BATCH-PROCESSOR] Batch output channel closed")
				return
			}

			// Process the batched data
			t.processBatchData(data)
		}
	}
}

// processBatchData handles the actual processing of batched tracking data
func (t *ApitoTokenService) processBatchData(batch []models.ProjectApiTracking) {
	if len(batch) == 0 {
		return
	}

	// Aggregate tracking data by project to reduce DB operations
	aggregated := make(map[string]*models.ApiTracking)

	for _, item := range batch {
		for projectID, tracking := range item {
			if existing, exists := aggregated[projectID]; exists {
				existing.Increment += tracking.Increment
				existing.Bandwidth += tracking.Bandwidth
			} else {
				aggregated[projectID] = &models.ApiTracking{
					Increment: tracking.Increment,
					Bandwidth: tracking.Bandwidth,
				}
			}
		}
	}

	// Process aggregated data with timeout
	ctx, cancel := context.WithTimeout(t.ctx, 30*time.Second)
	defer cancel()

	for projectID, tracking := range aggregated {
		select {
		case <-ctx.Done():
			fmt.Printf("⚠️ [BATCH-PROCESSOR] Processing timeout for remaining projects\n")
			return
		default:
			// Process each project's tracking data
			err := t.processProjectTracking(ctx, projectID, tracking)
			if err != nil {
				// Log error but continue processing other projects
				fmt.Printf("❌ [BATCH-PROCESSOR] Error processing tracking for project %s: %v\n", projectID, err)
			}
		}
	}

	fmt.Printf("✅ [BATCH-PROCESSOR] Successfully processed batch with %d projects\n", len(aggregated))
}

// processProjectTracking handles individual project tracking data
func (t *ApitoTokenService) processProjectTracking(ctx context.Context, projectID string, tracking *models.ApiTracking) error {
	// TODO: Implement your actual database update logic here
	// This is where you would update your tracking database
	fmt.Printf("📊 [BATCH-PROCESSOR] Processing tracking for project %s: increment=%d, bandwidth=%.2f MB\n",
		projectID, tracking.Increment, tracking.Bandwidth)

	// Example implementation (replace with your actual DB logic):
	// return t.systemDB.UpdateProjectTracking(ctx, projectID, tracking)

	return nil
}

// Shutdown gracefully stops the batch processor
func (t *ApitoTokenService) Shutdown() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// CustomResponseWriter is a wrapper around the standard http.ResponseWriter
// that captures the status code and response body.
type CustomResponseWriter struct {
	http.ResponseWriter
	Body   *bytes.Buffer
	Status int
}

// Header returns the header map that will be sent by WriteHeader.
func (w *CustomResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// WriteHeader captures the status code and calls the underlying WriteHeader.
func (w *CustomResponseWriter) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the response body and calls the underlying Write.
func (w *CustomResponseWriter) Write(b []byte) (int, error) {
	w.Body.Write(b)
	return w.ResponseWriter.Write(b)
}

// getFunctionName returns the name of the function being executed
func getFunctionName(i interface{}) string {
	val := runtime.FuncForPC(reflect.ValueOf(i).Pointer())
	return val.Name()
}

func (t *ApitoTokenService) ApitoPublicFunctionRouteHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		hashFlag := ctx.Request().Header.Get("X-Fn-Hash")
		if hashFlag == "" {
			return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid function request"})
		}

		ctx.Set("function_hash", hashFlag)

		return next(ctx)
	}
}

func (t *ApitoTokenService) ApitoTokenHandler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {

		var token *string
		var err error
		var projectID string
		var userID string

		var req models.GraphQLIncomingRequest

		requestPath := ctx.Request().URL.Path

		useCookies := ctx.Request().Header.Get("X-Use-Cookies")
		apitoKey := ctx.Request().Header.Get("X-Apito-Key")
		syncKey := ctx.Request().Header.Get("X-Apito-Sync-Key")
		if apitoKey != "" || ((requestPath == "/secured/graphql" || requestPath == "/secured/graphql/v2") || strings.HasPrefix(requestPath, "/secured/rest/") || strings.HasPrefix(requestPath, "/secured/upload/file")) && useCookies == "" {
			var token *string
			if apitoKey != "" {
				token = &apitoKey
			} else {
				// api token and bearer token handler
				token, err = tokenFromBearer(ctx.Request())
				if err != nil {
					return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid auth header or auth header missing"})
				}
			}
			var verifiedToken *models.TokenClaims
			if strings.HasPrefix(*token, "ak_") {
				// for projec token
				verifiedToken, err = t.apiKeyManager.ValidateAndSetContext(ctx, *token)
				if err != nil {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			} else if strings.HasPrefix(*token, "cli-") || strings.HasPrefix(*token, "sdk-") || strings.HasPrefix(*token, "mcp-") {
				// Optimized sync-style token (same payload as X-Apito-Sync-Key path), including console MCP-prefixed tokens.
				verifiedToken, err = t.syncKeyManager.ValidateSyncTokenOptimized(ctx.Request().Context(), *token)
				if err != nil {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
				err = utility.SetTokenClaimsToRouter(ctx, verifiedToken)
				if err != nil {
					return err
				}
				ctx.Set("token", *token)
				ctx.Set("sync_token_claims", verifiedToken)
				if len(verifiedToken.ProjectIDs) > 0 {
					ctx.Set("project_ids", verifiedToken.ProjectIDs)
				}
				if len(verifiedToken.Scopes) > 0 {
					ctx.Set("scopes", verifiedToken.Scopes)
				}
			} else {
				// Legacy Branka (non-optimized) cli/sdk payloads and other bearer material
				verifiedToken, err = t.blankaService.ValidateAndSetContext(ctx, *token)
				if err != nil {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			}
			t.runPostTokenValidateHook(ctx, verifiedToken)
			projectID = getPrimaryProjectIDFromClaims(verifiedToken)
			userID = verifiedToken.UserID

		} else if useCookies == "false" || syncKey != "" {
			var verifiedToken *models.TokenClaims
			if strings.HasPrefix(syncKey, "cli-") || strings.HasPrefix(syncKey, "sdk-") || strings.HasPrefix(syncKey, "mcp-") {
				verifiedToken, err = t.syncKeyManager.ValidateSyncTokenOptimized(ctx.Request().Context(), syncKey)
				if err != nil {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			} else {
				token, err = tokenFromBearer(ctx.Request())
				if err != nil {
					return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid auth header or auth header missing"})
				}
				verifiedToken, err = t.authService.VerifyIDToken(ctx.Request().Context(), *token)
				if err != nil {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			}

			/* if t.cfg.ProjectInjectId != "" {
				tokenClaims.ProjectID = t.cfg.ProjectInjectId
			} */

			err = utility.SetTokenClaimsToRouter(ctx, verifiedToken)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

			t.runPostTokenValidateHook(ctx, verifiedToken)

			projectID = getPrimaryProjectIDFromClaims(verifiedToken)
			userID = verifiedToken.UserID

		} else {
			// apito console cookie token handler
			_ctx := ctx.Request().Context()
			tokens, err := tokenFromCookies(ctx.Request())
			if err != nil {
				return ctx.JSON(http.StatusUnauthorized, map[string]interface{}{"message": "invalid Cookie header. Reload the page"})
			}
			err = t.authService.VerifyAccessToken(_ctx, tokens.AccessToken)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

			tokenClaims, err := t.authService.VerifyIDToken(_ctx, tokens.IDToken)
			if err != nil {
				if errors.Is(err, ae.LOGIN_CONFICT) {
					// reset the http Only Cookies
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "userToken", "", true, true))
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "accessToken", "", true, true))
					http.SetCookie(ctx.Response(), utility.SetTokenCookie(t.cfg, "email", "", false, true))

					/*url := ctx.Request().URL
					url.Path = "/login"
					params := url.Query()
					params.Add("message", "logged in from another device or browser. Please login again.")
					url.RawQuery = params.Encode()
					return ctx.Redirect(http.StatusTemporaryRedirect, url.String())
					*/

					return ctx.JSON(http.StatusConflict, map[string]interface{}{"message": ae.LOGIN_CONFICT.Error()})
				} else {
					return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
				}
			}

			err = utility.SetTokenClaimsToRouter(ctx, tokenClaims)
			if err != nil {
				return ctx.JSON(http.StatusForbidden, map[string]interface{}{"message": ae.InvalidToken})
			}

			t.runPostTokenValidateHook(ctx, tokenClaims)

			projectID = getPrimaryProjectIDFromClaims(tokenClaims)
			userID = tokenClaims.UserID
		}

		// Check if the request is an upgrade to WebSocket
		if websocket.IsWebSocketUpgrade(ctx.Request()) {
			// Bypass the custom response writer for WebSocket connections
			// If you dont do this then the graphql websocket connection will fail
			return next(ctx)
		}

		// Read and store the request body
		var requestBody bytes.Buffer
		if ctx.Request().Body != nil {
			// Read the request body
			bodyBytes, err := io.ReadAll(ctx.Request().Body)
			if err != nil {
				return err
			}
			// Store the body for logging
			requestBody.Write(bodyBytes)
			// Restore the body for downstream handlers
			ctx.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Create a new CustomResponseWriter
		customWriter := &CustomResponseWriter{
			ResponseWriter: ctx.Response().Writer,
			Body:           new(bytes.Buffer),
		}

		// Replace the context's response writer with the custom one
		ctx.Response().Writer = customWriter

		auditLogs := models.AuditLogs{
			UserID:      userID,
			ProjectID:   projectID,
			RequestPath: requestPath,
		}
		// pass the request
		err = next(ctx)

		if requestPath == "/secured/graphql" || requestPath == "/secured/graphql/v2" || strings.HasPrefix(requestPath, "/secured/rest/") {
			// Send to batch channel with timeout to prevent blocking
			var respBytes int64
			if ctx.Response().Size > 0 {
				respBytes = ctx.Response().Size
			} else {
				// Fallback to Content-Length header if present
				cl := ctx.Response().Header().Get("Content-Length")
				if cl != "" {
					if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
						respBytes = n
					}
				}
				// As a last resort, use captured response body size (CustomResponseWriter)
				if respBytes == 0 {
					if crw, ok := ctx.Response().Writer.(*CustomResponseWriter); ok && crw != nil && crw.Body != nil {
						respBytes = int64(crw.Body.Len())
					}
				}
			}

			bandwidthMB := (float64(respBytes) / 1024.0) / 1024.0
			tracking := models.ProjectApiTracking{projectID: models.ApiTracking{Increment: 1, Bandwidth: bandwidthMB}}

			// Non-blocking send with timeout
			select {
			case t.Batch.Input <- tracking:
				// Successfully sent to batch
			case <-time.After(100 * time.Millisecond):
				// Log warning if batch is full/slow, but don't block the request
				fmt.Printf("⚠️ [API-TRACKING] Batch channel full, dropping tracking data for project %s\n", projectID)
			}
		} else if requestPath == "/system/graphql" {
			go func(ctx echo.Context) {

				if requestBody.Len() > 0 {
					// parse json data
					err = json.Unmarshal(requestBody.Bytes(), &req)
					if err != nil {
						auditLogs.InternalError = "GraphQL Request Body Unmarshal Error"
					}

					// only take the mutation request as actions
					if strings.HasPrefix(req.Query, "mutation") {

						// Capture response details after the handler has executed
						auditLogs.ResponseCode = customWriter.Status
						auditLogs.ResponsePayload = customWriter.Body.String()

						meta := ctx.Request().Context().Value("meta")
						if val, ok := meta.(map[string]interface{}); ok && val != nil {
							if _val, ok := val["function"].(string); ok && _val != "" {
								auditLogs.InternalFunction = _val
							}
							if _val, ok := val["activity"].(string); ok && _val != "" {
								auditLogs.Activity = _val
							}
						}

						// Log or process the response data and error
						auditLogs.RequestPayload = requestBody.String()
						if err != nil {
							ctx.Logger().Errorf("Handler Error: %v", err)
						}

						if req.Query != "" {
							vari, _ := json.MarshalIndent(req.Variables, "", " ")
							auditLogs.GraphqlPayload = req.Query
							auditLogs.GraphqlVariable = string(vari)
							auditLogs.GraphqlOperationName = req.OperationName
						}

						if utility.IsInActionNameMap(req.OperationName) {
							err = t.systemDB.SaveAuditLog(context.Background(), &auditLogs)
							if err != nil {
								fmt.Println(err.Error())
							}
						}
					}
				}
			}(ctx)
		} else {
			/*contentType := ctx.Request().Header.Get("Content-Type")
			switch contentType {
			case "application/json":
				// parse json data
				var postJSONBody interface{}
				err = json.Unmarshal(bodyBytes, &postJSONBody)
				if err != nil {
					fields["raw_json"] = string(bodyBytes)
					fields["error_message"] = "POST Request JSON Body Unmarshal Error"
				}
				fields["post_body_source"] = "json"
				fields["post_body"] = postJSONBody
			case "x-www-form-urlencoded":
				// parst x-www-form-urlencoded
				if err := ctx.Request().ParseForm(); err == nil && len(ctx.Request().PostForm) > 0 {
					fields["post_body_source"] = "x-www-form-urlencoded"
					fields["post_body"] = ctx.Request().PostForm
				}
			default:
				fields["post_body_source"] = contentType
				fields["post_body"] = string(bodyBytes)
			}*/
		}
		return err
	}
}

func tokenFromCookies(r *http.Request) (*models.JWTTokens, error) {
	if r.URL.RawQuery == "v=2" {
		tokenString := r.Header.Get("Authorization")
		tokenType := "Bearer"
		// Missing Token
		if tokenString == "" {
			return nil, errors.New("token is missing")
		}

		// Check for tempered token , check with signing method RSA
		if strings.HasPrefix(tokenString, tokenType) {
			tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, tokenType))
			if tokenString != "" {
				return &models.JWTTokens{AccessToken: tokenString}, nil
			} else {
				return nil, errors.New("invalid token Claims ! WTF ")
			}
		}
	}

	var tokens models.JWTTokens
	c, err := r.Cookie("accessToken")
	if err != nil {
		return nil, errors.New("no token")
	}
	tokens.AccessToken = c.Value

	c, err = r.Cookie("userToken")
	if err != nil {
		return nil, errors.New("no token")
	}
	tokens.IDToken = c.Value

	return &tokens, nil
}

func tokenFromBearer(r *http.Request) (*string, error) {
	token := r.Header.Get("Authorization")
	tokenType := "Bearer"
	// Missing Token
	if token == "" {
		return nil, errors.New("unauthorized Request. API Key is missing")
	}

	var tokenString string
	// Check for tempered token , check with signing method RSA
	if strings.HasPrefix(token, tokenType) {
		tokenString = strings.TrimSpace(strings.TrimPrefix(token, tokenType))
		if tokenString == "" {
			return nil, errors.New("invalid API Request")
		}
		return &tokenString, nil
	}
	return nil, errors.New("invalid token Type")
}
