package resolver

import (
	"testing"

	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
)

func TestIsOrderModelName(t *testing.T) {
	if !isOrderModelName("order") || !isOrderModelName("Orders") || !isOrderModelName("invoice") {
		t.Fatal("expected order-like names")
	}
	if isOrderModelName("patient") {
		t.Fatal("patient is not an order model")
	}
}

func TestActivatedEventSinkIDs(t *testing.T) {
	pluginService.SetPluginCapabilities("hc-discord-plugin", []string{pluginService.CapEventSink, pluginService.CapProjectREST})
	pluginService.SetPluginCapabilities("hc-stripe-plugin", []string{pluginService.CapProjectREST})
	t.Cleanup(func() {
		pluginService.SetPluginCapabilities("hc-discord-plugin", nil)
		pluginService.SetPluginCapabilities("hc-stripe-plugin", nil)
	})
	project := &models.Project{
		Plugins: []*models.SavedPluginDetails{
			{ID: "hc-discord-plugin", Enable: true},
			{ID: "hc-stripe-plugin", Enable: true},
			{ID: "hc-discord-plugin-off", Enable: false},
		},
	}
	ids := activatedEventSinkIDs(project)
	if len(ids) != 1 || ids[0] != "hc-discord-plugin" {
		t.Fatalf("got %#v", ids)
	}
}
