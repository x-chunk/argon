# argon

[![CI](https://github.com/x-chunk/argon/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/x-chunk/argon/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/go.xchunk.org/argon.svg)](https://pkg.go.dev/go.xchunk.org/argon)
[![Release](https://img.shields.io/github/v/release/x-chunk/argon?include_prereleases&sort=semver)](https://github.com/x-chunk/argon/releases)

MCP framework for Go: build Model Context Protocol servers in a few lines.

Argon is a thin, opinionated layer over the official
[Go SDK](https://github.com/modelcontextprotocol/go-sdk). The SDK owns the
protocol; Argon owns ergonomics, safe defaults and token verification. SDK
types never appear in Argon's API.

> Status: **v0.1.0 Release Candidate 1** (`v0.1.0-rc.1`). The v0.1 API is
> frozen; it may still change before v1.0.0. See [CHANGELOG.md](CHANGELOG.md).

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

`Run` serves over stdio by default; pass `--http 127.0.0.1:8080` (or set
`ARGON_HTTP=127.0.0.1:8080`) for Streamable HTTP. Without `Auth`, listen on
loopback only: `:8080` exposes every tool to the network, and Argon logs a
warning when it does. `RunStdio`, `RunHTTP` and
`Handler` (an `http.Handler` to mount in your own server) are also available.

## Tools

- The input is a struct (or a pointer to one, or a map); its JSON Schema is
  inferred from `json` and `jsonschema` struct tags. Arguments are validated before your handler runs; invalid
  input becomes an error result the model can correct.
- Return a `string`, a struct or pointer to a struct (sent as structured
  content with an output schema, plus its JSON as text), any other
  JSON-serializable value (sent as JSON text), or a `*argon.Result` for
  several blocks:
  `argon.NewResult(argon.TextContent{...}, argon.ImageContent{...}, argon.ResourceLink{...})`.
- A returned `error` becomes an `isError` result, not a protocol failure.
- Options: `ReadOnly()`, `Destructive()`, `Idempotent()`, `OpenWorld()`,
  `Title(..)`, `Scopes(..)`, `ToolTimeout(..)`, `ToolMaxOutput(..)`, `With(middleware...)`.

## Resources

Expose read-only data by URI. A fixed resource:

```go
argon.Resource(s, "file:///readme.md", "readme", "Project readme",
	func(ctx context.Context) (argon.ResourceContents, error) {
		return argon.TextResource(readme), nil
	}, argon.MIMEType("text/markdown"))
```

A family of resources from an RFC 6570 URI template. Template variables
decode into a struct, converted to the field types and validated like tool
input; a URI that does not decode reads as "not found":

```go
type NoteRef struct {
	ID int `json:"id"`
}

argon.ResourceTemplate(s, "notes://{id}", "note", "A note by id",
	func(ctx context.Context, ref NoteRef) (argon.ResourceContents, error) {
		n, ok := notes[ref.ID]
		if !ok {
			return argon.ResourceContents{}, argon.ErrResourceNotFound
		}
		return argon.TextResource(n), nil
	})
```

- Return `argon.TextResource(..)` or `argon.BlobResource(mime, data)`;
  the content URI defaults to the requested URI.
- Options: `MIMEType`, `ResourceTitle`, `ResourceScopes`, `ResourceTimeout`,
  `ResourceMaxOutput`.
- Resources share the tools' safe defaults (timeout, size limit, panic
  recovery). Tool middleware does not apply to them.
- Reads made by an authenticated caller are marked `cacheScope: private`.
- Test with `argontest.ReadResource(t, s, uri)`.

## Prompts

Offer reusable prompt templates. Arguments are the fields of a struct:
names from `json` tags, descriptions from `jsonschema` tags, and fields
without `omitempty` are required. Clients send arguments as strings; they
are converted to the field types and validated before your handler runs:

```go
type ReviewArgs struct {
	Code     string `json:"code" jsonschema:"code to review"`
	Language string `json:"language,omitempty"`
}

argon.Prompt(s, "code_review", "Review a piece of code",
	func(ctx context.Context, a ReviewArgs) ([]argon.PromptMessage, error) {
		return []argon.PromptMessage{
			argon.UserMessage("Review this " + a.Language + " code:\n" + a.Code),
		}, nil
	}, argon.PromptTitle("Code review"))
```

- Messages: `argon.UserMessage(..)`, `argon.AssistantMessage(..)`, or
  `argon.PromptMessage{Role: argon.RoleUser, Content: argon.ImageContent{..}}`
  with any content block. Use `struct{}` for a prompt without arguments.
- Missing, unknown or malformed arguments are rejected with "invalid
  params"; handlers can reject input themselves with
  `argon.ErrInvalidArguments`.
- Options: `PromptTitle`, `PromptScopes`, `PromptTimeout`, `PromptMaxOutput`.
  Prompts share the tools' safe defaults; tool middleware does not apply.
- Test with `argontest.GetPrompt(t, s, name, args)`.

## Safe defaults

| Default | Value | Override |
|---|---|---|
| Per-call timeout (tools, resources, prompts; cancels the handler's context) | 60s | `argon.Timeout(d)`, `argon.ToolTimeout(d)` |
| Result size limit | 1 MiB (oversized → error, never truncated) | `argon.MaxOutput(n)`, `argon.ToolMaxOutput(n)` |
| Panic recovery | panics become error results, stack goes to logs | — |
| stdout protection (stdio) | fd 1 redirected to stderr; protocol uses a private copy | — |
| Graceful stdio shutdown | requests received before stdin EOF are answered | — |
| Logging | stderr, Warn and above; SDK chatter hidden | `argon.Logger(l)`, `argon.LogCalls()` |
| HTTP sessions | idle sessions expire after 30 min; request bodies limited by the SDK | — |
| HTTP connections | idle keep-alive closed after 2 min; open streams end on shutdown | — |

Server-wide limits are disabled with zero; per-item options (`ToolTimeout`,
`ResourceMaxOutput`, ...) keep the server default on zero and disable the
limit with a negative value. Timeouts are cooperative: a handler that
ignores its context keeps the call waiting.

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
  pointing at the RFC 9728 metadata: `/.well-known/oauth-protected-resource`
  followed by the audience path (e.g. `.../oauth-protected-resource/mcp` for
  `https://example.com/mcp`).
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
- Scopes fail closed: without authentication (stdio, or HTTP without
  `Auth`) items with `Scopes`/`ResourceScopes`/`PromptScopes` reject every
  call, and Argon logs a warning naming them at startup.
- `auth/oidc` is a separate package: stdio-only servers do not pull in JOSE.

Authorization servers tested end to end: **Keycloak 26** (JWT and
introspection; see `test/keycloak` and `examples/auth`). Reports for other
servers are welcome.

Keycloak note: recent versions only let a client introspect tokens whose
audience includes that client. When using `oidc.Introspection`, add an
audience mapper for your server's client (see `test/keycloak/realm-argon.json`).

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

## Known limitations (v0.1)

- Content blocks: text, images and resource links; no audio or embedded
  resources yet.
- A missing scope is reported as a tool error, not as HTTP 403 with
  `insufficient_scope`, and `tools/list` is not filtered by scope.
- Streamable HTTP options (stateless mode, stream resumption, custom
  session timeout) are not exposed yet.

## Layout

See [docs/architecture.md](docs/architecture.md).
