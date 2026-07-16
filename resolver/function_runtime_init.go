package resolver

import (
	"os/exec"

	apifn "github.com/apito-io/engine/functions"
)

// selectDenoProvider returns the real Deno provider when the binary is on PATH,
// otherwise a stub so GraphQL/callable wiring still works in CI without Deno.
func selectDenoProvider() apifn.RuntimeProvider {
	if _, err := exec.LookPath("deno"); err == nil {
		return &apifn.DenoRuntimeProvider{}
	}
	return &apifn.StubRuntimeProvider{RuntimeName: "deno"}
}
