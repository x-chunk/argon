package argon

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"go.xchunk.org/argon/internal/core"
)

// ResourceContents is what a resource handler returns: text or binary data.
// URI defaults to the requested URI and MIMEType to the resource's declared
// type (see MIMEType).
type ResourceContents = core.ResourceContents

// ErrResourceNotFound is returned (possibly wrapped) by a resource handler
// when the requested resource does not exist; the client receives the
// protocol's "resource not found" error.
var ErrResourceNotFound = core.ErrResourceNotFound

// TextResource returns text contents.
func TextResource(text string) ResourceContents { return ResourceContents{Text: text} }

// BlobResource returns binary contents of the given MIME type.
func BlobResource(mimeType string, data []byte) ResourceContents {
	return ResourceContents{MIMEType: mimeType, Blob: data}
}

// ResourceOption configures a resource or resource template.
type ResourceOption func(*core.Resource)

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
	s.addResource(&core.Resource{
		URI:         uri,
		Name:        name,
		Description: description,
		Handler: func(ctx context.Context, _ *core.ResourceRequest) (*core.ResourceContents, error) {
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
	sc, err := paramsSchema[P]()
	if err != nil {
		s.errs = append(s.errs, fmt.Errorf("argon: resource template %q: %w", uriTemplate, err))
		return
	}
	r := &core.Resource{
		URITemplate: uriTemplate,
		Name:        name,
		Description: description,
		Handler: func(ctx context.Context, req *core.ResourceRequest) (*core.ResourceContents, error) {
			var p P
			if err := sc.decodeParams(req.Params, &p); err != nil {
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
	for _, v := range r.Varnames() {
		if _, ok := sc.props[v]; !ok {
			s.errs = append(s.errs, fmt.Errorf("argon: resource template %q: variable %q has no matching field in %T", uriTemplate, v, *new(P)))
		}
	}
}

func (s *Server) addResource(r *core.Resource, opts []ResourceOption) bool {
	for _, o := range opts {
		o(r)
	}
	if err := s.reg.AddResource(r); err != nil {
		s.errs = append(s.errs, err)
		return false
	}
	return true
}

// paramSchema is the schema of a template parameter struct, with each
// property's JSON type for converting string template values.
type paramSchema struct {
	*schema
	props map[string]string // property name → JSON Schema type
}

func paramsSchema[P any]() (*paramSchema, error) {
	if reflect.TypeFor[P]().Kind() != reflect.Struct {
		return nil, fmt.Errorf("parameter type %s must be a struct", reflect.TypeFor[P]())
	}
	sc, err := schemaFor[P]()
	if err != nil {
		return nil, err
	}
	props := make(map[string]string)
	for name, p := range sc.resolved.Schema().Properties {
		props[name] = p.Type
	}
	return &paramSchema{schema: sc, props: props}, nil
}

// decodeParams converts template values to the property types, then
// validates and decodes them like tool input.
func (ps *paramSchema) decodeParams(params map[string]string, dst any) error {
	m := make(map[string]any, len(params))
	for k, v := range params {
		val, err := convertParam(v, ps.props[k])
		if err != nil {
			return fmt.Errorf("variable %q: %w", k, err)
		}
		m[k] = val
	}
	args, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return ps.decode(args, dst)
}

func convertParam(v, typ string) (any, error) {
	switch typ {
	case "integer":
		return strconv.ParseInt(v, 10, 64)
	case "number":
		return strconv.ParseFloat(v, 64)
	case "boolean":
		return strconv.ParseBool(v)
	}
	return v, nil
}

// MIMEType declares the resource's MIME type; it is listed to clients and
// used for contents that do not set their own.
func MIMEType(m string) ResourceOption { return func(r *core.Resource) { r.MIMEType = m } }

// ResourceTitle sets a human-readable title.
func ResourceTitle(t string) ResourceOption { return func(r *core.Resource) { r.Title = t } }

// ResourceScopes requires the caller to hold all of the given scopes
// (HTTP + auth only).
func ResourceScopes(scopes ...string) ResourceOption {
	return func(r *core.Resource) { r.Scopes = append(r.Scopes, scopes...) }
}

// ResourceTimeout overrides the server default timeout for this resource.
func ResourceTimeout(d time.Duration) ResourceOption { return func(r *core.Resource) { r.Timeout = d } }

// ResourceMaxOutput overrides the server default size limit for this resource.
func ResourceMaxOutput(n int) ResourceOption { return func(r *core.Resource) { r.MaxOutput = n } }
