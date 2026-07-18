/**
 * @apito-io/functions — guest SDK surface for Apito Functions (Deno primary).
 *
 * In the sandbox, these methods are fulfilled by the engine FunctionDataGateway
 * (injected host bridge). This package provides TypeScript types and a thin
 * client for local testing against a gateway HTTP/NATS endpoint.
 */
/** Create a DataClient over an injected transport (host bridge or test stub). */
export function createDataClient(transport) {
    return {
        getSingleResource: (model, id) => transport("getSingleResource", { model, id }),
        getMany: (model, ids) => transport("getMany", { model, ids }),
        getRelationDocuments: (id, spec) => transport("getRelationDocuments", { id, ...spec }),
        getList: (model, query = {}) => transport("getList", { model, ...query }),
        listAllPages: async (model, query = {}) => {
            const pageSize = query.limit ?? 100;
            const all = [];
            let page = 1;
            for (;;) {
                const res = (await transport("getList", {
                    model,
                    ...query,
                    limit: pageSize,
                    page,
                }));
                all.push(...(res.results ?? []));
                if (all.length >= (res.total ?? all.length) || !(res.results?.length))
                    break;
                page += 1;
            }
            return all;
        },
        createNewResource: (input) => transport("createNewResource", input),
        updateResource: (input) => transport("updateResource", input),
        deleteOne: async (model, id) => {
            await transport("deleteOne", { model, id });
        },
        transaction: (req) => transport("transaction", {
            idempotency_key: req.idempotencyKey,
            operations: req.operations,
        }),
    };
}
/** Build the full ApitoFunctions object for Deno guest code. */
export function createApitoFunctions(transport) {
    const data = createDataClient(transport);
    return {
        data,
        log: {
            info: (...args) => console.log("[apito]", ...args),
            warn: (...args) => console.warn("[apito]", ...args),
            error: (...args) => console.error("[apito]", ...args),
        },
        http: {
            fetch: (url, init) => transport("http.fetch", { url, init }),
        },
        email: {
            send: (input) => transport("email.send", input),
        },
        jwt: {
            sign: (payload, opts) => transport("jwt.sign", { payload, ...opts }),
            verify: (token) => transport("jwt.verify", { token }),
        },
        secrets: {
            get: (name) => transport("secrets.get", { name }),
        },
    };
}
/** Ensure `globalThis.apito` exists (host injects real transport in production). */
export function ensureGlobalApito(transport) {
    const g = globalThis;
    if (g.apito)
        return g.apito;
    if (!transport) {
        throw new Error("apito functions SDK: no host transport injected");
    }
    g.apito = createApitoFunctions(transport);
    return g.apito;
}
