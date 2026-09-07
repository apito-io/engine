package resolver

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apito-io/types"
)

const defaultCommonPromotePageSize = 500

var commonPromotePageSizeOverride int

// SetCommonModelPromotionPageSizeForTest shrinks pagination for migration tests.
func SetCommonModelPromotionPageSizeForTest(n int) {
	commonPromotePageSizeOverride = n
}

func commonPromotePageSize() int {
	if commonPromotePageSizeOverride > 0 {
		return commonPromotePageSizeOverride
	}
	return defaultCommonPromotePageSize
}

type commonPromoteIO struct {
	listTenant func(page, limit int) ([]*types.DefaultDocumentStructure, error)
	getBase    func(id string) (*types.DefaultDocumentStructure, error)
	addBase    func(doc *types.DefaultDocumentStructure) error
	baseHas    func(id string) (bool, error)
	reduce     func() error
}

func runCommonModelPromotion(pageSize int, io commonPromoteIO) (int, bool, error) {
	if pageSize <= 0 {
		pageSize = defaultCommonPromotePageSize
	}
	promoted := 0
	seen := map[string]struct{}{}
	var allIDs []string
	for page := 1; ; page++ {
		docs, err := io.listTenant(page, pageSize)
		if err != nil {
			return promoted, false, err
		}
		if len(docs) == 0 {
			break
		}
		for _, doc := range docs {
			if doc == nil || strings.TrimSpace(doc.ID) == "" {
				continue
			}
			id := strings.TrimSpace(doc.ID)
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				allIDs = append(allIDs, id)
			}
			existing, err := io.getBase(id)
			if err != nil {
				return promoted, false, err
			}
			if existing != nil {
				if err := commonPromotionConflict(existing, doc); err != nil {
					return promoted, false, err
				}
				continue
			}
			if doc.Meta == nil {
				doc.Meta = &types.MetaField{Status: "published"}
			}
			if err := io.addBase(doc); err != nil {
				return promoted, false, fmt.Errorf("promote %s: %w", id, err)
			}
			promoted++
		}
		if len(docs) < pageSize {
			break
		}
	}
	for _, id := range allIDs {
		ok, err := io.baseHas(id)
		if err != nil {
			return promoted, false, err
		}
		if !ok {
			return promoted, false, fmt.Errorf("promotion verification failed: id %q missing from base; tenant table not reduced", id)
		}
	}
	if err := io.reduce(); err != nil {
		return promoted, false, err
	}
	return promoted, true, nil
}

func commonPromotionConflict(existing, incoming *types.DefaultDocumentStructure) error {
	if existing == nil || incoming == nil {
		return nil
	}
	if len(existing.Data) == 0 || len(incoming.Data) == 0 {
		return nil
	}
	if dataMapsEqual(existing.Data, incoming.Data) {
		return nil
	}
	return fmt.Errorf("conflicting common-model id %q across tenants; resolve duplicates before promote", incoming.ID)
}

func dataMapsEqual(a, b map[string]interface{}) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

func validateCommonModelToggle(wantCommon bool, strategy string, hasRows bool, modelName string) error {
	strategy = strings.ToLower(strings.TrimSpace(strategy))
	if !wantCommon {
		return fmt.Errorf("demoting common model %q is not supported; keep it common or create a new tenant-scoped model", modelName)
	}
	if strategy != "" && strategy != "promote" {
		return fmt.Errorf("common_model_migration must be \"promote\" (demotion is not supported)")
	}
	if hasRows && strategy == "" {
		return fmt.Errorf("model %q already has documents; pass common_model_migration: promote to move storage", modelName)
	}
	return nil
}
