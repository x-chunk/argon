package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.xchunk.org/argon/auth"
)

func TestIntrospectionCached(t *testing.T) {
	var calls atomic.Int32
	now := time.Unix(1000, 0)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		claims := map[string]any{"active": false}
		if r.PostFormValue("token") == "good" {
			claims = map[string]any{"active": true, "sub": "bob", "aud": "https://mcp", "exp": now.Add(time.Hour).Unix()}
		}
		json.NewEncoder(w).Encode(claims)
	}))
	defer ts.Close()

	v := &verifier{
		audience:      "https://mcp",
		cfg:           &config{httpClient: ts.Client(), claimsMapper: auth.DefaultClaimsMapper, introspect: &introspection{}},
		now:           func() time.Time { return now },
		introspectURL: ts.URL,
		icache:        newIntrospectCache(),
		isem:          make(chan struct{}, introspectConcurrency),
	}
	for range 3 {
		id, err := v.Verify(t.Context(), "good")
		if err != nil || id.Subject != "bob" {
			t.Fatalf("Verify = %+v, %v", id, err)
		}
		id.Subject = "mallory" // must not leak into the cache
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("introspection calls = %d, want 1 (active results are cached)", n)
	}
	now = now.Add(introspectCacheTTL)
	if _, err := v.Verify(t.Context(), "good"); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("introspection calls = %d, want 2 after the cache TTL", n)
	}
	for range 2 {
		v.Verify(t.Context(), "bad")
	}
	if n := calls.Load(); n != 4 {
		t.Fatalf("introspection calls = %d, want 4 (inactive results are not cached)", n)
	}
}
