package schemadraft

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Op is one staged schema mutation. Old draft files use this shape.
type Op struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Args      map[string]interface{} `json:"args"`
	CreatedAt string                 `json:"created_at"`
}

// File is the on-disk draft. changeset_id is optional so older files still load.
type File struct {
	ProjectID   string `json:"project_id"`
	ChangesetID string `json:"changeset_id,omitempty"`
	Ops         []Op   `json:"ops"`
}

var mu sync.Mutex

// RootDir overrides ~/.apito/schema-drafts in tests.
var RootDir string

func draftDir() (string, error) {
	if strings.TrimSpace(RootDir) != "" {
		if err := os.MkdirAll(RootDir, 0o700); err != nil {
			return "", err
		}
		return RootDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".apito", "schema-drafts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func safeProjectID(projectID string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, projectID)
}

// ChangesetID is stable for a project so the console can refetch the same draft.
func ChangesetID(projectID string) string {
	return "local-" + safeProjectID(projectID)
}

func draftPath(projectID string) (string, error) {
	dir, err := draftDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, safeProjectID(projectID)+".json"), nil
}

// Load reads a draft. A missing file is an empty draft.
func Load(projectID string) (*File, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadUnlocked(projectID)
}

func loadUnlocked(projectID string) (*File, error) {
	path, err := draftPath(projectID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{ProjectID: projectID, ChangesetID: ChangesetID(projectID)}, nil
		}
		return nil, err
	}
	var draft File
	if err := json.Unmarshal(raw, &draft); err != nil {
		return nil, err
	}
	if draft.ProjectID == "" {
		draft.ProjectID = projectID
	}
	if draft.ChangesetID == "" {
		draft.ChangesetID = ChangesetID(projectID)
	}
	if draft.Ops == nil {
		draft.Ops = []Op{}
	}
	return &draft, nil
}

func saveUnlocked(draft *File) error {
	if draft.ChangesetID == "" {
		draft.ChangesetID = ChangesetID(draft.ProjectID)
	}
	path, err := draftPath(draft.ProjectID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Append adds one operation and keeps the changeset id stable.
func Append(projectID string, op Op) (*File, error) {
	mu.Lock()
	defer mu.Unlock()
	draft, err := loadUnlocked(projectID)
	if err != nil {
		return nil, err
	}
	draft.Ops = append(draft.Ops, op)
	if err := saveUnlocked(draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// Remove deletes the draft file. A missing file is success.
func Remove(projectID string) error {
	mu.Lock()
	defer mu.Unlock()
	path, err := draftPath(projectID)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
