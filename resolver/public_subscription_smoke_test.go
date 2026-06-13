//go:build integration

package resolver_test

// Manual / CI integration smoke test for public GraphQL subscriptions over WebSocket.
//
// Prerequisites:
//   - Engine running locally with REALTIME_ENGINE=memory (or nats)
//   - A project with an "author" model and valid secured API token/cookie
//
// Steps:
//  1. Connect graphql-transport-ws to ws://localhost:5050/secured/graphql/subscription
//  2. Send connection_init with auth cookies or Authorization header
//  3. Subscribe:
//       subscription { authorChanged { event id node } }
//  4. POST createAuthor mutation to /secured/graphql
//  5. Assert a CREATED event arrives on the subscription within a few seconds
//
// Example using wscat (after obtaining session cookie):
//   wscat -c 'ws://localhost:5050/secured/graphql/subscription' -s graphql-transport-ws
//
// This file documents the smoke path; automated WS harness is not in the default unit suite.

import "testing"

func TestPublicSubscriptionSmoke_Documented(t *testing.T) {
	t.Skip("manual smoke: see file comment for websocket subscription verification steps")
}
