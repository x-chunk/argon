// Package oidc provides an issuer-driven token verifier: OIDC Discovery /
// RFC 8414 metadata, JWKS-backed JWT verification and RFC 7662 introspection
// for opaque tokens.
//
// It is a separate package so that users who only need stdio or static
// tokens do not pull in JOSE dependencies.
//
// Hard requirements for the implementation:
//   - the algorithm allowlist is fixed by configuration; the token's "alg"
//     header is never trusted, "none" and HS* are always rejected;
//   - only the configured issuer is contacted (no issuer from tokens → no SSRF);
//   - "iss", "aud", "exp" and "nbf" are checked, with a small clock skew;
//   - unknown "kid" triggers a rate-limited JWKS refresh;
//   - crypto comes from a vetted library, never hand-written.
package oidc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.xchunk.org/argon/auth"
)

// ErrNotImplemented is returned while the verifier is a skeleton.
var ErrNotImplemented = errors.New("oidc: not implemented")

// DefaultAlgorithms is the default signature algorithm allowlist.
var DefaultAlgorithms = []string{"RS256", "ES256", "EdDSA"}

type config struct {
	algorithms   []string
	clockSkew    time.Duration
	httpClient   *http.Client
	claimsMapper auth.ClaimsMapper
	minRefresh   time.Duration
	introspect   *introspection
}

type introspection struct {
	clientID, clientSecret string
}

// Option configures the verifier.
type Option func(*config)

// Algorithms replaces the signature algorithm allowlist.
func Algorithms(algs ...string) Option { return func(c *config) { c.algorithms = algs } }

// ClockSkew sets the tolerance for exp/nbf checks. Default 30s.
func ClockSkew(d time.Duration) Option { return func(c *config) { c.clockSkew = d } }

// HTTPClient sets the client used for discovery, JWKS and introspection.
func HTTPClient(hc *http.Client) Option { return func(c *config) { c.httpClient = hc } }

// ClaimsMapper overrides auth.DefaultClaimsMapper.
func ClaimsMapper(m auth.ClaimsMapper) Option { return func(c *config) { c.claimsMapper = m } }

// MinRefreshInterval rate-limits JWKS refetches on unknown kid. Default 1m.
func MinRefreshInterval(d time.Duration) Option { return func(c *config) { c.minRefresh = d } }

// Introspection enables RFC 7662 introspection for opaque (non-JWT) tokens,
// used when the issuer advertises an introspection endpoint.
func Introspection(clientID, clientSecret string) Option {
	return func(c *config) { c.introspect = &introspection{clientID, clientSecret} }
}

// NewVerifier discovers issuer metadata and returns a verifier for tokens
// minted for audience.
func NewVerifier(ctx context.Context, issuer, audience string, opts ...Option) (auth.TokenVerifier, error) {
	if issuer == "" || audience == "" {
		return nil, errors.New("oidc: issuer and audience are required")
	}
	c := &config{
		algorithms:   DefaultAlgorithms,
		clockSkew:    30 * time.Second,
		httpClient:   http.DefaultClient,
		claimsMapper: auth.DefaultClaimsMapper,
		minRefresh:   time.Minute,
	}
	for _, o := range opts {
		o(c)
	}
	// TODO(v0.1): discovery (OIDC, then RFC 8414), JWKS cache, JWT
	// verification via a vetted library, introspection fallback.
	return nil, ErrNotImplemented
}
