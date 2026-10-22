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

// Resource pairs a resource definition with its fully built handler
// (see core.BuildResource).
type Resource struct {
	Def     *core.Resource
	Handler core.ResourceHandler
}

// Config describes the server to build.
type Config struct {
	Name         string
	Version      string
	Instructions string
	Logger       *slog.Logger
	Tools        []Tool
	Resources    []Resource
}

// NewServer builds an SDK server exposing cfg.Tools.
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
	for _, t := range cfg.Tools {
		srv.AddTool(toSDKTool(t.Def), toolHandler(t.Def.Name, t.Handler))
	}
	for _, r := range cfg.Resources {
		addResource(srv, r)
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
		if id := identityFromExtra(req.Extra); id != nil {
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
		if sc := toSDKContent(c); sc != nil {
			out.Content = append(out.Content, sc)
		}
	}
	return out
}

func toSDKContent(c core.Content) mcp.Content {
	switch c := c.(type) {
	case core.TextContent:
		return &mcp.TextContent{Text: c.Text}
	case core.ImageContent:
		return &mcp.ImageContent{MIMEType: c.MIMEType, Data: c.Data}
	case core.ResourceLink:
		return &mcp.ResourceLink{URI: c.URI, Name: c.Name, Title: c.Title, Description: c.Description, MIMEType: c.MIMEType}
	}
	return nil
}

func fromSDKContent(c mcp.Content) core.Content {
	switch c := c.(type) {
	case *mcp.TextContent:
		return core.TextContent{Text: c.Text}
	case *mcp.ImageContent:
		return core.ImageContent{MIMEType: c.MIMEType, Data: c.Data}
	case *mcp.ResourceLink:
		return core.ResourceLink{URI: c.URI, Name: c.Name, Title: c.Title, Description: c.Description, MIMEType: c.MIMEType}
	}
	return nil
}
