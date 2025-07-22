package models

type EmailSendRequest struct {
	AppURL       string   `json:"app_url"`
	Sender       string   `json:"sender"`
	ProjectName  string   `json:"project_name"`
	TempPassword string   `json:"temp_password"`
	Recipients   []string `json:"recipients"`
	Subject      string   `json:"subject"`
	HtmlBody     string   `json:"html_body"`
	TextBody     string   `json:"text_body"`
}
