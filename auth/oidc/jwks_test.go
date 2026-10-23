package oidc

import (
	"context"
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
	ks := newKeySet(ts.URL, ts.Client(), time.Minute, 0)
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

// A caller that cancels its request must not be able to abort the refresh it
// triggered: otherwise it burns the rate limit and new keys are never picked
// up.
func TestKeySetRefreshDetachedFromCaller(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var rotated atomic.Bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		set := jose.JSONWebKeySet{}
		if rotated.Load() {
			set.Keys = []jose.JSONWebKey{{Key: &priv.PublicKey, KeyID: "k2", Algorithm: "ES256", Use: "sig"}}
		}
		json.NewEncoder(w).Encode(set)
	}))
	defer ts.Close()

	ks := newKeySet(ts.URL, ts.Client(), time.Minute, 0)
	if err := ks.refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	rotated.Store(true)
	ks.lastFetch = time.Time{} // allow the next miss to refetch

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if keys, err := ks.lookup(ctx, "k2"); err != nil || len(keys) != 1 {
		t.Fatalf("lookup with a canceled context must still refresh: %v, %v", keys, err)
	}
}

// Keys the issuer withdraws must stop being trusted once the cache is older
// than maxAge; while the issuer is unreachable the cached keys stay in use.
func TestKeySetMaxAge(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	var withdrawn, down atomic.Bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		set := jose.JSONWebKeySet{}
		if !withdrawn.Load() {
			set.Keys = []jose.JSONWebKey{{Key: &priv.PublicKey, KeyID: "k1", Algorithm: "ES256", Use: "sig"}}
		}
		json.NewEncoder(w).Encode(set)
	}))
	defer ts.Close()

	now := time.Unix(1000, 0)
	ks := newKeySet(ts.URL, ts.Client(), time.Minute, time.Hour)
	ks.now = func() time.Time { return now }
	if err := ks.refresh(t.Context()); err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)
	down.Store(true)
	if _, err := ks.lookup(t.Context(), "k1"); err != nil {
		t.Fatalf("stale keys must stay usable while the issuer is down: %v", err)
	}

	now = now.Add(2 * time.Minute) // past minRefresh after the failed attempt
	down.Store(false)
	withdrawn.Store(true)
	if _, err := ks.lookup(t.Context(), "k1"); err == nil {
		t.Fatal("a withdrawn key must not be trusted after the cache expired")
	}
}
