package sdkadapter

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

func promptServer(t *testing.T) *Server {
	t.Helper()
	reg := core.NewRegistry()
	defs := []*core.Prompt{
		{Name: "greet", Description: "Greet someone",
			Arguments: []core.PromptArgument{{Name: "name", Description: "who", Required: true}},
			Handler: func(_ context.Context, r *core.PromptRequest) ([]core.PromptMessage, error) {
				if r.Arguments["name"] == "" {
					return nil, core.ErrInvalidArguments
				}
				return []core.PromptMessage{
					{Role: core.RoleUser, Content: core.TextContent{Text: "Say hi to " + r.Arguments["name"]}},
					{Role: core.RoleAssistant, Content: core.ImageContent{MIMEType: "image/png", Data: []byte{1}}},
				}, nil
			}},
		{Name: "admin", Scopes: []string{"admin"},
			Handler: func(context.Context, *core.PromptRequest) ([]core.PromptMessage, error) {
				return []core.PromptMessage{{Role: core.RoleUser, Content: core.TextContent{Text: "secret"}}}, nil
			}},
	}
	var ps []Prompt
	for _, d := range defs {
		if err := reg.AddPrompt(d); err != nil {
			t.Fatal(err)
		}
		ps = append(ps, Prompt{Def: d, Handler: core.BuildPrompt(d, core.Defaults{})})
	}
	return NewServer(Config{Name: "t", Version: "0", Prompts: ps})
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
	if len(msgs) != 2 || msgs[0].Role != core.RoleUser || msgs[0].Content.(core.TextContent).Text != "Say hi to Ann" {
		t.Fatalf("msgs = %+v", msgs)
	}
	if img, ok := msgs[1].Content.(core.ImageContent); !ok || msgs[1].Role != core.RoleAssistant || img.MIMEType != "image/png" {
		t.Fatalf("second message = %+v", msgs[1])
	}
	if _, err := GetPromptInMemory(ctx, promptServer(t), "greet", nil, nil); !errors.Is(err, core.ErrInvalidArguments) {
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
