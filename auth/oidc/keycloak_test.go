//go:build integration

package oidc

import (
	"errors"
	"strings"
	"testing"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/kctest"
)

func TestKeycloakJWT(t *testing.T) {
	iss := kctest.Issuer(t)
	v, err := NewVerifier(t.Context(), iss, kctest.Audience)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}

	id, err := v.Verify(t.Context(), kctest.Token(t, iss, "argon-test", "notes:write"))
	if err != nil {
		t.Fatalf("valid token: %v", err)
	}
	if id.Subject == "" || !id.HasScope("notes:read") || !id.HasScope("notes:write") {
		t.Fatalf("identity = %+v", id)
	}
	if id.Claims["azp"] != "argon-test" {
		t.Fatalf("azp = %v", id.Claims["azp"])
	}

	// Default scopes only: notes:write was not requested.
	id, err = v.Verify(t.Context(), kctest.Token(t, iss, "argon-test"))
	if err != nil || id.HasScope("notes:write") {
		t.Fatalf("default scopes: %+v, %v", id, err)
	}

	// Token minted for another resource.
	if _, err := v.Verify(t.Context(), kctest.Token(t, iss, "argon-other")); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("other audience: err = %v, want ErrInvalidToken", err)
	}

	// Tampered payload.
	tok := kctest.Token(t, iss, "argon-test")
	parts := strings.Split(tok, ".")
	parts[1] = parts[1][:len(parts[1])-2] + "AA"
	if _, err := v.Verify(t.Context(), strings.Join(parts, ".")); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("tampered: err = %v, want ErrInvalidToken", err)
	}
}

// Keycloak only issues JWTs, so introspection is exercised directly: the
// response must be accepted for our audience and rejected for another.
func TestKeycloakIntrospection(t *testing.T) {
	iss := kctest.Issuer(t)
	tv, err := NewVerifier(t.Context(), iss, kctest.Audience,
		Introspection(kctest.ServerClientID, kctest.ServerClientSecret))
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	v := tv.(*verifier)

	id, err := v.introspect(t.Context(), kctest.Token(t, iss, "argon-test", "notes:write"))
	if err != nil {
		t.Fatalf("introspect valid: %v", err)
	}
	if id.Subject == "" || !id.HasScope("notes:write") {
		t.Fatalf("identity = %+v", id)
	}
	if _, err := v.introspect(t.Context(), kctest.Token(t, iss, "argon-other")); err == nil {
		t.Fatal("introspect other audience: want error")
	}
	if _, err := v.introspect(t.Context(), "garbage"); err == nil {
		t.Fatal("introspect garbage: want error")
	}

	bad, err := NewVerifier(t.Context(), iss, kctest.Audience, Introspection("argon-mcp", "wrong"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.(*verifier).introspect(t.Context(), kctest.Token(t, iss, "argon-test")); err == nil {
		t.Fatal("introspect with bad client credentials: want error")
	}
}
