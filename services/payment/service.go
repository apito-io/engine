package payment

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/utility"
)

// ExecFunc runs one declared plugin operation. reconcile=true skips project-activation refusal
// so an already created transaction can still be verified.
type ExecFunc func(ctx context.Context, pluginID, operation string, args map[string]interface{}, reconcile bool) (map[string]interface{}, error)

// SettledFunc runs after a durable status change. Hook failure must not roll back provider success;
// the caller retries fulfillment separately. A non-nil error is returned to the webhook so the
// provider can retry when the event id was not recorded yet.
type SettledFunc func(ctx context.Context, before, after *models.PaymentTransaction) error

var settledHook atomic.Value

// SetOnSettled registers the process-wide fulfillment hook. Pro uses this to
// apply tenant plans without the open-core package importing Pro.
func SetOnSettled(fn SettledFunc) {
	if fn == nil {
		settledHook.Store(SettledFunc(nil))
		return
	}
	settledHook.Store(fn)
}

func currentSettledHook() SettledFunc {
	fn, _ := settledHook.Load().(SettledFunc)
	return fn
}

// Service is the provider-neutral hosted checkout orchestrator.
type Service struct {
	Store        interfaces.PaymentStore
	Exec         ExecFunc
	PublicAPIURL string
	Now          func() time.Time
	OnSettled    SettledFunc
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// CheckoutInput starts a hosted payment. Amount is server-derived. Clients do not supply it.
type CheckoutInput struct {
	ProjectID      string
	UserID         string
	CustomerRef    string
	Provider       string
	Flow           string
	ProductKey     string
	ExpectedAmount string
	Currency       string
	IdempotencyKey string
	MetadataJSON   string
	// ClientPlatform is android, ios, or web. ReturnURL is copied from the
	// project client runtime by the caller. Neither value is sent to the plugin.
	ClientPlatform string
	ReturnURL      string
}

// BeginCheckout persists a row before the plugin talks to the provider.
func (s *Service) BeginCheckout(ctx context.Context, in CheckoutInput) (*models.PaymentTransaction, error) {
	if s == nil || s.Store == nil || s.Exec == nil {
		return nil, fmt.Errorf("payment service is not configured")
	}
	projectID := strings.TrimSpace(in.ProjectID)
	providerID := strings.ToLower(strings.TrimSpace(in.Provider))
	if projectID == "" || providerID == "" {
		return nil, fmt.Errorf("project_id and provider are required")
	}
	pluginID, contrib, ok := plugin.LookupPaymentProvider(providerID)
	if !ok {
		return nil, fmt.Errorf("payment provider is not registered")
	}
	flow := strings.ToLower(strings.TrimSpace(in.Flow))
	if flow == "" {
		flow = "one_time"
	}
	if !flowAllowed(contrib.Flows, flow) {
		return nil, fmt.Errorf("provider does not support flow %s", flow)
	}
	meta, err := mergeReturnMeta(in.MetadataJSON, in.ClientPlatform, in.ReturnURL)
	if err != nil {
		return nil, err
	}
	in.MetadataJSON = meta
	if key := strings.TrimSpace(in.IdempotencyKey); key != "" {
		existing, err := s.Store.FindPaymentTransactionByIdempotency(ctx, projectID, key)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	now := s.now().UTC().Format(time.RFC3339)
	ref, err := newClientReference(s.now())
	if err != nil {
		return nil, err
	}
	tx := &models.PaymentTransaction{
		ID:              utility.NewID(),
		ProjectID:       projectID,
		UserID:          strings.TrimSpace(in.UserID),
		CustomerRef:     strings.TrimSpace(in.CustomerRef),
		Provider:        providerID,
		Flow:            flow,
		ProductKey:      strings.TrimSpace(in.ProductKey),
		Status:          StatusCreated,
		Amount:          strings.TrimSpace(in.ExpectedAmount),
		Currency:        strings.ToUpper(strings.TrimSpace(in.Currency)),
		ClientReference: ref,
		IdempotencyKey:  strings.TrimSpace(in.IdempotencyKey),
		MetadataJSON:    in.MetadataJSON,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.Store.UpsertPaymentTransaction(ctx, tx); err != nil {
		return nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(s.PublicAPIURL), "/")
	if base == "" {
		return s.fail(ctx, tx, fmt.Errorf("payment callback host is missing"))
	}
	callback := func(outcome string) string {
		return base + "/system/v1/payments/projects/" + projectID + "/providers/" + providerID + "/callback/" + outcome + "?merchant_transaction_id=" + ref
	}
	payload, err := s.Exec(ctx, pluginID, contrib.Operations.Initialize, CheckoutRequest{
		TransactionID:   tx.ID,
		ClientReference: ref,
		Flow:            flow,
		ProductKey:      tx.ProductKey,
		ServerAmount:    tx.Amount,
		ServerCurrency:  tx.Currency,
		SuccessURL:      callback("success"),
		FailURL:         callback("fail"),
		CancelURL:       callback("cancel"),
		ProjectID:       projectID,
		CustomerRef:     tx.CustomerRef,
	}.Args(), false)
	if err != nil {
		return s.fail(ctx, tx, err)
	}
	init, err := ResultFromMap(payload, false)
	if err != nil {
		return s.fail(ctx, tx, err)
	}
	redirect := firstString(payload, "redirect_url", "RedirectURL", "url")
	if redirect == "" {
		return s.fail(ctx, tx, fmt.Errorf("provider did not return a redirect url"))
	}
	if init.Amount != "" {
		if !amountsMatch(tx.Amount, init.Amount) && tx.Amount != "" {
			return s.fail(ctx, tx, fmt.Errorf("provider amount does not match the server amount"))
		}
		if tx.Amount == "" {
			tx.Amount = init.Amount
		}
	}
	if init.Currency != "" {
		if tx.Currency != "" && !strings.EqualFold(tx.Currency, init.Currency) {
			return s.fail(ctx, tx, fmt.Errorf("provider currency does not match the server currency"))
		}
		if tx.Currency == "" {
			tx.Currency = strings.ToUpper(init.Currency)
		}
	}
	tx.ProviderSessionID = firstNonEmpty(init.ProviderSessionID, firstString(payload, "transaction_id"))
	tx.Status = StatusPending
	tx.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	tx.MetadataJSON = mergeRedirect(tx.MetadataJSON, redirect)
	if err := s.Store.UpsertPaymentTransaction(ctx, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// ApplyVerification settles one transaction from a plugin verify/webhook result.
func (s *Service) ApplyVerification(ctx context.Context, tx *models.PaymentTransaction, result NormalizedResult) (*models.PaymentTransaction, error) {
	if s == nil || s.Store == nil || tx == nil {
		return nil, fmt.Errorf("payment service is not configured")
	}
	if s.OnSettled == nil {
		s.OnSettled = currentSettledHook()
	}
	if result.EventID != "" {
		inserted, err := s.Store.InsertPaymentProviderEvent(ctx, &models.PaymentProviderEvent{
			ID:        tx.ProjectID + "|" + tx.Provider + "|" + result.EventID,
			ProjectID: tx.ProjectID,
			Provider:  tx.Provider,
			EventID:   result.EventID,
			CreatedAt: s.now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			return nil, err
		}
		if !inserted {
			current, err := s.Store.GetPaymentTransaction(ctx, tx.ProjectID, tx.ID)
			if err != nil || current == nil {
				return tx, err
			}
			return current, nil
		}
	}
	insertedEvent := result.EventID != ""
	before := *tx
	next := result.Status
	if next == "" {
		next = StatusPending
	}
	if !canTransition(tx.Status, next) {
		next = tx.Status
	}
	if result.Amount != "" && tx.Amount != "" && !amountsMatch(tx.Amount, result.Amount) {
		next = StatusFailed
		tx.ErrorMessage = "verified amount does not match the checkout amount"
	}
	if result.Currency != "" && tx.Currency != "" && !strings.EqualFold(tx.Currency, result.Currency) && next == StatusSucceeded {
		next = StatusFailed
		tx.ErrorMessage = "verified currency does not match the checkout currency"
	}
	tx.Status = next
	if result.ProviderStatus != "" {
		tx.ProviderStatus = result.ProviderStatus
	}
	if result.Amount != "" {
		tx.VerifiedAmount = result.Amount
	}
	if result.Currency != "" {
		tx.VerifiedCurrency = strings.ToUpper(result.Currency)
	}
	if result.ProviderSessionID != "" && tx.ProviderSessionID == "" {
		tx.ProviderSessionID = result.ProviderSessionID
	}
	if result.ProviderPaymentID != "" {
		tx.ProviderPaymentID = result.ProviderPaymentID
	}
	if result.ProviderSubscriptionID != "" {
		tx.ProviderSubscriptionID = result.ProviderSubscriptionID
	}
	if result.FinancialEntity != "" {
		tx.FinancialEntity = result.FinancialEntity
	}
	if result.TransactionDate != "" {
		tx.ProviderTransactionDate = result.TransactionDate
	}
	if result.ErrorCode != "" {
		tx.ErrorCode = result.ErrorCode
	}
	if result.ErrorMessage != "" && tx.ErrorMessage == "" {
		tx.ErrorMessage = result.ErrorMessage
	}
	tx.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if tx.Status == StatusSucceeded || tx.Status == StatusFailed || tx.Status == StatusCanceled || tx.Status == StatusExpired {
		if tx.CompletedAt == "" {
			tx.CompletedAt = tx.UpdatedAt
		}
	}
	if err := s.Store.UpsertPaymentTransaction(ctx, tx); err != nil {
		return nil, err
	}
	if err := s.upsertSubscription(ctx, tx, result); err != nil {
		return nil, err
	}
	if s.OnSettled != nil && (before.Status != tx.Status || (insertedEvent && tx.Status == StatusSucceeded)) {
		if err := s.OnSettled(ctx, &before, tx); err != nil {
			if result.EventID != "" {
				_ = s.Store.DeletePaymentProviderEvent(ctx, tx.ProjectID, tx.Provider, result.EventID)
			}
			return tx, err
		}
	}
	return tx, nil
}

// Verify asks the plugin for the authoritative status of an existing transaction.
func (s *Service) Verify(ctx context.Context, tx *models.PaymentTransaction, raw map[string]interface{}) (*models.PaymentTransaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	pluginID, contrib, ok := plugin.LookupPaymentProvider(tx.Provider)
	if !ok {
		return nil, fmt.Errorf("payment provider is unavailable")
	}
	op := contrib.Operations.Verify
	if raw != nil && contrib.Operations.Webhook != "" {
		if _, webhook := raw["_raw_body"]; webhook {
			op = contrib.Operations.Webhook
		}
	}
	args := map[string]interface{}{
		"transaction_id":          tx.ID,
		"merchant_transaction_id": tx.ClientReference,
		"client_reference":        tx.ClientReference,
		"provider_session_id":     tx.ProviderSessionID,
		"project_id":              tx.ProjectID,
	}
	for k, v := range raw {
		args[k] = v
	}
	payload, err := s.Exec(ctx, pluginID, op, args, true)
	if err != nil {
		return nil, err
	}
	result, err := ResultFromMap(payload, true)
	if err != nil {
		return nil, err
	}
	if result.EventID == "" && (result.Status == StatusSucceeded || result.Status == StatusFailed || result.Status == StatusCanceled || result.Status == StatusExpired) {
		result.EventID = "status:" + tx.ClientReference + ":" + result.Status
	}
	return s.ApplyVerification(ctx, tx, result)
}

// IngestProviderEvent verifies a provider webhook that does not carry the merchant reference in the URL.
func (s *Service) IngestProviderEvent(ctx context.Context, projectID, providerID string, raw map[string]interface{}) (*models.PaymentTransaction, error) {
	if s == nil || s.Store == nil || s.Exec == nil {
		return nil, fmt.Errorf("payment service is not configured")
	}
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	pluginID, contrib, ok := plugin.LookupPaymentProvider(providerID)
	if !ok {
		return nil, fmt.Errorf("payment provider is unavailable")
	}
	op := contrib.Operations.Webhook
	if op == "" {
		op = contrib.Operations.Verify
	}
	payload, err := s.Exec(ctx, pluginID, op, raw, true)
	if err != nil {
		return nil, err
	}
	result, err := ResultFromMap(payload, true)
	if err != nil {
		return nil, err
	}
	var tx *models.PaymentTransaction
	if result.TransactionID != "" {
		tx, err = s.Store.GetPaymentTransaction(ctx, projectID, result.TransactionID)
	} else if result.ClientReference != "" {
		tx, err = s.Store.FindPaymentTransactionByClientRef(ctx, projectID, providerID, result.ClientReference)
	}
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction not found")
	}
	if result.EventID == "" && (result.Status == StatusSucceeded || result.Status == StatusFailed || result.Status == StatusCanceled || result.Status == StatusExpired) {
		result.EventID = "status:" + tx.ClientReference + ":" + result.Status
	}
	return s.ApplyVerification(ctx, tx, result)
}

func (s *Service) upsertSubscription(ctx context.Context, tx *models.PaymentTransaction, result NormalizedResult) error {
	ref := firstNonEmpty(result.ProviderSubscriptionID, tx.ProviderSubscriptionID)
	if ref == "" || tx.Flow != "subscription" {
		return nil
	}
	existing, err := s.Store.FindPaymentSubscriptionByProviderRef(ctx, tx.ProjectID, tx.Provider, ref)
	if err != nil {
		return err
	}
	now := s.now().UTC().Format(time.RFC3339)
	row := existing
	if row == nil {
		row = &models.PaymentSubscription{
			ID:        utility.NewID(),
			CreatedAt: now,
		}
	}
	row.ProjectID = tx.ProjectID
	row.UserID = tx.UserID
	row.CustomerRef = tx.CustomerRef
	row.Provider = tx.Provider
	row.ProviderSubscriptionID = ref
	row.ProviderCustomerID = result.ProviderCustomerID
	row.ProductKey = tx.ProductKey
	status := result.SubscriptionStatus
	if status == "" {
		status = tx.Status
	}
	row.Status = status
	if result.CurrentPeriodStart != "" {
		row.CurrentPeriodStart = result.CurrentPeriodStart
	}
	if result.CurrentPeriodEnd != "" {
		row.CurrentPeriodEnd = result.CurrentPeriodEnd
	}
	row.CancelAtPeriodEnd = result.CancelAtPeriodEnd
	row.UpdatedAt = now
	return s.Store.UpsertPaymentSubscription(ctx, row)
}

func (s *Service) fail(ctx context.Context, tx *models.PaymentTransaction, cause error) (*models.PaymentTransaction, error) {
	tx.Status = StatusFailed
	tx.ErrorMessage = cause.Error()
	tx.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	tx.CompletedAt = tx.UpdatedAt
	_ = s.Store.UpsertPaymentTransaction(ctx, tx)
	return tx, cause
}

func flowAllowed(flows []string, flow string) bool {
	for _, item := range flows {
		if strings.EqualFold(item, flow) {
			return true
		}
	}
	return false
}

func canTransition(from, to string) bool {
	if from == to {
		return true
	}
	if from == StatusSucceeded {
		return false
	}
	switch from {
	case StatusCreated:
		return to == StatusPending || to == StatusFailed || to == StatusCanceled
	case StatusPending, "":
		return to == StatusSucceeded || to == StatusFailed || to == StatusCanceled || to == StatusExpired || to == StatusPending
	case StatusFailed, StatusCanceled, StatusExpired:
		return false
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func mergeRedirect(metadata, redirect string) string {
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return metadata
	}
	if strings.TrimSpace(metadata) == "" {
		return `{"redirect_url":` + jsonString(redirect) + `}`
	}
	trimmed := strings.TrimSpace(metadata)
	if strings.HasSuffix(trimmed, "}") {
		return strings.TrimSuffix(trimmed, "}") + `,"redirect_url":` + jsonString(redirect) + `}`
	}
	return metadata
}

func jsonString(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func newClientReference(now time.Time) (string, error) {
	var buf [3]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	return now.UTC().Format("20060102150405") + fmt.Sprintf("%03d%06d", now.UTC().Nanosecond()/1e6, n%1000000), nil
}

// RedirectURL reads the redirect saved on the transaction metadata.
func RedirectURL(tx *models.PaymentTransaction) string {
	if tx == nil || strings.TrimSpace(tx.MetadataJSON) == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(tx.MetadataJSON), &payload); err != nil {
		return ""
	}
	return firstString(payload, "redirect_url")
}
