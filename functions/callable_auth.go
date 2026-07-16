package functions

import (
	"crypto/subtle"
	"fmt"

	"github.com/apito-io/engine/models"
)

// VerifyFunctionSecret compares the caller-provided hash to the function's RestAPISecretURLKey
// using constant-time comparison. Empty stored secrets fail closed.
func VerifyFunctionSecret(fn *models.ApitoFunction, provided string) error {
	if fn == nil {
		return fmt.Errorf("function not found")
	}
	expected := fn.RestAPISecretURLKey
	if expected == "" {
		return fmt.Errorf("function secret not configured")
	}
	if provided == "" {
		return fmt.Errorf("missing X-Fn-Hash")
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return fmt.Errorf("invalid function secret")
	}
	return nil
}

// FindFunctionByName looks up a function on the project schema.
func FindFunctionByName(project *models.Project, name string) *models.ApitoFunction {
	if project == nil || project.Schema == nil {
		return nil
	}
	for _, f := range project.Schema.Functions {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

// AllowCallableRuntime reports whether the REST /function route may execute this runtime.
// Deno/wasm require secret verification; Hashicorp may still use the route for legacy plugins.
func AllowCallableRuntime(fn *models.ApitoFunction) bool {
	if fn == nil {
		return false
	}
	if fn.IsApitoFunctionsRuntime() {
		return true
	}
	return fn.FunctionProviderID != ""
}
