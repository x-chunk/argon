// Package sdkadapter is the only package that imports the official MCP Go SDK.
// It exposes tools, resources and prompts described by its own plain types
// (see types.go) through an SDK server and wires transports. Swapping or
// upgrading the SDK should only ever touch this package.
package sdkadapter

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
)

// Config describes the server to build.
type Config struct {
	Name         string
	Version      string
	Instructions string
	Logger       *slog.Logger
	Tools        []Tool
	Resources    []Resource
	Prompts      []Prompt
}

// NewServer builds an SDK server exposing cfg's tools, resources and prompts.
func NewServer(cfg Config) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: cfg.Name, Version: cfg.Version},
		&mcp.ServerOptions{
			Instructions: cfg.Instructions,
			Logger:       sdkLogger(cfg.Logger),
			// Empty capabilities: tools are inferred; no legacy logging capability.
			Capabilities: &mcp.ServerCapabilities{},
		},
	)
	srv.AddReceivingMiddleware(recoverMiddleware(cfg.Logger))
	for _, t := range cfg.Tools {
		srv.AddTool(toSDKTool(t.Def), toolHandler(t.Call))
	}
	for _, r := range cfg.Resources {
		addResource(srv, r)
	}
	for _, p := range cfg.Prompts {
		addPrompt(srv, p)
	}
	return srv
}

func toSDKTool(d ToolDef) *mcp.Tool {
	out := &mcp.Tool{
		Name:        d.Name,
		Title:       d.Title,
		Description: d.Description,
		InputSchema: d.InputSchema,
	}
	if d.OutputSchema != nil {
		out.OutputSchema = d.OutputSchema
	}
	a := d.Annotations
	if a != (Annotations{}) {
		out.Annotations = &mcp.ToolAnnotations{
			DestructiveHint: a.Destructive,
			OpenWorldHint:   a.OpenWorld,
			ReadOnlyHint:    a.ReadOnly != nil && *a.ReadOnly,
			IdempotentHint:  a.Idempotent != nil && *a.Idempotent,
		}
	}
	return out
}

func toolHandler(call func(context.Context, json.RawMessage) (*ToolResult, error)) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if id := identityFromExtra(req.Extra); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		res, err := call(ctx, req.Params.Arguments)
		if err != nil {
			// Tool errors are results, not protocol failures.
			r := &mcp.CallToolResult{}
			r.SetError(err)
			return r, nil
		}
		return toSDKResult(res), nil
	}
}

func toSDKResult(r *ToolResult) *mcp.CallToolResult {
	out := &mcp.CallToolResult{
		Content:           make([]mcp.Content, 0, len(r.Content)),
		StructuredContent: r.Structured,
		IsError:           r.IsError,
	}
	for _, b := range r.Content {
		out.Content = append(out.Content, toSDKContent(b))
	}
	return out
}
