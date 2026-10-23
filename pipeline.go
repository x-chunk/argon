package argon

import (
	"context"
	"log/slog"
	"slices"
	"time"
)

// policy are server-wide policies applied when a tool does not override them.
type policy struct {
	Timeout   time.Duration
	MaxOutput int
	Logger    *slog.Logger
}

// buildTool assembles the effective handler for t. Order, outermost first:
//
//	meta → recover → check result → global middleware → scopes → timeout → max output → tool middleware → handler
//
// Recover is outermost so that panics anywhere in the chain are contained.
func buildTool(t *toolDef, d policy, global []Middleware) Handler {
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

	mws := make([]Middleware, 0, len(global)+len(t.Middleware)+6)
	mws = append(mws, withMeta(meta), recoverMiddleware(log), checkResultMiddleware())
	mws = append(mws, global...)
	mws = append(mws, scopesMiddleware(t.Scopes...), timeoutMiddleware(timeout), maxOutputMiddleware(maxOut))
	mws = append(mws, t.Middleware...)
	return chain(t.Handler, mws...)
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
