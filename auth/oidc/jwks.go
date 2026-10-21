package oidc

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// keySet caches a remote JWKS. A lookup miss triggers a refetch, but no more
// often than minRefresh, so tokens with random kids cannot be used to flood
// the issuer.
type keySet struct {
	url        string
	hc         *http.Client
	minRefresh time.Duration
	now        func() time.Time

	mu        sync.Mutex
	keys      jose.JSONWebKeySet
	lastFetch time.Time
}

func newKeySet(url string, hc *http.Client, minRefresh time.Duration) *keySet {
	return &keySet{url: url, hc: hc, minRefresh: minRefresh, now: time.Now}
}

// lookup returns candidate verification keys for kid. An empty kid returns
// all keys. The set is refetched on a miss, subject to rate limiting.
func (ks *keySet) lookup(ctx context.Context, kid string) ([]jose.JSONWebKey, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if found := ks.find(kid); len(found) > 0 {
		return found, nil
	}
	if !ks.lastFetch.IsZero() && ks.now().Sub(ks.lastFetch) < ks.minRefresh {
		return nil, errors.New("oidc: no matching key (refresh rate-limited)")
	}
	if err := ks.fetchLocked(ctx); err != nil {
		return nil, err
	}
	if found := ks.find(kid); len(found) > 0 {
		return found, nil
	}
	return nil, errors.New("oidc: no matching key")
}

// refresh fetches the key set unconditionally (used at startup).
func (ks *keySet) refresh(ctx context.Context) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	return ks.fetchLocked(ctx)
}

func (ks *keySet) fetchLocked(ctx context.Context) error {
	// Record the attempt even on failure so a broken issuer is not hammered.
	ks.lastFetch = ks.now()
	var set jose.JSONWebKeySet
	if err := getJSON(ctx, ks.hc, ks.url, &set); err != nil {
		return err
	}
	// Keep public signing keys only.
	var keys []jose.JSONWebKey
	for _, k := range set.Keys {
		if k.Valid() && k.IsPublic() && (k.Use == "" || k.Use == "sig") {
			keys = append(keys, k)
		}
	}
	ks.keys = jose.JSONWebKeySet{Keys: keys}
	return nil
}

func (ks *keySet) find(kid string) []jose.JSONWebKey {
	if kid == "" {
		return ks.keys.Keys
	}
	return ks.keys.Key(kid)
}
