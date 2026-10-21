package oidc_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"go.xchunk.org/argon/argontest"
	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/auth/oidc"
)

const aud = "https://mcp.example"

func newVerifier(t *testing.T, f *argontest.FakeIssuer, opts ...oidc.Option) auth.TokenVerifier {
	t.Helper()
	v, err := oidc.NewVerifier(t.Context(), f.URL, aud, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVerifyValidJWT(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	v := newVerifier(t, f)
	id, err := v.Verify(t.Context(), f.Token("alice", aud, "notes:read", "notes:write"))
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "alice" || !id.HasScope("notes:write") || id.Claims["iss"] != f.URL {
		t.Fatalf("identity = %+v", id)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	v := newVerifier(t, f, oidc.ClockSkew(time.Second))
	now := time.Now()
	claims := func(mut func(map[string]any)) string {
		c := map[string]any{"iss": f.URL, "sub": "u", "aud": aud, "exp": now.Add(time.Hour).Unix()}
		mut(c)
		return f.Sign(c)
	}
	tests := map[string]string{
		"empty":        "",
		"garbage":      "a.b.c",
		"expired":      claims(func(c map[string]any) { c["exp"] = now.Add(-time.Hour).Unix() }),
		"not yet":      claims(func(c map[string]any) { c["nbf"] = now.Add(time.Hour).Unix() }),
		"no exp":       claims(func(c map[string]any) { delete(c, "exp") }),
		"wrong aud":    claims(func(c map[string]any) { c["aud"] = "https://other.example" }),
		"wrong iss":    claims(func(c map[string]any) { c["iss"] = "https://evil.example" }),
		"hs256":        hs256Token(t, f.URL),
		"alg none":     noneToken(f.URL),
		"tampered sig": tamper(f.Token("u", aud)),
	}
	for name, tok := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(t.Context(), tok); !errors.Is(err, auth.ErrInvalidToken) {
				t.Fatalf("err = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyKeyRotation(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	// Both verifiers cache the original key at startup.
	eager := newVerifier(t, f, oidc.MinRefreshInterval(0))
	limited := newVerifier(t, f) // default 1m refresh interval
	f.RotateKey()
	tok := f.Token("u", aud)

	// Unknown kid triggers a JWKS refetch.
	if _, err := eager.Verify(t.Context(), tok); err != nil {
		t.Fatalf("after rotation: %v", err)
	}
	// Within MinRefreshInterval of the startup fetch, no refetch happens.
	if _, err := limited.Verify(t.Context(), tok); err == nil {
		t.Fatal("refetch within MinRefreshInterval must be rate-limited")
	}
}

func TestNewVerifierRejectsBadAlgorithms(t *testing.T) {
	f := argontest.NewFakeIssuer(t)
	for _, alg := range []string{"none", "HS256"} {
		if _, err := oidc.NewVerifier(t.Context(), f.URL, aud, oidc.Algorithms(alg)); err == nil {
			t.Errorf("%s must be rejected", alg)
		}
	}
	if _, err := oidc.NewVerifier(t.Context(), f.URL, ""); err == nil {
		t.Error("missing audience must be rejected")
	}
}

func hs256Token(t *testing.T, iss string) string {
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte("0123456789abcdef0123456789abcdef")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := jwt.Signed(sig).Claims(map[string]any{"iss": iss, "aud": aud, "exp": time.Now().Add(time.Hour).Unix()}).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// noneToken builds an unsigned token by hand (go-jose refuses to sign "none").
func noneToken(iss string) string {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return enc(`{"alg":"none","typ":"JWT"}`) + "." + enc(`{"iss":"`+iss+`","aud":"`+aud+`","exp":9999999999}`) + "."
}

func tamper(tok string) string {
	parts := strings.Split(tok, ".")
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	return parts[0] + "." + parts[1] + "." + string(sig)
}
