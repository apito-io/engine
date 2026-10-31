package plugin

import "testing"

func TestRegisterPaymentProviderRejectsDuplicateAndBadVersion(t *testing.T) {
	const pluginA = "hc-test-pay-a"
	const pluginB = "hc-test-pay-b"
	t.Cleanup(func() {
		UnregisterPaymentProvider(pluginA)
		UnregisterPaymentProvider(pluginB)
	})
	good := &PaymentProviderContribution{
		ContractVersion: PaymentContractVersion,
		ProviderID:      "demo_pay",
		Label:           "Demo",
		Icon:            "credit-card",
		Flows:           []string{"one_time"},
		Operations:      PaymentOperations{Initialize: "init", Verify: "verify"},
	}
	if err := RegisterPaymentProvider(pluginA, good); err != nil {
		t.Fatal(err)
	}
	if err := RegisterPaymentProvider(pluginB, good); err == nil {
		t.Fatal("expected duplicate provider rejection")
	}
	bad := *good
	bad.ContractVersion = 99
	bad.ProviderID = "other_pay"
	if err := RegisterPaymentProvider(pluginB, &bad); err == nil {
		t.Fatal("expected contract version rejection")
	}
	id, contrib, ok := LookupPaymentProvider("demo_pay")
	if !ok || id != pluginA || contrib.Operations.Verify != "verify" {
		t.Fatalf("lookup = %s %#v %v", id, contrib, ok)
	}
}

func TestSetPluginContributionsRegistersPayment(t *testing.T) {
	id := "hc-test-pay-set"
	t.Cleanup(func() { _ = SetPluginContributions(id, nil) })
	err := SetPluginContributions(id, &Contributions{Payment: &PaymentProviderContribution{
		ContractVersion: PaymentContractVersion,
		ProviderID:      "set_pay",
		Label:           "Set",
		Flows:           []string{"subscription"},
		Operations:      PaymentOperations{Initialize: "start", Verify: "check"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if PaymentPluginID("set_pay") != id {
		t.Fatalf("plugin id %s", PaymentPluginID("set_pay"))
	}
	if err := SetPluginContributions(id, nil); err != nil {
		t.Fatal(err)
	}
	if PaymentPluginID("set_pay") != "" {
		t.Fatal("provider still registered")
	}
}
