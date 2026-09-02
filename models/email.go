package models

type EmailSendRequest struct {
	AppURL           string   `json:"app_url"`
	AcceptURL        string   `json:"accept_url"`
	Sender           string   `json:"sender"`
	SenderName       string   `json:"sender_name"`
	ProjectName      string   `json:"project_name"`
	ProjectNames     []string `json:"project_names,omitempty"`
	TempPassword     string   `json:"temp_password"`
	VerificationCode string   `json:"verification_code"`
	Recipients       []string `json:"recipients"`
	Subject          string   `json:"subject"`
	HtmlBody         string   `json:"html_body"`
	TextBody         string   `json:"text_body"`
}
