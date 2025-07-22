package models

type Subscriber struct {
	Data   chan interface{}
	UserID string
}

type SubscriptionEvent struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
}
