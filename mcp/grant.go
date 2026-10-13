package mcp

import (
	"strings"

	"github.com/apito-io/engine/models"
)

// GrantScope is the project and tenant choice from the existing approval form.
type GrantScope struct {
	ProjectGrantMode  string
	ProjectIDs        []string
	TenantGrantMode   string
	TenantIDs         map[string][]string
	AcknowledgeDanger bool
}

// ApprovalMintRequest builds the same mcp_assistant mint used by device login.
// Capabilities stay on the preset. The returned secret is an internal grant.
func ApprovalMintRequest(label string, scope GrantScope) *models.CreateAccessTokenRequest {
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
	name := "MCP " + strings.TrimSpace(label)
	if strings.TrimSpace(label) == "" {
		name = "MCP assistant"
	}
	return &models.CreateAccessTokenRequest{
		Name:              name,
		Preset:            "mcp_assistant",
		ProjectGrantMode:  scope.ProjectGrantMode,
		ProjectIDs:        projectIDs,
		TenantGrantMode:   scope.TenantGrantMode,
		TenantIDs:         tenantIDs,
		AcknowledgeDanger: scope.AcknowledgeDanger,
	}
}
