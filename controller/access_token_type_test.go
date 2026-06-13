package controller

import "testing"

func TestNormalizeAccessTokenType(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "cli", false},
		{"cli", "cli", false},
		{"SDK", "sdk", false},
		{"mcp", "mcp", false},
		{"invalid", "", true},
	}
	for _, tc := range tests {
		got, err := normalizeAccessTokenType(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeAccessTokenType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrefixAccessToken(t *testing.T) {
	got := prefixAccessToken("mcp", "abc123")
	if got != "mcp-abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestSyncTokensMatch(t *testing.T) {
	raw := "encodedPayload"
	prefixed := "cli-" + raw
	if !syncTokensMatch(prefixed, prefixed) {
		t.Fatal("exact match failed")
	}
	if !syncTokensMatch(prefixed, raw) {
		t.Fatal("prefixed vs raw failed")
	}
	if !syncTokensMatch(raw, prefixed) {
		t.Fatal("raw vs prefixed failed")
	}
	if syncTokensMatch("sdk-"+raw, "mcp-"+raw) {
		t.Fatal("different prefixes should not match")
	}
}
