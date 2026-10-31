package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services/plugin"
)

type memStore struct {
	mu     sync.Mutex
	tx     map[string]*models.PaymentTransaction
	subs   map[string]*models.PaymentSubscription
	events map[string]struct{}
}

func newMemStore() *memStore {
	return &memStore{
		tx:     map[string]*models.PaymentTransaction{},
		subs:   map[string]*models.PaymentSubscription{},
		events: map[string]struct{}{},
	}
}

func (m *memStore) UpsertPaymentTransaction(ctx context.Context, row *models.PaymentTransaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *row
	m.tx[row.ID] = &cp
	return nil
}
func (m *memStore) GetPaymentTransaction(ctx context.Context, projectID, id string) (*models.PaymentTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row := m.tx[id]
	if row == nil || row.ProjectID != projectID {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}
func (m *memStore) FindPaymentTransactionByClientRef(ctx context.Context, projectID, provider, clientRef string) (*models.PaymentTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.tx {
		if row.ProjectID == projectID && row.Provider == provider && row.ClientReference == clientRef {
			cp := *row
			return &cp, nil
		}
	}
	return nil, nil
}
func (m *memStore) FindPaymentTransactionByIdempotency(ctx context.Context, projectID, key string) (*models.PaymentTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.tx {
		if row.ProjectID == projectID && row.IdempotencyKey == key {
			cp := *row
			return &cp, nil
		}
	}
	return nil, nil
}
func (m *memStore) ListPaymentTransactions(ctx context.Context, filter interfaces.PaymentListFilter) ([]*models.PaymentTransaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*models.PaymentTransaction
	for _, row := range m.tx {
		if row.ProjectID == filter.ProjectID {
			cp := *row
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *memStore) UpsertPaymentSubscription(ctx context.Context, row *models.PaymentSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *row
	m.subs[row.ID] = &cp
	return nil
}
func (m *memStore) GetPaymentSubscription(ctx context.Context, projectID, id string) (*models.PaymentSubscription, error) {
	return nil, nil
}
func (m *memStore) FindPaymentSubscriptionByProviderRef(ctx context.Context, projectID, provider, providerSubscriptionID string) (*models.PaymentSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.subs {
		if row.ProjectID == projectID && row.Provider == provider && row.ProviderSubscriptionID == providerSubscriptionID {
			cp := *row
			return &cp, nil
		}
	}
	return nil, nil
}
func (m *memStore) InsertPaymentProviderEvent(ctx context.Context, row *models.PaymentProviderEvent) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.events[row.ID]; ok {
		return false, nil
	}
	m.events[row.ID] = struct{}{}
	return true, nil
}
func (m *memStore) DeletePaymentProviderEvent(ctx context.Context, projectID, provider, eventID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.events, projectID+"|"+provider+"|"+eventID)
	return nil
}

func TestCheckoutThenVerifyIsIdempotent(t *testing.T) {
	const pluginID = "hc-test-hosted"
	t.Cleanup(func() { plugin.UnregisterPaymentProvider(pluginID) })
	if err := plugin.RegisterPaymentProvider(pluginID, &plugin.PaymentProviderContribution{
		ContractVersion: plugin.PaymentContractVersion,
		ProviderID:      "hosted",
		Label:           "Hosted",
		Icon:            "wallet-cards",
		Flows:           []string{"one_time"},
		Operations:      plugin.PaymentOperations{Initialize: "init", Verify: "verify"},
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	svc := &Service{
		Store:        store,
		PublicAPIURL: "https://api.example.test",
		Now:          func() time.Time { return time.Date(2026, 10, 29, 7, 9, 46, 0, time.UTC) },
		Exec: func(ctx context.Context, pluginID, operation string, args map[string]interface{}, reconcile bool) (map[string]interface{}, error) {
			if operation == "init" {
				if args["server_amount"] != "1000.00" {
					t.Fatalf("amount leaked or missing: %#v", args["server_amount"])
				}
				return map[string]interface{}{"redirect_url": "https://pay.example/redirect", "provider_session_id": "init-uuid", "amount": "1000.00", "currency": "BDT"}, nil
			}
			return map[string]interface{}{
				"status": "Success", "amount": "1000.00", "currency": "BDT",
				"provider_payment_id": "2264609071029C", "financial_entity": "BKash",
			}, nil
		},
	}
	tx, err := svc.BeginCheckout(context.Background(), CheckoutInput{
		ProjectID: "p1", UserID: "u1", Provider: "hosted", ProductKey: "pro",
		ExpectedAmount: "1000.00", Currency: "BDT",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != StatusPending || tx.ProviderSessionID != "init-uuid" {
		t.Fatalf("checkout row %#v", tx)
	}
	first, err := svc.Verify(context.Background(), tx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != StatusSucceeded || first.ProviderPaymentID != "2264609071029C" {
		t.Fatalf("verify %#v", first)
	}
	second, err := svc.Verify(context.Background(), first, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Status != StatusSucceeded {
		t.Fatalf("duplicate verify %#v", second)
	}
}

func TestAmountMismatchFails(t *testing.T) {
	const pluginID = "hc-test-mismatch"
	t.Cleanup(func() { plugin.UnregisterPaymentProvider(pluginID) })
	_ = plugin.RegisterPaymentProvider(pluginID, &plugin.PaymentProviderContribution{
		ContractVersion: plugin.PaymentContractVersion,
		ProviderID:      "mismatch",
		Label:           "Mismatch",
		Flows:           []string{"one_time"},
		Operations:      plugin.PaymentOperations{Initialize: "init", Verify: "verify"},
	})
	svc := &Service{
		Store:        newMemStore(),
		PublicAPIURL: "https://api.example.test",
		Exec: func(ctx context.Context, pluginID, operation string, args map[string]interface{}, reconcile bool) (map[string]interface{}, error) {
			if operation == "init" {
				return map[string]interface{}{"redirect_url": "https://pay.example", "amount": "10.00", "currency": "USD"}, nil
			}
			return nil, errors.New("unexpected")
		},
	}
	tx, err := svc.BeginCheckout(context.Background(), CheckoutInput{
		ProjectID: "p1", Provider: "mismatch", ExpectedAmount: "12.00", Currency: "USD",
	})
	if err == nil || tx == nil || tx.Status != StatusFailed {
		t.Fatalf("tx %#v err %v", tx, err)
	}
}

func TestNormalizeSuccess(t *testing.T) {
	if NormalizeStatus("Success") != StatusSucceeded {
		t.Fatal(NormalizeStatus("Success"))
	}
}

func TestReturnURLStaysOffThePlugin(t *testing.T) {
	const pluginID = "hc-test-return"
	t.Cleanup(func() { plugin.UnregisterPaymentProvider(pluginID) })
	if err := plugin.RegisterPaymentProvider(pluginID, &plugin.PaymentProviderContribution{
		ContractVersion: plugin.PaymentContractVersion,
		ProviderID:      "returnpay",
		Label:           "Return",
		Flows:           []string{"one_time"},
		Operations:      plugin.PaymentOperations{Initialize: "init", Verify: "verify"},
	}); err != nil {
		t.Fatal(err)
	}
	var args map[string]interface{}
	svc := &Service{
		Store:        newMemStore(),
		PublicAPIURL: "https://api.example.test",
		Exec: func(ctx context.Context, pluginID, operation string, got map[string]interface{}, reconcile bool) (map[string]interface{}, error) {
			args = got
			return map[string]interface{}{"redirect_url": "https://pay.example/redirect", "amount": "10.00", "currency": "BDT"}, nil
		},
	}
	tx, err := svc.BeginCheckout(context.Background(), CheckoutInput{
		ProjectID: "p1", Provider: "returnpay", ExpectedAmount: "10.00", Currency: "BDT",
		ClientPlatform: "android", ReturnURL: "kisti://payment/return",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := args["return_url"]; ok {
		t.Fatalf("plugin saw return_url: %#v", args)
	}
	if _, ok := args["client_platform"]; ok {
		t.Fatalf("plugin saw client_platform: %#v", args)
	}
	if _, ok := args["payment_return_url"]; ok {
		t.Fatalf("plugin saw payment_return_url: %#v", args)
	}
	if !strings.Contains(fmt.Sprint(args["success_url"]), "/callback/success") {
		t.Fatalf("success_url %#v", args["success_url"])
	}
	if metadataString(tx, "return_url") != "kisti://payment/return" || metadataString(tx, "client_platform") != "android" {
		t.Fatalf("metadata %s", tx.MetadataJSON)
	}
	if RedirectURL(tx) != "https://pay.example/redirect" {
		t.Fatalf("redirect %s", tx.MetadataJSON)
	}
	link := ReturnLink(tx)
	if !strings.Contains(link, "transaction_id="+tx.ID) || !strings.Contains(link, "status=pending") {
		t.Fatalf("link %s", link)
	}
	page := RenderResultPage(tx)
	if !strings.Contains(page, "Payment pending") || !strings.Contains(page, "Return to app") || strings.Contains(page, "eps") {
		t.Fatalf("page %s", page)
	}
	if strings.Contains(page, "ZgotmplZ") || !strings.Contains(page, "kisti://payment/return") {
		t.Fatalf("app link was stripped: %s", page)
	}
	if !strings.Contains(page, "intent://payment/return") || !strings.Contains(page, "scheme=kisti") {
		t.Fatalf("android intent missing: %s", page)
	}
	tx.Status = StatusSucceeded
	okPage := RenderResultPage(tx)
	if !strings.Contains(okPage, "Payment succeeded") || !strings.Contains(okPage, `class="mark ok"`) {
		t.Fatalf("success page %s", okPage)
	}
	bad, err := svc.BeginCheckout(context.Background(), CheckoutInput{
		ProjectID: "p1", Provider: "returnpay", ClientPlatform: "android", ReturnURL: "javascript:alert(1)",
	})
	if err == nil || bad != nil {
		t.Fatalf("javascript url must be rejected before a row: %#v %v", bad, err)
	}
	bad, err = svc.BeginCheckout(context.Background(), CheckoutInput{
		ProjectID: "p1", Provider: "returnpay", ClientPlatform: "web", ReturnURL: "http://suchok.example/payment/done",
	})
	if err == nil || bad != nil {
		t.Fatalf("http url must be rejected: %#v %v", bad, err)
	}
}

func TestCallbackBaseUsesRequestHost(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://10.0.3.2:5050/secured/graphql", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "10.0.3.2:5050"
	if got := CallbackBase("", req); got != "http://10.0.3.2:5050" {
		t.Fatal(got)
	}
	if got := CallbackBase("https://api.apito.io", req); got != "https://api.apito.io" {
		t.Fatal(got)
	}
}

func TestWantsResultHTML(t *testing.T) {
	if !WantsResultHTML("text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8") {
		t.Fatal("browser navigation should get HTML")
	}
	if WantsResultHTML("application/json") || WantsResultHTML("") {
		t.Fatal("json and empty accept stay JSON")
	}
	if WantsResultHTML("application/json, text/html") {
		t.Fatal("explicit json wins a tie")
	}
}
