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
- `auth/oidc` — issuer-driven verifier: OIDC / RFC 8414 discovery, JWKS cache
  with rate-limited refresh, JWT verification via `go-jose` (asymmetric
  allowlist; `none`/HMAC never accepted), RFC 7662 introspection.
- `internal/stdioguard` — keeps stdout clean for the stdio transport
  (fd-level `dup2` on Unix, so cgo and child processes are covered).
- `argontest` — in-memory calls, fake identities, fake issuer.
- `internal/testhook` — lets `argontest` build an SDK server from an
  `*argon.Server` without widening the public API.

## Dependency graph

```
argon ──► internal/core ──► auth
  │  └──► internal/sdkadapter ──► go-sdk
  │                    └────────► internal/core, auth
  └──► auth
auth/oidc ──► auth
argontest ──► argon, auth, internal/testhook, internal/sdkadapter
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
| Input validation against the inferred schema | `argon` |
| Resources and URI templates with typed variables | `argon`, `internal/core`, `internal/sdkadapter` |
| Content blocks: text, image, resource link | `internal/core`, `internal/sdkadapter` |
| Quiet logging (SDK filtered to Warn), opt-in call log | `argon`, `internal/core`, `internal/sdkadapter` |
| Answer in-flight stdio requests before EOF shutdown | `internal/sdkadapter` (wire-level drain) |
| Fake authorization server for tests | `argontest` |
| Identity in context, claims mapping (`scope`/`scp`) | `auth` |
| In-memory test helpers | `argontest` |

## Request flow (resources/read)

1. The SDK routes the read to the registered resource or template; on HTTP
   the caller identity is restored as for tools.
2. `sdkadapter` matches the URI against the template to extract variables.
3. `core.BuildResource` applies recover → scopes → timeout → size limit and
   defaults the content URI and MIME type. Tool middleware is not involved.
4. For templates, `argon` converts variables to the parameter struct's types
   and validates them against its schema; failures read as not found.
5. `core.ErrResourceNotFound` becomes the protocol's resource-not-found
   error; authenticated reads are marked `cacheScope: private`.

## Request flow (tools/call)

1. Transport (SDK) receives the request; on HTTP the SDK bearer middleware
   calls Argon's verifier adapter, which stores `auth.Identity` in context.
2. `sdkadapter` hands raw arguments to the core handler. Argon registers
   tools through the SDK's low-level API, so it validates arguments itself
   against the inferred schema (defaults applied) before decoding into `In`.
3. `core` runs the middleware chain and the user handler. Order, outermost
   first: meta (attaches `Request.Meta`) → recover → global (call log first)
   → scopes → timeout → max output → per-tool → handler.
4. The result is packed into content blocks; errors become `isError`.

## Deferred

Tracked in the project backlog (Notion): Streamable HTTP options and
resumability, prompts, audio and embedded-resource content.
