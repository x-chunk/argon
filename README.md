# argon

MCP framework for Go: build Model Context Protocol servers in a few lines.

Argon is a thin, opinionated layer over the official
[Go SDK](https://github.com/modelcontextprotocol/go-sdk). The SDK owns the
protocol; Argon owns ergonomics, safe defaults and token verification. SDK
types never appear in Argon's API.

> Status: v0.1, pre-release. The API may change.

```
go get go.xchunk.org/argon
```

## Quick start

```go
package main

import (
	"context"
	"log"

	"go.xchunk.org/argon"
)

type GreetInput struct {
	Name string `json:"name" jsonschema:"who to greet"`
}

func main() {
	s := argon.New("hello")
	argon.Tool(s, "greet", "Say hi", func(ctx context.Context, in GreetInput) (string, error) {
		return "Hi, " + in.Name + "!", nil
	}, argon.ReadOnly())
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}
```

`Run` serves over stdio by default; pass `--http :8080` (or set
`ARGON_HTTP=:8080`) for Streamable HTTP. `RunStdio`, `RunHTTP` and
`Handler` (an `http.Handler` to mount in your own server) are also available.

## Tools

- The input type's JSON Schema is inferred from `json` and `jsonschema`
  struct tags. Arguments are validated before your handler runs; invalid
  input becomes an error result the model can correct.
- Return a `string`, any JSON-serializable value (sent as structured
  content), or a `*argon.Result` for several blocks:
  `argon.NewResult(argon.TextContent{...}, argon.ImageContent{...}, argon.ResourceLink{...})`.
- A returned `error` becomes an `isError` result, not a protocol failure.
- Options: `ReadOnly()`, `Destructive()`, `Idempotent()`, `OpenWorld()`,
  `Title(..)`, `Scopes(..)`, `ToolTimeout(..)`, `ToolMaxOutput(..)`, `With(middleware...)`.

## Safe defaults

| Default | Value | Override |
|---|---|---|
| Per-call timeout | 60s | `argon.Timeout(d)`, `argon.ToolTimeout(d)` |
| Result size limit | 1 MiB (oversized → error, never truncated) | `argon.MaxOutput(n)`, `argon.ToolMaxOutput(n)` |
| Panic recovery | panics become error results, stack goes to logs | — |
| stdout protection (stdio) | fd 1 redirected to stderr; protocol uses a private copy | — |
| Graceful stdio shutdown | requests received before stdin EOF are answered | — |
| Logging | stderr, Warn and above; SDK chatter hidden | `argon.Logger(l)`, `argon.LogCalls()` |

Middleware (`argon.Use(...)` globally, `argon.With(...)` per tool) sees
`Request.Meta` with the tool's name, annotations, scopes and effective limits.

## Authentication (HTTP)

Argon is an OAuth 2.1 **resource server**: it verifies tokens, it never
issues them. Run your own authorization server (Keycloak, Auth0, Authelia,
Entra, …) and point Argon at it:

```go
v, err := oidc.NewVerifier(ctx, "https://auth.example.com", "https://mcp.example.com")
if err != nil {
	log.Fatal(err)
}
s := argon.New("notes", argon.Auth(auth.Config{
	Issuer:   "https://auth.example.com",
	Audience: "https://mcp.example.com",
	Verifier: v,
}))
argon.Tool(s, "add_note", "Save a note", addNote, argon.Scopes("notes:write"))
```

- `auth/oidc` discovers the issuer (OIDC, then RFC 8414), caches its JWKS
  (rate-limited refresh on unknown `kid`), verifies JWT signature, `iss`,
  `aud`, `exp` and `nbf`, and optionally introspects opaque tokens
  (RFC 7662, `oidc.Introspection(id, secret)`). Only asymmetric algorithms
  are accepted; `none` and HMAC never are.
- The audience is mandatory, so tokens minted for other resources are
  rejected.
- Unauthenticated requests get `401` with a `WWW-Authenticate` header
  pointing at `/.well-known/oauth-protected-resource` (RFC 9728).
- Mounting under a prefix: set `Audience` to the full endpoint URL and also
  mount the metadata endpoint at the root, since its RFC 9728 path lives
  outside the prefix:

  ```go
  h, _ := s.Handler()                     // Audience: "https://example.com/mcp"
  path, meta, _ := s.ResourceMetadata()   // "/.well-known/oauth-protected-resource/mcp"
  mux.Handle("/mcp", h)
  mux.Handle(path, meta)
  ```
- `auth.Static(token, identity)` covers simple self-hosted setups.
- Handlers read the caller with `argon.IdentityFrom(ctx)`.
- `auth/oidc` is a separate package: stdio-only servers do not pull in JOSE.

Authorization servers tested end to end: **Keycloak 26** (JWT and
introspection; see `test/keycloak` and `examples/auth`). Reports for other
servers are welcome.

stdio servers have no OAuth: they run as the user, and credentials for
downstream services come from the environment.

## Testing

```go
func TestGreet(t *testing.T) {
	got, err := argontest.Call[string](t, newServer(), "greet", GreetInput{Name: "Ann"})
	// ...
}
```

- `argontest.Call` / `CallResult` run the tool through a real in-memory MCP
  session — no subprocess, no network.
- `argontest.WithIdentity(...)` runs a call as a given caller, for testing
  `Scopes` and `IdentityFrom`.
- `argontest.NewFakeIssuer(t)` is an in-process authorization server
  (discovery, JWKS, introspection) that mints tokens for auth tests.

### Integration tests

Real-server tests carry the `integration` build tag and skip when the server
is not running:

```
docker compose -f test/keycloak/compose.yaml up -d --wait
go test -tags integration ./...
docker compose -f test/keycloak/compose.yaml down
```

## Designing tools safely

Prefer narrow, purpose-built tools — fixed commands, validated arguments,
aliases instead of raw IDs — over generic ones like "run any shell command".

Be careful when one session combines tools that read **untrusted content**
(web pages, files, email) with tools that have **real side effects**
(messaging, file writes, shell). Content read by the first kind can steer the
model into misusing the second. Mark side-effecting tools `Destructive()`,
gate them with scopes, and keep such combinations out of one server where
you can.

## Layout

See [docs/architecture.md](docs/architecture.md).
