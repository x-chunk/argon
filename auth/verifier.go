package auth

import (
	"context"
	"errors"
	"strings"
)

// ErrInvalidToken is returned (possibly wrapped) by a TokenVerifier when the
// token is missing, malformed, expired or otherwise not acceptable.
// Its details must never be echoed to the client.
var ErrInvalidToken = errors.New("auth: invalid token")

// TokenVerifier validates a bearer token and returns the caller's identity.
// Implementations must check signature, expiry, issuer and audience.
type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*Identity, error)
}

// VerifierFunc adapts a function to TokenVerifier.
type VerifierFunc func(ctx context.Context, token string) (*Identity, error)

// Verify implements TokenVerifier.
func (f VerifierFunc) Verify(ctx context.Context, token string) (*Identity, error) {
	return f(ctx, token)
}

// ClaimsMapper builds an Identity from verified token claims. It is the single
// place where provider-specific claim layouts are handled.
type ClaimsMapper func(claims map[string]any) *Identity

// DefaultClaimsMapper reads "sub" and scopes from either "scope"
// (space-separated string, RFC 8693) or "scp" (string or array).
func DefaultClaimsMapper(claims map[string]any) *Identity {
	id := &Identity{Claims: claims}
	id.Subject, _ = claims["sub"].(string)
	for _, key := range []string{"scope", "scp"} {
		switch v := claims[key].(type) {
		case string:
			id.Scopes = append(id.Scopes, strings.Fields(v)...)
		case []any:
			for _, s := range v {
				if s, ok := s.(string); ok {
					id.Scopes = append(id.Scopes, s)
				}
			}
		}
	}
	return id
}
