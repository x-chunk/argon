package argon

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// TextResource returns text contents.
func TextResource(text string) ResourceContents { return ResourceContents{Text: text} }

// BlobResource returns binary contents of the given MIME type.
func BlobResource(mimeType string, data []byte) ResourceContents {
	return ResourceContents{MIMEType: mimeType, Blob: data}
}

// ResourceOption configures a resource or resource template.
type ResourceOption func(*resourceDef)

// Resource registers a resource with a fixed, absolute URI on s.
//
//	argon.Resource(s, "file:///readme.md", "readme", "Project readme",
//		func(ctx context.Context) (argon.ResourceContents, error) {
//			return argon.TextResource(readme), nil
//		}, argon.MIMEType("text/markdown"))
//
// Resources get the same safe defaults as tools (timeout, output limit,
// panic recovery) and may require scopes. Tool middleware does not apply.
// Registration errors are reported by Run*.
func Resource(s *Server, uri, name, description string, h func(context.Context) (ResourceContents, error), opts ...ResourceOption) {
	s.addResource(&resourceDef{
		URI:         uri,
		Name:        name,
		Description: description,
		Handler: func(ctx context.Context, _ *resourceRequest) (*ResourceContents, error) {
			c, err := h(ctx)
			if err != nil {
				return nil, err
			}
			return &c, nil
		},
	}, opts)
}

// ResourceTemplate registers a family of resources described by an RFC 6570
// URI template, such as "notes://{id}". Template variables are decoded into
// P, a struct whose json field names match the variable names; values are
// converted to the field types (string, integers, floats, bool) and
// validated like tool input. A URI whose variables do not decode is
// reported to the client as not found.
//
// Variables arrive percent-decoded, so they may contain "/" or "..": treat
// them as untrusted input, for example before using one as a file path.
//
//	type NoteRef struct {
//		ID int `json:"id"`
//	}
//	argon.ResourceTemplate(s, "notes://{id}", "note", "A note by id",
//		func(ctx context.Context, ref NoteRef) (argon.ResourceContents, error) {
//			n, ok := notes[ref.ID]
//			if !ok {
//				return argon.ResourceContents{}, argon.ErrResourceNotFound
//			}
//			return argon.TextResource(n), nil
//		})
func ResourceTemplate[P any](s *Server, uriTemplate, name, description string, h func(context.Context, P) (ResourceContents, error), opts ...ResourceOption) {
	sc, err := stringArgsSchema[P]()
	if err != nil {
		s.errs = append(s.errs, fmt.Errorf("argon: resource template %q: %w", uriTemplate, err))
		return
	}
	r := &resourceDef{
		URITemplate: uriTemplate,
		Name:        name,
		Description: description,
		Handler: func(ctx context.Context, req *resourceRequest) (*ResourceContents, error) {
			var p P
			if err := sc.decodeStrings(req.Params, &p); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrResourceNotFound, err)
			}
			c, err := h(ctx, p)
			if err != nil {
				return nil, err
			}
			return &c, nil
		},
	}
	if !s.addResource(r, opts) {
		return
	}
	vars := r.Varnames()
	for _, v := range vars {
		if !sc.has(v) {
			s.errs = append(s.errs, fmt.Errorf("argon: resource template %q: variable %q has no matching field in %T", uriTemplate, v, *new(P)))
		}
	}
	// A required field the template never fills would make every read fail.
	for _, name := range sc.order {
		if sc.props[name].required && !slices.Contains(vars, name) {
			s.errs = append(s.errs, fmt.Errorf("argon: resource template %q: required field %q of %T is not a template variable", uriTemplate, name, *new(P)))
		}
	}
}

func (s *Server) addResource(r *resourceDef, opts []ResourceOption) bool {
	for _, o := range opts {
		o(r)
	}
	if err := s.reg.AddResource(r); err != nil {
		s.errs = append(s.errs, err)
		return false
	}
	return true
}

// MIMEType declares the resource's MIME type; it is listed to clients and
// used for contents that do not set their own.
func MIMEType(m string) ResourceOption { return func(r *resourceDef) { r.MIMEType = m } }

// ResourceTitle sets a human-readable title.
func ResourceTitle(t string) ResourceOption { return func(r *resourceDef) { r.Title = t } }

// ResourceScopes requires the caller to hold all of the given scopes.
// Without authentication (stdio, or HTTP without Auth) reads are always
// rejected; see Scopes.
func ResourceScopes(scopes ...string) ResourceOption {
	return func(r *resourceDef) { r.Scopes = append(r.Scopes, scopes...) }
}

// ResourceTimeout overrides the server default timeout for this resource.
// Zero keeps the default; a negative value disables the timeout.
func ResourceTimeout(d time.Duration) ResourceOption { return func(r *resourceDef) { r.Timeout = d } }

// ResourceMaxOutput overrides the server default size limit for this
// resource. Zero keeps the default; a negative value disables the limit.
func ResourceMaxOutput(n int) ResourceOption { return func(r *resourceDef) { r.MaxOutput = n } }
