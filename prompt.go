package argon

import (
	"context"
	"fmt"
	"time"
)

// UserMessage returns a text message from the user.
func UserMessage(text string) PromptMessage {
	return PromptMessage{Role: RoleUser, Content: TextContent{Text: text}}
}

// AssistantMessage returns a text message from the assistant.
func AssistantMessage(text string) PromptMessage {
	return PromptMessage{Role: RoleAssistant, Content: TextContent{Text: text}}
}

// PromptOption configures a prompt.
type PromptOption func(*promptDef)

// Prompt registers a prompt template on s.
//
// The prompt's arguments are the fields of In, a struct: names come from
// json tags, descriptions from jsonschema tags, and fields without
// omitempty are required. Clients send arguments as strings; they are
// converted to the field types (string, integers, floats, bool) and
// validated before the handler runs. Use struct{} for a prompt without
// arguments.
//
//	type ReviewArgs struct {
//		Code     string `json:"code" jsonschema:"code to review"`
//		Language string `json:"language,omitempty"`
//	}
//	argon.Prompt(s, "code_review", "Review a piece of code",
//		func(ctx context.Context, a ReviewArgs) ([]argon.PromptMessage, error) {
//			return []argon.PromptMessage{
//				argon.UserMessage("Review this " + a.Language + " code:\n" + a.Code),
//			}, nil
//		})
//
// Prompts get the same safe defaults as tools (timeout, output limit,
// panic recovery) and may require scopes. Tool middleware does not apply.
// Registration errors are reported by Run*.
func Prompt[In any](s *Server, name, description string, h func(context.Context, In) ([]PromptMessage, error), opts ...PromptOption) {
	sa, err := stringArgsSchema[In]()
	if err != nil {
		s.errs = append(s.errs, fmt.Errorf("argon: prompt %q: %w", name, err))
		return
	}
	p := &promptDef{
		Name:        name,
		Description: description,
		Handler: func(ctx context.Context, req *promptRequest) ([]PromptMessage, error) {
			var in In
			if err := sa.decodeStrings(req.Arguments, &in); err != nil {
				return nil, err // already wraps ErrInvalidArguments
			}
			return h(ctx, in)
		},
	}
	for _, n := range sa.order {
		prop := sa.props[n]
		p.Arguments = append(p.Arguments, promptArgument{Name: n, Description: prop.description, Required: prop.required})
	}
	for _, o := range opts {
		o(p)
	}
	if err := s.reg.AddPrompt(p); err != nil {
		s.errs = append(s.errs, err)
	}
}

// PromptTitle sets a human-readable title.
func PromptTitle(t string) PromptOption { return func(p *promptDef) { p.Title = t } }

// PromptScopes requires the caller to hold all of the given scopes
// (HTTP + auth only).
func PromptScopes(scopes ...string) PromptOption {
	return func(p *promptDef) { p.Scopes = append(p.Scopes, scopes...) }
}

// PromptTimeout overrides the server default timeout for this prompt.
func PromptTimeout(d time.Duration) PromptOption { return func(p *promptDef) { p.Timeout = d } }

// PromptMaxOutput overrides the server default size limit for this prompt.
func PromptMaxOutput(n int) PromptOption { return func(p *promptDef) { p.MaxOutput = n } }
