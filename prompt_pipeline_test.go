package argon

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
	quiet := policy{Logger: slog.New(slog.DiscardHandler)}
	ctx := context.Background()
	req := &promptRequest{Name: "p"}

	msgs, err := buildPrompt(&promptDef{Name: "p", Handler: say("hi")}, quiet)(ctx, req)
	if err != nil || len(msgs) != 1 || msgs[0].Content.(TextContent).Text != "hi" {
		t.Fatalf("ok: %v, %v", msgs, err)
	}

	bad := map[string]promptHandler{
		"panic": func(context.Context, *promptRequest) ([]PromptMessage, error) { panic("secret") },
		"bad role": func(context.Context, *promptRequest) ([]PromptMessage, error) {
			return []PromptMessage{{Role: "system", Content: TextContent{Text: "x"}}}, nil
		},
		"nil content": func(context.Context, *promptRequest) ([]PromptMessage, error) {
			return []PromptMessage{{Role: RoleUser}}, nil
		},
	}
	for name, h := range bad {
		_, err := buildPrompt(&promptDef{Name: "p", Handler: h}, quiet)(ctx, req)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	slow := &promptDef{Name: "p", Timeout: 10 * time.Millisecond, Handler: func(ctx context.Context, _ *promptRequest) ([]PromptMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, err := buildPrompt(slow, quiet)(ctx, req); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("timeout: err = %v", err)
	}

	if _, err := buildPrompt(&promptDef{Name: "p", Handler: say("12345")}, policy{MaxOutput: 4})(ctx, req); err == nil {
		t.Error("size: want error")
	}

	scoped := &promptDef{Name: "p", Scopes: []string{"prompts:use"}, Handler: say("x")}
	if _, err := buildPrompt(scoped, quiet)(ctx, req); err == nil {
		t.Error("scopes: want error without identity")
	}
	authed := auth.NewContext(ctx, &auth.Identity{Scopes: []string{"prompts:use"}})
	if _, err := buildPrompt(scoped, quiet)(authed, req); err != nil {
		t.Errorf("scopes: %v", err)
	}

	invalid := &promptDef{Name: "p", Handler: func(context.Context, *promptRequest) ([]PromptMessage, error) {
		return nil, ErrInvalidArguments
	}}
	if _, err := buildPrompt(invalid, quiet)(ctx, req); !errors.Is(err, ErrInvalidArguments) {
		t.Errorf("invalid arguments must pass through: %v", err)
	}
}
