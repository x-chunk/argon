package sdkadapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
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
//
// log receives the HTTP layer's warnings and errors (filtered like the
// server's SDK logs); nil discards them.
func HTTPHandler(srv *mcp.Server, ac *auth.Config, log *slog.Logger) (http.Handler, error) {
	mcpHandler := http.Handler(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, streamableOptions(log)))
	if ac == nil {
		return mcpHandler, nil
	}
	if err := ac.Validate(); err != nil {
		return nil, err
	}

	meta, err := newResourceMetadata(ac)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(meta.path, meta.handler)
	mux.Handle("/", sdkauth.RequireBearerToken(verifier(ac.Verifier), &sdkauth.RequireBearerTokenOptions{
		ResourceMetadataURL: meta.url,
		// Expiry is the Argon verifier's responsibility (see auth.TokenVerifier);
		// static tokens legitimately have none.
		AllowMissingExpiration: true,
	})(mcpHandler))
	return mux, nil
}

// ResourceMetadataHandler returns the protected-resource metadata endpoint
// and the absolute path it must be served at. HTTPHandler already serves it
// when mounted at the root; this is for mounting under a prefix.
func ResourceMetadataHandler(ac *auth.Config) (path string, h http.Handler, err error) {
	if err := ac.Validate(); err != nil {
		return "", nil, err
	}
	meta, err := newResourceMetadata(ac)
	if err != nil {
		return "", nil, err
	}
	return meta.path, meta.handler, nil
}

// streamableOptions configures the SDK's Streamable HTTP handler. Other
// options are intentionally not exposed yet; SDK defaults apply (see the
// roadmap). Without a logger the SDK discards its HTTP-layer errors.
func streamableOptions(log *slog.Logger) *mcp.StreamableHTTPOptions {
	return &mcp.StreamableHTTPOptions{Logger: sdkLogger(log)}
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

// resourceMetadata is the protected-resource metadata endpoint (RFC 9728).
type resourceMetadata struct {
	path    string // absolute path on the resource's origin
	url     string // advertised in WWW-Authenticate
	handler http.Handler
}

func newResourceMetadata(ac *auth.Config) (*resourceMetadata, error) {
	u, err := url.Parse(ac.Audience)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("auth: audience must be an absolute URL")
	}
	path := ac.ResourceMetadataPath
	if path == "" {
		path = metadataPath(u.Path)
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("auth: ResourceMetadataPath %q must be an absolute path", path)
	}
	authServer := ac.AuthServer
	if authServer == "" {
		authServer = ac.Issuer
	}
	return &resourceMetadata{
		path: path,
		url:  fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, path),
		handler: sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
			Resource:             ac.Audience,
			AuthorizationServers: nonEmpty(authServer),
		}),
	}, nil
}

// metadataPath derives the well-known path for a resource path per RFC 9728
// §3.1: the well-known segment is inserted between the host and the path, so
// resource https://host/mcp has metadata at
// https://host/.well-known/oauth-protected-resource/mcp.
func metadataPath(resourcePath string) string {
	return defaultResourceMetadataPath + strings.TrimSuffix(resourcePath, "/")
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
