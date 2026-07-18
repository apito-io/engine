package functions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// GatewayBridge is an ephemeral loopback HTTP server for Deno → host data calls.
type GatewayBridge struct {
	URL          string
	Token        string
	InvocationID string
	server       *http.Server
	listener     net.Listener
}

type bridgeRequest struct {
	Op      string                 `json:"op"`
	Payload map[string]interface{} `json:"payload"`
}

// StartGatewayBridge listens on 127.0.0.1 and serves POST /call for one invocation.
func StartGatewayBridge(ctx context.Context, gateway FunctionDataGateway, inv *InvocationContext) (*GatewayBridge, error) {
	if gateway == nil || inv == nil || inv.Envelope == nil {
		return nil, fmt.Errorf("gateway bridge requires gateway and invocation")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	token, err := randomToken(24)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	b := &GatewayBridge{
		URL:          "http://" + ln.Addr().String(),
		Token:        token,
		InvocationID: inv.Envelope.InvocationID,
		listener:     ln,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/call", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.EqualFold(auth, "Bearer "+b.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var req bridgeRequest
		if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Op) == "" {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if req.Payload == nil {
			req.Payload = map[string]interface{}{}
		}
		// Never trust guest-supplied identity/scope fields.
		delete(req.Payload, "project_id")
		delete(req.Payload, "tenant_id")
		delete(req.Payload, "capabilities")
		delete(req.Payload, "invocation_id")

		call := &DataGatewayCall{
			InvocationID: b.InvocationID,
			ProjectID:    inv.Envelope.ProjectID,
			TenantID:     inv.Envelope.TenantID,
			Op:           req.Op,
			Payload:      req.Payload,
		}
		resp, err := gateway.Handle(r.Context(), call)
		if err != nil {
			writeJSON(w, http.StatusOK, &DataGatewayResponse{OK: false, Error: err.Error()})
			return
		}
		if resp == nil {
			writeJSON(w, http.StatusOK, &DataGatewayResponse{OK: false, Error: "empty gateway response"})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
	b.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		_ = b.server.Serve(ln)
	}()
	return b, nil
}

// Close shuts down the bridge.
func (b *GatewayBridge) Close() {
	if b == nil || b.server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = b.server.Shutdown(ctx)
	if b.listener != nil {
		_ = b.listener.Close()
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
