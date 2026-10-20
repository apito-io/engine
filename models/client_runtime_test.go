package models

import "testing"

func TestParseClientRuntimeOneKeyExtras(t *testing.T) {
	parsed, err := ParseClientRuntime(map[string]interface{}{
		"android": map[string]interface{}{
			"min_version_name":  "2.5.0",
			"min_build_number":  float64(17),
			"force_update":      true,
			"store_url":         "https://play.google.com/store/apps/details?id=com.udbhabon.kisti",
			"maintenance":       false,
			"maintenance_hours": float64(0),
			"extras": []interface{}{
				map[string]interface{}{"min_corridor_m": "50"},
				map[string]interface{}{"hide_shop": "false"},
			},
		},
		"web": map[string]interface{}{
			"maintenance": true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Android == nil || !parsed.Android.ForceUpdate || parsed.Android.MinBuildNumber != 17 {
		t.Fatalf("android = %#v", parsed.Android)
	}
	if len(parsed.Android.Extras) != 2 || parsed.Android.Extras[0]["min_corridor_m"] != "50" {
		t.Fatalf("extras = %#v", parsed.Android.Extras)
	}
	if parsed.Web == nil || !parsed.Web.Maintenance || parsed.IOS != nil {
		t.Fatalf("platforms = %#v", parsed)
	}
	payload := PlatformRuntimePayload(parsed, "ios")
	if payload["force_update"] != false || payload["min_version_name"] != "0.0.0" {
		t.Fatalf("missing ios should allow, got %#v", payload)
	}
}

func TestParseClientRuntimeRejectsKeyValuePairs(t *testing.T) {
	_, err := ParseClientRuntime(map[string]interface{}{
		"android": map[string]interface{}{
			"extras": []interface{}{
				map[string]interface{}{"key": "min_corridor_m", "value": "50"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected key/value pairs to be rejected")
	}
}

func TestImportClientRuntimeFromRows(t *testing.T) {
	imported := ImportClientRuntimeFromRows(
		[]map[string]interface{}{{
			"platform":           "android",
			"min_version_name":   "2.5.0",
			"min_build_number":   float64(17),
			"force_update":       true,
			"system_maintenance": false,
			"maintenance_hours":  float64(0),
			"store_url":          "https://play.google.com/store/apps/details?id=com.udbhabon.kisti",
		}},
		[]map[string]interface{}{{
			"platform":       "android",
			"min_corridor_m": float64(50),
			"hide_shop":      false,
		}},
	)
	if imported == nil || imported.Android == nil {
		t.Fatal("expected imported android runtime")
	}
	if !imported.Android.ForceUpdate || imported.Android.MinBuildNumber != 17 {
		t.Fatalf("release fields = %#v", imported.Android)
	}
	if len(imported.Android.Extras) != 2 {
		t.Fatalf("extras = %#v", imported.Android.Extras)
	}
	if imported.Android.Extras[0]["min_corridor_m"] != "50" && imported.Android.Extras[1]["min_corridor_m"] != "50" {
		t.Fatalf("extras = %#v", imported.Android.Extras)
	}
}
