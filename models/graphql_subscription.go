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

// System notification event types (console operator bus).
const (
	SystemEventInfo                   = "info"
	SystemEventSchemaExecutionUpdated = "schema_execution_updated"
	SystemEventSchemaPublishProgress  = "schema_publish_progress"
	SystemEventPluginStatusChanged    = "plugin_status_changed"
	SystemEventProjectCreated         = "project_created"
	SystemEventProjectUpdated         = "project_updated"
)

// Change event types for auto-generated per-model subscriptions.
const (
	ChangeEventCreated = "CREATED"
	ChangeEventUpdated = "UPDATED"
	ChangeEventDeleted = "DELETED"
)

// ModelChangeEvent is the payload pushed to <model>Changed subscribers when a
// document is created, updated or deleted. node/previousValues carry the full
// document as JSON (DB-engine agnostic), mirroring Supabase postgres_changes.
type ModelChangeEvent struct {
	Event          string                 `json:"event"`
	Model          string                 `json:"model"`
	ProjectID      string                 `json:"project_id"`
	ID             string                 `json:"id"`
	Node           interface{}            `json:"node,omitempty"`
	PreviousValues interface{}            `json:"previousValues,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// BroadcastEvent is the payload for the generic broadcast/publish channel layer
// (chat, presence-lite, custom app messages) — Supabase Broadcast equivalent.
type BroadcastEvent struct {
	Channel   string      `json:"channel"`
	ProjectID string      `json:"project_id"`
	Event     string      `json:"event,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	At        string      `json:"at,omitempty"`
}

var ErrSubscriberNotFound = errors.New("subscriber not found")
