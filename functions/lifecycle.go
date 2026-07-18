package functions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// DeployDraft creates an immutable revision + successful build + active deployment
// and updates the function's ActiveRevisionID. Caller persists rows + function.
func DeployDraft(ctx context.Context, store ArtifactStore, fn *models.ApitoFunction, source []byte, deployedBy string) (*models.FunctionRevision, *models.FunctionBuild, *models.FunctionDeployment, error) {
	if fn == nil || fn.Name == "" || fn.ProjectID == "" {
		return nil, nil, nil, fmt.Errorf("function project_id and name required")
	}
	if len(source) == 0 && fn.Source != "" {
		source = []byte(fn.Source)
	}
	if len(source) == 0 {
		return nil, nil, nil, fmt.Errorf("empty source")
	}
	sum := sha256.Sum256(source)
	hash := hex.EncodeToString(sum[:])
	revID := utility.NewID()
	buildID := utility.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	key := fmt.Sprintf("%s/%s/%s", fn.ProjectID, fn.Name, revID)
	if store != nil {
		if err := store.Put(ctx, key, source, hash); err != nil {
			return nil, nil, nil, err
		}
	}
	rev := &models.FunctionRevision{
		ID:           revID,
		ProjectID:    fn.ProjectID,
		Name:         fn.Name,
		Revision:     time.Now().Unix(),
		Runtime:      fn.EffectiveRuntime(),
		Language:     fn.Language,
		Source:       string(source),
		ArtifactKey:  key,
		ArtifactHash: hash,
		ABIVersion:   ABIVersion,
		Capabilities: fn.Capabilities,
		CreatedBy:    deployedBy,
		CreatedAt:    now,
	}
	build := &models.FunctionBuild{
		ID:           buildID,
		ProjectID:    fn.ProjectID,
		Name:         fn.Name,
		RevisionID:   revID,
		Status:       "succeeded",
		ArtifactKey:  key,
		ArtifactHash: hash,
		CreatedAt:    now,
		CompletedAt:  now,
	}
	dep := &models.FunctionDeployment{
		ID:          utility.NewID(),
		ProjectID:   fn.ProjectID,
		Name:        fn.Name,
		RevisionID:  revID,
		BuildID:     buildID,
		Environment: "production",
		Status:      "active",
		DeployedBy:  deployedBy,
		CreatedAt:   now,
	}
	fn.ActiveRevisionID = revID
	fn.BinaryURL = key
	// Keep draft Source aligned with what was just deployed.
	fn.Source = string(source)
	fn.UpdatedAt = now
	return rev, build, dep, nil
}

// RollbackDeployment points the function at a previous revision.
// Does not mutate draft Source — only ActiveRevisionID / BinaryURL for live execution.
func RollbackDeployment(fn *models.ApitoFunction, target *models.FunctionRevision, deployedBy string) *models.FunctionDeployment {
	if fn == nil || target == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	prev := fn.ActiveRevisionID
	fn.ActiveRevisionID = target.ID
	if target.ArtifactKey != "" {
		fn.BinaryURL = target.ArtifactKey
	}
	fn.UpdatedAt = now
	return &models.FunctionDeployment{
		ID:          utility.NewID(),
		ProjectID:   fn.ProjectID,
		Name:        fn.Name,
		RevisionID:  target.ID,
		Environment: "production",
		Status:      "active",
		DeployedBy:  deployedBy,
		RollbackOf:  prev,
		CreatedAt:   now,
	}
}

// ResolveActiveSource returns the deployed revision source for live execution.
// Falls back to draft Source for never-deployed / legacy functions.
func ResolveActiveSource(ctx context.Context, store ArtifactStore, fn *models.ApitoFunction) string {
	if fn == nil {
		return ""
	}
	if fn.ActiveRevisionID != "" && store != nil {
		key := fn.BinaryURL
		if key == "" {
			key = fmt.Sprintf("%s/%s/%s", fn.ProjectID, fn.Name, fn.ActiveRevisionID)
		}
		if key != "" {
			data, _, err := store.Get(ctx, key)
			if err == nil && len(data) > 0 {
				return string(data)
			}
		}
	}
	return fn.Source
}
