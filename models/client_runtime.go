package models

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	clientRuntimeMaxExtras      = 32
	clientRuntimeMaxExtraValue  = 500
	clientRuntimeMaxMessage     = 2000
	clientRuntimeMaxVersion     = 64
	clientRuntimeDefaultVersion = "0.0.0"
)

var clientRuntimeExtraKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ClientRuntimeExtraKeys are copied from app_config when a project has no
// client_runtime yet. They stay developer-defined values, not engine fields.
var ClientRuntimeExtraKeys = []string{
	"min_corridor_m",
	"max_corridor_m",
	"accuracy_pad_m",
	"max_accuracy_m",
	"station_yard_pending_m",
	"ad_skip_on_fill_fail",
	"hide_shop",
	"hide_food_orders",
}

// ClientRuntime is the per-project pre-login config stored on project_settings.
type ClientRuntime struct {
	Android *PlatformClientRuntime `json:"android,omitempty"`
	IOS     *PlatformClientRuntime `json:"ios,omitempty"`
	Web     *PlatformClientRuntime `json:"web,omitempty"`
}

// PlatformClientRuntime is one of android, ios, or web.
// Extras are one-key objects, for example [{"min_corridor_m":"50"}].
type PlatformClientRuntime struct {
	MinVersionName   string              `json:"min_version_name,omitempty"`
	MinBuildNumber   int                 `json:"min_build_number,omitempty"`
	ForceUpdate      bool                `json:"force_update,omitempty"`
	StoreURL         string              `json:"store_url,omitempty"`
	MessageEn        string              `json:"message_en,omitempty"`
	MessageBn        string              `json:"message_bn,omitempty"`
	Maintenance      bool                `json:"maintenance,omitempty"`
	MaintenanceHours int                 `json:"maintenance_hours,omitempty"`
	Extras           []map[string]string `json:"extras,omitempty"`
}

// IsEmpty reports whether nothing has been saved for any platform.
func (c *ClientRuntime) IsEmpty() bool {
	if c == nil {
		return true
	}
	return platformRuntimeEmpty(c.Android) && platformRuntimeEmpty(c.IOS) && platformRuntimeEmpty(c.Web)
}

func platformRuntimeEmpty(p *PlatformClientRuntime) bool {
	if p == nil {
		return true
	}
	return p.MinVersionName == "" &&
		p.MinBuildNumber == 0 &&
		!p.ForceUpdate &&
		p.StoreURL == "" &&
		p.MessageEn == "" &&
		p.MessageBn == "" &&
		!p.Maintenance &&
		p.MaintenanceHours == 0 &&
		len(p.Extras) == 0
}

// ParseClientRuntime validates a console write. Extras must be a list of
// one-key objects such as [{"min_corridor_m":"50"}], not {key, value} pairs.
func ParseClientRuntime(raw map[string]interface{}) (*ClientRuntime, error) {
	if raw == nil {
		return &ClientRuntime{}, nil
	}
	for key := range raw {
		switch key {
		case "android", "ios", "web":
		default:
			return nil, fmt.Errorf("client_runtime platform %q is not android, ios, or web", key)
		}
	}
	android, err := parsePlatformClientRuntime(raw["android"])
	if err != nil {
		return nil, fmt.Errorf("android: %w", err)
	}
	ios, err := parsePlatformClientRuntime(raw["ios"])
	if err != nil {
		return nil, fmt.Errorf("ios: %w", err)
	}
	web, err := parsePlatformClientRuntime(raw["web"])
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	return &ClientRuntime{Android: android, IOS: ios, Web: web}, nil
}

