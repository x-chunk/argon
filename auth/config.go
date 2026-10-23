package auth

import "errors"

// Config describes how the HTTP transport authenticates callers.
// Argon acts as an OAuth 2.1 resource server only; it never issues tokens.
type Config struct {
	// Issuer is the authorization server issuer URL. It is only advertised
	// (as the default AuthServer); verification uses the issuer given to the
	// Verifier, such as oidc.NewVerifier, which must be the same.
	Issuer string
	// Audience is the resource identifier tokens must be minted for, i.e.
	// the server's URL. Required. It is advertised in protected-resource
	// metadata and must match the audience the Verifier checks.
	Audience string
	// AuthServer is advertised in protected-resource metadata (RFC 9728).
	// Defaults to Issuer.
	AuthServer string
	// Verifier validates tokens. Required. Use auth/oidc for issuer-driven
	// JWT/JWKS verification, or Static for a pre-shared token. Keeping the
	// verifier explicit lets stdio-only users avoid JOSE dependencies.
	Verifier TokenVerifier
	// ResourceMetadataPath is where protected-resource metadata is served,
	// as an absolute path on the Audience origin. Defaults to the RFC 9728
	// location derived from the Audience path:
	// "/.well-known/oauth-protected-resource" plus that path (e.g.
	// ".../oauth-protected-resource/mcp" for audience "https://host/mcp").
	ResourceMetadataPath string
}

// Validate reports configuration errors. Auth mode must refuse to start
// without an audience, so tokens for other resources are never accepted.
func (c *Config) Validate() error {
	if c.Audience == "" {
		return errors.New("auth: audience is required")
	}
	if c.Verifier == nil {
		return errors.New("auth: Verifier is required")
	}
	return nil
}
