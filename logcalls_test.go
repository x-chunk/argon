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
