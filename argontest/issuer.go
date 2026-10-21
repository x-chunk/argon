package argontest

import "testing"

// FakeIssuer is an in-process OAuth authorization server for auth tests:
// it serves discovery metadata and a JWKS, and mints signed tokens.
type FakeIssuer struct {
	// URL is the issuer URL (an httptest server).
	URL string
}

// NewFakeIssuer starts a fake issuer that is shut down with the test.
func NewFakeIssuer(t testing.TB) *FakeIssuer {
	t.Helper()
	// TODO(v0.1): httptest server with /.well-known/openid-configuration,
	// JWKS endpoint and a Token(sub, aud, scopes...) minting method.
	t.Skip("argontest: FakeIssuer not implemented yet")
	return nil
}
