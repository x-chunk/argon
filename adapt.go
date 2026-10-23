package argon

import (
	"context"
	"encoding/json"

	"go.xchunk.org/argon/internal/core"
	"go.xchunk.org/argon/internal/errs"
	"go.xchunk.org/argon/internal/sdkadapter"
)

// This file converts Argon's definitions and results into the SDK adapter's
// own plain types. The adapter never sees Argon's public API.

func adaptTool(t *core.Tool, h core.Handler) sdkadapter.Tool {
	a := t.Annotations
	return sdkadapter.Tool{
		Def: sdkadapter.ToolDef{
			Name:         t.Name,
			Title:        t.Title,
			Description:  t.Description,
			InputSchema:  t.InputSchema,
			OutputSchema: t.OutputSchema,
			Annotations: sdkadapter.Annotations{
				ReadOnly: a.ReadOnly, Destructive: a.Destructive, Idempotent: a.Idempotent, OpenWorld: a.OpenWorld,
			},
		},
		Call: func(ctx context.Context, args json.RawMessage) (*sdkadapter.ToolResult, error) {
			res, err := h(ctx, &core.Request{Tool: t.Name, Arguments: args})
			if err != nil {
				return nil, err
			}
			out := &sdkadapter.ToolResult{Structured: res.Structured, IsError: res.IsError}
			for _, c := range res.Content {
				out.Content = append(out.Content, toBlock(c))
			}
			return out, nil
		},
	}
}

func adaptResource(r *core.Resource, h core.ResourceHandler) sdkadapter.Resource {
	return sdkadapter.Resource{
		Def: sdkadapter.ResourceDef{
			URI:         r.URI,
			URITemplate: r.URITemplate,
			Name:        r.Name,
			Title:       r.Title,
			Description: r.Description,
			MIMEType:    r.MIMEType,
		},
		Read: func(ctx context.Context, uri string) (*sdkadapter.ResourceData, error) {
			params, ok := r.Match(uri)
			if !ok {
				return nil, errs.ResourceNotFound
			}
			c, err := h(ctx, &core.ResourceRequest{URI: uri, Params: params})
			if err != nil {
				return nil, err
			}
			return &sdkadapter.ResourceData{URI: c.URI, MIMEType: c.MIMEType, Text: c.Text, Blob: c.Blob}, nil
		},
	}
}

func adaptPrompt(p *core.Prompt, h core.PromptHandler) sdkadapter.Prompt {
	def := sdkadapter.PromptDef{Name: p.Name, Title: p.Title, Description: p.Description}
	for _, a := range p.Arguments {
		def.Arguments = append(def.Arguments, sdkadapter.PromptArg{Name: a.Name, Description: a.Description, Required: a.Required})
	}
	return sdkadapter.Prompt{
		Def: def,
		Get: func(ctx context.Context, args map[string]string) ([]sdkadapter.PromptMsg, error) {
			msgs, err := h(ctx, &core.PromptRequest{Name: p.Name, Arguments: args})
			if err != nil {
				return nil, err
			}
			out := make([]sdkadapter.PromptMsg, 0, len(msgs))
			for _, m := range msgs {
				out = append(out, sdkadapter.PromptMsg{Role: string(m.Role), Content: toBlock(m.Content)})
			}
			return out, nil
		},
	}
}

func toBlock(c core.Content) sdkadapter.Block {
	switch c := c.(type) {
	case core.ImageContent:
		return sdkadapter.Block{Kind: sdkadapter.BlockImage, MIMEType: c.MIMEType, Data: c.Data}
	case core.ResourceLink:
		return sdkadapter.Block{Kind: sdkadapter.BlockResourceLink, URI: c.URI, Name: c.Name, Title: c.Title, Description: c.Description, MIMEType: c.MIMEType}
	case core.TextContent:
		return sdkadapter.Block{Kind: sdkadapter.BlockText, Text: c.Text}
	}
	return sdkadapter.Block{Kind: sdkadapter.BlockText}
}
