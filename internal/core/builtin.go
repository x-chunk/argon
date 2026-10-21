package core

import (
	"context"
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

// MaxOutput limits the total size of a result's content, in bytes.
// A non-positive n disables the limit.
func MaxOutput(n int) Middleware {
	return func(next Handler) Handler {
		// TODO(v0.1): measure text + structured output and truncate or
		// replace with an error result when it exceeds n.
		return next
	}
}
