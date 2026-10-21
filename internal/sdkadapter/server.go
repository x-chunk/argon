// Package sdkadapter is the only package that imports the official MCP Go SDK.
// It translates Argon's core registry into an SDK server and wires transports.
// Swapping or upgrading the SDK should only ever touch this package.
package sdkadapter

import (
	"context"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// Tool pairs a tool definition with its fully built handler
// (see core.Build).
type Tool struct {
	Def     *core.Tool
	Handler core.Handler
}

// Config describes the server to build.
type Config struct {
	Name         string
	Version      string
	Instructions string
	Logger       *slog.Logger
	Tools        []Tool
}

// NewServer builds an SDK server exposing cfg.Tools.
func NewServer(cfg Config) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: cfg.Name, Version: cfg.Version},
		&mcp.ServerOptions{
			Instructions: cfg.Instructions,
			Logger:       cfg.Logger,
			// Empty capabilities: tools are inferred; no legacy logging capability.
			Capabilities: &mcp.ServerCapabilities{},
		},
	)
	for _, t := range cfg.Tools {
		srv.AddTool(toSDKTool(t.Def), toolHandler(t.Def.Name, t.Handler))
	}
	return srv
}

func toSDKTool(t *core.Tool) *mcp.Tool {
	out := &mcp.Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		InputSchema: t.InputSchema,
	}
	if t.OutputSchema != nil {
		out.OutputSchema = t.OutputSchema
	}
	a := t.Annotations
	if a != (core.Annotations{}) {
		out.Annotations = &mcp.ToolAnnotations{
			DestructiveHint: a.Destructive,
			OpenWorldHint:   a.OpenWorld,
			ReadOnlyHint:    a.ReadOnly != nil && *a.ReadOnly,
			IdempotentHint:  a.Idempotent != nil && *a.Idempotent,
		}
	}
	return out
}

func toolHandler(name string, h core.Handler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if id := identityFromRequest(req); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		res, err := h(ctx, &core.Request{Tool: name, Arguments: req.Params.Arguments})
		if err != nil {
			// Tool errors are results, not protocol failures.
			r := &mcp.CallToolResult{}
			r.SetError(err)
			return r, nil
		}
		return toSDKResult(res), nil
	}
}

func toSDKResult(r *core.Result) *mcp.CallToolResult {
	out := &mcp.CallToolResult{
		Content:           make([]mcp.Content, 0, len(r.Content)),
		StructuredContent: r.Structured,
		IsError:           r.IsError,
	}
	for _, c := range r.Content {
		out.Content = append(out.Content, &mcp.TextContent{Text: c.Text})
	}
	return out
}
