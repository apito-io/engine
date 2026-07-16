package functions_test

import (
	"context"
	"testing"
	"time"

	"github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestStubRuntimeEcho(t *testing.T) {
	m := functions.NewRuntimeManager(
		[]functions.RuntimeProvider{&functions.StubRuntimeProvider{RuntimeName: "deno"}},
		functions.WithLimitsProvider(functions.DefaultLimitsProvider{}),
	)
	res, err := m.Invoke(context.Background(), &functions.InvocationEnvelope{
		Runtime:  "deno",
		Function: "hello",
		Request:  map[string]interface{}{"x": 1},
		DeadlineMs: 5000,
	})
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, "hello", res.Response["function"])
}

func TestTimeoutClassification(t *testing.T) {
	m := functions.NewRuntimeManager(
		[]functions.RuntimeProvider{&functions.StubRuntimeProvider{RuntimeName: "deno"}},
		functions.WithLimitsProvider(functions.DefaultLimitsProvider{}),
	)
	res, _ := m.Invoke(context.Background(), &functions.InvocationEnvelope{
		Runtime:    "deno",
		Function:   "slow",
		Source:     "__TIMEOUT__",
		DeadlineMs: 50,
	})
	require.NotNil(t, res)
	require.False(t, res.OK)
	require.Equal(t, "timeout", res.ErrorClass)
}

func TestCallableSecret(t *testing.T) {
	fn := &models.ApitoFunction{Name: "f", RestAPISecretURLKey: "secret123"}
	require.Error(t, functions.VerifyFunctionSecret(fn, "wrong"))
	require.NoError(t, functions.VerifyFunctionSecret(fn, "secret123"))
	require.Error(t, functions.VerifyFunctionSecret(fn, ""))
}

func TestLocalTransportRequestReply(t *testing.T) {
	tr := functions.NewLocalTransport()
	unsub, err := tr.RespondHandler(context.Background(), "fn.invoke", "g", func(payload []byte) ([]byte, error) {
		return append([]byte("echo:"), payload...), nil
	})
	require.NoError(t, err)
	defer unsub()
	out, err := tr.Request(context.Background(), "fn.invoke", []byte("hi"), time.Second)
	require.NoError(t, err)
	require.Equal(t, "echo:hi", string(out))
}

func TestBatchIdempotency(t *testing.T) {
	ex := functions.NewMemoryBatchExecutor()
	req := &functions.BatchRequest{
		IdempotencyKey: "close:1",
		Operations: []functions.BatchOp{
			{Op: "update", Model: "food_order", ID: "o1", Payload: map[string]interface{}{"status": "processed"}},
		},
	}
	require.NoError(t, functions.ValidateBatchOps(req.Operations))
	r1, err := ex.ExecuteBatch(context.Background(), "p1", "t1", req)
	require.NoError(t, err)
	require.True(t, r1.OK)
	r2, err := ex.ExecuteBatch(context.Background(), "p1", "t1", req)
	require.NoError(t, err)
	require.True(t, r2.OK)
	require.True(t, r2.Replay)
}

func TestEffectiveRuntime(t *testing.T) {
	fn := &models.ApitoFunction{
		RuntimeConfig: &models.ApitoFunctionRuntimeConfig{Runtime: models.FunctionRuntimeDeno},
	}
	require.True(t, fn.IsApitoFunctionsRuntime())
	hc := &models.ApitoFunction{FunctionProviderID: "hc-rosna-plugin"}
	require.Equal(t, models.FunctionRuntimeHashicorp, hc.EffectiveRuntime())
}
