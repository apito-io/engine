package models

import "testing"

func TestValidatePlanSlugAgainstProject(t *testing.T) {
	p := &Project{}
	EnsureProjectPlansSeeds(p)
	slug, err := ValidatePlanSlugAgainstProject(p, "")
	if err != nil || slug != "free" {
		t.Fatalf("empty → free, got %q err=%v", slug, err)
	}
	_, err = ValidatePlanSlugAgainstProject(p, "paid+")
	if err == nil {
		t.Fatal("paid+ is not seeded by default; must error until a custom plan exists")
	}
	p.Plans["paid_plus"] = &Plan{ID: "paid_plus", Name: "Pro Plus"}
	slug, err = ValidatePlanSlugAgainstProject(p, "paid+")
	if err != nil || slug != "paid_plus" {
		t.Fatalf("paid+ → paid_plus when custom exists, got %q err=%v", slug, err)
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

func TestEnsureProjectPlansSeedsOnlyFree(t *testing.T) {
	p := &Project{}
	EnsureProjectPlansSeeds(p)
	if len(p.Plans) != 1 {
		t.Fatalf("expected only free seed, got %d plans", len(p.Plans))
	}
	free := p.Plans["free"]
	if free == nil || !free.SystemGenerated {
		t.Fatal("free must seed as system_generated")
	}
}

func TestEnsureProjectPlansSeedsDemotesLegacySystemPlans(t *testing.T) {
	p := &Project{
		Plans: map[string]*Plan{
			"paid":      {ID: "paid", Name: "Pro", SystemGenerated: true},
			"paid_plus": {ID: "paid_plus", Name: "Paid Plus", SystemGenerated: true},
			"ultra":     {ID: "ultra", Name: "Ultra", SystemGenerated: true},
		},
	}
	EnsureProjectPlansSeeds(p)
	if p.Plans["free"] == nil || !p.Plans["free"].SystemGenerated {
		t.Fatal("free must be present and system_generated")
	}
	for _, id := range []string{"paid", "paid_plus", "ultra"} {
		pl := p.Plans[id]
		if pl == nil {
			t.Fatalf("legacy %s should remain until operator deletes via Console/MCP/script", id)
		}
		if pl.SystemGenerated {
			t.Fatalf("%s must demote to custom (system_generated=false)", id)
		}
	}
}

func TestDefaultSeededPlansOnlyFree(t *testing.T) {
	seeds := DefaultSeededPlans()
	if len(seeds) != 1 || seeds["free"] == nil {
		t.Fatalf("expected only free, got %#v", seeds)
	}
	pl := seeds["free"]
	if !pl.SystemGenerated {
		t.Fatal("free should be system_generated")
	}
	ap := pl.APIPermissions["*"]
	if ap == nil || ap.Read != "all" || ap.Create != "all" {
		t.Fatalf("free should be fully permissive: %#v", ap)
	}
}
