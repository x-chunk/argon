# Argon — project conventions

Argon is an MCP server framework for Go built on top of the official SDK
(`github.com/modelcontextprotocol/go-sdk`). See `docs/architecture.md`.

## Commits
- **Atomic commits**: one logical change per commit. Do not mix refactoring,
  features and formatting in one commit.
- Every commit must build and pass checks on its own:
  `gofmt -l . && go build ./... && go vet ./... && go test ./...`
  (`gofmt -l` must print nothing).
- Conventional Commits, in English: `feat: ...`, `fix(auth): ...`,
  `docs: ...`, `chore: ...`, `test(...)`, `refactor(...)`.
  Scope is the package name (`auth`, `oidc`, `sdkadapter`, `argontest`);
  omit it for the root package.

- Integration tests against real services use the `integration` build tag
  (see `test/keycloak`). Run them when touching `auth/oidc` or the HTTP path:
  `go test -tags integration ./...` with Keycloak up.

- CI (`.github/workflows/ci.yml`) enforces gofmt, tidy go.mod, vet,
  govulncheck, the layering rules below, tests on Linux/macOS/Windows and
  the Keycloak integration tests. Keep it green; run its checks locally
  before committing.

## Layering rules
- SDK types (`modelcontextprotocol/go-sdk`) are imported **only** in
  `internal/sdkadapter`. The public API exposes Argon's own types only.
- `auth` is a leaf package with no heavy dependencies (no JOSE/JWT libs).
  Token crypto lives in `auth/oidc`.
- Public types are defined in the root package `argon`, never aliased from
  internal packages, so their docs show on pkg.go.dev.
- `internal/sdkadapter` works on its own plain types and must not import
  `argon`; `argon/adapt.go` converts at the boundary.
- Dependency direction: `argon → internal/sdkadapter, internal/errs, auth`;
  `internal/sdkadapter → go-sdk, internal/errs, auth`; `auth/oidc → auth`.
  Nothing imports the root `argon` package except users, `argontest`
  and examples.

## Code
- Unimplemented code returns `ErrNotImplemented`-style errors and is marked
  `// TODO(v0.1)`; no panics as placeholders.
- Never log tokens; never return raw verification errors to clients.
- Do not write crypto — use vetted libraries behind Argon interfaces.
