package sdkadapter

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/core"
)

// addResource registers r as a fixed resource or a resource template.
// Definitions are validated by core.Registry, so the SDK's panics on invalid
// URIs cannot trigger here.
func addResource(srv *mcp.Server, r Resource) {
	d := r.Def
	h := resourceHandler(d, r.Handler)
	if d.IsTemplate() {
		srv.AddResourceTemplate(&mcp.ResourceTemplate{
			URITemplate: d.URITemplate,
			Name:        d.Name,
			Title:       d.Title,
			Description: d.Description,
			MIMEType:    d.MIMEType,
		}, h)
		return
	}
	srv.AddResource(&mcp.Resource{
		URI:         d.URI,
		Name:        d.Name,
		Title:       d.Title,
		Description: d.Description,
		MIMEType:    d.MIMEType,
	}, h)
}

func resourceHandler(d *core.Resource, h core.ResourceHandler) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if id := identityFromExtra(req.Extra); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		uri := req.Params.URI
		params, ok := d.Match(uri)
		if !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		c, err := h(ctx, &core.ResourceRequest{URI: uri, Params: params})
		if errors.Is(err, core.ErrResourceNotFound) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      c.URI,
			MIMEType: c.MIMEType,
			Text:     c.Text,
			Blob:     c.Blob,
		}}}, nil
	}
}
