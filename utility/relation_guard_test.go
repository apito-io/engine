package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility/securitytest"
)

func TestValidateProjectUserRelationEdges(t *testing.T) {
	role := &models.Role{ID: "passenger", IsProjectUser: true}
	if err := ValidateProjectUserRelationEdges(role, securitytest.FixtureUserA, map[string]interface{}{"owner_id": securitytest.FixtureUserA}, nil); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProjectUserRelationEdges(role, securitytest.FixtureUserA, map[string]interface{}{"owner_id": securitytest.FixtureUserB}, nil); err != ErrRelationTargetForbidden {
		t.Fatalf("cross-user connect: %v", err)
	}
	if err := ValidateProjectUserRelationEdges(role, securitytest.FixtureUserA, nil, map[string]interface{}{"owner_id": securitytest.FixtureUserA}); err != ErrRelationTargetForbidden {
		t.Fatalf("owner disconnect: %v", err)
	}
	if err := ValidateProjectUserRelationEdges(role, securitytest.FixtureUserA, map[string]interface{}{
		"owner_id": map[string]interface{}{"connect": []interface{}{securitytest.FixtureUserB}},
	}, nil); err != ErrRelationTargetForbidden {
		t.Fatalf("nested cross-user connect: %v", err)
	}
	if err := ValidateProjectUserRelationEdges(role, securitytest.FixtureUserA, map[string]interface{}{
		"owner_id": map[string]interface{}{"id": securitytest.FixtureUserA},
	}, nil); err != nil {
		t.Fatalf("nested self connect: %v", err)
	}
	admin := &models.Role{ID: "admin", IsAdmin: true, IsProjectUser: true}
	if err := ValidateProjectUserRelationEdges(admin, securitytest.FixtureUserA, map[string]interface{}{"owner_id": securitytest.FixtureUserB}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRelationPolicySurvivesDriverParamConstruction(t *testing.T) {
	role := &models.Role{ID: "passenger", IsProjectUser: true}
	params := []*models.ConnectDisconnectParam{{
		RequestedRelationField: "owner_id",
		ActionIDs:              []string{securitytest.FixtureUserB},
	}}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != ErrRelationTargetForbidden {
		t.Fatalf("driver connect must reject cross-user owner: %v", err)
	}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, true); err != ErrRelationTargetForbidden {
		t.Fatalf("driver disconnect must reject owner edge: %v", err)
	}
	params[0].ActionIDs = []string{securitytest.FixtureUserA}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != nil {
		t.Fatal(err)
	}
}

func TestRelationPolicyRecognizesArbitraryUserTargetMetadata(t *testing.T) {
	role := &models.Role{ID: "passenger", IsProjectUser: true}
	params := []*models.ConnectDisconnectParam{{
		RequestedRelationField: "assignee_ref",
		TargetModelName:        "user",
		TargetIsUserModel:      true,
		ActionIDs:              []string{securitytest.FixtureUserB},
	}}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != ErrRelationTargetForbidden {
		t.Fatalf("arbitrary user relation must reject cross-user connect: %v", err)
	}
	params[0].ActionIDs = []string{securitytest.FixtureUserA}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != nil {
		t.Fatalf("self connect should pass: %v", err)
	}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, true); err != ErrRelationTargetForbidden {
		t.Fatalf("arbitrary user relation disconnect must be denied: %v", err)
	}
}

func TestOwnScopedRelationRequiresVerifiedTarget(t *testing.T) {
	role := &models.Role{
		ID:            "passenger",
		IsProjectUser: true,
		APIPermissions: map[string]*models.APIPermission{
			"saved_train": {Read: "own", Update: "own", Delete: "own"},
		},
	}
	params := []*models.ConnectDisconnectParam{{
		RequestedRelationField:  "saved_train_id",
		TargetModelName:         "saved_train",
		TargetRequiresOwnership: true,
		ActionIDs:               []string{"saved-a"},
	}}
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != ErrRelationTargetForbidden {
		t.Fatalf("unverified own target must be denied: %v", err)
	}
	params[0].MarkTargetOwnershipVerified()
	if err := ValidateProjectUserRelationParams(role, securitytest.FixtureUserA, params, false); err != nil {
		t.Fatalf("verified own target should pass driver boundary: %v", err)
	}
}

func TestOwnerFieldMutationIsImmutableForProjectUsers(t *testing.T) {
	role := &models.Role{ID: "passenger", IsProjectUser: true}
	before := map[string]interface{}{"owner_id": securitytest.FixtureUserA}
	if err := ValidateOwnerFieldMutation(role, securitytest.FixtureUserA, before, map[string]interface{}{"owner_id": securitytest.FixtureUserB}); err != ErrRelationTargetForbidden {
		t.Fatalf("owner transfer must be denied: %v", err)
	}
	if err := ValidateOwnerFieldMutation(role, securitytest.FixtureUserA, before, map[string]interface{}{}); err != nil {
		t.Fatalf("omitted owner field should remain unchanged: %v", err)
	}
	if err := ValidateOwnerFieldMutation(role, securitytest.FixtureUserA, before, before); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerFieldCreateIsSelfOnlyAndFailClosed(t *testing.T) {
	role := &models.Role{ID: "passenger", IsProjectUser: true}

	for name, payload := range map[string]map[string]interface{}{
		"self owner id":  map[string]interface{}{"owner_id": securitytest.FixtureUserA},
		"self owner uid": map[string]interface{}{"owner_uid": securitytest.FixtureUserA},
		"self owner pair": map[string]interface{}{
			"owner_id":  securitytest.FixtureUserA,
			"owner_uid": securitytest.FixtureUserA,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateOwnerFieldOnCreate(role, securitytest.FixtureUserA, payload); err != nil {
				t.Fatalf("self owner should pass: %v", err)
			}
		})
	}

	for name, payload := range map[string]map[string]interface{}{
		"foreign owner":    map[string]interface{}{"owner_id": securitytest.FixtureUserB},
		"conflicting pair": map[string]interface{}{"owner_id": securitytest.FixtureUserA, "owner_uid": securitytest.FixtureUserB},
		"malformed owner":  map[string]interface{}{"owner_id": map[string]interface{}{"unexpected": "shape"}},
		"empty owner":      map[string]interface{}{"owner_id": ""},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateOwnerFieldOnCreate(role, securitytest.FixtureUserA, payload); err != ErrRelationTargetForbidden {
				t.Fatalf("unsafe owner payload should deny: %v", err)
			}
		})
	}

	if err := ValidateOwnerFieldOnCreate(role, securitytest.FixtureUserA, map[string]interface{}{"title": "safe"}); err != nil {
		t.Fatalf("payload without owner fields should pass: %v", err)
	}
}
