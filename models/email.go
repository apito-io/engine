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

	// Freelance client billing (Request Client Billing). Unused by team invite.
	ProjectDescription string `json:"project_description,omitempty"`
	ProjectKind        string `json:"project_kind,omitempty"`
	SelectedPlan       string `json:"selected_plan,omitempty"`
	TemplateName       string `json:"template_name,omitempty"`
	FreelancerName     string `json:"freelancer_name,omitempty"`
	FreelancerEmail    string `json:"freelancer_email,omitempty"`
	ActivateBy         string `json:"activate_by,omitempty"`
	TrialEndsAt        string `json:"trial_ends_at,omitempty"`
	QuotaNote          string `json:"quota_note,omitempty"`
}
