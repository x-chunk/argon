package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"maps"
	"slices"
)

// Static returns a verifier that accepts a single pre-shared bearer token and
// returns id for it. Intended for simple self-hosted setups where full OAuth
// is overkill. Comparison is constant-time.
func Static(token string, id Identity) TokenVerifier {
	return VerifierFunc(func(_ context.Context, got string) (*Identity, error) {
		if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			return nil, fmt.Errorf("%w: static token mismatch", ErrInvalidToken)
		}
		// Return a fresh copy so callers cannot mutate the configured identity.
		return &Identity{
			Subject: id.Subject,
			Scopes:  slices.Clone(id.Scopes),
			Claims:  maps.Clone(id.Claims),
		}, nil
	})
}
