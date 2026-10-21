# Changelog

All notable changes to this project are documented here. The project
follows [Semantic Versioning](https://semver.org); until v1.0.0 the API may
change between minor versions and previews.

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

[v0.1.0-preview.1]: https://github.com/x-chunk/argon/releases/tag/v0.1.0-preview.1
