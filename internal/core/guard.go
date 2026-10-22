package core

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"go.xchunk.org/argon/auth"
)

// The helpers below implement the safety policies shared by every kind of
// handler (tools, resources). Each takes the item kind ("tool", "resource")
// and name only for messages.

// guardPanic runs f and turns a panic into an error. The stack is logged; it
// is never returned, so it cannot reach the client.
func guardPanic[T any](log *slog.Logger, kind, name string, f func() (T, error)) (res T, err error) {
	defer func() {
		if p := recover(); p != nil {
			log.Error(kind+" panicked", kind, name, "panic", p, "stack", string(debug.Stack()))
			var zero T
			res, err = zero, fmt.Errorf("internal error in %s %q", kind, name)
		}
	}()
	return f()
}

// withTimeout derives a context bounded by d. A non-positive d disables the
// limit; the returned cancel func must always be called.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// checkScopes reports an error unless the caller identity in ctx holds every
// scope in scopes.
func checkScopes(ctx context.Context, kind, name string, scopes []string) error {
	id := auth.FromContext(ctx)
	for _, s := range scopes {
		if !id.HasScope(s) {
			return fmt.Errorf("insufficient scope for %s %q", kind, name)
		}
	}
	return nil
}

// checkSize reports an error when size exceeds a positive limit.
func checkSize(kind, name string, size, limit int) error {
	if limit > 0 && size > limit {
		return fmt.Errorf("%s %q output too large: %d bytes, limit %d", kind, name, size, limit)
	}
	return nil
}
