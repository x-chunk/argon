package argon

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"go.xchunk.org/argon/internal/core"
)

// ToolOption configures a single tool.
type ToolOption func(*core.Tool)

// Tool registers a typed tool on s.
//
// In must be a struct (or map); its JSON Schema is inferred from json and
// jsonschema struct tags, and input is validated against it. Out may be a
// string, *Result or any JSON-serializable type. A returned error becomes an
// isError result, not a protocol failure.
//
// Tool is a function rather than a method because Go methods cannot have
// type parameters. Registration errors are reported by Run*.
func Tool[In, Out any](s *Server, name, description string, h func(context.Context, In) (Out, error), opts ...ToolOption) {
	t, err := newTool(name, description, h)
	if err != nil {
		s.errs = append(s.errs, err)
		return
	}
	for _, o := range opts {
		o(t)
	}
	if err := s.reg.Add(t); err != nil {
		s.errs = append(s.errs, err)
	}
}

func newTool[In, Out any](name, description string, h func(context.Context, In) (Out, error)) (*core.Tool, error) {
	in, err := schemaFor[In]()
	if err != nil {
		return nil, fmt.Errorf("argon: tool %q input schema: %w", name, err)
	}
	var outSchema json.RawMessage
	if t := reflect.TypeFor[Out](); t.Kind() == reflect.Struct {
		out, err := schemaFor[Out]()
		if err != nil {
			return nil, fmt.Errorf("argon: tool %q output schema: %w", name, err)
		}
		outSchema = out.raw
	}
	return &core.Tool{
		Name:         name,
		Description:  description,
		InputSchema:  in.raw,
		OutputSchema: outSchema,
		Handler: func(ctx context.Context, req *core.Request) (*core.Result, error) {
			var input In
			if err := in.decode(req.Arguments, &input); err != nil {
				return nil, err
			}
			out, err := h(ctx, input)
			if err != nil {
				return nil, err
			}
			return pack(out)
		},
	}, nil
}

func ptr[T any](v T) *T { return &v }

// Title sets a human-readable title.
func Title(t string) ToolOption { return func(c *core.Tool) { c.Title = t } }

// ReadOnly marks the tool as not modifying its environment.
func ReadOnly() ToolOption { return func(c *core.Tool) { c.Annotations.ReadOnly = ptr(true) } }

// Destructive marks the tool as possibly performing destructive updates.
func Destructive() ToolOption { return func(c *core.Tool) { c.Annotations.Destructive = ptr(true) } }

// Idempotent marks repeated calls with the same arguments as having no extra effect.
func Idempotent() ToolOption { return func(c *core.Tool) { c.Annotations.Idempotent = ptr(true) } }

// OpenWorld marks the tool as interacting with external entities.
func OpenWorld() ToolOption { return func(c *core.Tool) { c.Annotations.OpenWorld = ptr(true) } }

// Scopes requires the caller to hold all of the given scopes (HTTP + auth only).
func Scopes(scopes ...string) ToolOption {
	return func(c *core.Tool) { c.Scopes = append(c.Scopes, scopes...) }
}

// ToolTimeout overrides the server default timeout for this tool.
func ToolTimeout(d time.Duration) ToolOption { return func(c *core.Tool) { c.Timeout = d } }

// ToolMaxOutput overrides the server default output limit for this tool.
func ToolMaxOutput(n int) ToolOption { return func(c *core.Tool) { c.MaxOutput = n } }

// With adds middleware that applies to this tool only.
func With(mws ...Middleware) ToolOption {
	return func(c *core.Tool) { c.Middleware = append(c.Middleware, mws...) }
}
