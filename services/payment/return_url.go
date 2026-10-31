package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/apito-io/engine/models"
)

// PaymentReturnExtraKey is the client_runtime extra that holds the app return URL.
const PaymentReturnExtraKey = "payment_return_url"

// CallbackBase is the origin EPS and Stripe send the browser back to.
// A configured PUBLIC_API_URL wins. Otherwise the origin is the API host
// the client already called.
func CallbackBase(configured string, r *http.Request) string {
	if base := strings.TrimRight(strings.TrimSpace(configured), "/"); base != "" {
		return base
	}
	return requestOrigin(r)
}

func requestOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	host := strings.TrimSpace(r.Host)
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

var paymentReturnScheme = regexp.MustCompile(`^[a-z][a-z0-9+.-]{0,31}$`)

// mergeReturnMeta stores the app return link on the ledger. An empty URL
// leaves metadata unchanged. The plugin never receives this object.
func mergeReturnMeta(metadata, platform, returnURL string) (string, error) {
	returnURL = strings.TrimSpace(returnURL)
	if returnURL == "" {
		return metadata, nil
	}
	name, ok := models.NormalizeClientRuntimePlatform(platform)
	if !ok {
		return "", fmt.Errorf("client_platform must be android, ios, or web")
	}
	if !validPaymentReturnURL(returnURL) {
		return "", fmt.Errorf("payment return url must be https or an app scheme")
	}
	payload := map[string]interface{}{}
	if strings.TrimSpace(metadata) != "" {
		if err := json.Unmarshal([]byte(metadata), &payload); err != nil {
			return "", fmt.Errorf("metadata_json is not an object")
		}
	}
	payload["client_platform"] = name
	payload["return_url"] = returnURL
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func validPaymentReturnURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") || strings.Contains(raw, "\\") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.User != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https":
		return parsed.Host != "" && parsed.Opaque == ""
	case "http", "javascript", "data", "file", "vbscript", "about", "blob":
		return false
	default:
		if !paymentReturnScheme.MatchString(scheme) {
			return false
		}
		return parsed.Host != "" || parsed.Path != "" || parsed.Opaque != ""
	}
}

func metadataString(tx *models.PaymentTransaction, key string) string {
	if tx == nil || strings.TrimSpace(tx.MetadataJSON) == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(tx.MetadataJSON), &payload); err != nil {
		return ""
	}
	return firstString(payload, key)
}

// ReturnLink is the stored app URL with transaction_id and status appended.
// A missing or rejected URL returns empty. The result page omits the button.
func ReturnLink(tx *models.PaymentTransaction) string {
	if tx == nil {
		return ""
	}
	raw := metadataString(tx, "return_url")
	if !validPaymentReturnURL(raw) {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set("transaction_id", tx.ID)
	query.Set("status", tx.Status)
	parsed.RawQuery = query.Encode()
	if !validPaymentReturnURL(parsed.String()) {
		return ""
	}
	return parsed.String()
}
