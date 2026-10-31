package controller

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

type ticketRecord struct {
	headers map[string]string
	expires time.Time
}

type subscriptionTicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketRecord
}

func newSubscriptionTicketStore() *subscriptionTicketStore {
	return &subscriptionTicketStore{tickets: map[string]ticketRecord{}}
}

func (s *subscriptionTicketStore) issue(headers map[string]string, ttl time.Duration) string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	ticket := hex.EncodeToString(buf)
	s.mu.Lock()
	s.tickets[ticket] = ticketRecord{headers: headers, expires: time.Now().Add(ttl)}
	s.mu.Unlock()
	return ticket
}

func (s *subscriptionTicketStore) redeem(ticket string) (map[string]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.tickets[ticket]
	if !ok {
		return nil, false
	}
	delete(s.tickets, ticket)
	if time.Now().After(rec.expires) {
		return nil, false
	}
	return rec.headers, true
}

func ticketFromProtocol(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		const prefix = "apito-ticket."
		if strings.HasPrefix(part, prefix) {
			return strings.TrimPrefix(part, prefix)
		}
	}
	return ""
}

// RedeemSubscriptionTicket copies a one-time web ticket onto the request
// before Authorize runs. Mobile clients send normal headers and skip this.
func (g *GraphCtrl) RedeemSubscriptionTicket(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if g == nil || g.tickets == nil || !strings.HasSuffix(c.Request().URL.Path, "/graphql/subscription") {
			return next(c)
		}
		ticket := ticketFromProtocol(c.Request().Header.Get("Sec-WebSocket-Protocol"))
		if ticket == "" {
			return next(c)
		}
		headers, ok := g.tickets.redeem(ticket)
		if !ok {
			return c.JSON(http.StatusUnauthorized, map[string]string{"message": "subscription ticket expired"})
		}
		for key, value := range headers {
			if c.Request().Header.Get(key) == "" {
				c.Request().Header.Set(key, value)
			}
		}
		return next(c)
	}
}

// IssueSubscriptionTicket returns a 30-second ticket for browser sockets that
// cannot set the Authorization header. The raw credential never goes in the URL.
func (g *GraphCtrl) IssueSubscriptionTicket(c echo.Context) error {
	if g.tickets == nil {
		g.tickets = newSubscriptionTicketStore()
	}
	copied := map[string]string{}
	for _, key := range []string{"Authorization", "X-Apito-Key", "X-Apito-Project-Id", "X-Apito-Tenant-ID", "Cookie", "X-Use-Cookies"} {
		if value := c.Request().Header.Get(key); value != "" {
			copied[key] = value
		}
	}
	if len(copied) == 0 {
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "authentication required"})
	}
	ticket := g.tickets.issue(copied, 30*time.Second)
	return c.JSON(http.StatusOK, map[string]interface{}{"ticket": ticket, "expires_in": 30})
}

func realtimeOriginAllowed(cfg *models.Config, r *http.Request) bool {
	if cfg == nil || strings.TrimSpace(cfg.RealtimeAllowedOrigins) == "" || r == nil {
		return true
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	for _, allowed := range strings.Split(cfg.RealtimeAllowedOrigins, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}
