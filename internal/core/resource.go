package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/yosida95/uritemplate/v3"

	"go.xchunk.org/argon/internal/errs"
)

// ErrResourceNotFound is returned by a resource handler when the requested
// resource does not exist. The adapter maps it to the protocol's
// "resource not found" error.
var ErrResourceNotFound = errs.ResourceNotFound

// ResourceRequest is a single resources/read call as seen by the core.
type ResourceRequest struct {
	// URI is the requested resource URI.
	URI string
	// Params holds the URI template variables matched from URI. It is nil
	// for fixed-URI resources.
	Params map[string]string
}

// ResourceContents is the content of a resource: either text or binary.
type ResourceContents struct {
	// URI of the content; defaults to the requested URI when empty.
	URI      string
	MIMEType string
	Text     string
	Blob     []byte
}

func (c *ResourceContents) size() int { return len(c.Text) + len(c.Blob) }

// ResourceHandler reads a resource.
type ResourceHandler func(ctx context.Context, req *ResourceRequest) (*ResourceContents, error)

// Resource is a registered resource: either a fixed URI or a URI template.
type Resource struct {
	// Exactly one of URI and URITemplate is set.
	URI         string
	URITemplate string

	Name        string
	Title       string
	Description string
	MIMEType    string

	// Per-resource policy. Zero values mean "use server defaults".
	Scopes    []string
	Timeout   time.Duration
	MaxOutput int

	Handler ResourceHandler

	tmpl *uritemplate.Template // compiled URITemplate
}

// Key identifies the resource in the registry: its URI or URI template.
func (r *Resource) Key() string {
	if r.URITemplate != "" {
		return r.URITemplate
	}
	return r.URI
}

// IsTemplate reports whether r is a URI template.
func (r *Resource) IsTemplate() bool { return r.URITemplate != "" }

// Varnames returns the template's variable names (nil for fixed URIs).
func (r *Resource) Varnames() []string {
	if r.tmpl == nil {
		return nil
	}
	return r.tmpl.Varnames()
}

// Match extracts template variables from uri. ok is false when uri does not
// match the template.
func (r *Resource) Match(uri string) (params map[string]string, ok bool) {
	if r.tmpl == nil {
		return nil, uri == r.URI
	}
	vals := r.tmpl.Match(uri)
	if vals == nil {
		return nil, false
	}
	params = make(map[string]string, len(vals))
	for k, v := range vals {
		params[k] = v.String()
	}
	return params, true
}

// validate checks the resource definition and compiles its template. URIs
// must be absolute: the SDK panics on anything else, so we refuse earlier
// with an error.
func (r *Resource) validate() error {
	if (r.URI == "") == (r.URITemplate == "") {
		return errors.New("argon: resource needs exactly one of URI and URI template")
	}
	if r.Name == "" {
		return fmt.Errorf("argon: resource %q has no name", r.Key())
	}
	if r.Handler == nil {
		return fmt.Errorf("argon: resource %q has no handler", r.Key())
	}
	if r.URI != "" {
		u, err := url.Parse(r.URI)
		if err != nil || u.Scheme == "" {
			return fmt.Errorf("argon: resource URI %q must be absolute", r.URI)
		}
		return nil
	}
	t, err := uritemplate.New(r.URITemplate)
	if err != nil {
		return fmt.Errorf("argon: invalid URI template %q: %w", r.URITemplate, err)
	}
	if !hasScheme(r.URITemplate) {
		return fmt.Errorf("argon: URI template %q must be absolute", r.URITemplate)
	}
	r.tmpl = t
	return nil
}

// hasScheme reports whether s starts with a literal URI scheme (RFC 3986
// §3.1: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":"), before any template
// expression. url.Parse cannot be used on templates: braces in the host make
// it fail.
func hasScheme(s string) bool {
	for i, c := range s {
		switch {
		case c == ':':
			return i > 0
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case i > 0 && ('0' <= c && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return false
}
