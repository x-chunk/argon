package oidc

import (
	"crypto/sha256"
	"maps"
	"slices"
	"sync"
	"time"

	"go.xchunk.org/argon/auth"
)

// Introspection runs an outbound request per token, and any bearer string
// that is not a JWT ends up there, so it is bounded on two sides: active
// results are cached briefly, and at most introspectConcurrency requests run
// at a time. Inactive results are not cached: a negative cache cannot stop
// random tokens, and the concurrency limit already bounds them.
const (
	introspectCacheTTL     = 30 * time.Second
	introspectCacheMax     = 10000
	introspectConcurrency  = 16
	introspectCacheKeySize = sha256.Size
)

type introspectEntry struct {
	id      *auth.Identity
	expires time.Time
}

// introspectCache maps token hashes to identities from active introspection
// results. Tokens themselves are never stored.
type introspectCache struct {
	mu      sync.Mutex
	entries map[[introspectCacheKeySize]byte]introspectEntry
}

func newIntrospectCache() *introspectCache {
	return &introspectCache{entries: make(map[[introspectCacheKeySize]byte]introspectEntry)}
}

func (c *introspectCache) get(token string, now time.Time) (*auth.Identity, bool) {
	key := sha256.Sum256([]byte(token))
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(e.expires) {
		delete(c.entries, key)
		return nil, false
	}
	return cloneIdentity(e.id), true
}

// put caches id until the earlier of the token's expiry and the cache TTL.
func (c *introspectCache) put(token string, id *auth.Identity, exp, now time.Time) {
	expires := now.Add(introspectCacheTTL)
	if !exp.IsZero() && exp.Before(expires) {
		expires = exp
	}
	if !now.Before(expires) {
		return
	}
	key := sha256.Sum256([]byte(token))
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= introspectCacheMax {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= introspectCacheMax {
			return
		}
	}
	c.entries[key] = introspectEntry{id: cloneIdentity(id), expires: expires}
}

// cloneIdentity copies id so callers cannot mutate a cached identity.
func cloneIdentity(id *auth.Identity) *auth.Identity {
	return &auth.Identity{Subject: id.Subject, Scopes: slices.Clone(id.Scopes), Claims: maps.Clone(id.Claims)}
}
