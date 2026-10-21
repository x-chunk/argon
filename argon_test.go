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
