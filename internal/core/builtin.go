package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"go.xchunk.org/argon/auth"
)

// Recover turns a panic in the handler into an error result instead of a
// dead server. The stack is logged; it is never sent to the client.
func Recover(log *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (res *Result, err error) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("tool panicked", "tool", req.Tool, "panic", p, "stack", string(debug.Stack()))
					res, err = nil, fmt.Errorf("internal error in tool %q", req.Tool)
				}
			}()
			return next(ctx, req)
		}
	}
}

// Timeout bounds a call's duration via context cancellation.
// A non-positive d disables the limit.
func Timeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		if d <= 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return next(ctx, req)
		}
	}
}

// RequireScopes rejects calls whose identity lacks any of scopes.
// With no scopes it is a no-op.
func RequireScopes(scopes ...string) Middleware {
	return func(next Handler) Handler {
		if len(scopes) == 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			id := auth.FromContext(ctx)
			for _, s := range scopes {
				if !id.HasScope(s) {
					return nil, fmt.Errorf("insufficient scope for tool %q", req.Tool)
				}
			}
			return next(ctx, req)
		}
	}
}

// MaxOutput limits the total size of a result's content, in bytes: the sum
// of all blocks (text, raw image bytes, link fields) plus the JSON encoding
// of structured output. An oversized
// result is replaced by an error, rather than truncated, because truncation
// would silently break structured output. A non-positive n disables the limit.
func MaxOutput(n int) Middleware {
	return func(next Handler) Handler {
		if n <= 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			res, err := next(ctx, req)
			if err != nil || res == nil {
				return res, err
			}
			if size, err := resultSize(res); err != nil {
				return nil, err
			} else if size > n {
				return nil, fmt.Errorf("tool %q output too large: %d bytes, limit %d", req.Tool, size, n)
			}
			return res, nil
		}
	}
}

func resultSize(r *Result) (int, error) {
	size := 0
	for _, c := range r.Content {
		size += c.size()
	}
	if r.Structured != nil {
		b, err := json.Marshal(r.Structured)
		if err != nil {
			return 0, fmt.Errorf("encode structured output: %w", err)
		}
		size += len(b)
	}
	return size, nil
}
