package models

import "errors"

type Subscriber struct {
	Data     chan interface{}
	UserID   string
	IsActive bool
}

type SubscriptionEvent struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
}

var ErrSubscriberNotFound = errors.New("subscriber not found")
