package oidc

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// jwksFetchTimeout bounds a single JWKS fetch.
const jwksFetchTimeout = 10 * time.Second

// keySet caches a remote JWKS. A lookup miss triggers a refetch, but no more
// often than minRefresh, so tokens with random kids cannot be used to flood
// the issuer.
//
// Fetches are detached from the request that triggered them: a caller that
// cancels its request cannot abort the fetch and so burn the refresh budget
// without updating the keys. Lookups of cached keys never wait for a fetch.
type keySet struct {
	url        string
	hc         *http.Client
	minRefresh time.Duration
	now        func() time.Time

	fetchMu sync.Mutex // serializes fetches

	mu        sync.RWMutex // guards the fields below
	keys      jose.JSONWebKeySet
	lastFetch time.Time
}

func newKeySet(url string, hc *http.Client, minRefresh time.Duration) *keySet {
	return &keySet{url: url, hc: hc, minRefresh: minRefresh, now: time.Now}
}

// lookup returns candidate verification keys for kid. An empty kid returns
// all keys. The set is refetched on a miss, subject to rate limiting.
func (ks *keySet) lookup(ctx context.Context, kid string) ([]jose.JSONWebKey, error) {
	if found := ks.find(kid); len(found) > 0 {
		return found, nil
	}
	ks.fetchMu.Lock()
	defer ks.fetchMu.Unlock()
	// A fetch that finished while we waited may have brought the key.
	if found := ks.find(kid); len(found) > 0 {
		return found, nil
	}
	ks.mu.RLock()
	last := ks.lastFetch
	ks.mu.RUnlock()
	if !last.IsZero() && ks.now().Sub(last) < ks.minRefresh {
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
	ks.fetchMu.Lock()
	defer ks.fetchMu.Unlock()
	return ks.fetchLocked(ctx)
}

// fetchLocked fetches the key set; ks.fetchMu must be held. On failure the
// previous keys stay in use.
func (ks *keySet) fetchLocked(ctx context.Context) error {
	// Record the attempt even on failure so a broken issuer is not hammered.
	ks.mu.Lock()
	ks.lastFetch = ks.now()
	ks.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jwksFetchTimeout)
	defer cancel()
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
	ks.mu.Lock()
	ks.keys = jose.JSONWebKeySet{Keys: keys}
	ks.mu.Unlock()
	return nil
}

func (ks *keySet) find(kid string) []jose.JSONWebKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if kid == "" {
		return ks.keys.Keys
	}
	return ks.keys.Key(kid)
}
