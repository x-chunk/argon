package argon

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.xchunk.org/argon/internal/core"
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
	if addr := httpAddr(os.Args[1:], os.Getenv); addr != "" {
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
	out, restore, err := stdioguard.Capture()
	if err != nil {
		return err
	}
	defer restore()
	return sdkadapter.ServeStdio(ctx, srv, os.Stdin, out)
}

// RunHTTP serves s over Streamable HTTP on addr until ctx is done.
func (s *Server) RunHTTP(ctx context.Context, addr string) error {
	h, err := s.Handler()
	if err != nil {
		return err
	}
	hs := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- hs.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hs.Shutdown(shutdownCtx)
	}
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
	return sdkadapter.HTTPHandler(srv, s.auth)
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
		built = append(built, sdkadapter.Tool{Def: t, Handler: core.Build(t, s.defaults, s.global)})
	}
	builtRes := make([]sdkadapter.Resource, 0, len(resources))
	for _, r := range resources {
		builtRes = append(builtRes, sdkadapter.Resource{Def: r, Handler: core.BuildResource(r, s.defaults)})
	}
	builtPrompts := make([]sdkadapter.Prompt, 0, len(prompts))
	for _, p := range prompts {
		builtPrompts = append(builtPrompts, sdkadapter.Prompt{Def: p, Handler: core.BuildPrompt(p, s.defaults)})
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

// httpAddr extracts the HTTP listen address from args or the environment.
func httpAddr(args []string, getenv func(string) string) string {
	for i, a := range args {
		switch {
		case a == "--http" || a == "-http":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--http="), strings.HasPrefix(a, "-http="):
			return a[strings.Index(a, "=")+1:]
		}
	}
	return getenv(EnvHTTP)
}
