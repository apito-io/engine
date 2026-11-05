package plugin

import (
	"strings"
	"testing"
)

func TestRedactPluginConfigSecretsBlanksPassword(t *testing.T) {
	raw := []byte(`plugin:
  id: hc-eps-payment-plugin
  env_vars:
    - key: EPS_PASSWORD
      value: super-secret
    - key: STORE_ID
      value: public-store
`)
	out, err := RedactPluginConfigSecrets(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "super-secret") {
		t.Fatalf("password leaked: %s", text)
	}
	if !strings.Contains(text, "public-store") {
		t.Fatalf("non-secret value was blanked: %s", text)
	}
	if !strings.Contains(text, "EPS_PASSWORD") {
		t.Fatalf("env key was removed: %s", text)
	}
}
