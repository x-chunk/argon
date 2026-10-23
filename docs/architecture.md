# Architecture

## Layers

```
user code
   │
argon                     public API and its implementation:
   │                      Server, Tool / Resource / ResourceTemplate / Prompt,
   │                      public types (Request, Result, Content, ...),
   │                      registry, handler pipelines, built-in middleware
   │   adapt.go converts definitions and results to the adapter's own types
   ▼
internal/sdkadapter       the only place that imports the official SDK;
   │                      works on its own plain types (ToolDef, Block, ...)
   ▼
go-sdk                    protocol, transports, HTTP bearer plumbing
```

Side packages:

- `auth` — Identity, TokenVerifier, Config, static verifier. Lightweight leaf.
- `auth/oidc` — issuer-driven verifier: OIDC / RFC 8414 discovery, JWKS cache
  with rate-limited refresh, JWT verification via `go-jose` (asymmetric
  allowlist; `none`/HMAC never accepted), RFC 7662 introspection.
- `internal/errs` — sentinel errors shared by `argon` and the adapter
  (`ErrResourceNotFound`, `ErrInvalidArguments`), so the adapter can map them
  to protocol errors without importing `argon`.
- `internal/stdioguard` — keeps stdout clean for the stdio transport
  (fd-level `dup2` on Unix, so cgo and child processes are covered).
- `argontest` — in-memory calls, reads and prompt renders, fake identities,
  fake issuer.
- `internal/testhook` — lets `argontest` build an SDK server from an
  `*argon.Server` without widening the public API.

## Dependency graph

```
argon ──► internal/sdkadapter ──► go-sdk
  │  │                    └─────► auth, internal/errs
  │  └──► internal/errs, internal/stdioguard
  └──► auth
auth/oidc ──► auth
argontest ──► argon, auth, internal/testhook, internal/sdkadapter
```

Why the adapter has its own types: it must translate Argon's content and
results into SDK types, but `argon` imports the adapter, so the adapter
cannot import `argon`. Instead it describes what to expose with plain data
plus functions (`Tool{Def, Call}`, `Resource{Def, Read}`,
`Prompt{Def, Get}`, content as a `Block` union), and `argon/adapt.go`
converts at the boundary. The public types therefore live in `argon` itself
and are fully documented there.

## SDK vs Argon

The SDK provides: typed tools with schema inference and input validation,
error → `isError` results, stdio / Streamable HTTP / in-memory transports,
method-level middleware, tool annotations, `RequireBearerToken` and the
protected-resource metadata handler.

Argon adds:

| Concern | Where |
|---|---|
| Short handler `func(ctx, In) (Out, error)` | `argon` |
| Per-tool middleware: recover, timeout, output limit, scopes | `argon` |
| Transport auto-selection (`Run`) and stdout guard | `argon`, `internal/stdioguard` |
| JWT/JWKS + introspection verifier, alg allowlist | `auth/oidc` |
| Input validation against the inferred schema | `argon` |
| Resources and URI templates with typed variables | `argon`, `internal/sdkadapter` |
| Prompts with typed, validated arguments | `argon`, `internal/sdkadapter` |
| Content blocks: text, image, resource link | `argon`, `internal/sdkadapter` |
| Quiet logging (SDK filtered to Warn), opt-in call log | `argon`, `internal/sdkadapter` |
| Answer in-flight stdio requests before EOF shutdown | `internal/sdkadapter` (wire-level drain) |
| Fake authorization server for tests | `argontest` |
| Identity in context, claims mapping (`scope`/`scp`) | `auth` |
| In-memory test helpers | `argontest` |

## Request flow (tools/call)

1. Transport (SDK) receives the request; on HTTP the SDK bearer middleware
   calls Argon's verifier adapter, which stores `auth.Identity` in context.
2. `sdkadapter` restores the identity and calls the tool's `Call` function
   with the raw arguments. Argon registers tools through the SDK's low-level
   API, so it validates arguments itself against the inferred schema
   (defaults applied) before decoding into `In`.
3. `argon` runs the middleware chain and the user handler. Order, outermost
   first: meta (attaches `Request.Meta`) → recover → result check → global (call log first)
   → scopes → timeout → max output → per-tool → handler.
4. The result is converted to adapter blocks and packed into SDK content;
   errors become `isError`.

## Request flow (resources/read)

1. The SDK routes the read to the registered resource or template; on HTTP
   the caller identity is restored as for tools.
2. The resource's `Read` function (in `argon`) matches the URI against the
   template to extract variables; a URI that does not match reads as not
   found.
3. The resource pipeline applies recover → scopes → timeout → size limit and
   defaults the content URI and MIME type. Tool middleware is not involved.
4. For templates, variables are converted to the parameter struct's types
   (`stringargs.go`) and validated against its schema; failures read as not
   found.
5. `ErrResourceNotFound` becomes the protocol's resource-not-found error;
   authenticated reads are marked `cacheScope: private`.

## Request flow (prompts/get)

1. The SDK routes the get to the registered prompt; on HTTP the caller
   identity is restored as for tools.
2. The prompt pipeline applies recover → scopes → timeout, then checks every
   message's role and content and the size limit. Tool middleware is not
   involved.
3. String arguments are converted to the argument struct's types (shared with
   resource templates in `stringargs.go`) and validated against its schema;
   failures wrap `ErrInvalidArguments`.
4. `ErrInvalidArguments` becomes the protocol's invalid-params error.

## Deferred

New features are scheduled after v0.1 (Notion: Plans → Roadmap to v1.0).
