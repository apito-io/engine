package relationmigrate

import "testing"

func TestShapeMatrix(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"has_one", "has_one", ShapeOneToOne},
		{"has_many", "has_one", ShapeOneToMany},
		{"has_one", "has_many", ShapeManyToOne},
		{"has_many", "has_many", ShapeManyToMany},
	}
	for _, tc := range cases {
		if got := ShapeOf(tc.from, tc.to); got != tc.want {
			t.Fatalf("%s/%s = %s, want %s", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestWideningIsConditionalAndHashStable(t *testing.T) {
	req := Request{
		SchemaVersion: 10, FromModel: "lab_order", ToModel: "lab_test",
		FromRelation: "has_many", ToRelation: "has_one",
		TargetFromRelation: "has_many", TargetToRelation: "has_many",
	}
	snaps := []DatabaseSnapshot{{
		ScopeKey: "sqlite:/tmp/a.db", Capability: CapabilitySQL,
		Counts: Counts{SourcePairs: 3, ValidPairs: 2, Dangling: 1, ProjectedDestination: 2},
	}}
	p := BuildPreview(req, "FK on lab_test", "pivot lab_order_lab_test", snaps)
	if p.TransitionKind != KindWidenToNN || p.SafetyClass != SafetyLosslessWidening {
		t.Fatalf("kind/safety = %s/%s", p.TransitionKind, p.SafetyClass)
	}
	if p.Reversibility != RevNotBySchema {
		t.Fatalf("dangling widen reversibility = %s", p.Reversibility)
	}
	if p.Blocked || p.PlanHash == "" || !p.BackupRequired || !p.RequiresTypedConfirmation {
		t.Fatalf("preview flags: %+v", p)
	}
	again := BuildPreview(req, "FK on lab_test", "pivot lab_order_lab_test", snaps)
	if again.PlanHash != p.PlanHash {
		t.Fatal("plan hash drifted")
	}
	snaps[0].Counts.ValidPairs = 9
	if BuildPreview(req, "", "", snaps).PlanHash == p.PlanHash {
		t.Fatal("count change must change the plan hash")
	}
}

func TestNarrowingBlocksDegree(t *testing.T) {
	req := Request{
		FromModel: "lab_order", ToModel: "lab_test",
		FromRelation: "has_many", ToRelation: "has_many",
		TargetFromRelation: "has_many", TargetToRelation: "has_one",
	}
	p := BuildPreview(req, "pivot", "FK", []DatabaseSnapshot{{
		ScopeKey: "db", Counts: Counts{ValidPairs: 4, Conflicts: 2, ProjectedDestination: 4},
	}})
	if !p.Blocked || p.SafetyClass != SafetyBlocked {
		t.Fatalf("expected blocked, got %+v", p)
	}
	if len(p.ConflictReport) == 0 {
		t.Fatal("missing conflict report")
	}
}

func TestNoOp(t *testing.T) {
	req := Request{FromModel: "a", ToModel: "b", FromRelation: "has_one", ToRelation: "has_many", TargetFromRelation: "has_one", TargetToRelation: "has_many"}
	p := BuildPreview(req, "fk", "fk", nil)
	if p.TransitionKind != KindNoOp || p.Blocked || p.BackupRequired {
		t.Fatalf("%+v", p)
	}
}
