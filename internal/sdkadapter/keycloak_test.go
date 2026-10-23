//go:build integration

package sdkadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
	"go.xchunk.org/argon/internal/kctest"
)

// TestKeycloakEndToEnd runs the full HTTP path — SDK bearer middleware,
// Argon OIDC verifier, identity propagation, per-tool scopes — with real
// Keycloak tokens and a real SDK client.
func TestKeycloakEndToEnd(t *testing.T) {
	iss := kctest.Issuer(t)
	v, err := oidc.NewVerifier(t.Context(), iss, kctest.Audience)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	h, err := HTTPHandler(testServer(), &auth.Config{Issuer: iss, Audience: kctest.Audience, Verifier: v}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	// Cleanup (not defer), so client sessions registered later by connect
	// close first; otherwise ts.Close waits on their open SSE streams.
	t.Cleanup(ts.Close)

	// Tokens for another resource are refused at the HTTP layer.
	req, _ := http.NewRequest(http.MethodPost, ts.URL, nil)
	req.Header.Set("Authorization", "Bearer "+kctest.Token(t, iss, "argon-other"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("foreign audience: status %d, want 401", resp.StatusCode)
	}

	// testServer's "admin" tool requires the "admin" scope, which Keycloak
	// never grants here; "whoami" needs no scope.
	cs := connect(t, ts.URL, kctest.Token(t, iss, "argon-test", "notes:write"))
	sub, isErr := callText(t, cs, "whoami")
	if isErr || sub == "" || sub == "anonymous" {
		t.Fatalf("whoami = %q (isError=%v), want the Keycloak subject", sub, isErr)
	}
	if _, isErr := callText(t, cs, "admin"); !isErr {
		t.Fatal("admin without scope must be an error result")
	}
}
