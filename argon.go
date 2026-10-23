// Package argon is a small framework for building MCP (Model Context
// Protocol) servers in Go.
//
//	s := argon.New("notes")
//	argon.Tool(s, "add_note", "Save a note", func(ctx context.Context, in struct {
//		Text string `json:"text" jsonschema:"Note text"`
//	}) (string, error) {
//		return save(in.Text)
//	})
//	s.Run()
//
// Argon is built on the official Go SDK but never exposes its types.
package argon

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"go.xchunk.org/argon/auth"
)

// Safe defaults applied when not overridden.
const (
	DefaultTimeout   = 60 * time.Second
	DefaultMaxOutput = 1 << 20 // 1 MiB
)

// Server is an MCP server definition: a set of tools plus policies.
// It is transport-independent; see Run, RunStdio and RunHTTP.
//
// Register tools, resources and prompts before serving, from a single
// goroutine. The set is fixed when Run*, Handler or argontest builds the
// server: later registrations are not served by servers already built.
type Server struct {
	name         string
	version      string
	instructions string
	logger       *slog.Logger
	logCalls     bool

	reg      *registry
	defaults policy
	global   []Middleware
	auth     *auth.Config

	// errs collects registration errors; they are reported by Run*.
	errs []error
}

// Option configures a Server.
type Option func(*Server)

// New creates a server named name.
func New(name string, opts ...Option) *Server {
	s := &Server{
		name:     name,
		version:  "dev",
		reg:      newRegistry(),
		defaults: policy{Timeout: DefaultTimeout, MaxOutput: DefaultMaxOutput},
	}
	for _, o := range opts {
		o(s)
	}
	if s.logger == nil {
		s.logger = defaultLogger(s.logCalls)
	}
	if s.logCalls {
		// Outermost global middleware, so it measures the whole call.
		s.global = append([]Middleware{logCallsMiddleware(s.logger)}, s.global...)
	}
	s.defaults.Logger = s.logger
	return s
}

// Err reports registration errors accumulated so far.
func (s *Server) Err() error { return errors.Join(s.errs...) }

// Version sets the server version reported to clients.
func Version(v string) Option { return func(s *Server) { s.version = v } }

// Instructions sets instructions sent to clients on initialization.
func Instructions(text string) Option { return func(s *Server) { s.instructions = text } }

// Logger sets the server logger. On stdio it must not write to stdout.
//
// By default Argon logs to stderr at Warn level (Info when LogCalls is set).
// Records from the underlying SDK are always limited to Warn and above.
func Logger(l *slog.Logger) Option { return func(s *Server) { s.logger = l } }

// LogCalls logs every tool call (name, duration, error) at Info level.
// Arguments and results are never logged; invalid input is logged without
// the rejected values. Errors returned by handlers are logged as they are.
func LogCalls() Option { return func(s *Server) { s.logCalls = true } }

func defaultLogger(verbose bool) *slog.Logger {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// Timeout sets the default per-call timeout. Zero disables it.
//
// The timeout cancels the handler's context; a handler that ignores its
// context keeps running, and the call waits for it.
func Timeout(d time.Duration) Option { return func(s *Server) { s.defaults.Timeout = d } }

// MaxOutput sets the default result size limit in bytes. Zero disables it.
func MaxOutput(n int) Option { return func(s *Server) { s.defaults.MaxOutput = n } }

// Use adds global middleware, applied to every tool.
func Use(mws ...Middleware) Option { return func(s *Server) { s.global = append(s.global, mws...) } }

// Auth enables bearer-token authentication on the HTTP transport.
// It has no effect on stdio.
func Auth(cfg auth.Config) Option { return func(s *Server) { s.auth = &cfg } }
