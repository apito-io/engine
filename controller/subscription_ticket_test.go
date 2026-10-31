package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
)

func TestRealtimeOriginAllowlist(t *testing.T) {
	cfg := &models.Config{RealtimeAllowedOrigins: "https://console.example"}
	allowed := &http.Request{Header: http.Header{"Origin": []string{"https://console.example"}}}
	denied := &http.Request{Header: http.Header{"Origin": []string{"https://evil.example"}}}
	native := &http.Request{Header: http.Header{}}
	if !realtimeOriginAllowed(cfg, allowed) || !realtimeOriginAllowed(cfg, native) {
		t.Fatal("expected configured and headerless origins to pass")
	}
	if realtimeOriginAllowed(cfg, denied) {
		t.Fatal("expected foreign origin to fail")
	}
	if !realtimeOriginAllowed(&models.Config{}, denied) {
		t.Fatal("empty allowlist keeps local and mobile clients working")
	}
}

func TestSubscriptionTicketRoundTrip(t *testing.T) {
	store := newSubscriptionTicketStore()
	ticket := store.issue(map[string]string{"X-Apito-Key": "ak_test"}, time.Second)
	headers, ok := store.redeem(ticket)
	if !ok || headers["X-Apito-Key"] != "ak_test" {
		t.Fatalf("redeem=%v %v", ok, headers)
	}
	if _, ok := store.redeem(ticket); ok {
		t.Fatal("ticket must be single use")
	}
}

func TestTicketFromProtocol(t *testing.T) {
	got := ticketFromProtocol("graphql-transport-ws, apito-ticket.abc123")
	if got != "abc123" {
		t.Fatalf("ticket=%q", got)
	}
}
