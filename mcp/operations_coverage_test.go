package mcp

import (
	"strings"
	"testing"
)

func TestCatalogToolsAreWired(t *testing.T) {
	for _, tool := range Catalog() {
		if _, ok := guides()[tool.Name]; ok {
			continue
		}
		switch tool.Name {
		case "prepare_project_scope", "confirm_project_scope", "get_project_scope",
			"inspect_access_token", "publish_schema",
			"list_media", "upload_media_from_url", "delete_media", "execute_function":
			continue
		}
		if _, ok := operationQuery(tool.Name); !ok {
			t.Errorf("%s has no system operation mapping", tool.Name)
		}
	}
}

func TestOperationsNeverPublishSchema(t *testing.T) {
	for name, query := range operations {
		if strings.Contains(query, "approveSchemaChanges") || strings.Contains(query, "publishSchema") {
			t.Errorf("%s publishes schema: %s", name, query)
		}
	}
	h := &Handler{}
	_, handled, err := h.localTool("publish_schema", nil)
	if !handled || err == nil || !strings.Contains(err.Error(), "does not publish") {
		t.Fatalf("publish_schema handled=%v err=%v", handled, err)
	}
}

func TestAliasArgsDataNames(t *testing.T) {
	out := aliasArgs("upsert_data", map[string]any{"model": "order", "id": "1", "data": map[string]any{"n": 1}})
	if out["model_name"] != "order" || out["_id"] != "1" {
		t.Fatalf("alias = %#v", out)
	}
	logs := aliasArgs("search_system_logs", map[string]any{"q": "timeout"})
	if logs["text"] != "timeout" {
		t.Fatalf("logs = %#v", logs)
	}
	order := aliasArgs("reorder_fields", map[string]any{"serial": 2})
	if order["new_position"] != 2 || order["move_type"] != "reorder" {
		t.Fatalf("reorder = %#v", order)
	}
}

func TestQueryToolsDoNotRequireWriteLease(t *testing.T) {
	for _, name := range []string{
		"google_oauth_state", "login_app_user", "login_app_user_google",
		"summarize_schema_draft_for_review", "summarize_system_logs",
	} {
		tool, ok := findTool(name)
		if !ok || tool.Access != accessRead {
			t.Fatalf("%s access = %q", name, tool.Access)
		}
	}
}
