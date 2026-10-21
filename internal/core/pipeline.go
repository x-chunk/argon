package core

import (
	"context"
	"log/slog"
	"slices"
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
//	meta → recover → global middleware → scopes → timeout → max output → tool middleware → handler
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

	meta := &ToolMeta{
		Name:        t.Name,
		Title:       t.Title,
		Annotations: t.Annotations,
		Scopes:      slices.Clone(t.Scopes),
		Timeout:     timeout,
		MaxOutput:   maxOut,
	}

	mws := make([]Middleware, 0, len(global)+len(t.Middleware)+5)
	mws = append(mws, withMeta(meta), Recover(log))
	mws = append(mws, global...)
	mws = append(mws, RequireScopes(t.Scopes...), Timeout(timeout), MaxOutput(maxOut))
	mws = append(mws, t.Middleware...)
	return Chain(t.Handler, mws...)
}

// withMeta attaches the tool description to every request. A shallow copy
// of the request is made so callers' Request values are never mutated.
func withMeta(meta *ToolMeta) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Result, error) {
			r := *req
			r.Meta = meta
			return next(ctx, &r)
		}
	}
}
