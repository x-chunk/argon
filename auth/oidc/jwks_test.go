package oidc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

func TestKeySetRefreshRateLimited(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var fetches atomic.Int32
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &priv.PublicKey, KeyID: "k1", Algorithm: "ES256", Use: "sig"},
		{Key: priv, KeyID: "private"}, // private keys must be ignored
	}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		json.NewEncoder(w).Encode(set)
	}))
	defer ts.Close()

	now := time.Unix(1000, 0)
	ks := newKeySet(ts.URL, ts.Client(), time.Minute)
	ks.now = func() time.Time { return now }
	ctx := t.Context()

	if err := ks.refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if keys, err := ks.lookup(ctx, "k1"); err != nil || len(keys) != 1 {
		t.Fatalf("k1: %v, %v", keys, err)
	}
	if _, err := ks.lookup(ctx, "private"); err == nil {
		t.Fatal("private key must not be usable")
	}
	if _, err := ks.lookup(ctx, "unknown"); err == nil {
		t.Fatal("unknown kid must fail")
	}
	if n := fetches.Load(); n != 1 {
		t.Fatalf("fetches = %d, want 1 (misses within minRefresh are rate-limited)", n)
	}

	now = now.Add(2 * time.Minute)
	ks.lookup(ctx, "unknown")
	if n := fetches.Load(); n != 2 {
		t.Fatalf("fetches = %d, want 2 after minRefresh elapsed", n)
	}
}
