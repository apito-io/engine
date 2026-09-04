package resolver

import "errors"

var (
	errPluginIDRequired      = errors.New("plugin id is required")
	errPluginNotInRegistry   = errors.New("plugin not found in HashiCorp registry")
	errPluginProjectRequired = errors.New("project is required")
	errPluginNotActivated    = errors.New("plugin is not activated for this project")
)
