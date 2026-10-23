package argon

import (
	"context"
	"testing"
)

func say(text string) promptHandler {
	return func(context.Context, *promptRequest) ([]PromptMessage, error) {
		return []PromptMessage{{Role: RoleUser, Content: TextContent{Text: text}}}, nil
	}
}

func TestAddPrompt(t *testing.T) {
	r := newRegistry()
	for _, n := range []string{"review", "summarize"} {
		if err := r.AddPrompt(&promptDef{Name: n, Handler: say(n)}); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
	}
	for name, p := range map[string]*promptDef{
		"duplicate":  {Name: "review", Handler: say("")},
		"bad name":   {Name: "has space", Handler: say("")},
		"no handler": {Name: "nohandler"},
	} {
		if err := r.AddPrompt(p); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	got := r.Prompts()
	if len(got) != 2 || got[0].Name != "review" || got[1].Name != "summarize" {
		t.Fatalf("prompts = %v, want registration order", got)
	}
}
