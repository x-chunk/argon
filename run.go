package argon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.xchunk.org/argon/internal/sdkadapter"
	"go.xchunk.org/argon/internal/stdioguard"
)

// EnvHTTP selects the HTTP transport when set to a listen address.
const EnvHTTP = "ARGON_HTTP"

// Run serves s until SIGINT/SIGTERM. It uses stdio unless an HTTP address is
// given via the --http flag (--http :8080 or --http=:8080) or ARGON_HTTP.
func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr, err := httpAddr(os.Args[1:], os.Getenv)
	if err != nil {
		return err
	}
	if addr != "" {
		return s.RunHTTP(ctx, addr)
	}
	return s.RunStdio(ctx)
}

// RunStdio serves s over stdin/stdout. While it runs, stdout is redirected to
// stderr (at the file-descriptor level on Unix) so stray writes cannot
// corrupt the protocol stream.
func (s *Server) RunStdio(ctx context.Context) error {
	srv, err := s.build()
	if err != nil {
		return err
	}
	s.warnScopesWithoutAuth("stdio")
	out, restore, err := stdioguard.Capture()
	if err != nil {
		return err
	}
	defer restore()
	return sdkadapter.ServeStdio(ctx, srv, os.Stdin, out)
}

// HTTP server limits used by RunHTTP.
const (
	httpReadHeaderTimeout = 10 * time.Second
	httpIdleTimeout       = 2 * time.Minute
	httpShutdownTimeout   = 5 * time.Second
)

// RunHTTP serves s over Streamable HTTP on addr until ctx is done.
//
// On shutdown it stops accepting connections, ends open server-to-client
// streams (GET requests) and waits up to 5s for in-flight requests before
// closing the remaining connections.
func (s *Server) RunHTTP(ctx context.Context, addr string) error {
	h, err := s.Handler()
	if err != nil {
		return err
	}
	if s.auth == nil && !isLoopback(addr) {
		s.logger.Warn("argon: serving HTTP without Auth on a non-loopback address; anyone who can reach it can call every tool", "addr", addr)
	}
	streams, endStreams := context.WithCancel(context.Background())
	defer endStreams()
	hs := &http.Server{
		Addr:              addr,
		Handler:           endOnShutdown(h, streams),
		ReadHeaderTimeout: httpReadHeaderTimeout,
		IdleTimeout:       httpIdleTimeout,
	}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		endStreams()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
		defer cancel()
		if err := hs.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("argon: HTTP shutdown timed out; closing remaining connections", "error", err)
			return hs.Close()
		}
		return nil
	}
}

// isLoopback reports whether the listen address addr only accepts local
// connections. An empty host listens on all interfaces.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// endOnShutdown cancels GET requests (long-lived server-to-client streams)
// when streams is done. They never finish on their own, so without this a
// graceful shutdown would always wait out its timeout. Other requests keep
// their context so in-flight calls can still complete.
func endOnShutdown(h http.Handler, streams context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			h.ServeHTTP(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		stop := context.AfterFunc(streams, cancel)
		defer stop()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Handler returns s as an http.Handler for embedding into an existing HTTP
// server. Authentication configured via Auth is applied.
//
// The handler serves MCP on any path it receives, so it can be mounted
// under a prefix (mux.Handle("/mcp", h)) without http.StripPrefix. With
// Auth, set the Audience to the full endpoint URL (e.g.
// "https://example.com/mcp") and also mount ResourceMetadata at the root
// mux: the RFC 9728 metadata path lives outside the prefix.
func (s *Server) Handler() (http.Handler, error) {
	srv, err := s.build()
	if err != nil {
		return nil, err
	}
	if s.auth == nil {
		s.warnScopesWithoutAuth("HTTP without Auth")
	}
	return sdkadapter.HTTPHandler(srv, s.auth, s.logger)
}

// ResourceMetadata returns the OAuth protected-resource metadata endpoint
// (RFC 9728) and the absolute path to serve it at. Only needed when Handler
// is mounted under a prefix; RunHTTP and root mounts serve it already.
// It fails when Auth is not configured.
//
//	h, _ := s.Handler()
//	path, meta, _ := s.ResourceMetadata()
//	mux.Handle("/mcp", h)
//	mux.Handle(path, meta)
func (s *Server) ResourceMetadata() (path string, h http.Handler, err error) {
	if s.auth == nil {
		return "", nil, errors.New("argon: ResourceMetadata requires Auth")
	}
	return sdkadapter.ResourceMetadataHandler(s.auth)
}

func (s *Server) build() (*sdkadapter.Server, error) {
	if err := s.Err(); err != nil {
		return nil, err
	}
	tools, resources, prompts := s.reg.All(), s.reg.Resources(), s.reg.Prompts()
	if len(tools) == 0 && len(resources) == 0 && len(prompts) == 0 {
		return nil, errors.New("argon: no tools, resources or prompts registered")
	}
	built := make([]sdkadapter.Tool, 0, len(tools))
	for _, t := range tools {
		built = append(built, adaptTool(t, buildTool(t, s.defaults, s.global)))
	}
	builtRes := make([]sdkadapter.Resource, 0, len(resources))
	for _, r := range resources {
		builtRes = append(builtRes, adaptResource(r, buildResource(r, s.defaults)))
	}
	builtPrompts := make([]sdkadapter.Prompt, 0, len(prompts))
	for _, p := range prompts {
		builtPrompts = append(builtPrompts, adaptPrompt(p, buildPrompt(p, s.defaults)))
	}
	return sdkadapter.NewServer(sdkadapter.Config{
		Name:         s.name,
		Version:      s.version,
		Instructions: s.instructions,
		Logger:       s.logger,
		Tools:        built,
		Resources:    builtRes,
		Prompts:      builtPrompts,
	}), nil
}

// warnScopesWithoutAuth logs the items that require scopes when the
// transport cannot authenticate callers: every call to them is rejected.
func (s *Server) warnScopesWithoutAuth(transport string) {
	var scoped []string
	for _, t := range s.reg.All() {
		if len(t.Scopes) > 0 {
			scoped = append(scoped, "tool "+t.Name)
		}
	}
	for _, r := range s.reg.Resources() {
		if len(r.Scopes) > 0 {
			scoped = append(scoped, "resource "+r.Key())
		}
	}
	for _, p := range s.reg.Prompts() {
		if len(p.Scopes) > 0 {
			scoped = append(scoped, "prompt "+p.Name)
		}
	}
	if len(scoped) > 0 {
		s.logger.Warn("argon: items requiring scopes always reject unauthenticated callers", "transport", transport, "items", scoped)
	}
}

// httpAddr extracts the HTTP listen address from args or the environment.
// A --http flag without a value is an error rather than a silent fallback
// to stdio.
func httpAddr(args []string, getenv func(string) string) (string, error) {
	for i, a := range args {
		switch {
		case a == "--http" || a == "-http":
			if i+1 >= len(args) || args[i+1] == "" {
				return "", fmt.Errorf("argon: %s requires a listen address, e.g. %s :8080", a, a)
			}
			return args[i+1], nil
		case strings.HasPrefix(a, "--http="), strings.HasPrefix(a, "-http="):
			addr := a[strings.Index(a, "=")+1:]
			if addr == "" {
				return "", fmt.Errorf("argon: %s requires a listen address", a)
			}
			return addr, nil
		}
	}
	return getenv(EnvHTTP), nil
}
