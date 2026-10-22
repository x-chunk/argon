package sdkadapter

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// Prompt pairs a prompt definition with its fully built handler
// (see core.BuildPrompt).
type Prompt struct {
	Def     *core.Prompt
	Handler core.PromptHandler
}

func addPrompt(srv *mcp.Server, p Prompt) {
	d := p.Def
	args := make([]*mcp.PromptArgument, 0, len(d.Arguments))
	for _, a := range d.Arguments {
		args = append(args, &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required})
	}
	srv.AddPrompt(&mcp.Prompt{
		Name:        d.Name,
		Title:       d.Title,
		Description: d.Description,
		Arguments:   args,
	}, promptHandler(d.Name, p.Handler))
}

func promptHandler(name string, h core.PromptHandler) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		if id := identityFromExtra(req.Extra); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		msgs, err := h(ctx, &core.PromptRequest{Name: name, Arguments: req.Params.Arguments})
		if errors.Is(err, core.ErrInvalidArguments) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
		}
		if err != nil {
			return nil, err
		}
		out := &mcp.GetPromptResult{Messages: make([]*mcp.PromptMessage, 0, len(msgs))}
		for _, m := range msgs {
			out.Messages = append(out.Messages, &mcp.PromptMessage{Role: mcp.Role(m.Role), Content: toSDKContent(m.Content)})
		}
		return out, nil
	}
}
