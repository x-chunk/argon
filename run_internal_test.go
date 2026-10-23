package argon

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestHTTPAddr(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{"none", nil, "", ""},
		{"env", nil, ":9000", ":9000"},
		{"flag space", []string{"--http", ":8080"}, "", ":8080"},
		{"flag eq", []string{"--http=:8080"}, "", ":8080"},
		{"single dash", []string{"-http", ":8080"}, "", ":8080"},
		{"flag beats env", []string{"--http=:1"}, ":2", ":1"},
		{"dangling flag", []string{"--http"}, "", ""},
		{"other args", []string{"-v", "x"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := httpAddr(tt.args, env(tt.env)); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWarnScopesWithoutAuth(t *testing.T) {
	var buf bytes.Buffer
	s := New("test", Logger(slog.New(slog.NewTextHandler(&buf, nil))))
	Tool(s, "open", "", func(context.Context, struct{}) (string, error) { return "", nil })
	Tool(s, "guarded", "", func(context.Context, struct{}) (string, error) { return "", nil }, Scopes("x"))
	if _, err := s.Handler(); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "tool guarded") || strings.Contains(out, "tool open") {
		t.Fatalf("log = %q, want a warning naming only the scoped tool", out)
	}
}
