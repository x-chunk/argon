package argon_test

import (
	"context"
	"errors"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
)

type echoIn struct {
	Text string `json:"text"`
}

type lenOut struct {
	N int `json:"n"`
}

func newServer() *argon.Server {
	s := argon.New("test")
	argon.Tool(s, "echo", "Echo text", func(_ context.Context, in echoIn) (string, error) {
		return in.Text, nil
	})
	argon.Tool(s, "len", "Text length", func(_ context.Context, in echoIn) (lenOut, error) {
		return lenOut{N: len(in.Text)}, nil
	})
	argon.Tool(s, "fail", "Always fails", func(context.Context, echoIn) (string, error) {
		return "", errors.New("boom")
	})
	argon.Tool(s, "panic", "Always panics", func(context.Context, echoIn) (string, error) {
		panic("oops")
	})
	return s
}

func TestCallString(t *testing.T) {
	got, err := argontest.Call[string](t, newServer(), "echo", echoIn{Text: "hi"})
	if err != nil || got != "hi" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCallStructured(t *testing.T) {
	got, err := argontest.Call[lenOut](t, newServer(), "len", echoIn{Text: "abc"})
	if err != nil || got.N != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestToolErrorIsResult(t *testing.T) {
	if _, err := argontest.Call[string](t, newServer(), "fail", echoIn{}); err == nil {
		t.Fatal("want tool error")
	}
}

func TestPanicRecovered(t *testing.T) {
	if _, err := argontest.Call[string](t, newServer(), "panic", echoIn{}); err == nil {
		t.Fatal("want error result from panic")
	}
}

func TestDuplicateToolReported(t *testing.T) {
	s := newServer()
	argon.Tool(s, "echo", "dup", func(_ context.Context, in echoIn) (string, error) { return "", nil })
	if s.Err() == nil {
		t.Fatal("want duplicate registration error")
	}
}

type strictIn struct {
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

func TestInputValidated(t *testing.T) {
	s := argon.New("test")
	called := false
	argon.Tool(s, "strict", "Strict input", func(_ context.Context, in strictIn) (string, error) {
		called = true
		return in.Name, nil
	})
	for _, in := range []any{
		map[string]any{},                           // missing required "name"
		map[string]any{"name": 42},                 // wrong type
		map[string]any{"name": "x", "extra": true}, // unknown property
	} {
		if _, err := argontest.Call[string](t, s, "strict", in); err == nil {
			t.Errorf("input %v: want validation error", in)
		}
	}
	if called {
		t.Fatal("handler must not run on invalid input")
	}
	if got, err := argontest.Call[string](t, s, "strict", map[string]any{"name": "ok"}); err != nil || got != "ok" {
		t.Fatalf("valid input: got %q, %v", got, err)
	}
}

func TestWithIdentityAndScopes(t *testing.T) {
	s := argon.New("test")
	argon.Tool(s, "whoami", "Caller subject", func(ctx context.Context, _ struct{}) (string, error) {
		if id := argon.IdentityFrom(ctx); id != nil {
			return id.Subject, nil
		}
		return "anonymous", nil
	})
	argon.Tool(s, "admin", "Needs admin", func(context.Context, struct{}) (string, error) {
		return "ok", nil
	}, argon.Scopes("admin"))

	if got, _ := argontest.Call[string](t, s, "whoami", struct{}{}); got != "anonymous" {
		t.Fatalf("no identity: got %q", got)
	}
	alice := argon.Identity{Subject: "alice"}
	if got, _ := argontest.Call[string](t, s, "whoami", struct{}{}, argontest.WithIdentity(alice)); got != "alice" {
		t.Fatalf("with identity: got %q", got)
	}
	if _, err := argontest.Call[string](t, s, "admin", struct{}{}, argontest.WithIdentity(alice)); err == nil {
		t.Fatal("missing scope must fail")
	}
	root := argon.Identity{Subject: "root", Scopes: []string{"admin"}}
	if _, err := argontest.Call[string](t, s, "admin", struct{}{}, argontest.WithIdentity(root)); err != nil {
		t.Fatalf("admin scope: %v", err)
	}
}
