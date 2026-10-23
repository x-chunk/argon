package core

import (
	"context"
	"fmt"
	"time"

	"go.xchunk.org/argon/internal/errs"
)

// ErrInvalidArguments is returned by a prompt handler when the arguments are
// missing, unknown or malformed. The adapter maps it to the protocol's
// "invalid params" error.
var ErrInvalidArguments = errs.InvalidArguments

// Role is the speaker of a prompt message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// PromptMessage is one message of a rendered prompt.
type PromptMessage struct {
	Role    Role
	Content Content
}

// PromptArgument describes an argument the prompt accepts.
type PromptArgument struct {
	Name        string
	Description string
	Required    bool
}

// PromptRequest is a single prompts/get call as seen by the core.
type PromptRequest struct {
	Name      string
	Arguments map[string]string
}

// PromptHandler renders a prompt.
type PromptHandler func(ctx context.Context, req *PromptRequest) ([]PromptMessage, error)

// Prompt is a registered prompt.
type Prompt struct {
	Name        string
	Title       string
	Description string
	Arguments   []PromptArgument

	// Per-prompt policy. Zero values mean "use server defaults".
	Scopes    []string
	Timeout   time.Duration
	MaxOutput int

	Handler PromptHandler
}

func (p *Prompt) validate() error {
	if !validName.MatchString(p.Name) {
		return fmt.Errorf("argon: invalid prompt name %q", p.Name)
	}
	if p.Handler == nil {
		return fmt.Errorf("argon: prompt %q has no handler", p.Name)
	}
	return nil
}
