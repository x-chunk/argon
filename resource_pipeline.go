package argon

import (
	"context"
	"errors"
	"log/slog"
)

// buildResource assembles the effective handler for r. It applies the same
// policies as tools — panic recovery (outermost), scopes, timeout and output
// size — and fills in the content URI and MIME type when the handler leaves
// them empty. Tool middleware does not apply to resources.
func buildResource(r *resourceDef, d policy) resourceHandler {
	timeout := d.Timeout
	if r.Timeout != 0 {
		timeout = r.Timeout
	}
	maxOut := d.MaxOutput
	if r.MaxOutput != 0 {
		maxOut = r.MaxOutput
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	name := r.Key()

	return func(ctx context.Context, req *resourceRequest) (*ResourceContents, error) {
		return guardPanic(log, "resource", name, func() (*ResourceContents, error) {
			if err := checkScopes(ctx, "resource", name, r.Scopes); err != nil {
				return nil, err
			}
			ctx, cancel := withTimeout(ctx, timeout)
			defer cancel()
			c, err := r.Handler(ctx, req)
			if err != nil {
				return nil, err
			}
			if c == nil {
				return nil, errors.New("resource handler returned no contents")
			}
			if c.Text != "" && len(c.Blob) > 0 {
				// MCP contents are either text or binary, never both.
				return nil, errors.New("resource handler returned both text and blob contents")
			}
			if err := checkSize("resource", name, c.size(), maxOut); err != nil {
				return nil, err
			}
			out := *c
			if out.URI == "" {
				out.URI = req.URI
			}
			if out.MIMEType == "" {
				out.MIMEType = r.MIMEType
			}
			return &out, nil
		})
	}
}
