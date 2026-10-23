package argon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// recoverMiddleware turns a panic in the handler into an error result instead of a
// dead server. The stack is logged; it is never sent to the client.
func recoverMiddleware(log *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Result, error) {
			return guardPanic(log, "tool", req.Tool, func() (*Result, error) { return next(ctx, req) })
		}
	}
}

// timeoutMiddleware bounds a call's duration via context cancellation.
// A non-positive d disables the limit.
func timeoutMiddleware(d time.Duration) Middleware {
	return func(next Handler) Handler {
		if d <= 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			ctx, cancel := withTimeout(ctx, d)
			defer cancel()
			return next(ctx, req)
		}
	}
}

// scopesMiddleware rejects calls whose identity lacks any of scopes.
// With no scopes it is a no-op.
func scopesMiddleware(scopes ...string) Middleware {
	return func(next Handler) Handler {
		if len(scopes) == 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			if err := checkScopes(ctx, "tool", req.Tool, scopes); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

// maxOutputMiddleware limits the total size of a result's content, in bytes: the sum
// of all blocks (text, raw image bytes, link fields) plus the JSON encoding
// of structured output. An oversized
// result is replaced by an error, rather than truncated, because truncation
// would silently break structured output. A non-positive n disables the limit.
func maxOutputMiddleware(n int) Middleware {
	return func(next Handler) Handler {
		if n <= 0 {
			return next
		}
		return func(ctx context.Context, req *Request) (*Result, error) {
			res, err := next(ctx, req)
			if err != nil || res == nil {
				return res, err
			}
			size, err := resultSize(res)
			if err != nil {
				return nil, err
			}
			if err := checkSize("tool", req.Tool, size, n); err != nil {
				return nil, err
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
