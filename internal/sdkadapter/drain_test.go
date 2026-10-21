package sdkadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"go.xchunk.org/argon/internal/core"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestServeStdioAnswersBeforeEOF(t *testing.T) {
	slow := &core.Tool{
		Name:        "slow",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, _ *core.Request) (*core.Result, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return &core.Result{Content: []core.Content{{Text: "done"}}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	srv := NewServer(Config{Name: "t", Version: "0", Tools: []Tool{{Def: slow, Handler: slow.Handler}}})

	// The client writes everything and closes stdin immediately.
	in := io.NopCloser(strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"c","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"slow","arguments":{}}}`,
	}, "\n") + "\n"))
	var out bytes.Buffer

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ServeStdio(ctx, srv, in, nopWriteCloser{&out})

	if !strings.Contains(out.String(), `"id":2`) || !strings.Contains(out.String(), "done") {
		t.Fatalf("tool call was not answered before shutdown; output:\n%s", out.String())
	}
}
