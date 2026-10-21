# Architecture

## Layers

```
user code
   │
argon (public API)        Server, Tool[In,Out], options, Run*, Identity
   │
internal/core             tool registry, middleware chain, built-in middleware
   │
internal/sdkadapter       the only place that imports the official SDK
   │
go-sdk                    protocol, transports, HTTP bearer plumbing
```

Side packages:

- `auth` — Identity, TokenVerifier, Config, static verifier. Lightweight leaf.
- `auth/oidc` — discovery, JWKS and introspection verifier (pulls JWT deps).
- `internal/stdioguard` — keeps stdout clean for the stdio transport.
- `argontest` — in-memory calls, fake identities, fake issuer.

## Dependency graph

```
argon ──► internal/core ──► auth
  │  └──► internal/sdkadapter ──► go-sdk
  │                    └────────► internal/core, auth
  └──► auth
auth/oidc ──► auth
argontest ──► argon, auth
```

## SDK vs Argon

The SDK (v1.8.0) already provides: typed tools with schema inference and
input validation, error → `isError` results, stdio / Streamable HTTP /
in-memory transports, method-level middleware, tool annotations,
`RequireBearerToken` and the protected-resource metadata handler.

Argon adds:

| Concern | Where |
|---|---|
| Short handler `func(ctx, In) (Out, error)` | `argon`, `internal/core` |
| Per-tool middleware: recover, timeout, output limit, scopes | `internal/core` |
| Transport auto-selection (`Run`) and stdout guard | `argon`, `internal/stdioguard` |
| JWT/JWKS + introspection verifier, alg allowlist | `auth/oidc` |
| Identity in context, claims mapping (`scope`/`scp`) | `auth` |
| In-memory test helpers | `argontest` |

## Request flow (tools/call)

1. Transport (SDK) receives the request; on HTTP the SDK bearer middleware
   calls Argon's verifier adapter, which stores `auth.Identity` in context.
2. `sdkadapter` decodes arguments (SDK validates against the schema) and
   invokes the core handler.
3. `core` runs the middleware chain (global, then per-tool) and the user
   handler.
4. The result is packed into content blocks; errors become `isError`.
