package oidc_test

import (
	"errors"
	"testing"

	"go.xchunk.org/argon/argontest"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
)

func TestIntrospection(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	v := newVerifier(t, f, oidc.Introspection(argontest.FakeClientID, argontest.FakeClientSecret))

	id, err := v.Verify(t.Context(), f.OpaqueToken("bob", aud, "notes:read"))
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "bob" || !id.HasScope("notes:read") {
		t.Fatalf("identity = %+v", id)
	}

	for name, tok := range map[string]string{
		"unknown":   "not-a-real-token",
		"wrong aud": f.OpaqueToken("bob", "https://other.example"),
	} {
		if _, err := v.Verify(t.Context(), tok); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("%s: err = %v, want ErrInvalidToken", name, err)
		}
	}

	// JWTs still go through local verification.
	if _, err := v.Verify(t.Context(), f.Token("alice", aud)); err != nil {
		t.Fatalf("jwt with introspection enabled: %v", err)
	}
}

func TestIntrospectionBadCredentials(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	v := newVerifier(t, f, oidc.Introspection("wrong", "creds"))
	if _, err := v.Verify(t.Context(), f.OpaqueToken("bob", aud)); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestOpaqueWithoutIntrospection(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	v := newVerifier(t, f)
	if _, err := v.Verify(t.Context(), f.OpaqueToken("bob", aud)); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}
