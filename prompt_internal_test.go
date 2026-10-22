package argon

import (
	"context"
	"testing"
)

func TestPromptRegistration(t *testing.T) {
	type args struct {
		Code     string `json:"code" jsonschema:"code to review"`
		Language string `json:"language,omitempty"`
	}
	h := func(context.Context, args) ([]PromptMessage, error) { return nil, nil }

	s := New("t")
	Prompt(s, "review", "Review code", h, PromptTitle("Review"))
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	p := s.reg.Prompts()[0]
	if p.Title != "Review" || len(p.Arguments) != 2 ||
		p.Arguments[0].Name != "code" || !p.Arguments[0].Required || p.Arguments[0].Description != "code to review" ||
		p.Arguments[1].Name != "language" || p.Arguments[1].Required {
		t.Fatalf("prompt = %+v", p)
	}
	if _, err := s.build(); err != nil {
		t.Fatalf("prompt-only server must build: %v", err)
	}

	s = New("t")
	Prompt(s, "bad", "", func(context.Context, string) ([]PromptMessage, error) { return nil, nil })
	if s.Err() == nil {
		t.Error("non-struct arguments must be rejected")
	}
	s = New("t")
	Prompt(s, "dup", "", h)
	Prompt(s, "dup", "", h)
	if s.Err() == nil {
		t.Error("duplicate prompt must be rejected")
	}
}
