package plugin

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// PaymentContractVersion is the envelope version plugins must declare.
const PaymentContractVersion = 1

var providerIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

var allowedPaymentIcons = map[string]struct{}{
	"wallet-cards":       {},
	"receipt-text":       {},
	"credit-card":        {},
	"banknote":           {},
	"circle-dollar-sign": {},
	"landmark":           {},
}

var allowedPaymentFlows = map[string]struct{}{
	"one_time":     {},
	"subscription": {},
}

// PaymentProviderContribution is the provider-neutral plugin declaration.
// The host never branches on a provider id; it dispatches declared operations.
type PaymentProviderContribution struct {
	ContractVersion int               `json:"contract_version" yaml:"contract_version"`
	ProviderID      string            `json:"provider_id" yaml:"provider_id"`
	Label           string            `json:"label" yaml:"label"`
	Icon            string            `json:"icon,omitempty" yaml:"icon,omitempty"`
	Flows           []string          `json:"flows,omitempty" yaml:"flows,omitempty"`
	Capabilities    []string          `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Operations      PaymentOperations `json:"operations" yaml:"operations"`
}

// PaymentOperations names plugin Execute functions. Initialize and Verify are required.
type PaymentOperations struct {
	Initialize string `json:"initialize" yaml:"initialize"`
	Verify     string `json:"verify" yaml:"verify"`
	Cancel     string `json:"cancel,omitempty" yaml:"cancel,omitempty"`
	Webhook    string `json:"webhook,omitempty" yaml:"webhook,omitempty"`
}

type paymentRegistration struct {
	PluginID string
	Payment  PaymentProviderContribution
}

var (
	paymentMu       sync.RWMutex
	paymentByID     = map[string]paymentRegistration{}
	paymentByPlugin = map[string]string{}
)

// ValidatePaymentContribution checks contract version, identity, flows, and operations.
func ValidatePaymentContribution(c *PaymentProviderContribution) error {
	if c == nil {
		return fmt.Errorf("payment contribution is nil")
	}
	if c.ContractVersion != PaymentContractVersion {
		return fmt.Errorf("payment contract_version %d is not supported (want %d)", c.ContractVersion, PaymentContractVersion)
	}
	id := strings.ToLower(strings.TrimSpace(c.ProviderID))
	if !providerIDPattern.MatchString(id) {
		return fmt.Errorf("payment provider_id %q is invalid", c.ProviderID)
	}
	if strings.TrimSpace(c.Label) == "" {
		return fmt.Errorf("payment label is required")
	}
	if strings.TrimSpace(c.Operations.Initialize) == "" || strings.TrimSpace(c.Operations.Verify) == "" {
		return fmt.Errorf("payment initialize and verify operations are required")
	}
	if len(c.Flows) == 0 {
		return fmt.Errorf("payment flows are required")
	}
	for _, flow := range c.Flows {
		if _, ok := allowedPaymentFlows[strings.ToLower(strings.TrimSpace(flow))]; !ok {
			return fmt.Errorf("payment flow %q is not supported", flow)
		}
	}
	if icon := strings.TrimSpace(c.Icon); icon != "" {
		if _, ok := allowedPaymentIcons[icon]; !ok {
			return fmt.Errorf("payment icon %q is not allowlisted", icon)
		}
	}
	return nil
}

// NormalizePaymentContribution lowercases identity fields and drops unknown icons.
func NormalizePaymentContribution(c *PaymentProviderContribution) *PaymentProviderContribution {
	if c == nil {
		return nil
	}
	out := *c
	out.ProviderID = strings.ToLower(strings.TrimSpace(c.ProviderID))
	out.Label = strings.TrimSpace(c.Label)
	out.Icon = strings.TrimSpace(c.Icon)
	if out.Icon != "" {
		if _, ok := allowedPaymentIcons[out.Icon]; !ok {
			out.Icon = ""
		}
	}
	out.Operations.Initialize = strings.TrimSpace(c.Operations.Initialize)
	out.Operations.Verify = strings.TrimSpace(c.Operations.Verify)
	out.Operations.Cancel = strings.TrimSpace(c.Operations.Cancel)
	out.Operations.Webhook = strings.TrimSpace(c.Operations.Webhook)
	flows := make([]string, 0, len(c.Flows))
	seen := map[string]struct{}{}
	for _, flow := range c.Flows {
		flow = strings.ToLower(strings.TrimSpace(flow))
		if flow == "" {
			continue
		}
		if _, ok := seen[flow]; ok {
			continue
		}
		seen[flow] = struct{}{}
		flows = append(flows, flow)
	}
	out.Flows = flows
	caps := make([]string, 0, len(c.Capabilities))
	for _, cap := range c.Capabilities {
		cap = strings.ToLower(strings.TrimSpace(cap))
		if cap != "" {
			caps = append(caps, cap)
		}
	}
	out.Capabilities = caps
	return &out
}

func clonePayment(c *PaymentProviderContribution) *PaymentProviderContribution {
	if c == nil {
		return nil
	}
	out := *c
	if c.Flows != nil {
		out.Flows = append([]string{}, c.Flows...)
	}
	if c.Capabilities != nil {
		out.Capabilities = append([]string{}, c.Capabilities...)
	}
	return &out
}

// RegisterPaymentProvider binds a plugin to one provider id. The same plugin may replace its own id.
func RegisterPaymentProvider(pluginID string, c *PaymentProviderContribution) error {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return fmt.Errorf("plugin id is required")
	}
	normalized := NormalizePaymentContribution(c)
	if err := ValidatePaymentContribution(normalized); err != nil {
		return err
	}
	paymentMu.Lock()
	defer paymentMu.Unlock()
	if prev, ok := paymentByPlugin[pluginID]; ok && prev != normalized.ProviderID {
		delete(paymentByID, prev)
	}
	if existing, ok := paymentByID[normalized.ProviderID]; ok && existing.PluginID != pluginID {
		return fmt.Errorf("payment provider %q is already registered by %s", normalized.ProviderID, existing.PluginID)
	}
	paymentByID[normalized.ProviderID] = paymentRegistration{PluginID: pluginID, Payment: *normalized}
	paymentByPlugin[pluginID] = normalized.ProviderID
	return nil
}

// UnregisterPaymentProvider removes the provider owned by pluginID.
func UnregisterPaymentProvider(pluginID string) {
	pluginID = strings.TrimSpace(pluginID)
	paymentMu.Lock()
	defer paymentMu.Unlock()
	if provider, ok := paymentByPlugin[pluginID]; ok {
		delete(paymentByID, provider)
		delete(paymentByPlugin, pluginID)
	}
}

// LookupPaymentProvider returns the plugin that owns providerID.
func LookupPaymentProvider(providerID string) (pluginID string, contrib PaymentProviderContribution, ok bool) {
	paymentMu.RLock()
	defer paymentMu.RUnlock()
	entry, found := paymentByID[strings.ToLower(strings.TrimSpace(providerID))]
	if !found {
		return "", PaymentProviderContribution{}, false
	}
	return entry.PluginID, entry.Payment, true
}

// ListPaymentProviders returns registered providers. Callers filter by project activation.
func ListPaymentProviders() []PaymentProviderContribution {
	paymentMu.RLock()
	defer paymentMu.RUnlock()
	out := make([]PaymentProviderContribution, 0, len(paymentByID))
	for _, entry := range paymentByID {
		out = append(out, entry.Payment)
	}
	return out
}

// PaymentPluginID returns the plugin id registered for a provider, if any.
func PaymentPluginID(providerID string) string {
	id, _, ok := LookupPaymentProvider(providerID)
	if !ok {
		return ""
	}
	return id
}
