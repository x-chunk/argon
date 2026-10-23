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
		err  bool
	}{
		{"none", nil, "", "", false},
		{"env", nil, ":9000", ":9000", false},
		{"flag space", []string{"--http", ":8080"}, "", ":8080", false},
		{"flag eq", []string{"--http=:8080"}, "", ":8080", false},
		{"single dash", []string{"-http", ":8080"}, "", ":8080", false},
		{"flag beats env", []string{"--http=:1"}, ":2", ":1", false},
		{"dangling flag", []string{"--http"}, "", "", true},
		{"empty value", []string{"--http="}, ":1", "", true},
		{"after double dash", []string{"--", "--http", ":8080"}, "", ":8080", false},
		{"other args", []string{"-v", "x"}, "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := httpAddr(tt.args, env(tt.env))
			if got != tt.want || (err != nil) != tt.err {
				t.Fatalf("got %q, %v; want %q, error %v", got, err, tt.want, tt.err)
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

func TestIsLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:8080": true,
		"[::1]:8080":     true,
		"localhost:8080": true,
		":8080":          false,
		"0.0.0.0:8080":   false,
		"10.0.0.5:8080":  false,
		"example.com:80": false,
	} {
		if got := isLoopback(addr); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", addr, got, want)
		}
	}
}
