package core

import (
	"log/slog"
	"time"
)

// Defaults are server-wide policies applied when a tool does not override them.
type Defaults struct {
	Timeout   time.Duration
	MaxOutput int
	Logger    *slog.Logger
}

// Build assembles the effective handler for t. Order, outermost first:
//
//	recover → global middleware → scopes → timeout → max output → tool middleware → handler
//
// Recover is outermost so that panics anywhere in the chain are contained.
func Build(t *Tool, d Defaults, global []Middleware) Handler {
	timeout := d.Timeout
	if t.Timeout != 0 {
		timeout = t.Timeout
	}
	maxOut := d.MaxOutput
	if t.MaxOutput != 0 {
		maxOut = t.MaxOutput
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}

	mws := make([]Middleware, 0, len(global)+len(t.Middleware)+4)
	mws = append(mws, Recover(log))
	mws = append(mws, global...)
	mws = append(mws, RequireScopes(t.Scopes...), Timeout(timeout), MaxOutput(maxOut))
	mws = append(mws, t.Middleware...)
	return Chain(t.Handler, mws...)
}
