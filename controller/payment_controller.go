package controller

import (
	"io"
	"net/http"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/services/payment"
	"github.com/apito-io/engine/services/plugin"
	"github.com/labstack/echo/v4"
)

// PaymentController exposes the provider-neutral OSS payment routes.
type PaymentController struct {
	Server *resolver.GraphQLServer
}

func NewPaymentController(server *resolver.GraphQLServer) *PaymentController {
	return &PaymentController{Server: server}
}

func (pc *PaymentController) service() (*payment.Service, interfaces.PaymentStore, error) {
	if pc == nil || pc.Server == nil || pc.Server.SystemDriver == nil {
		return nil, nil, echo.NewHTTPError(http.StatusServiceUnavailable, "payment ledger is not ready")
	}
	store, ok := pc.Server.SystemDriver.(interfaces.PaymentStore)
	if !ok {
		return nil, nil, echo.NewHTTPError(http.StatusServiceUnavailable, "this system driver does not support the payment ledger")
	}
	configured := ""
	if pc.Server.Cfg != nil {
		configured = pc.Server.Cfg.PublicAPIURL
	}
	svc := &payment.Service{
		Store:        store,
		PublicAPIURL: configured,
		Exec:         pc.Server.DispatchPayment,
	}
	return svc, store, nil
}

