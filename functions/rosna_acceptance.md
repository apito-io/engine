# Rosna acceptance sequence (Apito Functions)

Real services under `udbhabon/rosna-astro/src/server/rosna/`:

1. pure compute hello / timeout / error
2. caller-scoped read-only (`getSingleResource`)
3. create/update with capability enforcement
4. `getRelationDocuments` + `getMany`
5. SQLite `transaction` batch with rollback + durable idempotency
6. port `closeOrder`
7. port `closeAllOrders`
8. stock helpers last (`$inc`)

Do not remove Astro `/api/rosna/*` or `hc-rosna-plugin` until shadow results match.
