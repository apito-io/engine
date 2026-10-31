package payment

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	StatusCreated   = "created"
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
	StatusExpired   = "expired"
)

// NormalizedResult is the provider-neutral verify/webhook envelope.
type NormalizedResult struct {
	Status                 string
	ProviderStatus         string
	Amount                 string
	Currency               string
	ProviderSessionID      string
	ProviderPaymentID      string
	ProviderSubscriptionID string
	ProviderCustomerID     string
	SubscriptionStatus     string
	CurrentPeriodStart     string
	CurrentPeriodEnd       string
	CancelAtPeriodEnd      bool
	EventID                string
	FinancialEntity        string
	TransactionDate        string
	ErrorCode              string
	ErrorMessage           string
	ClientReference        string
	TransactionID          string
}

// CheckoutRequest is what the host sends to a plugin initialize operation.
type CheckoutRequest struct {
	TransactionID   string
	ClientReference string
	Flow            string
	ProductKey      string
	ServerAmount    string
	ServerCurrency  string
	SuccessURL      string
	FailURL         string
	CancelURL       string
	ProjectID       string
	CustomerRef     string
}

func (r CheckoutRequest) Args() map[string]interface{} {
	return map[string]interface{}{
		"transaction_id":          r.TransactionID,
		"client_reference":        r.ClientReference,
		"flow":                    r.Flow,
		"product_key":             r.ProductKey,
		"server_amount":           r.ServerAmount,
		"server_currency":         r.ServerCurrency,
		"success_url":             r.SuccessURL,
		"fail_url":                r.FailURL,
		"cancel_url":              r.CancelURL,
		"project_id":              r.ProjectID,
		"customer_ref":            r.CustomerRef,
		"merchant_transaction_id": r.ClientReference,
		"total_amount":            r.ServerAmount,
		"currency":                r.ServerCurrency,
	}
}

// NormalizeStatus maps provider words onto the ledger state machine.
func NormalizeStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "succeeded", "success", "paid", "complete", "completed", "active":
		return StatusSucceeded
	case "failed", "fail", "failure", "error":
		return StatusFailed
	case "canceled", "cancelled", "cancel":
		return StatusCanceled
	case "expired", "expire":
		return StatusExpired
	case "pending", "processing", "unpaid", "created", "open":
		return StatusPending
	default:
		return ""
	}
}

// ParsePluginPayload accepts a plugin Execute result and returns a map.
func ParsePluginPayload(result interface{}) (map[string]interface{}, error) {
	switch v := result.(type) {
	case map[string]interface{}:
		if raw, ok := v["data"].(string); ok && strings.HasPrefix(strings.TrimSpace(raw), "{") {
			return ParsePluginPayload(raw)
		}
		if msg, ok := v["error"].(string); ok && msg != "" && v["redirect_url"] == nil && v["status"] == nil {
			return nil, fmt.Errorf("%s", msg)
		}
		return v, nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil, fmt.Errorf("empty plugin result")
		}
		if strings.HasPrefix(text, "❌") {
			return nil, fmt.Errorf("%s", text)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return nil, fmt.Errorf("plugin result is not json: %w", err)
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported plugin result %T", result)
	}
}

// ResultFromMap reads a normalized envelope. Unknown status is an error when required.
func ResultFromMap(payload map[string]interface{}, requireStatus bool) (NormalizedResult, error) {
	if payload == nil {
		return NormalizedResult{}, fmt.Errorf("empty verification")
	}
	out := NormalizedResult{
		Status:                 NormalizeStatus(firstString(payload, "status", "Status")),
		ProviderStatus:         firstString(payload, "provider_status", "Status", "status"),
		Amount:                 firstString(payload, "amount", "verified_amount", "TotalAmount", "total_amount"),
		Currency:               strings.ToUpper(firstString(payload, "currency", "Currency")),
		ProviderSessionID:      firstString(payload, "provider_session_id", "eps_transaction_id", "session_id"),
		ProviderPaymentID:      firstString(payload, "provider_payment_id", "EPSTransactionId", "eps_payment_id"),
		ProviderSubscriptionID: firstString(payload, "provider_subscription_id", "subscription_id"),
		ProviderCustomerID:     firstString(payload, "provider_customer_id", "customer_id"),
		SubscriptionStatus:     NormalizeStatus(firstString(payload, "subscription_status")),
		CurrentPeriodStart:     firstString(payload, "current_period_start"),
		CurrentPeriodEnd:       firstString(payload, "current_period_end"),
		EventID:                firstString(payload, "event_id", "id"),
		FinancialEntity:        firstString(payload, "financial_entity", "FinancialEntity"),
		TransactionDate:        firstString(payload, "transaction_date", "TransactionDate"),
		ErrorCode:              firstString(payload, "error_code", "ErrorCode"),
		ErrorMessage:           firstString(payload, "error_message", "ErrorMessage"),
		ClientReference:        firstString(payload, "client_reference", "merchant_transaction_id", "MerchantTransactionId", "client_reference_id"),
		TransactionID:          firstString(payload, "transaction_id", "apito_transaction_id"),
	}
	if raw, ok := payload["cancel_at_period_end"].(bool); ok {
		out.CancelAtPeriodEnd = raw
	}
	if out.ProviderPaymentID == "" {
		out.ProviderPaymentID = firstString(payload, "eps_transaction_id")
	}
	if requireStatus && out.Status == "" {
		return out, fmt.Errorf("plugin did not return a status")
	}
	return out, nil
}

func firstString(payload map[string]interface{}, keys ...string) string {
	if payload == nil {
		return ""
	}
	folded := map[string]interface{}{}
	for k, v := range payload {
		folded[strings.ToLower(strings.TrimSpace(k))] = v
	}
	for _, key := range keys {
		v, ok := folded[strings.ToLower(strings.TrimSpace(key))]
		if !ok || v == nil {
			continue
		}
		switch n := v.(type) {
		case string:
			if s := strings.TrimSpace(n); s != "" && s != "<nil>" {
				return s
			}
		case float64:
			if math.IsNaN(n) || math.IsInf(n, 0) {
				continue
			}
			return strconv.FormatFloat(n, 'f', -1, 64)
		case json.Number:
			return n.String()
		case bool:
			continue
		default:
			s := strings.TrimSpace(fmt.Sprint(n))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func amountsMatch(expected, got string) bool {
	expected = strings.TrimSpace(expected)
	got = strings.TrimSpace(got)
	if expected == "" || got == "" {
		return true
	}
	a, errA := strconv.ParseFloat(expected, 64)
	b, errB := strconv.ParseFloat(got, 64)
	if errA != nil || errB != nil {
		return expected == got
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.001
}
