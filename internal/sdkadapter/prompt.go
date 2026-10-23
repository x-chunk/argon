package sdkadapter

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/errs"
)

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
	}, promptHandler(p.Get))
}

func promptHandler(get func(context.Context, map[string]string) ([]PromptMsg, error)) mcp.PromptHandler {
	return func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		if id := identityFromExtra(req.Extra); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		msgs, err := get(ctx, req.Params.Arguments)
		if errors.Is(err, errs.InvalidArguments) {
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
