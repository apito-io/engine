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
// and updates the function's ActiveRevisionID. Caller persists the function row.
func DeployDraft(ctx context.Context, store ArtifactStore, fn *models.ApitoFunction, source []byte, deployedBy string) (*models.FunctionRevision, *models.FunctionDeployment, error) {
	_ = ctx
	if fn == nil || fn.Name == "" || fn.ProjectID == "" {
		return nil, nil, fmt.Errorf("function project_id and name required")
	}
	if len(source) == 0 && fn.Source != "" {
		source = []byte(fn.Source)
	}
	if len(source) == 0 {
		return nil, nil, fmt.Errorf("empty source")
	}
	sum := sha256.Sum256(source)
	hash := hex.EncodeToString(sum[:])
	revID := utility.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	key := fmt.Sprintf("%s/%s/%s", fn.ProjectID, fn.Name, revID)
	if store != nil {
		if err := store.Put(ctx, key, source, hash); err != nil {
			return nil, nil, err
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
	dep := &models.FunctionDeployment{
		ID:         utility.NewID(),
		ProjectID:  fn.ProjectID,
		Name:       fn.Name,
		RevisionID: revID,
		Environment: "production",
		Status:     "active",
		DeployedBy: deployedBy,
		CreatedAt:  now,
	}
	fn.ActiveRevisionID = revID
	fn.BinaryURL = key
	fn.Source = string(source)
	fn.UpdatedAt = now
	return rev, dep, nil
}

// RollbackDeployment points the function at a previous revision ID.
func RollbackDeployment(fn *models.ApitoFunction, previousRevisionID, deployedBy string) *models.FunctionDeployment {
	if fn == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	prev := fn.ActiveRevisionID
	fn.ActiveRevisionID = previousRevisionID
	fn.UpdatedAt = now
	return &models.FunctionDeployment{
		ID:         utility.NewID(),
		ProjectID:  fn.ProjectID,
		Name:       fn.Name,
		RevisionID: previousRevisionID,
		Environment: "production",
		Status:     "active",
		DeployedBy: deployedBy,
		RollbackOf: prev,
		CreatedAt:  now,
	}
}
