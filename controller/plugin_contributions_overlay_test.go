package controller

import (
	"testing"

	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/services/plugin/registry"
)

func TestOverlayCatalogContributionsUsesRuntimeIndexOnly(t *testing.T) {
	id := "hc-example-plugin"
	t.Cleanup(func() { pluginService.SetPluginContributions(id, nil) })

	empty := overlayCatalogContributions([]registry.MergedPlugin{
		{CatalogEntry: registry.CatalogEntry{ID: id, Name: "Example"}},
	})
	if len(empty) != 1 || empty[0].Contributions != nil {
		t.Fatalf("vendor-less plugin must not get baked-in contributions: %+v", empty[0].Contributions)
	}

	pluginService.SetPluginContributions(id, &pluginService.Contributions{
		UI: &pluginService.UIContribution{
			Available:  true,
			Navigation: []pluginService.NavPlacement{{After: "storage", Label: "Example"}},
		},
	})
	filled := overlayCatalogContributions([]registry.MergedPlugin{
		{CatalogEntry: registry.CatalogEntry{ID: id, Name: "Example"}},
	})
	if filled[0].Contributions == nil || len(filled[0].Contributions.UI.Navigation) != 1 {
		t.Fatalf("expected runtime overlay, got %+v", filled[0].Contributions)
	}
	if filled[0].Contributions.UI.Navigation[0].After != "storage" {
		t.Fatalf("after=%q", filled[0].Contributions.UI.Navigation[0].After)
	}
}
