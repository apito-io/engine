package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/types/protobuff"
)

func TestApplyPluginUpsertAppendsWhenProjectAlreadyHasPlugins(t *testing.T) {
	project := &models.Project{
		ID: "p1",
		Plugins: []*models.SavedPluginDetails{
			{ID: "hc-old-plugin", Enable: true},
		},
	}
	catalog := &models.SavedPluginDetails{ID: "hc-new-plugin"}
	enable := true
	got, err := applyPluginUpsert(project, catalog, pluginUpsertInput{
		ID:     "hc-new-plugin",
		Enable: &enable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "hc-new-plugin" || !got.Enable {
		t.Fatalf("got %+v", got)
	}
	if got.ActivateStatus != protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED {
		t.Fatalf("activate_status=%v", got.ActivateStatus)
	}
	if len(project.Plugins) != 2 {
		t.Fatalf("len=%d", len(project.Plugins))
	}
}

func TestApplyPluginUpsertHonorsEnableFalse(t *testing.T) {
	project := &models.Project{
		ID: "p1",
		Plugins: []*models.SavedPluginDetails{
			{ID: "hc-foo-plugin", Enable: true, ActivateStatus: protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED},
		},
	}
	enable := false
	got, err := applyPluginUpsert(project, nil, pluginUpsertInput{
		ID:     "hc-foo-plugin",
		Enable: &enable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Enable {
		t.Fatal("expected enable=false")
	}
	if got.ActivateStatus != protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED {
		t.Fatalf("activate_status=%v", got.ActivateStatus)
	}
}

func TestApplyPluginUpsertActivateStatusNotInverted(t *testing.T) {
	project := &models.Project{
		ID:      "p1",
		Plugins: []*models.SavedPluginDetails{{ID: "hc-foo-plugin"}},
		Settings: &models.ProjectSettings{
			DefaultStoragePlugin: "keep-me",
		},
	}
	activated := protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED
	got, err := applyPluginUpsert(project, nil, pluginUpsertInput{
		ID:             "hc-foo-plugin",
		ActivateStatus: &activated,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ActivateStatus != protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED {
		t.Fatalf("inverted? %v", got.ActivateStatus)
	}
	if !got.Enable {
		t.Fatal("enable should follow activated")
	}
	if project.Settings.DefaultStoragePlugin != "keep-me" {
		t.Fatal("must not rewrite DefaultStoragePlugin")
	}
}

func TestRemoveProjectPlugin(t *testing.T) {
	plugins := []*models.SavedPluginDetails{
		{ID: "hc-a-plugin"},
		{ID: "hc-b-plugin"},
	}
	got := removeProjectPlugin(plugins, "hc-a-plugin")
	if len(got) != 1 || got[0].ID != "hc-b-plugin" {
		t.Fatalf("got %#v", got)
	}
}

func TestIsProjectPluginActivated(t *testing.T) {
	project := &models.Project{
		Plugins: []*models.SavedPluginDetails{
			{ID: "hc-on-plugin", Enable: true},
			{ID: "hc-off-plugin", Enable: false, ActivateStatus: protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED},
		},
	}
	if !isProjectPluginActivated(project, "hc-on-plugin") {
		t.Fatal("on")
	}
	if isProjectPluginActivated(project, "hc-off-plugin") {
		t.Fatal("off")
	}
	if isProjectPluginActivated(project, "hc-missing-plugin") {
		t.Fatal("missing")
	}
}

func TestPluginRESTRequiresActivation(t *testing.T) {
	pluginService.SetPluginCapabilities("hc-proj-rest", []string{pluginService.CapProjectREST})
	pluginService.SetPluginCapabilities("hc-sys-rest", []string{pluginService.CapSystemREST})
	pluginService.SetPluginCapabilities("hc-unknown-caps", nil)
	t.Cleanup(func() {
		pluginService.SetPluginCapabilities("hc-proj-rest", nil)
		pluginService.SetPluginCapabilities("hc-sys-rest", nil)
		pluginService.SetPluginCapabilities("hc-unknown-caps", nil)
	})
	if !pluginRESTRequiresActivation("hc-proj-rest") {
		t.Fatal("project rest must be gated")
	}
	if pluginRESTRequiresActivation("hc-sys-rest") {
		t.Fatal("system-only rest must not be gated")
	}
	if !pluginRESTRequiresActivation("hc-unknown-caps") {
		t.Fatal("unknown caps default to gated")
	}
}

func TestPluginRequiresProjectActivation(t *testing.T) {
	pluginService.SetPluginCapabilities("hc-proj-plugin", []string{pluginService.CapProjectREST})
	pluginService.SetPluginCapabilities("hc-sys-plugin", []string{pluginService.CapSystemREST})
	t.Cleanup(func() {
		pluginService.SetPluginCapabilities("hc-proj-plugin", nil)
		pluginService.SetPluginCapabilities("hc-sys-plugin", nil)
	})
	if !pluginRequiresProjectActivation("hc-proj-plugin") {
		t.Fatal("project rest should require activation")
	}
	if pluginRequiresProjectActivation("hc-sys-plugin") {
		t.Fatal("system rest should not require activation")
	}
}

func TestEnvVarsMap(t *testing.T) {
	got := envVarsMap(&models.SavedPluginDetails{
		EnvVars: []*protobuff.EnvVariable{
			{Key: "DISCORD_WEBHOOK_URL", Value: "https://discord.example/hook"},
			{Key: "", Value: "skip"},
			nil,
		},
	})
	if got["DISCORD_WEBHOOK_URL"] != "https://discord.example/hook" {
		t.Fatalf("got %#v", got)
	}
	if envVarsMap(nil) != nil {
		t.Fatal("nil details")
	}
}

func TestMergeYAMLAndProjectEnvVars(t *testing.T) {
	got := mergeYAMLAndProjectEnvVars(
		[]*protobuff.EnvVariable{
			{Key: "CLOUDINARY_CLOUD_NAME", Value: ""},
			{Key: "CLOUDINARY_API_KEY", Value: ""},
		},
		[]*protobuff.EnvVariable{
			{Key: "CLOUDINARY_CLOUD_NAME", Value: "demo"},
			{Key: "EXTRA", Value: "x"},
		},
	)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Key != "CLOUDINARY_CLOUD_NAME" || got[0].Value != "demo" {
		t.Fatalf("cloud name %+v", got[0])
	}
	if got[1].Key != "CLOUDINARY_API_KEY" || got[1].Value != "" {
		t.Fatalf("api key %+v", got[1])
	}
	if got[2].Key != "EXTRA" || got[2].Value != "x" {
		t.Fatalf("extra %+v", got[2])
	}
}

func TestApplyPluginUpsertClonesCatalogEnvVars(t *testing.T) {
	yamlEnv := []*protobuff.EnvVariable{{Key: "CLOUDINARY_CLOUD_NAME", Value: ""}}
	project := &models.Project{ID: "p1"}
	catalog := &models.SavedPluginDetails{ID: "hc-cloudinary-plugin", EnvVars: yamlEnv}
	enable := true
	got, err := applyPluginUpsert(project, catalog, pluginUpsertInput{
		ID:     "hc-cloudinary-plugin",
		Enable: &enable,
	})
	if err != nil {
		t.Fatal(err)
	}
	got.EnvVars[0].Value = "mutated"
	if yamlEnv[0].Value != "" {
		t.Fatal("yaml env slice must not be shared with project record")
	}
}

func TestPluginExecuteContextAddsPluginID(t *testing.T) {
	s := &GraphQLServer{}
	got := s.pluginExecuteContext(context.Background(), "hc-discord-plugin", "")
	if got["plugin_id"] != "hc-discord-plugin" {
		t.Fatalf("got %#v", got)
	}
	if _, ok := got["env_vars"]; ok {
		t.Fatal("no project → no env_vars")
	}
}

func TestEnforcePluginGraphQLActivation(t *testing.T) {
	pluginService.SetPluginCapabilities("hc-sys-gql", []string{pluginService.CapSystemGraphQL})
	pluginService.SetPluginCapabilities("hc-proj-gql", []string{pluginService.CapProjectGraphQL})
	t.Cleanup(func() {
		pluginService.SetPluginCapabilities("hc-sys-gql", nil)
		pluginService.SetPluginCapabilities("hc-proj-gql", nil)
	})
	s := &GraphQLServer{}
	if err := s.enforcePluginGraphQLActivation(context.Background(), "hc-sys-gql"); err != nil {
		t.Fatal(err)
	}
	if err := s.enforcePluginGraphQLActivation(context.Background(), "hc-proj-gql"); err != errPluginNotActivated {
		t.Fatalf("got %v", err)
	}
}
