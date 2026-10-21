// Package auth defines the caller identity and token verification contracts
// used by Argon's HTTP transport.
//
// The package is intentionally lightweight: it has no JOSE/JWT dependencies.
// Concrete JWT/JWKS verification lives in the auth/oidc subpackage.
package auth

import (
	"context"
	"slices"
)

// Identity is what a handler sees after successful authentication.
type Identity struct {
	// Subject is the authenticated principal (the "sub" claim).
	Subject string
	// Scopes granted to the token.
	Scopes []string
	// Claims holds the raw token claims, if any.
	Claims map[string]any
}

// HasScope reports whether the identity was granted scope s.
func (id *Identity) HasScope(s string) bool {
	return id != nil && slices.Contains(id.Scopes, s)
}

type ctxKey struct{}

// NewContext returns a copy of ctx carrying id.
func NewContext(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the identity stored in ctx, or nil.
func FromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	return id
}
