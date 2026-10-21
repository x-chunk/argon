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
//
// If id is non-nil, the call runs as that identity, as if it had arrived with
// a verified bearer token. srv must be freshly built for this call, because
// injecting the identity installs middleware on it.
func CallInMemory(ctx context.Context, srv *Server, tool string, args json.RawMessage, id *auth.Identity) (*core.Result, error) {
	if id != nil {
		srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				return next(auth.NewContext(ctx, id), method, req)
			}
		})
	}
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
		if cc := fromSDKContent(c); cc != nil {
			out.Content = append(out.Content, cc)
		}
	}
	return out, nil
}
