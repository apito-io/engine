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

// SendTeamAddEmail sends the project team invite email via Resend.
func SendTeamAddEmail(ctx context.Context, apiKey string, req *models.EmailSendRequest) error {
	if req == nil {
		return fmt.Errorf("email request is required")
	}
	req.Subject = "Welcome to Apito.io! You've been added to a new project"
	if req.TempPassword != "" {
		req.TextBody = fmt.Sprintf(`
Hi,

Welcome to the %s project! You have been added to the project. Your temporary password is: %s.

Please log in and change your password as soon as possible.

This is an automated email, please do not reply. If you need assistance, contact your administrator.

Best regards,
Apito.io Team
`, req.ProjectName, req.TempPassword)
		req.HtmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Welcome to Apito.io</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0">
<div style="max-width:600px;margin:0 auto;background:#fff;border:1px solid #ddd;border-top:10px solid #EA3A60">
<div style="text-align:center;padding:40px 0">
<img width="50" height="50" src="https://apito.io/img/logo.png" alt="Apito.io Logo">
<h1 style="font-size:28px;color:#000;margin:20px 0 10px">Welcome to %s</h1>
</div>
<div style="padding:20px;text-align:center">
<p>Hi,</p>
<p>You have been added to the project, and here is your temporary login password:</p>
<p><strong>Password: %s</strong></p>
<p>Please log in and change your password as soon as possible.</p>
<p style="color:#EA3A60;font-weight:bold">This is an automated email, please do not reply.</p>
</div>
<div style="background:#f4f4f4;padding:10px;text-align:center;font-size:12px;color:#888">
<p>Best regards,<br>Apito.io Team</p>
</div>
</div>
</body>
</html>`, req.ProjectName, req.TempPassword)
	} else {
		req.TextBody = fmt.Sprintf(`
Hi,

Welcome to the %s project! You have been added to the project.

This is an automated email, please do not reply. If you need assistance, contact your administrator.

Best regards,
Apito.io Team
`, req.ProjectName)
		req.HtmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Welcome to Apito.io</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0">
<div style="max-width:600px;margin:0 auto;background:#fff;border:1px solid #ddd;border-top:10px solid #EA3A60">
<div style="text-align:center;padding:40px 0">
<img width="50" height="50" src="https://apito.io/img/logo.png" alt="Apito.io Logo">
<h1 style="font-size:28px;color:#000;margin:20px 0 10px">Welcome to %s</h1>
</div>
<div style="padding:20px;text-align:center">
<p>Hi,</p>
<p>You have been added to the project.</p>
<p>Please log in with your existing email and password.</p>
<p style="color:#EA3A60;font-weight:bold">This is an automated email, please do not reply.</p>
</div>
<div style="background:#f4f4f4;padding:10px;text-align:center;font-size:12px;color:#888">
<p>Best regards,<br>Apito.io Team</p>
</div>
</div>
</body>
</html>`, req.ProjectName)
	}
	return SendResendEmail(ctx, apiKey, req)
}
