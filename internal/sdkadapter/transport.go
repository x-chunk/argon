package sdkadapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"go.xchunk.org/argon/auth"
)

const defaultResourceMetadataPath = "/.well-known/oauth-protected-resource"

// ServeStdio runs srv over in/out until the client disconnects or ctx ends.
// When in reaches EOF, requests already received are answered before the
// server stops (bounded by DrainTimeout).
func ServeStdio(ctx context.Context, srv *mcp.Server, in io.ReadCloser, out io.WriteCloser) error {
	d := newDrain(ctx, DrainTimeout)
	return srv.Run(ctx, &mcp.IOTransport{
		Reader: &drainReader{d: d, r: in},
		Writer: &drainWriter{d: d, w: out},
	})
}

// HTTPHandler exposes srv over Streamable HTTP. When ac is non-nil, every MCP
// request requires a valid bearer token and protected-resource metadata
// (RFC 9728) is served.
func HTTPHandler(srv *mcp.Server, ac *auth.Config) (http.Handler, error) {
	// StreamableHTTPOptions are intentionally not exposed yet; SDK defaults
	// apply (see the project backlog).
	mcpHandler := http.Handler(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil))
	if ac == nil {
		return mcpHandler, nil
	}
	if err := ac.Validate(); err != nil {
		return nil, err
	}

	metaPath := ac.ResourceMetadataPath
	if metaPath == "" {
		metaPath = defaultResourceMetadataPath
	}
	metaURL, err := resourceMetadataURL(ac.Audience, metaPath)
	if err != nil {
		return nil, err
	}
	authServer := ac.AuthServer
	if authServer == "" {
		authServer = ac.Issuer
	}

	mux := http.NewServeMux()
	mux.Handle(metaPath, sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             ac.Audience,
		AuthorizationServers: nonEmpty(authServer),
	}))
	mux.Handle("/", sdkauth.RequireBearerToken(verifier(ac.Verifier), &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: metaURL,
		// Expiry is the Argon verifier's responsibility (see auth.TokenVerifier);
		// static tokens legitimately have none.
		AllowMissingExpiration: true,
	})(mcpHandler))
	return mux, nil
}

// verifier adapts an Argon verifier to the SDK's bearer middleware. The
// Argon identity rides along in TokenInfo.Extra and is restored in the tool
// handler. Verification details are never sent to the client.
func verifier(v auth.TokenVerifier) sdkauth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
		id, err := v.Verify(ctx, token)
		if err != nil || id == nil {
			return nil, sdkauth.ErrInvalidToken
		}
		return &sdkauth.TokenInfo{
			Scopes:     id.Scopes,
			UserID:     id.Subject,
			Expiration: expiration(id.Claims),
			Extra:      map[string]any{identityKey: id},
		}, nil
	}
}

func expiration(claims map[string]any) time.Time {
	if exp, ok := claims["exp"].(float64); ok {
		return time.Unix(int64(exp), 0)
	}
	return time.Time{}
}

func resourceMetadataURL(audience, path string) (string, error) {
	u, err := url.Parse(audience)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("auth: audience must be an absolute URL")
	}
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, path), nil
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