func parsePlatformClientRuntime(raw interface{}) (*PlatformClientRuntime, error) {
	if raw == nil {
		return nil, nil
	}
	fields, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("must be an object")
	}
	for key := range fields {
		switch key {
		case "min_version_name", "min_build_number", "force_update", "store_url",
			"message_en", "message_bn", "maintenance", "maintenance_hours", "extras":
		default:
			return nil, fmt.Errorf("unknown field %q", key)
		}
	}
	out := &PlatformClientRuntime{}
	if v, ok := fields["min_version_name"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("min_version_name must be a string")
		}
		s = strings.TrimSpace(s)
		if len(s) > clientRuntimeMaxVersion {
			return nil, fmt.Errorf("min_version_name is too long")
		}
		out.MinVersionName = s
	}
	if v, ok := fields["min_build_number"]; ok && v != nil {
		n, err := clientRuntimeInt(v, "min_build_number")
		if err != nil {
			return nil, err
		}
		out.MinBuildNumber = n
	}
	if v, ok := fields["force_update"]; ok && v != nil {
		b, err := clientRuntimeBool(v, "force_update")
		if err != nil {
			return nil, err
		}
		out.ForceUpdate = b
	}
	if v, ok := fields["store_url"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("store_url must be a string")
		}
		s = strings.TrimSpace(s)
		if !validClientRuntimeURL(s) {
			return nil, fmt.Errorf("store_url must be empty or an http(s) URL")
		}
		out.StoreURL = s
	}
	if v, ok := fields["message_en"]; ok && v != nil {
		s, err := clientRuntimeMessage(v, "message_en")
		if err != nil {
			return nil, err
		}
		out.MessageEn = s
	}
	if v, ok := fields["message_bn"]; ok && v != nil {
		s, err := clientRuntimeMessage(v, "message_bn")
		if err != nil {
			return nil, err
		}
		out.MessageBn = s
	}
	if v, ok := fields["maintenance"]; ok && v != nil {
		b, err := clientRuntimeBool(v, "maintenance")
		if err != nil {
			return nil, err
		}
		out.Maintenance = b
	}
	if v, ok := fields["maintenance_hours"]; ok && v != nil {
		n, err := clientRuntimeInt(v, "maintenance_hours")
		if err != nil {
			return nil, err
		}
		out.MaintenanceHours = n
	}
	if v, ok := fields["extras"]; ok && v != nil {
		extras, err := parseClientRuntimeExtras(v)
		if err != nil {
			return nil, err
		}
		out.Extras = extras
	}
	return out, nil
}

func parseClientRuntimeExtras(raw interface{}) ([]map[string]string, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("extras must be a list of one-key objects")
	}
	if len(list) > clientRuntimeMaxExtras {
		return nil, fmt.Errorf("extras cannot exceed %d keys", clientRuntimeMaxExtras)
	}
	seen := map[string]struct{}{}
	out := make([]map[string]string, 0, len(list))
	for _, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok || len(obj) != 1 {
			return nil, fmt.Errorf("each extra must be an object with exactly one key")
		}
		for key, value := range obj {
			if !clientRuntimeExtraKey.MatchString(key) {
				return nil, fmt.Errorf("extra key %q is invalid", key)
			}
			if _, dup := seen[key]; dup {
				return nil, fmt.Errorf("duplicate extra key %q", key)
			}
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("extra %q must be a string", key)
			}
			if len(text) > clientRuntimeMaxExtraValue {
				return nil, fmt.Errorf("extra %q is too long", key)
			}
			seen[key] = struct{}{}
			out = append(out, map[string]string{key: text})
		}
	}
	return out, nil
}

func clientRuntimeMessage(v interface{}, name string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", name)
	}
	s = strings.TrimSpace(s)
	if len(s) > clientRuntimeMaxMessage {
		return "", fmt.Errorf("%s is too long", name)
	}
	return s, nil
}

func clientRuntimeBool(v interface{}, name string) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return b, nil
}

func clientRuntimeInt(v interface{}, name string) (int, error) {
	var n int
	switch t := v.(type) {
	case int:
		n = t
	case int32:
		n = int(t)
	case int64:
		n = int(t)
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) || t != math.Trunc(t) {
			return 0, fmt.Errorf("%s must be an integer", name)
		}
		n = int(t)
	default:
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if n < 0 {
		return 0, fmt.Errorf("%s must be >= 0", name)
	}
	return n, nil
}

