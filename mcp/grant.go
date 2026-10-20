package mcp

import (
	"errors"
	"strings"

	"github.com/apito-io/engine/models"
)

// GrantScope is the project and tenant choice from the existing approval form.
type GrantScope struct {
	Name              string
	Description       string
	Duration          string
	Preset            string
	Capabilities      []string
	ProjectGrantMode  string
	ProjectIDs        []string
	TenantGrantMode   string
	TenantIDs         map[string][]string
	AcknowledgeDanger bool
}

// ApprovalMintRequest builds the mint for an MCP approval.
// An empty preset stays mcp_assistant. MCP advanced and Custom are the other
// choices. Named presets ignore client-supplied capabilities.
func ApprovalMintRequest(label string, scope GrantScope) (*models.CreateAccessTokenRequest, error) {
	preset, caps, err := NormalizeApprovalPreset(scope.Preset, scope.Capabilities)
	if err != nil {
		return nil, err
	}
	if scope.ProjectGrantMode == "" {
		scope.ProjectGrantMode = models.AccessTokenProjectGrantSelected
	}
	if scope.TenantGrantMode == "" {
		scope.TenantGrantMode = models.AccessTokenTenantGrantNone
	}
	projectIDs := scope.ProjectIDs
	if scope.ProjectGrantMode != models.AccessTokenProjectGrantSelected {
		projectIDs = nil
	}
	tenantIDs := scope.TenantIDs
	if scope.TenantGrantMode != models.AccessTokenTenantGrantSelected {
		tenantIDs = nil
	}
	name := strings.TrimSpace(scope.Name)
	if name == "" {
		name = "MCP " + strings.TrimSpace(label)
	}
	if strings.TrimSpace(name) == "" || name == "MCP" {
		name = "MCP assistant"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	description := strings.TrimSpace(scope.Description)
	if len(description) > 500 {
		description = description[:500]
	}
	return &models.CreateAccessTokenRequest{
		Name:              name,
		Description:       description,
		Duration:          strings.TrimSpace(scope.Duration),
		Preset:            preset,
		Capabilities:      caps,
		ProjectGrantMode:  scope.ProjectGrantMode,
		ProjectIDs:        projectIDs,
		TenantGrantMode:   scope.TenantGrantMode,
		TenantIDs:         tenantIDs,
		AcknowledgeDanger: scope.AcknowledgeDanger,
	}, nil
}

// NormalizeApprovalPreset keeps MCP approval on mcp_assistant, mcp_advanced, or custom.
func NormalizeApprovalPreset(raw string, caps []string) (string, []string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "mcp_assistant":
		return "mcp_assistant", nil, nil
	case "mcp_advanced":
		return "mcp_advanced", nil, nil
	case "custom":
		if len(caps) == 0 {
			return "", nil, errors.New("custom preset requires capabilities")
		}
		return "custom", append([]string(nil), caps...), nil
	default:
		return "", nil, errors.New("choose MCP assistant, MCP advanced, or Custom")
	}
}
