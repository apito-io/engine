package services

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"

	"github.com/apito-io/engine/models"
)

// Site nav uses https://apito.io/logo.svg. Gmail often drops SVG in <img>,
// so mail uses the published PNG mark. /img/logo.png 404s.
const apitoEmailLogoURL = "https://apito.io/favicon-192x192.png"

// EmailSender sends transactional mail. Swap via EMAIL_PROVIDER.
type EmailSender interface {
	Send(ctx context.Context, req *models.EmailSendRequest) error
}

type noopEmailSender struct{}

func (noopEmailSender) Send(_ context.Context, req *models.EmailSendRequest) error {
	to := ""
	if req != nil && len(req.Recipients) > 0 {
		to = req.Recipients[0]
	}
	log.Printf("email: noop skip send to %s subject %q", to, reqSubject(req))
	return nil
}

func reqSubject(req *models.EmailSendRequest) string {
	if req == nil {
		return ""
	}
	return req.Subject
}

func defaultFromAddress(cfg *models.Config) string {
	if cfg == nil {
		return "no-reply@apito.io"
	}
	from := strings.TrimSpace(cfg.EmailFrom)
	if from == "" {
		return "no-reply@apito.io"
	}
	return from
}

func defaultFromName(cfg *models.Config) string {
	if cfg == nil {
		return "Apito"
	}
	name := strings.TrimSpace(cfg.EmailFromName)
	if name == "" {
		return "Apito"
	}
	return name
}

func fillSender(cfg *models.Config, req *models.EmailSendRequest) {
	if req == nil {
		return
	}
	if strings.TrimSpace(req.Sender) == "" {
		req.Sender = defaultFromAddress(cfg)
	}
	if strings.TrimSpace(req.SenderName) == "" {
		req.SenderName = defaultFromName(cfg)
	}
}

// NewEmailSender picks Cloudflare, Resend, or noop from Config.
func NewEmailSender(cfg *models.Config) EmailSender {
	if cfg == nil {
		log.Println("email: no config, using noop")
		return noopEmailSender{}
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	if provider == "" {
		provider = "cloudflare"
	}
	switch provider {
	case "noop":
		return noopEmailSender{}
	case "resend":
		if strings.TrimSpace(cfg.ResendAPIKey) == "" {
			log.Println("email: EMAIL_PROVIDER=resend but RESEND_API_KEY empty, using noop")
			return noopEmailSender{}
		}
		return &resendEmailSender{cfg: cfg, apiKey: cfg.ResendAPIKey}
	case "cloudflare":
		if strings.TrimSpace(cfg.CloudflareAPIToken) == "" || strings.TrimSpace(cfg.CloudflareAccountID) == "" {
			log.Println("email: CLOUDFLARE_API_TOKEN or CLOUDFLARE_ACCOUNT_ID empty, using noop")
			return noopEmailSender{}
		}
		return newCloudflareEmailSender(cfg)
	default:
		log.Printf("email: unknown EMAIL_PROVIDER %q, using noop", provider)
		return noopEmailSender{}
	}
}

func inviteProjectNames(req *models.EmailSendRequest) []string {
	if req == nil {
		return nil
	}
	var names []string
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		names = append(names, s)
	}
	for _, n := range req.ProjectNames {
		add(n)
	}
	add(req.ProjectName)
	return names
}

func inviteProjectListText(names []string) string {
	if len(names) == 0 {
		return "You have been invited to join Apito."
	}
	var b strings.Builder
	b.WriteString("You have been invited to:\n")
	for _, n := range names {
		b.WriteString("- ")
		b.WriteString(n)
		b.WriteByte('\n')
	}
	return b.String()
}

func inviteProjectListHTML(names []string) string {
	if len(names) == 0 {
		return `<p>You have been invited to join Apito.</p>`
	}
	var b strings.Builder
	b.WriteString(`<p>You have been invited to:</p><ul style="text-align:left;display:inline-block;margin:8px 0 16px;padding-left:20px">`)
	for _, n := range names {
		b.WriteString("<li>")
		b.WriteString(html.EscapeString(n))
		b.WriteString("</li>")
	}
	b.WriteString("</ul>")
	return b.String()
}

func emailLogoImg() string {
	return fmt.Sprintf(`<img width="48" height="48" src="%s" alt="Apito" style="display:block;margin:0 auto">`, apitoEmailLogoURL)
}

