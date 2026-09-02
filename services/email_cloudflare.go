package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
)

const cloudflareSendAPI = "https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send"

type cloudflareEmailSender struct {
	cfg       *models.Config
	token     string
	accountID string
	client    *http.Client
	endpoint  string
}

type cloudflareFrom struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type cloudflareSendPayload struct {
	To      []string       `json:"to"`
	From    cloudflareFrom `json:"from"`
	Subject string         `json:"subject"`
	HTML    string         `json:"html,omitempty"`
	Text    string         `json:"text,omitempty"`
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareSendResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
}

func newCloudflareEmailSender(cfg *models.Config) *cloudflareEmailSender {
	accountID := strings.TrimSpace(cfg.CloudflareAccountID)
	return &cloudflareEmailSender{
		cfg:       cfg,
		token:     strings.TrimSpace(cfg.CloudflareAPIToken),
		accountID: accountID,
		client:    &http.Client{Timeout: 15 * time.Second},
		endpoint:  fmt.Sprintf(cloudflareSendAPI, accountID),
	}
}

func (s *cloudflareEmailSender) Send(ctx context.Context, req *models.EmailSendRequest) error {
	if s == nil || strings.TrimSpace(s.token) == "" || strings.TrimSpace(s.accountID) == "" {
		return fmt.Errorf("cloudflare email sender is not configured")
	}
	if req == nil || len(req.Recipients) == 0 {
		return fmt.Errorf("email recipients are required")
	}
	fillSender(s.cfg, req)
	body, err := json.Marshal(cloudflareSendPayload{
		To: req.Recipients,
		From: cloudflareFrom{
			Address: req.Sender,
			Name:    req.SenderName,
		},
		Subject: req.Subject,
		HTML:    req.HtmlBody,
		Text:    req.TextBody,
	})
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.token)
	httpReq.Header.Set("Content-Type", "application/json")
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare email API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed cloudflareSendResponse
	if err := json.Unmarshal(raw, &parsed); err == nil && !parsed.Success && len(parsed.Errors) > 0 {
		return fmt.Errorf("cloudflare email API error (%d): %s", parsed.Errors[0].Code, parsed.Errors[0].Message)
	}
	return nil
}
