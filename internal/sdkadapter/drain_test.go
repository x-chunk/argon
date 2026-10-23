package sdkadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func TestServeStdioAnswersBeforeEOF(t *testing.T) {
	slow := Tool{
		Def: ToolDef{Name: "slow", InputSchema: json.RawMessage(`{"type":"object"}`)},
		Call: func(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return &ToolResult{Content: []Block{{Kind: BlockText, Text: "done"}}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	srv := NewServer(Config{Name: "t", Version: "0", Tools: []Tool{slow}})

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

func TestCanonicalID(t *testing.T) {
	same := [][]string{
		{`2`, `2.0`, `2e0`, `20e-1`},
		{`"a"`, `"a"`},
	}
	for _, group := range same {
		for _, id := range group[1:] {
			if got, want := canonicalID(json.RawMessage(id)), canonicalID(json.RawMessage(group[0])); got != want {
				t.Errorf("canonicalID(%s) = %q, want %q like %s", id, got, want, group[0])
			}
		}
	}
	if canonicalID(json.RawMessage(`1`)) == canonicalID(json.RawMessage(`"1"`)) {
		t.Error("number and string ids must differ")
	}
}