// ComposeTeamInvite fills subject/html/text for a workspace/project invite.
func ComposeTeamInvite(req *models.EmailSendRequest) {
	if req == nil {
		return
	}
	req.Subject = "You're invited"
	names := inviteProjectNames(req)
	accept := strings.TrimSpace(req.AcceptURL)
	var extraText, extraHTML string
	if req.TempPassword != "" {
		extraText = fmt.Sprintf("\nYour temporary password is: %s\n", req.TempPassword)
		extraHTML = fmt.Sprintf(`<p>Your temporary password is: <strong>%s</strong></p>`, html.EscapeString(req.TempPassword))
	}
	if accept != "" {
		extraText += fmt.Sprintf("\nAccept this invite (expires in a few days):\n%s\n", accept)
		href := html.EscapeString(accept)
		extraHTML += fmt.Sprintf(`<p><a href="%s" style="display:inline-block;background:#EA3A60;color:#fff;padding:12px 24px;text-decoration:none;border-radius:4px">Accept invite</a></p><p style="font-size:12px;color:#888">Or paste: %s</p>`, href, href)
	} else {
		extraText += "\nLog in with your existing email and password.\n"
		extraHTML += `<p>Log in with your existing email and password.</p>`
	}
	req.TextBody = fmt.Sprintf(`
Hi,

%s
%s
This is an automated email, please do not reply.

Best regards,
Apito.io Team
`, inviteProjectListText(names), extraText)
	req.HtmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>You're invited</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0">
<div style="max-width:600px;margin:0 auto;background:#fff;border:1px solid #ddd;border-top:10px solid #EA3A60">
<div style="text-align:center;padding:40px 0">
%s
<h1 style="font-size:28px;color:#000;margin:20px 0 10px">You're invited</h1>
</div>
<div style="padding:20px;text-align:center">
<p>Hi,</p>
%s
%s
<p style="color:#EA3A60;font-weight:bold">This is an automated email, please do not reply.</p>
</div>
<div style="background:#f4f4f4;padding:10px;text-align:center;font-size:12px;color:#888">
<p>Best regards,<br>Apito.io Team</p>
</div>
</div>
</body>
</html>`, emailLogoImg(), inviteProjectListHTML(names), extraHTML)
}

// ComposePasswordReset fills subject/html/text for a 6-digit recovery code.
func ComposePasswordReset(req *models.EmailSendRequest) {
	if req == nil {
		return
	}
	req.Subject = "Reset your Apito password"
	loginHint := ""
	if strings.TrimSpace(req.AppURL) != "" {
		loginHint = fmt.Sprintf("\nLog in at %s after you set a new password.\n", strings.TrimSpace(req.AppURL))
	}
	req.TextBody = fmt.Sprintf(`
Hi,

Your Apito password reset code is: %s

This code expires in 15 minutes. If you did not request a reset, ignore this email.
%s
This is an automated email, please do not reply.

Best regards,
Apito.io Team
`, req.VerificationCode, loginHint)
	loginHTML := ""
	if strings.TrimSpace(req.AppURL) != "" {
		loginHTML = fmt.Sprintf(`<p>Log in at <a href="%s">%s</a> after you set a new password.</p>`, req.AppURL, req.AppURL)
	}
	req.HtmlBody = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Reset your Apito password</title></head>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;margin:0;padding:0">
<div style="max-width:600px;margin:0 auto;background:#fff;border:1px solid #ddd;border-top:10px solid #EA3A60">
<div style="text-align:center;padding:40px 0">
%s
<h1 style="font-size:28px;color:#000;margin:20px 0 10px">Password reset</h1>
</div>
<div style="padding:20px;text-align:center">
<p>Hi,</p>
<p>Your password reset code is:</p>
<p style="font-size:28px;letter-spacing:4px;font-weight:bold">%s</p>
<p>This code expires in 15 minutes. If you did not request a reset, ignore this email.</p>
%s
<p style="color:#EA3A60;font-weight:bold">This is an automated email, please do not reply.</p>
</div>
<div style="background:#f4f4f4;padding:10px;text-align:center;font-size:12px;color:#888">
<p>Best regards,<br>Apito.io Team</p>
</div>
</div>
</body>
</html>`, emailLogoImg(), req.VerificationCode, loginHTML)
}

// SendComposed sends req after ensuring subject/body are set.
func SendComposed(ctx context.Context, mailer EmailSender, req *models.EmailSendRequest) error {
	if mailer == nil {
		return fmt.Errorf("email sender is not configured")
	}
	if req == nil {
		return fmt.Errorf("email request is required")
	}
	return mailer.Send(ctx, req)
}
