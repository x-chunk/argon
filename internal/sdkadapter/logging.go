package sdkadapter

import (
	"context"
	"log/slog"
)

// sdkLogLevel is the minimum level of SDK-internal log records that are
// passed through. The SDK is chatty at Info (connects, sessions); those
// records are noise for Argon users, while warnings and errors still matter.
const sdkLogLevel = slog.LevelWarn

// minLevel wraps a handler and drops records below min.
type minLevel struct {
	slog.Handler
	min slog.Level
}

func (h minLevel) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.min && h.Handler.Enabled(ctx, l)
}

func (h minLevel) WithAttrs(as []slog.Attr) slog.Handler {
	return minLevel{h.Handler.WithAttrs(as), h.min}
}

func (h minLevel) WithGroup(name string) slog.Handler {
	return minLevel{h.Handler.WithGroup(name), h.min}
}

func sdkLogger(l *slog.Logger) *slog.Logger {
	if l == nil {
		return nil
	}
	return slog.New(minLevel{l.Handler(), sdkLogLevel})
}
