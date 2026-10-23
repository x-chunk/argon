package argon

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// logCallsMiddleware logs every tool call at Info level: tool name, duration and, on
// failure, the error. Arguments and results are never logged, since they may
// carry user data or secrets. That includes validation errors, which quote
// the offending values: they are logged as ErrInvalidArguments only.
func logCallsMiddleware(log *slog.Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Result, error) {
			start := time.Now()
			res, err := next(ctx, req)
			attrs := []any{"tool", req.Tool, "duration", time.Since(start)}
			switch {
			case errors.Is(err, ErrInvalidArguments):
				log.InfoContext(ctx, "tool call failed", append(attrs, "error", ErrInvalidArguments)...)
			case err != nil:
				log.InfoContext(ctx, "tool call failed", append(attrs, "error", err)...)
			case res != nil && res.IsError:
				log.InfoContext(ctx, "tool call returned error result", attrs...)
			default:
				log.InfoContext(ctx, "tool call", attrs...)
			}
			return res, err
		}
	}
}
