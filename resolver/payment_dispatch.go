package resolver

import (
	"context"
	"strings"

	"github.com/apito-io/engine/services/payment"
)

// DispatchPayment executes one declared payment operation and returns a JSON object.
func (s *GraphQLServer) DispatchPayment(ctx context.Context, pluginID, operation string, args map[string]interface{}, reconcile bool) (map[string]interface{}, error) {
	if s == nil {
		return nil, errPluginNotActivated
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if projectID, _ := args["project_id"].(string); strings.TrimSpace(projectID) != "" {
		ctx = context.WithValue(ctx, "project_id", strings.TrimSpace(projectID))
	}
	if reconcile {
		ctx = WithPaymentReconcileBypass(ctx)
	}
	result, err := s.executePluginGraphQLResolver(ctx, pluginID, operation, "mutation", args)
	if err != nil {
		return nil, err
	}
	return payment.ParsePluginPayload(result)
}
