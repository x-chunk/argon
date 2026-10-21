package argon

import (
	"context"

	"go.xchunk.org/argon/auth"
)

// Identity is the authenticated caller. See package auth.
type Identity = auth.Identity

// IdentityFrom returns the caller identity, or nil when the call is not
// authenticated (e.g. on stdio).
func IdentityFrom(ctx context.Context) *Identity { return auth.FromContext(ctx) }
