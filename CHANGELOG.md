# Changelog

All notable changes to this project are documented here. The project
follows [Semantic Versioning](https://semver.org); until v1.0.0 the API may
change between minor versions and previews.

## [v0.1.0-rc.1] — Release Candidate 1

Fixes from the pre-release review. The API is now frozen for v0.1: until
v0.1.0 only fixes land.

### Breaking

- `auth.Config.ClaimsMapper` is removed. It was never applied; configure
  claims mapping on the verifier with `oidc.ClaimsMapper`.
- `Run` returns an error for `--http` (or `--http=`) without an address
  instead of silently serving stdio.

### Security

- A tool handler or middleware returning a nil `*Result` no longer crashes
  the process; it is a tool error. Panics anywhere in request handling are
  also recovered at the SDK boundary.
- `auth/oidc`: JWKS refreshes are detached from the request that triggered
  them, so a client dropping its connection can no longer abort refreshes
  and keep rotated keys from being picked up; cached keys are read without
  waiting for a fetch.
- `auth/oidc`: the JWKS is refetched once older than `MaxKeyAge` (new
  option, default 1h), so withdrawn keys stop being trusted.
- `auth/oidc`: the default HTTP client times out after 15s.
- `auth/oidc`: active introspection results are cached for up to 30s and at
  most 16 introspection requests run concurrently.
- Streamable HTTP sessions expire after 30 minutes without requests;
  `RunHTTP` closes idle keep-alive connections after 2 minutes.
- `LogCalls` no longer logs rejected argument values from validation
  errors.
- `RunHTTP` warns when serving without `Auth` on a non-loopback address;
  serving or building a handler without authentication warns about items
  that require scopes (they reject every call).
- The stdio protocol descriptor gets close-on-exec under `syscall.ForkLock`,
  so concurrently started children cannot inherit it.

### Fixed

- Non-object tool inputs (scalars, slices) are registration errors instead
  of an SDK panic at startup; a pointer to a struct is accepted as input.
- Integers beyond 2^53 keep their precision in tool inputs, template
  variables and prompt arguments.
- Pointer fields (e.g. `*int`) work in prompt arguments and template
  variables.
- `RunHTTP` ends open server-to-client streams on shutdown and returns nil
  instead of waiting 5s and reporting `context deadline exceeded`.
- Structured output counts once towards `MaxOutput`, not twice.
- Results with nil content blocks, and tools with an output schema that
  return no structured content, are tool errors.
- Resource contents with both text and blob are rejected.
- Resource templates whose parameter struct has a required field that is
  not a template variable are registration errors.
- The stdio drain matches request ids regardless of their spelling (`2.0`
  vs `2`), so shutdown no longer waits the full drain timeout.
- Middleware gets its own copy of tool annotations.

### Documentation

- Scopes fail closed without authentication; per-item limits keep the
  default on zero and are disabled by negative values; timeouts are
  cooperative; registration happens before serving; `Identity.Subject`
  binds HTTP sessions; template variables arrive percent-decoded.
- README: structured output rules, metadata path, HTTP defaults, known
  limitations.

### CI

- Actions are pinned to commit SHAs.

## [v0.1.0-preview.4] — Developer Preview 4

Fixes before the first release candidate; no new features.

### Changed

- Public types (`Request`, `Result`, `Content` and its blocks, `Handler`,
  `Middleware`, `ToolMeta`, `Annotations`, `ResourceContents`,
  `PromptMessage`, `Role`) are now defined in package `argon` instead of
  being aliases to an internal package, so their fields and documentation
  show in `go doc` and on pkg.go.dev. Names and behaviour are unchanged.

### Fixed

- Tools returning a pointer to a struct (`*T`) now declare `T`'s output
  schema and send structured content, like tools returning `T`; a nil `*T`
  is reported as a tool error.
- The Streamable HTTP handler now logs its errors through the server logger
  (Warn and above) instead of discarding them.

### Added

- Examples for pkg.go.dev and README badges.
- Tests for `Run` (transport selection, SIGINT/SIGTERM, stdio EOF) and
  `RunHTTP` (graceful shutdown, listen errors).

## [v0.1.0-preview.3] — Developer Preview 3

Adds MCP prompts and raises the minimum Go version to 1.26.

### Added

- **Prompts**: `argon.Prompt[In]` registers a prompt whose arguments are the
  fields of a struct (names, descriptions and required-ness from tags).
  String arguments are converted to the field types and validated before
  the handler runs; missing, unknown or malformed arguments yield invalid
  params. Messages via `UserMessage`, `AssistantMessage` or any content
  block, `ErrInvalidArguments`, and options `PromptTitle`, `PromptScopes`,
  `PromptTimeout`, `PromptMaxOutput`. Prompts share the tools' safe
  defaults and per-item scopes.
- Servers may expose only prompts.
- **`argontest.GetPrompt`** for rendering prompts in tests.

### Changed

- **Minimum Go version is now 1.26** (was 1.25), following the Go release
  support policy; required by `golang.org/x/sys` v0.49.
- Tested against Keycloak 26.8. Recent Keycloak versions only allow a client
  to introspect tokens whose audience includes it; see the README note for
  `oidc.Introspection`.

## [v0.1.0-preview.2] — Developer Preview 2

Adds MCP resources.

### Added

- **Resources**: `argon.Resource` for fixed URIs and
  `argon.ResourceTemplate[P]` for RFC 6570 URI templates whose variables
  decode into a typed struct (converted to field types and validated via
  its JSON Schema). Text and binary contents (`TextResource`,
  `BlobResource`), `ErrResourceNotFound`, and options `MIMEType`,
  `ResourceTitle`, `ResourceScopes`, `ResourceTimeout`, `ResourceMaxOutput`.
  Resources share the tools' safe defaults and per-item scopes.
- Servers may expose resources without any tools.
- **`argontest.ReadResource`** for reading resources in tests.

### Changed

- Resource reads made by an authenticated caller are marked
  `cacheScope: private` (the SDK defaults to `public`).

## [v0.1.0-preview.1] — Developer Preview 1

First public preview. The API is not stable yet; feedback is welcome.

### Added

- **Tools**: `argon.Tool[In, Out]` with handlers `func(ctx, In) (Out, error)`;
  JSON Schema inferred from struct tags; input validated (with defaults)
  before the handler runs; errors become `isError` results.
- **Results**: text, structured output (struct outputs), and multi-block
  results with `TextContent`, `ImageContent` and `ResourceLink`.
- **Tool options**: `ReadOnly`, `Destructive`, `Idempotent`, `OpenWorld`,
  `Title`, `Scopes`, `ToolTimeout`, `ToolMaxOutput`, `With`.
- **Transports**: stdio (default) and Streamable HTTP; `Run` picks one from
  `--http` / `ARGON_HTTP`; `Handler` for embedding, including under a path
  prefix with `ResourceMetadata`.
- **Safe defaults**: 60s per-call timeout, 1 MiB result limit, panic
  recovery, fd-level stdout protection on stdio (Unix), answering in-flight
  stdio requests before EOF shutdown, quiet logging.
- **Middleware**: global (`Use`) and per-tool (`With`), with tool metadata on
  `Request.Meta`; opt-in call logging (`LogCalls`) that never logs arguments.
- **Authentication** (HTTP, resource server only): bearer verification,
  `401` + `WWW-Authenticate`, RFC 9728 protected-resource metadata,
  per-tool scopes, `IdentityFrom(ctx)`.
- **`auth/oidc`**: OIDC / RFC 8414 discovery, JWKS cache with rate-limited
  refresh, JWT verification (asymmetric algorithms only; `iss`, `aud`,
  `exp`, `nbf`), RFC 7662 introspection. Tested against Keycloak 26.
- **`auth.Static`** verifier for simple self-hosted setups.
- **`argontest`**: `Call` / `CallResult` over an in-memory MCP session,
  `WithIdentity`, and `FakeIssuer` (discovery, JWKS, introspection).

### Known limitations

- Resources, prompts, audio and embedded-resource content are not supported
  yet.
- Streamable HTTP options (stateless mode, session timeout, resumability)
  are not configurable; SDK defaults apply.
- On non-Unix platforms stdout protection covers `os.Stdout` only.
- Keycloak is the only real authorization server tested so far.

[v0.1.0-rc.1]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-rc.1
[v0.1.0-preview.4]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-preview.4
[v0.1.0-preview.3]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-preview.3
[v0.1.0-preview.2]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-preview.2
[v0.1.0-preview.1]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-preview.1
