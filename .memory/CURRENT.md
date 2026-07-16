# open-core — Current

## Working on
- **Apito Functions (2026-07-16):** Runtime-neutral `functions/` package landed — contracts, Deno + wazero providers, local/NATS transport, artifact store, lifecycle, data gateway (stub ops), memory batch/idempotency, events dispatcher, callable auth, GraphQL invoke wiring.

## Next
- Wire `FunctionDataGateway` handlers to real project driver + tenant context
- SQLite-backed `ProjectBatchExecutor` + durable idempotency table
- Pin Deno Edge Runtime / ESZip artifact pipeline
- Rosna close-order shadow port per `functions/rosna_acceptance.md`

## Last Updated
2026-07-16
