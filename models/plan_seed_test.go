package models

import "testing"

func TestValidatePlanSlugAgainstProject(t *testing.T) {
	p := &Project{}
	EnsureProjectPlansSeeds(p)
	slug, err := ValidatePlanSlugAgainstProject(p, "")
	if err != nil || slug != "free" {
		t.Fatalf("empty → free, got %q err=%v", slug, err)
	}
	slug, err = ValidatePlanSlugAgainstProject(p, "paid+")
	if err != nil || slug != "paid_plus" {
		t.Fatalf("paid+ → paid_plus, got %q err=%v", slug, err)
	}
	_, err = ValidatePlanSlugAgainstProject(p, "gold")
	if err == nil {
		t.Fatal("unknown slug must error before custom plan is defined")
	}
	p.Plans["gold"] = &Plan{ID: "gold", Name: "Gold"}
	slug, err = ValidatePlanSlugAgainstProject(p, "gold")
	if err != nil || slug != "gold" {
		t.Fatalf("custom plan should validate, got %q err=%v", slug, err)
	}
}

func TestDefaultSeededPlansPermissive(t *testing.T) {
	seeds := DefaultSeededPlans()
	for _, id := range []string{"free", "paid", "paid_plus", "ultra"} {
		pl, ok := seeds[id]
		if !ok || pl == nil {
			t.Fatalf("missing seed %s", id)
		}
		if !pl.SystemGenerated {
			t.Fatalf("%s should be system_generated", id)
		}
		ap := pl.APIPermissions["*"]
		if ap == nil || ap.Read != "all" || ap.Create != "all" {
			t.Fatalf("%s should be fully permissive: %#v", id, ap)
		}
	}
}
