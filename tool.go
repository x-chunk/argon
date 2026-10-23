package argon

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// ToolOption configures a single tool.
type ToolOption func(*toolDef)

// Tool registers a typed tool on s.
//
// In must be a struct, a pointer to a struct or a map with string keys; its
// JSON Schema is inferred from json and jsonschema struct tags, and input is
// validated against it. Other input types are reported as registration
// errors. Out may be a
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

func newTool[In, Out any](name, description string, h func(context.Context, In) (Out, error)) (*toolDef, error) {
	in, err := inputSchema(reflect.TypeFor[In]())
	if err != nil {
		return nil, fmt.Errorf("argon: tool %q input schema: %w", name, err)
	}
	var outSchema json.RawMessage
	if elem, ok := structuredOutput(reflect.TypeFor[Out]()); ok {
		out, err := schemaForType(elem)
		if err != nil {
			return nil, fmt.Errorf("argon: tool %q output schema: %w", name, err)
		}
		outSchema = out.raw
	}
	return &toolDef{
		Name:         name,
		Description:  description,
		InputSchema:  in.raw,
		OutputSchema: outSchema,
		Handler: func(ctx context.Context, req *Request) (*Result, error) {
			var input In
			if err := in.decode(req.Arguments, &input); err != nil {
				return nil, err
			}
			out, err := h(ctx, input)
			if err != nil {
				return nil, err
			}
			if outSchema != nil && isNilPointer(out) {
				// A declared output schema obliges a structured result.
				return nil, fmt.Errorf("tool %q returned a nil result", name)
			}
			return pack(out, outSchema != nil)
		},
	}, nil
}

// inputSchema infers the schema for a tool input type. MCP requires an
// object schema, so a pointer to a struct is described by its element and
// anything that is not an object is rejected here rather than by the SDK at
// startup.
func inputSchema(t reflect.Type) (*schema, error) {
	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct {
		t = t.Elem()
	}
	sc, err := schemaForType(t)
	if err != nil {
		return nil, err
	}
	if typ := sc.resolved.Schema().Type; typ != "object" {
		return nil, fmt.Errorf("input type %s must be a struct, a pointer to a struct or a map (schema type %q)", t, typ)
	}
	return sc, nil
}

// structuredOutput reports whether a tool with output type t produces
// structured content, and the struct type its output schema describes:
// structs and pointers to structs do, except *Result, which carries its own
// content.
func structuredOutput(t reflect.Type) (reflect.Type, bool) {
	switch {
	case t.Kind() == reflect.Struct:
		return t, true
	case t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct && t != reflect.TypeFor[*Result]():
		return t.Elem(), true
	}
	return nil, false
}

func isNilPointer(v any) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

func ptr[T any](v T) *T { return &v }

// Title sets a human-readable title.
func Title(t string) ToolOption { return func(c *toolDef) { c.Title = t } }

// ReadOnly marks the tool as not modifying its environment.
func ReadOnly() ToolOption { return func(c *toolDef) { c.Annotations.ReadOnly = ptr(true) } }

// Destructive marks the tool as possibly performing destructive updates.
func Destructive() ToolOption { return func(c *toolDef) { c.Annotations.Destructive = ptr(true) } }

// Idempotent marks repeated calls with the same arguments as having no extra effect.
func Idempotent() ToolOption { return func(c *toolDef) { c.Annotations.Idempotent = ptr(true) } }

// OpenWorld marks the tool as interacting with external entities.
func OpenWorld() ToolOption { return func(c *toolDef) { c.Annotations.OpenWorld = ptr(true) } }

// Scopes requires the caller to hold all of the given scopes. Scopes come
// from the verified token, so without authentication (stdio, or HTTP
// without Auth) calls are always rejected.
func Scopes(scopes ...string) ToolOption {
	return func(c *toolDef) { c.Scopes = append(c.Scopes, scopes...) }
}

// ToolTimeout overrides the server default timeout for this tool. Zero
// keeps the default; a negative value disables the timeout.
func ToolTimeout(d time.Duration) ToolOption { return func(c *toolDef) { c.Timeout = d } }

// ToolMaxOutput overrides the server default output limit for this tool.
// Zero keeps the default; a negative value disables the limit.
func ToolMaxOutput(n int) ToolOption { return func(c *toolDef) { c.MaxOutput = n } }

// With adds middleware that applies to this tool only.
func With(mws ...Middleware) ToolOption {
	return func(c *toolDef) { c.Middleware = append(c.Middleware, mws...) }
}
