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
//   - unknown "kid" triggers a rate-limited JWKS refresh, and the JWKS is
//     refetched once it is older than MaxKeyAge;
//   - crypto comes from a vetted library, never hand-written.
package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v4"

	"go.xchunk.org/argon/auth"
)

// DefaultAlgorithms is the default signature algorithm allowlist.
var DefaultAlgorithms = []string{"RS256", "ES256", "EdDSA"}

type config struct {
	algorithms   []string
	clockSkew    time.Duration
	httpClient   *http.Client
	claimsMapper auth.ClaimsMapper
	minRefresh   time.Duration
	maxKeyAge    time.Duration
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

// defaultHTTPTimeout bounds each request of the default HTTP client, so an
// unresponsive issuer cannot hold requests indefinitely.
const defaultHTTPTimeout = 15 * time.Second

// HTTPClient sets the client used for discovery, JWKS and introspection.
// The default is a client with a 15s timeout; a custom client should set
// its own timeout.
func HTTPClient(hc *http.Client) Option { return func(c *config) { c.httpClient = hc } }

// ClaimsMapper overrides auth.DefaultClaimsMapper.
func ClaimsMapper(m auth.ClaimsMapper) Option { return func(c *config) { c.claimsMapper = m } }

// MinRefreshInterval rate-limits JWKS refetches on unknown kid. Default 1m.
func MinRefreshInterval(d time.Duration) Option { return func(c *config) { c.minRefresh = d } }

// MaxKeyAge sets how long a fetched JWKS is trusted before it is refetched,
// so keys the issuer withdraws (rotation, compromise) stop being accepted.
// If the refetch fails, the cached keys stay in use. Default 1h; zero
// disables age-based refresh.
func MaxKeyAge(d time.Duration) Option { return func(c *config) { c.maxKeyAge = d } }

// Introspection enables RFC 7662 introspection for opaque (non-JWT) tokens,
// used when the issuer advertises an introspection endpoint.
func Introspection(clientID, clientSecret string) Option {
	return func(c *config) { c.introspect = &introspection{clientID, clientSecret} }
}

// NewVerifier discovers issuer metadata and returns a verifier for tokens
// minted for audience. It fetches the JWKS eagerly, so misconfiguration
// fails at startup rather than on the first request.
func NewVerifier(ctx context.Context, issuer, audience string, opts ...Option) (auth.TokenVerifier, error) {
	if issuer == "" || audience == "" {
		return nil, errors.New("oidc: issuer and audience are required")
	}
	c := &config{
		algorithms:   DefaultAlgorithms,
		clockSkew:    30 * time.Second,
		httpClient:   &http.Client{Timeout: defaultHTTPTimeout},
		claimsMapper: auth.DefaultClaimsMapper,
		minRefresh:   time.Minute,
		maxKeyAge:    time.Hour,
	}
	for _, o := range opts {
		o(c)
	}
	algs, err := allowedAlgorithms(c.algorithms)
	if err != nil {
		return nil, err
	}
	md, err := discover(ctx, c.httpClient, issuer)
	if err != nil {
		return nil, err
	}
	v := &verifier{issuer: issuer, audience: audience, cfg: c, algs: algs, now: time.Now}
	if md.JWKSURI != "" {
		v.keys = newKeySet(md.JWKSURI, c.httpClient, c.minRefresh, c.maxKeyAge)
		if err := v.keys.refresh(ctx); err != nil {
			return nil, fmt.Errorf("oidc: fetch JWKS: %w", err)
		}
	}
	if c.introspect != nil {
		if md.IntrospectionEndpoint == "" {
			return nil, errors.New("oidc: introspection requested but issuer advertises no introspection_endpoint")
		}
		v.introspectURL = md.IntrospectionEndpoint
	}
	if v.keys == nil && v.introspectURL == "" {
		return nil, errors.New("oidc: issuer has no jwks_uri and introspection is not configured")
	}
	return v, nil
}

// allowedAlgorithms validates the allowlist: only asymmetric algorithms are
// accepted, since Argon verifies with public keys from a JWKS.
func allowedAlgorithms(names []string) ([]jose.SignatureAlgorithm, error) {
	if len(names) == 0 {
		return nil, errors.New("oidc: empty algorithm allowlist")
	}
	out := make([]jose.SignatureAlgorithm, 0, len(names))
	for _, n := range names {
		switch a := jose.SignatureAlgorithm(n); a {
		case jose.RS256, jose.RS384, jose.RS512,
			jose.PS256, jose.PS384, jose.PS512,
			jose.ES256, jose.ES384, jose.ES512,
			jose.EdDSA:
			out = append(out, a)
		default:
			return nil, fmt.Errorf("oidc: algorithm %q not allowed (none and HMAC are never accepted)", n)
		}
	}
	return out, nil
}
