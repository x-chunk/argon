package core

import (
	"context"
	"fmt"
	"log/slog"
)

// BuildPrompt assembles the effective handler for p: panic recovery
// (outermost), scopes, timeout, and the output size limit over all message
// contents. It also checks that every message has a valid role and content.
// Tool middleware does not apply to prompts.
func BuildPrompt(p *Prompt, d Defaults) PromptHandler {
	timeout := d.Timeout
	if p.Timeout != 0 {
		timeout = p.Timeout
	}
	maxOut := d.MaxOutput
	if p.MaxOutput != 0 {
		maxOut = p.MaxOutput
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}

	return func(ctx context.Context, req *PromptRequest) ([]PromptMessage, error) {
		return guardPanic(log, "prompt", p.Name, func() ([]PromptMessage, error) {
			if err := checkScopes(ctx, "prompt", p.Name, p.Scopes); err != nil {
				return nil, err
			}
			ctx, cancel := withTimeout(ctx, timeout)
			defer cancel()
			msgs, err := p.Handler(ctx, req)
			if err != nil {
				return nil, err
			}
			size := 0
			for i, m := range msgs {
				if m.Role != RoleUser && m.Role != RoleAssistant {
					return nil, fmt.Errorf("prompt %q: message %d has invalid role %q", p.Name, i, m.Role)
				}
				if m.Content == nil {
					return nil, fmt.Errorf("prompt %q: message %d has no content", p.Name, i)
				}
				size += m.Content.size()
			}
			if err := checkSize("prompt", p.Name, size, maxOut); err != nil {
				return nil, err
			}
			return msgs, nil
		})
	}
}
