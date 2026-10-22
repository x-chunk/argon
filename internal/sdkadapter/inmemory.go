package sdkadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// session connects an in-memory client to srv, optionally injecting id
// (see CallInMemory), and returns the client session and a close func.
func session(ctx context.Context, srv *Server, id *auth.Identity) (*mcp.ClientSession, func(), error) {
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
		return nil, nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "argontest", Version: "dev"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		ss.Close()
		return nil, nil, err
	}
	return cs, func() { cs.Close(); ss.Close() }, nil
}

// ReadInMemory reads uri from srv over an in-memory session. A missing
// resource is reported as core.ErrResourceNotFound. Like CallInMemory, srv
// must be freshly built when id is non-nil.
func ReadInMemory(ctx context.Context, srv *Server, uri string, id *auth.Identity) (*core.ResourceContents, error) {
	cs, closeAll, err := session(ctx, srv, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == mcp.CodeResourceNotFound {
			return nil, fmt.Errorf("%w: %s", core.ErrResourceNotFound, uri)
		}
		return nil, err
	}
	if len(res.Contents) == 0 {
		return nil, errors.New("read returned no contents")
	}
	c := res.Contents[0]
	return &core.ResourceContents{URI: c.URI, MIMEType: c.MIMEType, Text: c.Text, Blob: c.Blob}, nil
}

// GetPromptInMemory renders prompt name from srv over an in-memory session.
// Rejected arguments are reported as core.ErrInvalidArguments. Like
// CallInMemory, srv must be freshly built when id is non-nil.
func GetPromptInMemory(ctx context.Context, srv *Server, name string, args map[string]string, id *auth.Identity) ([]core.PromptMessage, error) {
	cs, closeAll, err := session(ctx, srv, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	res, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: name, Arguments: args})
	if err != nil {
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeInvalidParams {
			// The server message already starts with the sentinel's text.
			msg := strings.TrimPrefix(rpcErr.Message, core.ErrInvalidArguments.Error()+": ")
			return nil, fmt.Errorf("%w: %s", core.ErrInvalidArguments, msg)
		}
		return nil, err
	}
	out := make([]core.PromptMessage, 0, len(res.Messages))
	for _, m := range res.Messages {
		out = append(out, core.PromptMessage{Role: core.Role(m.Role), Content: fromSDKContent(m.Content)})
	}
	return out, nil
}

// CallInMemory connects an in-memory client to srv, calls tool once and
// returns the result in core form. It exercises the full SDK path
// (schema, encoding, result packing) without a subprocess or network.
//
// If id is non-nil, the call runs as that identity, as if it had arrived with
// a verified bearer token. srv must be freshly built for this call, because
// injecting the identity installs middleware on it.
func CallInMemory(ctx context.Context, srv *Server, tool string, args json.RawMessage, id *auth.Identity) (*core.Result, error) {
	cs, closeAll, err := session(ctx, srv, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()

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