func validClientRuntimeURL(raw string) bool {
	if raw == "" {
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

// ClientRuntimeSnapshot is the admin settings shape: one block per platform.
func ClientRuntimeSnapshot(runtime *ClientRuntime) map[string]interface{} {
	return map[string]interface{}{
		"android": PlatformRuntimePayload(runtime, "android"),
		"ios":     PlatformRuntimePayload(runtime, "ios"),
		"web":     PlatformRuntimePayload(runtime, "web"),
	}
}

// PlatformRuntimePayload is the public pre-login read for one platform.
// A missing block allows the app through: force update and maintenance are off.
func PlatformRuntimePayload(runtime *ClientRuntime, platform string) map[string]interface{} {
	var stored *PlatformClientRuntime
	if runtime != nil {
		switch platform {
		case "android":
			stored = runtime.Android
		case "ios":
			stored = runtime.IOS
		case "web":
			stored = runtime.Web
		}
	}
	if stored == nil {
		stored = &PlatformClientRuntime{}
	}
	version := stored.MinVersionName
	if version == "" {
		version = clientRuntimeDefaultVersion
	}
	extras := stored.Extras
	if extras == nil {
		extras = []map[string]string{}
	}
	return map[string]interface{}{
		"platform":          platform,
		"min_version_name":  version,
		"min_build_number":  stored.MinBuildNumber,
		"force_update":      stored.ForceUpdate,
		"store_url":         stored.StoreURL,
		"message_en":        stored.MessageEn,
		"message_bn":        stored.MessageBn,
		"maintenance":       stored.Maintenance,
		"maintenance_hours": stored.MaintenanceHours,
		"extras":            extras,
	}
}

// NormalizeClientRuntimePlatform accepts only android, ios, and web.
func NormalizeClientRuntimePlatform(platform string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "android", "ios", "web":
		return strings.ToLower(strings.TrimSpace(platform)), true
	default:
		return "", false
	}
}

// ImportClientRuntimeFromRows copies legacy common-model rows into project settings.
// release rows supply the shared fields. config rows add known extras as one-key objects.
// A nil result means there was nothing to copy.
func ImportClientRuntimeFromRows(releaseRows, configRows []map[string]interface{}) *ClientRuntime {
	out := &ClientRuntime{}
	found := false
	for _, row := range releaseRows {
		platform, ok := NormalizeClientRuntimePlatform(clientRuntimeString(row["platform"]))
		if !ok {
			continue
		}
		block := importReleaseBlock(row)
		setImportedPlatform(out, platform, block)
		found = true
	}
	for _, row := range configRows {
		platform, ok := NormalizeClientRuntimePlatform(clientRuntimeString(row["platform"]))
		if !ok {
			continue
		}
		extras := importExtraObjects(row)
		imported := importReleaseBlock(row)
		if len(extras) == 0 && platformRuntimeEmpty(imported) {
			continue
		}
		block := platformByName(out, platform)
		if block == nil {
			block = imported
			setImportedPlatform(out, platform, block)
			found = true
		}
		if len(extras) > 0 {
			block.Extras = mergeExtraObjects(block.Extras, extras)
			found = true
		}
	}
	if !found || out.IsEmpty() {
		return nil
	}
	return out
}

func importReleaseBlock(row map[string]interface{}) *PlatformClientRuntime {
	maintenance := clientRuntimeBoolish(row["maintenance"]) || clientRuntimeBoolish(row["system_maintenance"])
	return &PlatformClientRuntime{
		MinVersionName:   clientRuntimeString(row["min_version_name"]),
		MinBuildNumber:   clientRuntimeLooseInt(row["min_build_number"]),
		ForceUpdate:      clientRuntimeBoolish(row["force_update"]),
		StoreURL:         clientRuntimeString(row["store_url"]),
		MessageEn:        clientRuntimeString(row["message_en"]),
		MessageBn:        clientRuntimeString(row["message_bn"]),
		Maintenance:      maintenance,
		MaintenanceHours: clientRuntimeLooseInt(firstPresent(row, "maintenance_hours", "maintenance_countdown_hours")),
	}
}

func importExtraObjects(row map[string]interface{}) []map[string]string {
	var out []map[string]string
	for _, key := range ClientRuntimeExtraKeys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		text := strings.TrimSpace(clientRuntimeLooseString(value))
		if text == "" {
			continue
		}
		out = append(out, map[string]string{key: text})
	}
	return out
}

func mergeExtraObjects(base, extra []map[string]string) []map[string]string {
	seen := map[string]struct{}{}
	out := make([]map[string]string, 0, len(base)+len(extra))
	for _, item := range base {
		for key := range item {
			seen[key] = struct{}{}
		}
		out = append(out, item)
	}
	for _, item := range extra {
		for key := range item {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func setImportedPlatform(runtime *ClientRuntime, platform string, block *PlatformClientRuntime) {
	switch platform {
	case "android":
		runtime.Android = block
	case "ios":
		runtime.IOS = block
	case "web":
		runtime.Web = block
	}
}

// ClientRuntimeExtra reads one extra from a platform block. Extras are a list
// of one-key objects. An unknown platform or a missing key returns empty.
func ClientRuntimeExtra(runtime *ClientRuntime, platform, key string) string {
	name, ok := NormalizeClientRuntimePlatform(platform)
	if !ok || runtime == nil {
		return ""
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	block := platformByName(runtime, name)
	if block == nil {
		return ""
	}
	for _, item := range block.Extras {
		if value, found := item[key]; found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func platformByName(runtime *ClientRuntime, platform string) *PlatformClientRuntime {
	switch platform {
	case "android":
		return runtime.Android
	case "ios":
		return runtime.IOS
	case "web":
		return runtime.Web
	default:
		return nil
	}
}

func firstPresent(row map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := row[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func clientRuntimeString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]interface{}:
		if text, ok := t["text"].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
		if text, ok := t["markdown"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func clientRuntimeLooseString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return clientRuntimeString(v)
	}
}

func clientRuntimeBoolish(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}

func clientRuntimeLooseInt(v interface{}) int {
	switch t := v.(type) {
	case int:
		if t < 0 {
			return 0
		}
		return t
	case int64:
		if t < 0 {
			return 0
		}
		return int(t)
	case float64:
		if t < 0 || t != math.Trunc(t) {
			return 0
		}
		return int(t)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil || n < 0 {
			return 0
		}
		return n
	default:
		return 0
	}
}
