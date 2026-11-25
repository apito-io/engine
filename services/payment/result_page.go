package payment

import (
	"encoding/json"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"github.com/apito-io/engine/models"
)

var resultPageTmpl = template.Must(template.New("payment-result").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
{{if and .ReturnURL (not .WebReturn)}}<meta http-equiv="refresh" content="0;url={{.ReturnURL}}">{{end}}
<style>
html,body{height:100%}
body{margin:0;display:flex;align-items:center;justify-content:center;background:#f3f5f7;color:#14202b;font-family:system-ui,sans-serif}
main{width:min(22rem,calc(100% - 2rem));text-align:center;background:#fff;border-radius:18px;padding:2.25rem 1.5rem;box-shadow:0 10px 32px rgba(20,32,43,.08)}
.mark{width:4.75rem;height:4.75rem;margin:0 auto 1.25rem;border-radius:50%;display:flex;align-items:center;justify-content:center}
.mark svg{width:2.5rem;height:2.5rem}
.mark.ok{background:#e7f7ee;color:#0f8a45}
.mark.bad{background:#fdecec;color:#c62828}
.mark.warn{background:#fff4df;color:#b26a00}
.mark.wait{background:#eef2f6;color:#3d4c5c}
h1{margin:0 0 .5rem;font-size:1.55rem}
h1.ok{color:#0f8a45}
h1.bad{color:#c62828}
h1.warn{color:#b26a00}
p{margin:0;color:#52616f;line-height:1.5}
a{display:block;margin-top:1.5rem;padding:.95rem 1rem;border-radius:12px;background:#0f8a45;color:#fff;text-decoration:none;font-weight:650}
a.bad{background:#c62828}
a.warn,a.wait{background:#14202b}
</style>
</head>
<body>
<main>
<div class="mark {{.Tone}}" aria-hidden="true">{{.Mark}}</div>
<h1 class="{{.Tone}}">{{.Title}}</h1>
<p>The app checks this payment with the server.</p>
{{if .ReturnURL}}<a class="{{.Tone}}" href="{{.ReturnURL}}">Return to app</a>{{else}}<p style="margin-top:1rem">You can close this page and return to the app.</p>{{end}}
</main>
{{if .WebReturn}}<script>
setTimeout(function(){ window.location.assign({{.ReturnJSON}}); }, 1200);
</script>{{end}}
</body>
</html>
`))

const (
	markCheck = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5 9.5 17 19 7"/></svg>`)
	markCross = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M7 7l10 10M17 7 7 17"/></svg>`)
	markMinus = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M6 12h12"/></svg>`)
	markWait  = template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="12" cy="12" r="8"/><path d="M12 8v5l3 2"/></svg>`)
)

// WantsResultHTML is true when the caller prefers text/html over application/json.
// An empty Accept header stays JSON. Webhooks do not use this.
func WantsResultHTML(accept string) bool {
	htmlQ := -1.0
	jsonQ := -1.0
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bits := strings.Split(part, ";")
		media := strings.ToLower(strings.TrimSpace(bits[0]))
		q := 1.0
		for _, param := range bits[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(strings.ToLower(param), "q=") {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(param[2:]), 64); err == nil {
					q = parsed
				}
			}
		}
		switch media {
		case "text/html":
			if q > htmlQ {
				htmlQ = q
			}
		case "application/json":
			if q > jsonQ {
				jsonQ = q
			}
		}
	}
	return htmlQ > jsonQ
}

// RenderResultPage shows the ledger status. It does not call a plugin.
func RenderResultPage(tx *models.PaymentTransaction) string {
	title := "Payment update"
	tone := "wait"
	mark := markWait
	if tx != nil {
		switch tx.Status {
		case StatusSucceeded:
			title = "Payment succeeded"
			tone = "ok"
			mark = markCheck
		case StatusFailed:
			title = "Payment failed"
			tone = "bad"
			mark = markCross
		case StatusCanceled:
			title = "Payment canceled"
			tone = "warn"
			mark = markMinus
		case StatusExpired:
			title = "Payment expired"
			tone = "warn"
			mark = markMinus
		case StatusPending, StatusCreated:
			title = "Payment pending"
		}
	} else {
		title = "Payment failed"
		tone = "bad"
		mark = markCross
	}
	link := ReturnLink(tx)
	webReturn, returnJSON := webReturnAssign(link)
	var buf strings.Builder
	_ = resultPageTmpl.Execute(&buf, struct {
		Title      string
		Tone       string
		Mark       template.HTML
		ReturnURL  template.URL
		WebReturn  bool
		ReturnJSON template.JS
	}{
		Title:      title,
		Tone:       tone,
		Mark:       mark,
		ReturnURL:  template.URL(link),
		WebReturn:  webReturn,
		ReturnJSON: returnJSON,
	})
	return buf.String()
}

// webReturnAssign is the JS literal for an http(s) shop return.
// Any other configured scheme is a plain link plus a meta refresh.
func webReturnAssign(link string) (bool, template.JS) {
	parsed, err := url.Parse(link)
	if err != nil || link == "" {
		return false, ""
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false, ""
	}
	raw, err := json.Marshal(link)
	if err != nil {
		return false, ""
	}
	return true, template.JS(raw)
}
