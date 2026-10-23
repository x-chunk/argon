package argon

import (
	"context"
	"fmt"
	"time"

	"go.xchunk.org/argon/internal/errs"
)

// ErrInvalidArguments reports rejected arguments. Missing, unknown and
// malformed prompt arguments are rejected with it automatically before the
// handler runs; a prompt handler may also return it (possibly wrapped) to
// reject input itself. The client receives the protocol's "invalid params"
// error. Invalid tool input wraps it as well, reported as an isError result.
var ErrInvalidArguments = errs.InvalidArguments

// Role is the speaker of a prompt message.
type Role string

// Prompt message roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// PromptMessage is one message of a rendered prompt: a role and a content
// block. Use UserMessage and AssistantMessage for text, or set Content to
// any Content block such as an ImageContent.
type PromptMessage struct {
	Role    Role
	Content Content
}

// promptArgument describes an argument the prompt accepts.
type promptArgument struct {
	Name        string
	Description string
	Required    bool
}

// promptRequest is a single prompts/get call as seen by the pipeline.
type promptRequest struct {
	Name      string
	Arguments map[string]string
}

// promptHandler renders a prompt.
type promptHandler func(ctx context.Context, req *promptRequest) ([]PromptMessage, error)

// promptDef is a registered prompt.
type promptDef struct {
	Name        string
	Title       string
	Description string
	Arguments   []promptArgument

	// Per-prompt policy. Zero values mean "use server defaults".
	Scopes    []string
	Timeout   time.Duration
	MaxOutput int

	Handler promptHandler
}

func (p *promptDef) validate() error {
	if !validName.MatchString(p.Name) {
		return fmt.Errorf("argon: invalid prompt name %q", p.Name)
	}
	if p.Handler == nil {
		return fmt.Errorf("argon: prompt %q has no handler", p.Name)
	}
	return nil
}
