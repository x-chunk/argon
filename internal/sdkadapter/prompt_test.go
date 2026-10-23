package sdkadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/errs"
)

func promptServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(Config{Name: "t", Version: "0", Prompts: []Prompt{
		{Def: PromptDef{Name: "greet", Description: "Greet someone",
			Arguments: []PromptArg{{Name: "name", Description: "who", Required: true}}},
			Get: func(_ context.Context, args map[string]string) ([]PromptMsg, error) {
				if args["name"] == "" {
					return nil, fmt.Errorf("%w: name is required", errs.InvalidArguments)
				}
				return []PromptMsg{
					{Role: "user", Content: Block{Kind: BlockText, Text: "Say hi to " + args["name"]}},
					{Role: "assistant", Content: Block{Kind: BlockImage, MIMEType: "image/png", Data: []byte{1}}},
				}, nil
			}},
		// Argon checks scopes above the adapter; this stands in for it.
		{Def: PromptDef{Name: "admin"},
			Get: func(ctx context.Context, _ map[string]string) ([]PromptMsg, error) {
				if !auth.FromContext(ctx).HasScope("admin") {
					return nil, errors.New("insufficient scope")
				}
				return []PromptMsg{{Role: "user", Content: Block{Kind: BlockText, Text: "secret"}}}, nil
			}},
	}})
}

func TestPromptList(t *testing.T) {
	cs, closeAll, err := session(t.Context(), promptServer(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAll()
	res, err := cs.ListPrompts(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Prompts) != 2 {
		t.Fatalf("prompts = %+v", res.Prompts)
	}
	// The SDK lists prompts sorted by name.
	var g *mcp.Prompt
	for _, p := range res.Prompts {
		if p.Name == "greet" {
			g = p
		}
	}
	if g == nil || len(g.Arguments) != 1 || !g.Arguments[0].Required || g.Arguments[0].Description != "who" {
		t.Fatalf("greet = %+v", g)
	}
}

func TestPromptGet(t *testing.T) {
	ctx := t.Context()
	msgs, err := GetPromptInMemory(ctx, promptServer(t), "greet", map[string]string{"name": "Ann"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[0].Content.Text != "Say hi to Ann" {
		t.Fatalf("msgs = %+v", msgs)
	}
	if img := msgs[1].Content; img.Kind != BlockImage || msgs[1].Role != "assistant" || img.MIMEType != "image/png" {
		t.Fatalf("second message = %+v", msgs[1])
	}
	if _, err := GetPromptInMemory(ctx, promptServer(t), "greet", nil, nil); !errors.Is(err, errs.InvalidArguments) {
		t.Fatalf("missing argument: err = %v, want ErrInvalidArguments", err)
	}
	if _, err := GetPromptInMemory(ctx, promptServer(t), "nope", nil, nil); err == nil {
		t.Fatal("unknown prompt: want error")
	}
}

func TestPromptScopesOverHTTP(t *testing.T) {
	if _, err := GetPromptInMemory(t.Context(), promptServer(t), "admin", nil, nil); err == nil {
		t.Fatal("scoped prompt without identity must fail")
	}
	h, err := HTTPHandler(promptServer(t), &auth.Config{
		Issuer:   "https://issuer.example",
		Audience: "https://mcp.example",
		Verifier: auth.Static("tok", auth.Identity{Subject: "root", Scopes: []string{"admin"}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	cs := connect(t, ts.URL, "tok")
	res, err := cs.GetPrompt(t.Context(), &mcp.GetPromptParams{Name: "admin"})
	if err != nil || len(res.Messages) != 1 {
		t.Fatalf("scoped prompt over HTTP: %+v, %v", res, err)
	}
}
