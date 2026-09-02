package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
)

const resendAPIURL = "https://api.resend.com/emails"

type resendEmailSender struct {
	cfg    *models.Config
	apiKey string
}

func (s *resendEmailSender) Send(ctx context.Context, req *models.EmailSendRequest) error {
	if s == nil {
		return fmt.Errorf("RESEND_API_KEY is not configured")
	}
	fillSender(s.cfg, req)
	return SendResendEmail(ctx, s.apiKey, req)
}

type resendEmailPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

// SendResendEmail sends a transactional email via Resend.
func SendResendEmail(ctx context.Context, apiKey string, req *models.EmailSendRequest) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return fmt.Errorf("RESEND_API_KEY is not configured")
	}
	if req == nil || len(req.Recipients) == 0 {
		return fmt.Errorf("email recipients are required")
	}
	from := strings.TrimSpace(req.Sender)
	if from == "" {
		from = "no-reply@apito.io"
	}
	if name := strings.TrimSpace(req.SenderName); name != "" && !strings.Contains(from, "<") {
		from = fmt.Sprintf("%s <%s>", name, from)
	}
	body, err := json.Marshal(resendEmailPayload{
		From:    from,
		To:      req.Recipients,
		Subject: req.Subject,
		HTML:    req.HtmlBody,
		Text:    req.TextBody,
	})
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// SendTeamAddEmail composes the invite template and sends via Resend.
// Prefer GraphQLServer.Mailer + ComposeTeamInvite for new code.
func SendTeamAddEmail(ctx context.Context, apiKey string, req *models.EmailSendRequest) error {
	if req == nil {
		return fmt.Errorf("email request is required")
	}
	ComposeTeamInvite(req)
	return SendResendEmail(ctx, apiKey, req)
}
