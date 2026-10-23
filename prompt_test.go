package argon_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.xchunk.org/argon"
	"go.xchunk.org/argon/argontest"
)

type reviewArgs struct {
	Code     string `json:"code" jsonschema:"code to review"`
	Language string `json:"language,omitempty"`
	Depth    int    `json:"depth,omitempty"`
	Strict   bool   `json:"strict,omitempty"`
}

func promptServer() *argon.Server {
	s := argon.New("prompts")
	argon.Prompt(s, "code_review", "Review a piece of code",
		func(_ context.Context, a reviewArgs) ([]argon.PromptMessage, error) {
			text := "Review this " + a.Language + " code (depth " + strconv.Itoa(a.Depth) + ", strict " + strconv.FormatBool(a.Strict) + "):\n" + a.Code
			return []argon.PromptMessage{
				argon.UserMessage(text),
				argon.AssistantMessage("Sure, let me look."),
			}, nil
		})
	argon.Prompt(s, "diagram", "Explain a diagram",
		func(context.Context, struct{}) ([]argon.PromptMessage, error) {
			return []argon.PromptMessage{
				{Role: argon.RoleUser, Content: argon.ImageContent{MIMEType: "image/png", Data: []byte{0x89}}},
				{Role: argon.RoleUser, Content: argon.ResourceLink{URI: "file:///diagram.png", Name: "diagram"}},
			}, nil
		}, argon.PromptScopes("diagrams:read"))
	argon.Prompt(s, "picky", "Rejects odd input itself",
		func(_ context.Context, a struct {
			N int `json:"n"`
		}) ([]argon.PromptMessage, error) {
			if a.N%2 != 0 {
				return nil, argon.ErrInvalidArguments
			}
			return []argon.PromptMessage{argon.UserMessage("even")}, nil
		})
	return s
}

func TestGetPrompt(t *testing.T) {
	msgs, err := argontest.GetPrompt(t, promptServer(), "code_review", map[string]string{
		"code": "x := 1", "language": "Go", "depth": "2", "strict": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != argon.RoleUser || msgs[1].Role != argon.RoleAssistant {
		t.Fatalf("msgs = %+v", msgs)
	}
	want := "Review this Go code (depth 2, strict true):\nx := 1"
	if got := msgs[0].Content.(argon.TextContent).Text; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestGetPromptRejectsArguments(t *testing.T) {
	s := promptServer()
	for name, args := range map[string]map[string]string{
		"missing required": {"language": "Go"},
		"unknown":          {"code": "x", "extra": "y"},
		"not an int":       {"code": "x", "depth": "deep"},
		"not a bool":       {"code": "x", "strict": "maybe"},
	} {
		if _, err := argontest.GetPrompt(t, s, "code_review", args); !errors.Is(err, argon.ErrInvalidArguments) {
			t.Errorf("%s: err = %v, want ErrInvalidArguments", name, err)
		}
	}
	if _, err := argontest.GetPrompt(t, s, "picky", map[string]string{"n": "3"}); !errors.Is(err, argon.ErrInvalidArguments) {
		t.Errorf("handler rejection: err = %v", err)
	}
	if _, err := argontest.GetPrompt(t, s, "picky", map[string]string{"n": "4"}); err != nil {
		t.Errorf("handler accepts even: %v", err)
	}
}

func TestGetPromptContentAndScopes(t *testing.T) {
	s := promptServer()
	if _, err := argontest.GetPrompt(t, s, "diagram", nil); err == nil {
		t.Fatal("scoped prompt without identity must fail")
	}
	msgs, err := argontest.GetPrompt(t, s, "diagram", nil,
		argontest.WithIdentity(argon.Identity{Subject: "u", Scopes: []string{"diagrams:read"}}))
	if err != nil || len(msgs) != 2 {
		t.Fatalf("msgs = %+v, %v", msgs, err)
	}
	if _, ok := msgs[0].Content.(argon.ImageContent); !ok {
		t.Errorf("first = %#v, want image", msgs[0].Content)
	}
	if l, ok := msgs[1].Content.(argon.ResourceLink); !ok || l.URI != "file:///diagram.png" {
		t.Errorf("second = %#v, want resource link", msgs[1].Content)
	}
}

func TestPromptArgumentErrorMessage(t *testing.T) {
	_, err := argontest.GetPrompt(t, promptServer(), "code_review", map[string]string{"code": "x", "extra": "y"})
	if err == nil {
		t.Fatal("want error")
	}
	if n := strings.Count(err.Error(), "invalid arguments"); n != 1 {
		t.Fatalf("message %q repeats the prefix %d times", err, n)
	}
}

type limitArgs struct {
	Limit *int `json:"limit,omitempty" jsonschema:"maximum number of items"`
}

// Pointer fields are the Go idiom for optional values; they must convert
// like their element type.
func TestPromptPointerArgument(t *testing.T) {
	s := argon.New("test")
	argon.Prompt(s, "list", "List items", func(_ context.Context, a limitArgs) ([]argon.PromptMessage, error) {
		if a.Limit == nil {
			return []argon.PromptMessage{argon.UserMessage("all")}, nil
		}
		return []argon.PromptMessage{argon.UserMessage(strconv.Itoa(*a.Limit))}, nil
	})
	for args, want := range map[string]string{"": "all", "5": "5"} {
		in := map[string]string{}
		if args != "" {
			in["limit"] = args
		}
		msgs, err := argontest.GetPrompt(t, s, "list", in)
		if err != nil || msgs[0].Content.(argon.TextContent).Text != want {
			t.Fatalf("limit %q: %+v, %v", args, msgs, err)
		}
	}
}
