package models

type WebhookPost struct {
	Id      string      `json:"id"`
	Event   string      `json:"event"`
	Model   string      `json:"model"`
	Payload interface{} `json:"payload"`
}
