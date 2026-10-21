package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestWellKnownURLs(t *testing.T) {
	got, err := wellKnownURLs("https://idp.example/tenant/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"https://idp.example/tenant/.well-known/openid-configuration",
		"https://idp.example/.well-known/oauth-authorization-server/tenant",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, bad := range []string{"", "idp.example", "ftp://idp", "https://idp/?x=1"} {
		if _, err := wellKnownURLs(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestDiscover(t *testing.T) {
	var issuer string
	mux := http.NewServeMux()
	// Only RFC 8414 is served, so the OIDC attempt must fall through.
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(metadata{Issuer: issuer, JWKSURI: issuer + "/jwks"})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	issuer = ts.URL

	md, err := discover(t.Context(), ts.Client(), issuer)
	if err != nil {
		t.Fatal(err)
	}
	if md.JWKSURI != issuer+"/jwks" {
		t.Fatalf("jwks_uri = %q", md.JWKSURI)
	}

	if _, err := discover(t.Context(), ts.Client(), issuer+"/other"); err == nil {
		t.Fatal("unknown issuer path must fail")
	}
}

func TestDiscoverIssuerMismatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(metadata{Issuer: "https://evil.example", JWKSURI: "https://evil.example/jwks"})
	}))
	defer ts.Close()
	if _, err := discover(t.Context(), ts.Client(), ts.URL); err == nil {
		t.Fatal("issuer mismatch must be rejected")
	}
}
