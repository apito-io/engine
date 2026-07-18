package functions

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
)

func TestDenoListFoodNamesViaBridge(t *testing.T) {
	if _, err := exec.LookPath("deno"); err != nil {
		t.Skip("deno not on PATH")
	}
	sdkPath, err := filepath.Abs("embed/apito_functions.js")
	if err != nil || !fileExists(sdkPath) {
		t.Skip("embedded SDK missing")
	}

	store, err := NewFilesystemArtifactStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gw := NewEngineDataGateway()
	reg := NewInvocationRegistry()
	gw.CheckCapability = RequireCapability([]string{"data.read:food"})
	gw.Register("getList", func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		_ = ctx
		if call.Payload["model"] != "food" {
			return &DataGatewayResponse{OK: false, Error: "wrong model"}, nil
		}
		return &DataGatewayResponse{OK: true, Data: map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "f1", "data": map[string]interface{}{"name": "Biryani"}},
				{"id": "f2", "data": map[string]interface{}{"name": "Kacchi"}},
			},
			"total": 2,
		}}, nil
	})

	src := `import { ensureGlobalApito } from "@apito-io/functions";
const apito = ensureGlobalApito();
export default async function (req: Record<string, unknown>) {
  const rawLimit = Number(req.limit ?? 20);
  const limit = Number.isFinite(rawLimit) ? Math.min(Math.max(Math.trunc(rawLimit), 1), 100) : 20;
  const page = await apito.data.getList("food", { limit, page: 1 });
  const results = (page?.results ?? []).map((doc: { id?: string; data?: { name?: string } }) => ({
    id: doc.id,
    name: doc.data?.name ?? null,
  }));
  return { ok: true, total: page?.total ?? results.length, foods: results };
}
`
	fn := &models.ApitoFunction{
		ProjectID:    "rosna_v2_jpn6o",
		Name:         "listFoodNames",
		Source:       src,
		Language:     "typescript",
		Capabilities: []string{"data.read:food"},
		RuntimeConfig: &models.ApitoFunctionRuntimeConfig{
			Runtime: "deno",
			Handler: "default",
			TimeOut: 15,
		},
	}
	env := BuildEnvelope(fn, fn.ProjectID, "tenant1", "user1", "admin", map[string]interface{}{"limit": 5}, "inv-food-1")
	inv := &InvocationContext{
		Envelope:     env,
		DBCtx:        context.Background(),
		Capabilities: fn.Capabilities,
		Gateway:      gw,
	}
	if err := reg.Register(inv); err != nil {
		t.Fatal(err)
	}
	defer reg.Unregister(env.InvocationID)

	p := &DenoRuntimeProvider{Gateway: gw, Registry: reg}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	res, err := p.Execute(ctx, env, FunctionLimits{Timeout: 30 * time.Second, MaxLogBytes: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || !res.OK {
		t.Fatalf("invoke failed: %#v", res)
	}
	b, _ := json.Marshal(res.Response)
	if !strings.Contains(string(b), "Biryani") || !strings.Contains(string(b), "Kacchi") {
		t.Fatalf("expected food names in response: %s", b)
	}
	_ = store
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
