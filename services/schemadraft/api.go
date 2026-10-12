package schemadraft

import (
	"context"

	"github.com/apito-io/engine/models"
)

// Capabilities tells the console which Cloud-only controls to show.
// The public engine leaves every flag false.
type Capabilities struct {
	History       bool `json:"history"`
	Collaboration bool `json:"collaboration"`
	Revert        bool `json:"revert"`
	Repair        bool `json:"repair"`
	RemoteSync    bool `json:"remote_sync"`
	AsyncPublish  bool `json:"async_publish"`
}

// API is the pending-draft surface both engines implement.
// History, revert, review, repair, and remote sync stay outside this interface.
type API interface {
	Status(ctx context.Context, project *models.Project) (map[string]interface{}, error)
	Preview(ctx context.Context, project *models.Project, source string, version int) (string, error)
	Diff(ctx context.Context, project *models.Project, changesetID string) (string, error)
	Events(ctx context.Context, project *models.Project, changesetID string) ([]map[string]interface{}, error)
	Records(ctx context.Context, project *models.Project, changesetID string, version, limit, offset int) ([]map[string]interface{}, error)
	Approve(ctx context.Context, project *models.Project, changesetID, message, actor string) (map[string]interface{}, error)
	Discard(ctx context.Context, project *models.Project, changesetID string) (bool, error)
}

func (c Capabilities) Map() map[string]interface{} {
	return map[string]interface{}{
		"history":       c.History,
		"collaboration": c.Collaboration,
		"revert":        c.Revert,
		"repair":        c.Repair,
		"remote_sync":   c.RemoteSync,
		"async_publish": c.AsyncPublish,
	}
}

// PublicCapabilities is the local file-backed draft. Publish is synchronous.
func PublicCapabilities() Capabilities { return Capabilities{} }

// CloudCapabilities marks the controls the Cloud console composes around the shared pending panel.
func CloudCapabilities() Capabilities {
	return Capabilities{
		History:       true,
		Collaboration: true,
		Revert:        true,
		Repair:        true,
		RemoteSync:    true,
		AsyncPublish:  true,
	}
}
