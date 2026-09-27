## Rate limiting

Two limits, at two layers:

| Limit           | Where                   | Keyed on              | Covers                                      | Refuses with         |
| --------------- | ----------------------- | --------------------- | ------------------------------------------- | -------------------- |
| Rate limit      | Traefik, on the Ingress | client IP             | every route                                 | 429, plain text      |
| Concurrency cap | app, route line         | nothing, a slot count | `POST /api/v1/users`, `POST /api/v1/tokens` | 429, `ErrorResponse` |

```mermaid
flowchart TD
    C["client"] --> T["Traefik"]
    subgraph EDGE["Traefik, per client IP"]
        T --> R["redirect-https"]
        R --> RL["rate-limit"]
    end
    RL --> MUX["app: middleware chain, then the mux"]
    KUBE["kubelet probe"] -->|"direct to the pod,<br/>no Traefik"| MUX
    MUX --> CAP["limitConcurrent<br/>POST /users, POST /tokens"]
    CAP --> BC["bcrypt handlers"]
    MUX --> OTHER["every other handler"]
```

### Edge: per-IP rate limit

A Traefik `rateLimit` Middleware (token bucket like).
The Ingress applies it after `redirect-https`, a plain HTTP request is redirected without
spending a token.

```mermaid
flowchart TD
    A["request"] --> B{"token in the<br/>client's bucket?"}
    B -- yes --> P["pass to the app"]
    B -- no --> D{"next token due within<br/>half a refill interval?"}
    D -- yes --> W["wait for it, then pass"]
    D -- no --> E["429 Too Many Requests<br/>Retry-After, X-Retry-In"]
```

The 429 comes from Traefik: plain text body, not in `api.yaml`.

**The key is the connection's remote address** (`ipStrategy.depth: 0`).

The remote address is the real client only if `traefikSourceIP` is applied
(`externalTrafficPolicy: Local` on the k3s Traefik Service). Without it every client arrives as
the node address `10.42.0.1` and shares one bucket.

### App: concurrency cap on bcrypt

`POST /api/v1/users` and `POST /api/v1/tokens` run bcrypt, a few hundred ms of CPU.

Both routes share one semaphore: at most `AUTH_CONCUR_LIMIT` bcrypt requests run at once,
across all clients. It caps CPU, not request rate.

```mermaid
flowchart TD
    A["POST /api/v1/tokens or /api/v1/users"] --> B{"free slot?"}
    B -- yes --> C["take a slot"]
    B -- no --> D{"wait up to AUTH_CONCUR_WAIT"}
    D -- "slot freed" --> C
    D -- "deadline passed" --> E["429, Retry-After, ErrorResponse"]
    D -- "client hung up" --> Z["nothing written"]
    C --> F["bcrypt"]
    F --> G["release the slot"]
```

### Local

Compose has no Traefik, so only in app limits apply.
