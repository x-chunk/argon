package sdkadapter

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.xchunk.org/argon/auth"
	"go.xchunk.org/argon/internal/errs"
)

// addResource registers r as a fixed resource or a resource template. URIs
// are validated before they reach the adapter, so the SDK's panics on
// invalid URIs cannot trigger here.
func addResource(srv *mcp.Server, r Resource) {
	d := r.Def
	h := resourceHandler(r.Read)
	if d.URITemplate != "" {
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

func resourceHandler(read func(context.Context, string) (*ResourceData, error)) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if id := identityFromExtra(req.Extra); id != nil {
			ctx = auth.NewContext(ctx, id)
		}
		// Contents read on behalf of an authenticated caller must not be
		// cached by intermediaries or shared with other users.
		cacheScope := "public"
		if auth.FromContext(ctx) != nil {
			cacheScope = "private"
		}
		uri := req.Params.URI
		c, err := read(ctx, uri)
		if errors.Is(err, errs.ResourceNotFound) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		if err != nil {
			return nil, err
		}
		return &mcp.ReadResourceResult{Cacheable: mcp.Cacheable{CacheScope: cacheScope}, Contents: []*mcp.ResourceContents{{
			URI:      c.URI,
			MIMEType: c.MIMEType,
			Text:     c.Text,
			Blob:     c.Blob,
		}}}, nil
	}
}
