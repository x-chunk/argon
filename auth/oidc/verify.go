package oidc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"go.xchunk.org/argon/auth"
)

type verifier struct {
	issuer   string
	audience string
	cfg      *config
	algs     []jose.SignatureAlgorithm
	now      func() time.Time

	keys          *keySet // nil when the issuer has no JWKS
	introspectURL string  // empty when introspection is disabled
}

// Verify implements auth.TokenVerifier. Every failure wraps
// auth.ErrInvalidToken; details are for server logs only.
func (v *verifier) Verify(ctx context.Context, token string) (*auth.Identity, error) {
	id, err := v.verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
	}
	return id, nil
}

func (v *verifier) verify(ctx context.Context, token string) (*auth.Identity, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	if strings.Count(token, ".") == 2 && v.keys != nil {
		return v.verifyJWT(ctx, token)
	}
	if v.introspectURL != "" {
		return v.introspect(ctx, token)
	}
	return nil, errors.New("unsupported token format")
}

func (v *verifier) verifyJWT(ctx context.Context, token string) (*auth.Identity, error) {
	// The allowlist is enforced here: tokens whose "alg" is not in it,
	// including "none" and HS*, are rejected before any key is touched.
	tok, err := jwt.ParseSigned(token, v.algs)
	if err != nil {
		return nil, err
	}
	hdr := tok.Headers[0]
	keys, err := v.keys.lookup(ctx, hdr.KeyID)
	if err != nil {
		return nil, err
	}
	var std jwt.Claims
	var raw map[string]any
	verified := false
	for _, k := range keys {
		if k.Algorithm != "" && k.Algorithm != hdr.Algorithm {
			continue
		}
		if err := tok.Claims(k.Key, &std, &raw); err == nil {
			verified = true
			break
		}
	}
	if !verified {
		return nil, errors.New("signature verification failed")
	}
	if std.Expiry == nil {
		return nil, errors.New("missing exp")
	}
	err = std.ValidateWithLeeway(jwt.Expected{
		Issuer:      v.issuer,
		AnyAudience: jwt.Audience{v.audience},
		Time:        v.now(),
	}, v.cfg.clockSkew)
	if err != nil {
		return nil, err
	}
	return v.identity(raw), nil
}

func (v *verifier) identity(claims map[string]any) *auth.Identity {
	id := v.cfg.claimsMapper(claims)
	if id == nil {
		id = &auth.Identity{}
	}
	if id.Claims == nil {
		id.Claims = claims
	}
	return id
}

// introspect is implemented in introspect.go.
func (v *verifier) introspect(ctx context.Context, token string) (*auth.Identity, error) {
	return nil, errors.New("introspection not implemented")
}
