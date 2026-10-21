package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
)

// Static returns a verifier that accepts a single pre-shared bearer token and
// returns id for it. Intended for simple self-hosted setups where full OAuth
// is overkill. Comparison is constant-time.
func Static(token string, id Identity) TokenVerifier {
	return VerifierFunc(func(_ context.Context, got string) (*Identity, error) {
		if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			return nil, fmt.Errorf("%w: static token mismatch", ErrInvalidToken)
		}
		out := id
		return &out, nil
	})
}
