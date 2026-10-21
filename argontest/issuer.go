package argontest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Fake issuer client credentials accepted by its introspection endpoint.
const (
	FakeClientID     = "argontest-client"
	FakeClientSecret = "argontest-secret"
)

// FakeIssuer is an in-process OAuth authorization server for auth tests.
// It serves OIDC discovery metadata, a JWKS and an RFC 7662 introspection
// endpoint, and mints ES256-signed JWTs and opaque tokens.
type FakeIssuer struct {
	// URL is the issuer URL.
	URL string

	t      testing.TB
	mu     sync.Mutex
	key    *ecdsa.PrivateKey
	kid    string
	opaque map[string]map[string]any
}

// NewFakeIssuer starts a fake issuer that is shut down with the test.
func NewFakeIssuer(t testing.TB) *FakeIssuer {
	t.Helper()
	f := &FakeIssuer{t: t, opaque: make(map[string]map[string]any)}
	f.RotateKey()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.serveMetadata)
	mux.HandleFunc("GET /jwks", f.serveJWKS)
	mux.HandleFunc("POST /introspect", f.serveIntrospect)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	f.URL = ts.URL
	return f
}

// RotateKey replaces the signing key with a new one under a new kid.
// Tokens minted afterwards are signed with the new key.
func (f *FakeIssuer) RotateKey() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		f.t.Fatalf("argontest: generate key: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.key, f.kid = key, randomID()
}

// Token mints a valid JWT for audience with the given subject and scopes,
// expiring in one hour.
func (f *FakeIssuer) Token(subject, audience string, scopes ...string) string {
	now := time.Now()
	return f.Sign(map[string]any{
		"iss":   f.URL,
		"sub":   subject,
		"aud":   audience,
		"scope": strings.Join(scopes, " "),
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
}

// Sign mints a JWT with exactly the given claims, signed with the current
// key. Use it to build invalid tokens (expired, wrong audience, ...).
func (f *FakeIssuer) Sign(claims map[string]any) string {
	f.mu.Lock()
	key, kid := f.key, f.kid
	f.mu.Unlock()
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid))
	if err != nil {
		f.t.Fatalf("argontest: signer: %v", err)
	}
	tok, err := jwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatalf("argontest: sign: %v", err)
	}
	return tok
}

// OpaqueToken mints a non-JWT token that is only valid via introspection.
func (f *FakeIssuer) OpaqueToken(subject, audience string, scopes ...string) string {
	tok := randomID()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opaque[tok] = map[string]any{
		"active":    true,
		"iss":       f.URL,
		"sub":       subject,
		"aud":       audience,
		"scope":     strings.Join(scopes, " "),
		"exp":       time.Now().Add(time.Hour).Unix(),
		"client_id": FakeClientID,
	}
	return tok
}

func (f *FakeIssuer) serveMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                 f.URL,
		"jwks_uri":               f.URL + "/jwks",
		"introspection_endpoint": f.URL + "/introspect",
	})
}

func (f *FakeIssuer) serveJWKS(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &f.key.PublicKey, KeyID: f.kid, Algorithm: string(jose.ES256), Use: "sig",
	}}})
}

func (f *FakeIssuer) serveIntrospect(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok || id != FakeClientID || secret != FakeClientSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	claims, found := f.opaque[r.PostFormValue("token")]
	claims = maps.Clone(claims)
	f.mu.Unlock()
	if !found {
		claims = map[string]any{"active": false}
	}
	writeJSON(w, claims)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
