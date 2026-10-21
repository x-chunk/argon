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