func (pc *PaymentController) Checkout(c echo.Context) error {
	svc, _, err := pc.service()
	if err != nil {
		return err
	}
	projectID := c.Param("projectId")
	if err := pc.requireProject(c, projectID); err != nil {
		return err
	}
	var body struct {
		Flow           string `json:"flow"`
		ProductKey     string `json:"product_key"`
		CustomerRef    string `json:"customer_ref"`
		IdempotencyKey string `json:"idempotency_key"`
		Amount         string `json:"amount"`
		Currency       string `json:"currency"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
	}
	if strings.TrimSpace(body.Amount) != "" || strings.TrimSpace(body.Currency) != "" {
		return echo.NewHTTPError(http.StatusBadRequest, "amount and currency are server-derived")
	}
	svc.PublicAPIURL = payment.CallbackBase(svc.PublicAPIURL, c.Request())
	tx, err := svc.BeginCheckout(c.Request().Context(), payment.CheckoutInput{
		ProjectID:      projectID,
		UserID:         echoString(c, "user"),
		CustomerRef:    body.CustomerRef,
		Provider:       c.Param("provider"),
		Flow:           body.Flow,
		ProductKey:     body.ProductKey,
		IdempotencyKey: firstStringValue(body.IdempotencyKey, c.Request().Header.Get("Idempotency-Key")),
	})
	if err != nil {
		status := http.StatusBadGateway
		if tx == nil {
			status = http.StatusBadRequest
		}
		return c.JSON(status, map[string]interface{}{"error": err.Error(), "transaction": publicTransaction(tx)})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"transaction":  publicTransaction(tx),
		"redirect_url": payment.RedirectURL(tx),
	})
}

func (pc *PaymentController) Callback(c echo.Context) error {
	return pc.dispatchCallback(c, true)
}

func (pc *PaymentController) finishCallback(c echo.Context, tx *models.PaymentTransaction, raw map[string]interface{}, allowHTML bool) error {
	svc, _, err := pc.service()
	if err != nil {
		return err
	}
	updated, verr := svc.Verify(c.Request().Context(), tx, raw)
	code := http.StatusOK
	if verr != nil && (updated == nil || updated.Status != payment.StatusSucceeded) {
		code = http.StatusBadGateway
	}
	if allowHTML && payment.WantsResultHTML(c.Request().Header.Get("Accept")) {
		return c.HTML(code, payment.RenderResultPage(updated))
	}
	return c.JSON(code, map[string]interface{}{
		"transaction": publicTransaction(updated),
		"error":       errorString(verr),
	})
}

func (pc *PaymentController) dispatchCallback(c echo.Context, allowHTML bool) error {
	_, store, err := pc.service()
	if err != nil {
		return err
	}
	projectID := c.Param("projectId")
	provider := strings.ToLower(strings.TrimSpace(c.Param("provider")))
	ref := merchantRef(c)
	if ref == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "merchant_transaction_id required")
	}
	tx, err := store.FindPaymentTransactionByClientRef(c.Request().Context(), projectID, provider, ref)
	if err != nil || tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, "transaction not found")
	}
	return pc.finishCallback(c, tx, callbackRaw(c), allowHTML)
}

func callbackRaw(c echo.Context) map[string]interface{} {
	raw := map[string]interface{}{}
	body, _ := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if len(body) > 0 {
		raw["_raw_body"] = string(body)
	}
	headers := map[string]interface{}{}
	for k, v := range c.Request().Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	raw["_headers"] = headers
	return raw
}

func (pc *PaymentController) Webhook(c echo.Context) error {
	if merchantRef(c) != "" {
		return pc.dispatchCallback(c, false)
	}
	svc, _, err := pc.service()
	if err != nil {
		return err
	}
	raw := map[string]interface{}{}
	body, _ := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if len(body) > 0 {
		raw["_raw_body"] = string(body)
		raw["raw_body"] = string(body)
	}
	headers := map[string]interface{}{}
	for k, v := range c.Request().Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	raw["_headers"] = headers
	updated, verr := svc.IngestProviderEvent(c.Request().Context(), c.Param("projectId"), c.Param("provider"), raw)
	code := http.StatusOK
	if verr != nil && (updated == nil || updated.Status != payment.StatusSucceeded) {
		code = http.StatusBadGateway
	}
	return c.JSON(code, map[string]interface{}{
		"transaction": publicTransaction(updated),
		"error":       errorString(verr),
	})
}

func (pc *PaymentController) GetTransaction(c echo.Context) error {
	_, store, err := pc.service()
	if err != nil {
		return err
	}
	projectID := c.Param("projectId")
	if err := pc.requireProject(c, projectID); err != nil {
		return err
	}
	tx, err := store.GetPaymentTransaction(c.Request().Context(), projectID, c.Param("transactionId"))
	if err != nil || tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, "transaction not found")
	}
	if !pc.canRead(c, tx) {
		return echo.NewHTTPError(http.StatusForbidden, "transaction access denied")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"transaction": publicTransaction(tx)})
}

func (pc *PaymentController) Reconcile(c echo.Context) error {
	svc, store, err := pc.service()
	if err != nil {
		return err
	}
	projectID := c.Param("projectId")
	if err := pc.requireProject(c, projectID); err != nil {
		return err
	}
	tx, err := store.GetPaymentTransaction(c.Request().Context(), projectID, c.Param("transactionId"))
	if err != nil || tx == nil {
		return echo.NewHTTPError(http.StatusNotFound, "transaction not found")
	}
	if !pc.canRead(c, tx) {
		return echo.NewHTTPError(http.StatusForbidden, "transaction access denied")
	}
	updated, verr := svc.Verify(c.Request().Context(), tx, nil)
	if verr != nil {
		return c.JSON(http.StatusBadGateway, map[string]interface{}{"error": verr.Error(), "transaction": publicTransaction(updated)})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"transaction": publicTransaction(updated)})
}

func (pc *PaymentController) List(c echo.Context) error {
	_, store, err := pc.service()
	if err != nil {
		return err
	}
	projectID := c.Param("projectId")
	if err := pc.requireProject(c, projectID); err != nil {
		return err
	}
	filter := interfaces.PaymentListFilter{
		ProjectID: projectID,
		Provider:  c.QueryParam("provider"),
		Status:    c.QueryParam("status"),
		Query:     c.QueryParam("q"),
		Limit:     50,
	}
	if !pc.isOperator(c) {
		filter.UserID = echoString(c, "user")
	}
	rows, err := store.ListPaymentTransactions(c.Request().Context(), filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "list failed")
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		out = append(out, publicTransaction(row))
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"transactions": out})
}

func (pc *PaymentController) Providers(c echo.Context) error {
	projectID := c.Param("projectId")
	if err := pc.requireProject(c, projectID); err != nil {
		return err
	}
	project, err := pc.Server.SystemDriver.GetProject(c.Request().Context(), projectID)
	if err != nil || project == nil {
		return echo.NewHTTPError(http.StatusNotFound, "project not found")
	}
	out := make([]map[string]interface{}, 0)
	for _, item := range plugin.ListPaymentProviders() {
		pluginID := plugin.PaymentPluginID(item.ProviderID)
		if resolver.ProjectPluginActivated(project, pluginID) {
			out = append(out, map[string]interface{}{
				"provider_id": item.ProviderID,
				"label":       item.Label,
				"icon":        item.Icon,
				"flows":       item.Flows,
				"plugin_id":   pluginID,
			})
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"providers": out})
}

func (pc *PaymentController) requireProject(c echo.Context, projectID string) error {
	tokenProject := echoString(c, "project")
	if tokenProject != "" && tokenProject != projectID {
		return echo.NewHTTPError(http.StatusForbidden, "project access denied")
	}
	return nil
}

func (pc *PaymentController) canRead(c echo.Context, tx *models.PaymentTransaction) bool {
	if tx == nil {
		return false
	}
	if pc.isOperator(c) {
		return true
	}
	userID := echoString(c, "user")
	return userID != "" && userID == tx.UserID
}

func (pc *PaymentController) isOperator(c echo.Context) bool {
	switch strings.ToLower(echoString(c, "role")) {
	case "admin", "owner", "super_admin":
		return true
	default:
		return false
	}
}

func publicTransaction(tx *models.PaymentTransaction) map[string]interface{} {
	if tx == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                       tx.ID,
		"project_id":               tx.ProjectID,
		"user_id":                  tx.UserID,
		"customer_ref":             tx.CustomerRef,
		"provider":                 tx.Provider,
		"flow":                     tx.Flow,
		"product_key":              tx.ProductKey,
		"status":                   tx.Status,
		"provider_status":          tx.ProviderStatus,
		"amount":                   tx.Amount,
		"currency":                 tx.Currency,
		"verified_amount":          tx.VerifiedAmount,
		"verified_currency":        tx.VerifiedCurrency,
		"client_reference":         tx.ClientReference,
		"provider_session_id":      tx.ProviderSessionID,
		"provider_payment_id":      tx.ProviderPaymentID,
		"provider_subscription_id": tx.ProviderSubscriptionID,
		"financial_entity":         tx.FinancialEntity,
		"error_code":               tx.ErrorCode,
		"error_message":            tx.ErrorMessage,
		"created_at":               tx.CreatedAt,
		"updated_at":               tx.UpdatedAt,
		"completed_at":             tx.CompletedAt,
		"redirect_url":             payment.RedirectURL(tx),
	}
}

func echoString(c echo.Context, key string) string {
	v, _ := c.Get(key).(string)
	return strings.TrimSpace(v)
}

func merchantRef(c echo.Context) string {
	names := []string{"merchant_transaction_id", "MerchantTransactionId", "merchantTransactionId"}
	for key, values := range c.QueryParams() {
		trimmed := strings.TrimSpace(key)
		for _, name := range names {
			if strings.EqualFold(trimmed, name) && len(values) > 0 {
				return strings.TrimSpace(values[0])
			}
		}
	}
	return strings.TrimSpace(c.FormValue("merchantTransactionId"))
}

func firstStringValue(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func errorString(err error) interface{} {
	if err == nil {
		return nil
	}
	return err.Error()
}
