package sdkadapter

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestPanicOutsidePipelineIsRecovered(t *testing.T) {
	srv := NewServer(Config{
		Name:   "test",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tools: []Tool{{
			Def: ToolDef{Name: "boom", InputSchema: json.RawMessage(`{"type":"object"}`)},
			Call: func(context.Context, json.RawMessage) (*ToolResult, error) {
				panic("conversion bug")
			},
		}},
	})
	// Twice: the first panic must neither crash the process nor the server.
	for range 2 {
		if _, err := CallInMemory(t.Context(), srv, "boom", nil, nil); err == nil {
			t.Fatal("want an internal error from the recovered panic")
		}
	}
}
