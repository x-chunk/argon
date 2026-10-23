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
// the issuer. Keys older than maxAge are refetched too, so keys the issuer
// has withdrawn stop being trusted; if that refetch fails, the cached keys
// stay in use rather than failing every request while the issuer is down.
//
// Fetches are detached from the request that triggered them: a caller that
// cancels its request cannot abort the fetch and so burn the refresh budget
// without updating the keys. Lookups of fresh cached keys never wait for a
// fetch.
type keySet struct {
	url        string
	hc         *http.Client
	minRefresh time.Duration
	maxAge     time.Duration // zero disables age-based refresh
	now        func() time.Time

	fetchMu sync.Mutex // serializes fetches

	mu          sync.RWMutex // guards the fields below
	keys        jose.JSONWebKeySet
	lastFetch   time.Time // last attempt
	lastSuccess time.Time
}

func newKeySet(url string, hc *http.Client, minRefresh, maxAge time.Duration) *keySet {
	return &keySet{url: url, hc: hc, minRefresh: minRefresh, maxAge: maxAge, now: time.Now}
}

// lookup returns candidate verification keys for kid. An empty kid returns
// all keys. The set is refetched on a miss or when stale, subject to rate
// limiting.
func (ks *keySet) lookup(ctx context.Context, kid string) ([]jose.JSONWebKey, error) {
	if found := ks.find(kid); len(found) > 0 && !ks.stale() {
		return found, nil
	}
	ks.fetchMu.Lock()
	defer ks.fetchMu.Unlock()
	// A fetch that finished while we waited may have brought the key.
	found := ks.find(kid)
	if len(found) > 0 && !ks.stale() {
		return found, nil
	}
	ks.mu.RLock()
	last := ks.lastFetch
	ks.mu.RUnlock()
	if !last.IsZero() && ks.now().Sub(last) < ks.minRefresh {
		if len(found) > 0 {
			return found, nil // stale, but a refetch was just attempted
		}
		return nil, errors.New("oidc: no matching key (refresh rate-limited)")
	}
	if err := ks.fetchLocked(ctx); err != nil {
		if len(found) > 0 {
			return found, nil // keep stale keys while the issuer is unreachable
		}
		return nil, err
	}
	if found := ks.find(kid); len(found) > 0 {
		return found, nil
	}
	return nil, errors.New("oidc: no matching key")
}

// stale reports whether the cached keys are older than maxAge.
func (ks *keySet) stale() bool {
	if ks.maxAge <= 0 {
		return false
	}
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.now().Sub(ks.lastSuccess) >= ks.maxAge
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
	ks.lastSuccess = ks.now()
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
