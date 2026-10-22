package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.xchunk.org/argon/auth"
)

func TestBuildPrompt(t *testing.T) {
	quiet := Defaults{Logger: slog.New(slog.DiscardHandler)}
	ctx := context.Background()
	req := &PromptRequest{Name: "p"}

	msgs, err := BuildPrompt(&Prompt{Name: "p", Handler: say("hi")}, quiet)(ctx, req)
	if err != nil || len(msgs) != 1 || msgs[0].Content.(TextContent).Text != "hi" {
		t.Fatalf("ok: %v, %v", msgs, err)
	}

	bad := map[string]PromptHandler{
		"panic": func(context.Context, *PromptRequest) ([]PromptMessage, error) { panic("secret") },
		"bad role": func(context.Context, *PromptRequest) ([]PromptMessage, error) {
			return []PromptMessage{{Role: "system", Content: TextContent{Text: "x"}}}, nil
		},
		"nil content": func(context.Context, *PromptRequest) ([]PromptMessage, error) {
			return []PromptMessage{{Role: RoleUser}}, nil
		},
	}
	for name, h := range bad {
		_, err := BuildPrompt(&Prompt{Name: "p", Handler: h}, quiet)(ctx, req)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	slow := &Prompt{Name: "p", Timeout: 10 * time.Millisecond, Handler: func(ctx context.Context, _ *PromptRequest) ([]PromptMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, err := BuildPrompt(slow, quiet)(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("timeout: err = %v", err)
	}

	if _, err := BuildPrompt(&Prompt{Name: "p", Handler: say("12345")}, Defaults{MaxOutput: 4})(ctx, req); err == nil {
		t.Error("size: want error")
	}

	scoped := &Prompt{Name: "p", Scopes: []string{"prompts:use"}, Handler: say("x")}
	if _, err := BuildPrompt(scoped, quiet)(ctx, req); err == nil {
		t.Error("scopes: want error without identity")
	}
	authed := auth.NewContext(ctx, &auth.Identity{Scopes: []string{"prompts:use"}})
	if _, err := BuildPrompt(scoped, quiet)(authed, req); err != nil {
		t.Errorf("scopes: %v", err)
	}

	invalid := &Prompt{Name: "p", Handler: func(context.Context, *PromptRequest) ([]PromptMessage, error) {
		return nil, ErrInvalidArguments
	}}
	if _, err := BuildPrompt(invalid, quiet)(ctx, req); !errors.Is(err, ErrInvalidArguments) {
		t.Errorf("invalid arguments must pass through: %v", err)
	}
}
