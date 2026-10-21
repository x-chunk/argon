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
	"time"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// Safe defaults applied when not overridden.
const (
	DefaultTimeout   = 60 * time.Second
	DefaultMaxOutput = 1 << 20 // 1 MiB
)

// Server is an MCP server definition: a set of tools plus policies.
// It is transport-independent; see Run, RunStdio and RunHTTP.
type Server struct {
	name         string
	version      string
	instructions string
	logger       *slog.Logger

	reg      *core.Registry
	defaults core.Defaults
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
		logger:   slog.Default(),
		reg:      core.NewRegistry(),
		defaults: core.Defaults{Timeout: DefaultTimeout, MaxOutput: DefaultMaxOutput},
	}
	for _, o := range opts {
		o(s)
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
func Logger(l *slog.Logger) Option { return func(s *Server) { s.logger = l } }

// Timeout sets the default per-call timeout. Zero disables it.
func Timeout(d time.Duration) Option { return func(s *Server) { s.defaults.Timeout = d } }

// MaxOutput sets the default result size limit in bytes. Zero disables it.
func MaxOutput(n int) Option { return func(s *Server) { s.defaults.MaxOutput = n } }

// Use adds global middleware, applied to every tool.
func Use(mws ...Middleware) Option { return func(s *Server) { s.global = append(s.global, mws...) } }

// Auth enables bearer-token authentication on the HTTP transport.
// It has no effect on stdio.
func Auth(cfg auth.Config) Option { return func(s *Server) { s.auth = &cfg } }
