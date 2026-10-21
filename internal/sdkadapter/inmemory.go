package sdkadapter

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// CallInMemory connects an in-memory client to srv, calls tool once and
// returns the result in core form. It exercises the full SDK path
// (schema, encoding, result packing) without a subprocess or network.
func CallInMemory(ctx context.Context, srv *Server, tool string, args json.RawMessage, id *auth.Identity) (*core.Result, error) {
	// TODO(v0.1): inject id into the server-side request (in-memory
	// transport has no bearer token); needed for scope tests.
	_ = id
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		return nil, err
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "argontest", Version: "dev"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		return nil, err
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return nil, err
	}
	out := &core.Result{Structured: res.StructuredContent, IsError: res.IsError}
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			out.Content = append(out.Content, core.Content{Text: t.Text})
		}
	}
	return out, nil
}
