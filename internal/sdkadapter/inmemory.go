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
	"go.xchunk.org/argon/internal/errs"
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
// resource is reported as an error wrapping errs.ResourceNotFound. Like
// CallInMemory, srv must be freshly built when id is non-nil.
func ReadInMemory(ctx context.Context, srv *Server, uri string, id *auth.Identity) (*ResourceData, error) {
	cs, closeAll, err := session(ctx, srv, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == mcp.CodeResourceNotFound {
			return nil, fmt.Errorf("%w: %s", errs.ResourceNotFound, uri)
		}
		return nil, err
	}
	if len(res.Contents) == 0 {
		return nil, errors.New("read returned no contents")
	}
	c := res.Contents[0]
	return &ResourceData{URI: c.URI, MIMEType: c.MIMEType, Text: c.Text, Blob: c.Blob}, nil
}

// GetPromptInMemory renders prompt name from srv over an in-memory session.
// Rejected arguments are reported as an error wrapping errs.InvalidArguments.
// Like CallInMemory, srv must be freshly built when id is non-nil.
func GetPromptInMemory(ctx context.Context, srv *Server, name string, args map[string]string, id *auth.Identity) ([]PromptMsg, error) {
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
			msg := strings.TrimPrefix(rpcErr.Message, errs.InvalidArguments.Error()+": ")
			return nil, fmt.Errorf("%w: %s", errs.InvalidArguments, msg)
		}
		return nil, err
	}
	out := make([]PromptMsg, 0, len(res.Messages))
	for _, m := range res.Messages {
		b, _ := fromSDKContent(m.Content)
		out = append(out, PromptMsg{Role: string(m.Role), Content: b})
	}
	return out, nil
}

// CallInMemory connects an in-memory client to srv, calls tool once and
// returns the result. It exercises the full SDK path (schema, encoding,
// result packing) without a subprocess or network.
//
// If id is non-nil, the call runs as that identity, as if it had arrived with
// a verified bearer token. srv must be freshly built for this call, because
// injecting the identity installs middleware on it.
func CallInMemory(ctx context.Context, srv *Server, tool string, args json.RawMessage, id *auth.Identity) (*ToolResult, error) {
	cs, closeAll, err := session(ctx, srv, id)
	if err != nil {
		return nil, err
	}
	defer closeAll()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return nil, err
	}
	out := &ToolResult{Structured: res.StructuredContent, IsError: res.IsError}
	for _, c := range res.Content {
		if b, ok := fromSDKContent(c); ok {
			out.Content = append(out.Content, b)
		}
	}
	return out, nil
}
