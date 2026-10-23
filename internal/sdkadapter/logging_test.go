package sdkadapter

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSDKLoggerFiltersInfo(t *testing.T) {
	var buf bytes.Buffer
	l := sdkLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	l.Info("noise")
	l.With("k", "v").Warn("problem")
	out := buf.String()
	if strings.Contains(out, "noise") || !strings.Contains(out, "problem") || !strings.Contains(out, "k=v") {
		t.Fatalf("output = %q", out)
	}
}

func TestStreamableOptionsLogger(t *testing.T) {
	if got := streamableOptions(nil).Logger; got != nil {
		t.Fatalf("nil logger must stay nil (SDK discards), got %v", got)
	}
	var buf bytes.Buffer
	opts := streamableOptions(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	opts.Logger.Info("noise")
	opts.Logger.Error("failed to connect")
	out := buf.String()
	if strings.Contains(out, "noise") || !strings.Contains(out, "failed to connect") {
		t.Fatalf("HTTP-layer logs must reach the server logger, filtered to Warn+: %q", out)
	}
}
