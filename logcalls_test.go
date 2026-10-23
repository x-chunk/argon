package argon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestLogCalls(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	ok := logCallsMiddleware(log)(textHandler("ok"))
	fail := logCallsMiddleware(log)(func(context.Context, *Request) (*Result, error) { return nil, errors.New("boom") })

	ok(context.Background(), &Request{Tool: "good", Arguments: []byte(`{"secret":"hunter2"}`)})
	fail(context.Background(), &Request{Tool: "bad"})

	out := buf.String()
	for _, want := range []string{"tool=good", "tool=bad", "error=boom", "duration="} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "hunter2") {
		t.Fatal("arguments must never be logged")
	}
}

// Validation errors quote the rejected value; the call log must not.
func TestLogCallsHidesRejectedValues(t *testing.T) {
	var buf bytes.Buffer
	s := New("test", LogCalls(), Logger(slog.New(slog.NewTextHandler(&buf, nil))))
	Tool(s, "login", "", func(context.Context, struct {
		Pin int `json:"pin"`
	}) (string, error) {
		return "", nil
	})
	h := buildTool(s.reg.All()[0], s.defaults, s.global)
	h(context.Background(), &Request{Tool: "login", Arguments: []byte(`{"pin":"hunter2"}`)})
	out := buf.String()
	if !strings.Contains(out, "invalid arguments") || strings.Contains(out, "hunter2") {
		t.Fatalf("log = %q, want the failure without the rejected value", out)
	}
}
