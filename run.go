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

// RunStdio serves s over stdin/stdout. While it runs, os.Stdout is redirected
// to stderr so stray prints cannot corrupt the protocol stream.
func (s *Server) RunStdio(ctx context.Context) error {
	srv, err := s.build()
	if err != nil {
		return err
	}
	out, restore := stdioguard.Capture()
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
func (s *Server) Handler() (http.Handler, error) {
	srv, err := s.build()
	if err != nil {
		return nil, err
	}
	return sdkadapter.HTTPHandler(srv, s.auth)
}

func (s *Server) build() (*sdkadapter.Server, error) {
	if err := s.Err(); err != nil {
		return nil, err
	}
	tools := s.reg.All()
	if len(tools) == 0 {
		return nil, errors.New("argon: no tools registered")
	}
	built := make([]sdkadapter.Tool, 0, len(tools))
	for _, t := range tools {
		built = append(built, sdkadapter.Tool{Def: t, Handler: core.Build(t, s.defaults, s.global)})
	}
	return sdkadapter.NewServer(sdkadapter.Config{
		Name:         s.name,
		Version:      s.version,
		Instructions: s.instructions,
		Logger:       s.logger,
		Tools:        built,
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
