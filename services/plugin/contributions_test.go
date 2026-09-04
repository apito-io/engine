package plugin

import (
	"testing"

	"github.com/apito-io/types/protobuff"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestNormalizeContributionsDropsUnknownNavAndStorage(t *testing.T) {
	got := NormalizeContributions(&Contributions{
		UI: &UIContribution{
			Available: true,
			Navigation: []NavPlacement{
				{After: "storage", Label: "Cloudinary"},
				{After: "not-a-nav", Label: "Bad"},
			},
		},
		Fields: []FieldContribution{
			{ID: "cloudinary_asset", Label: "Asset", StorageType: "media"},
			{ID: "bad", Label: "Bad", StorageType: "magic"},
		},
	})
	if got == nil {
		t.Fatal("expected contributions")
	}
	if len(got.UI.Navigation) != 2 {
		t.Fatalf("nav len=%d", len(got.UI.Navigation))
	}
	if got.UI.Navigation[1].After != "" {
		t.Fatalf("unknown after kept: %q", got.UI.Navigation[1].After)
	}
	if len(got.Fields) != 1 || got.Fields[0].ID != "cloudinary_asset" {
		t.Fatalf("fields=%+v", got.Fields)
	}
}

func TestMergeRuntimeContributionsPrefersSchemaRegister(t *testing.T) {
	id := "hc-test-plugin"
	t.Cleanup(func() { SetPluginContributions(id, nil) })
	SetPluginContributions(id, &Contributions{
		API: &APIContribution{
			Scope:   "project",
			Queries: []APIOperation{{Name: "handWritten"}},
		},
	})
	queries, err := structpb.NewStruct(map[string]interface{}{
		"cloudinaryAssets": map[string]interface{}{"description": "live", "type": "JSON"},
	})
	if err != nil {
		t.Fatal(err)
	}
	MergeRuntimeContributions(id, SnapshotRuntimeAPI(&protobuff.ThirdPartyGraphQLSchemas{Queries: queries}, []*protobuff.ThirdPartyRESTApi{
		{Method: "GET", Path: "/assets", Description: "list"},
	}))
	got := ContributionsFor(id)
	if got == nil || got.API == nil {
		t.Fatal("missing api")
	}
	if len(got.API.Queries) != 1 || got.API.Queries[0].Name != "cloudinaryAssets" {
		t.Fatalf("queries=%+v", got.API.Queries)
	}
	if len(got.API.REST) != 1 || got.API.REST[0].Path != "/assets" {
		t.Fatalf("rest=%+v", got.API.REST)
	}
}

func TestConvertYAMLPluginLoadsContributions(t *testing.T) {
	id := "hc-yaml-contrib-plugin"
	t.Cleanup(func() {
		SetPluginCapabilities(id, nil)
		SetPluginContributions(id, nil)
	})
	_, err := convertYAMLPluginToProtobuf(YAMLPlugin{
		ID:           id,
		Language:     "go",
		Capabilities: []string{CapProjectGraphQL, CapConsoleRoutes, CapContentFields},
		Contributions: &Contributions{
			UI: &UIContribution{
				Available:  true,
				Navigation: []NavPlacement{{After: "storage", Label: "Media"}},
			},
			Fields: []FieldContribution{{ID: "asset", Label: "Asset", StorageType: "media", ContentForm: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := ContributionsFor(id)
	if got == nil || len(got.Fields) != 1 {
		t.Fatalf("contributions=%+v", got)
	}
}
