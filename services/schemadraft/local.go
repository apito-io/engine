package schemadraft

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apito-io/engine/models"
)

// Publisher applies staged operations to the live project. The public engine
// replays them synchronously. A returned error keeps the draft file.
type Publisher func(ctx context.Context, project *models.Project, ops []Op) error

// Local is the file-backed pending draft.
type Local struct {
	Publish Publisher
}

func NewLocal(publish Publisher) *Local {
	return &Local{Publish: publish}
}

func (l *Local) Status(_ context.Context, project *models.Project) (map[string]interface{}, error) {
	draft, err := l.file(project)
	if err != nil {
		return nil, err
	}
	status := "draft"
	errMsg := ""
	if project != nil && project.Schema != nil && len(draft.Ops) > 0 {
		if _, replayErr := Replay(project.Schema, draft.Ops); replayErr != nil {
			errMsg = replayErr.Error()
		}
	}
	if len(draft.Ops) == 0 {
		status = ""
	}
	return map[string]interface{}{
		"enabled":            true,
		"active_version":     0,
		"has_draft":          len(draft.Ops) > 0,
		"changeset_id":       draft.ChangesetID,
		"changeset_status":   status,
		"pending_operations": len(draft.Ops),
		"publish_total":      len(draft.Ops),
		"publish_done":       0,
		"error":              errMsg,
		"capabilities":       PublicCapabilities().Map(),
	}, nil
}

func (l *Local) Preview(_ context.Context, project *models.Project, source string, _ int) (string, error) {
	if project == nil {
		return "{}", nil
	}
	schema := project.Schema
	if source == "draft" || source == "effective" {
		draft, err := l.file(project)
		if err != nil {
			return "", err
		}
		if source == "draft" || len(draft.Ops) > 0 {
			replayed, err := Replay(schema, draft.Ops)
			if err != nil {
				return "", err
			}
			schema = replayed
		}
	}
	if schema == nil {
		schema = &models.ProjectSchema{}
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (l *Local) Diff(_ context.Context, project *models.Project, _ string) (string, error) {
	if project == nil {
		return "{}", nil
	}
	draft, err := l.file(project)
	if err != nil {
		return "", err
	}
	replayed, err := Replay(project.Schema, draft.Ops)
	if err != nil {
		return EncodeDiff(nil), nil
	}
	return EncodeDiff(ComputeDiff(project.Schema, replayed)), nil
}

func (l *Local) Events(_ context.Context, project *models.Project, _ string) ([]map[string]interface{}, error) {
	draft, err := l.file(project)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(draft.Ops))
	for _, op := range draft.Ops {
		payload, _ := json.Marshal([]map[string]interface{}{{
			"type":       op.Type,
			"model_name": firstStr(op.Args, "model_name", "name"),
			"field_name": firstStr(op.Args, "field_name", "field"),
			"payload":    op.Args,
		}})
		out = append(out, map[string]interface{}{
			"event_type":   "operation_added",
			"actor_id":     "",
			"created_at":   op.CreatedAt,
			"payload_json": string(payload),
		})
	}
	return out, nil
}

func (l *Local) Records(_ context.Context, project *models.Project, changesetID string, _, limit, offset int) ([]map[string]interface{}, error) {
	draft, err := l.file(project)
	if err != nil {
		return nil, err
	}
	if changesetID != "" && changesetID != draft.ChangesetID {
		return []map[string]interface{}{}, nil
	}
	rows := Plan(projectID(project), draft.ChangesetID, draft.Ops)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return []map[string]interface{}{}, nil
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows, nil
}

func (l *Local) Approve(ctx context.Context, project *models.Project, changesetID, _, _ string) (map[string]interface{}, error) {
	draft, err := l.file(project)
	if err != nil {
		return nil, err
	}
	if changesetID != "" && changesetID != draft.ChangesetID {
		return nil, fmt.Errorf("changeset %s is not the active draft", changesetID)
	}
	if len(draft.Ops) == 0 {
		return nil, fmt.Errorf("no schema draft to publish")
	}
	if l.Publish == nil {
		return nil, fmt.Errorf("schema draft publish is unavailable")
	}
	if err := l.Publish(ctx, project, draft.Ops); err != nil {
		return nil, err
	}
	if err := Remove(projectID(project)); err != nil {
		return nil, err
	}
	n := len(draft.Ops)
	return map[string]interface{}{
		"version":      nil,
		"changeset_id": draft.ChangesetID,
		"message":      "Schema published",
		"status":       "published",
		"total":        n,
		"done":         n,
	}, nil
}

func (l *Local) Discard(_ context.Context, project *models.Project, changesetID string) (bool, error) {
	draft, err := l.file(project)
	if err != nil {
		return false, err
	}
	if changesetID != "" && changesetID != draft.ChangesetID {
		return false, fmt.Errorf("changeset %s is not the active draft", changesetID)
	}
	if err := Remove(projectID(project)); err != nil {
		return false, err
	}
	return true, nil
}

func (l *Local) file(project *models.Project) (*File, error) {
	id := projectID(project)
	if id == "" {
		return nil, fmt.Errorf("project is required")
	}
	return Load(id)
}

func projectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	return project.ID
}
