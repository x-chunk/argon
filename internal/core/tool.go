// Package core holds Argon's transport-independent machinery: the tool
// registry, handler types and the middleware chain. It knows nothing about
// the MCP SDK; internal/sdkadapter translates between the two.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotImplemented marks skeleton code paths that are not written yet.
var ErrNotImplemented = errors.New("argon: not implemented")

// Request is a single tool invocation as seen by the core.
type Request struct {
	// Tool is the name of the invoked tool.
	Tool string
	// Arguments are the raw JSON arguments sent by the client.
	Arguments json.RawMessage
}

// Content is one content block of a tool result.
// Only text is modelled for now; more kinds are added as needed.
type Content struct {
	Text string
}

// Result is the outcome of a tool call.
type Result struct {
	Content []Content
	// Structured is the structured output (serialized as structuredContent).
	Structured any
	// IsError marks a tool-level error (not a protocol failure).
	IsError bool
}

// Handler executes a tool call.
type Handler func(ctx context.Context, req *Request) (*Result, error)

// Annotations are behavioural hints exposed to clients.
// Nil means "unspecified".
type Annotations struct {
	ReadOnly    *bool
	Destructive *bool
	Idempotent  *bool
	OpenWorld   *bool
}

// Tool is a registered tool with everything needed to expose and run it.
type Tool struct {
	Name         string
	Title        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage // nil when the tool has no structured output
	Annotations  Annotations

	// Per-tool policy. Zero values mean "use server defaults".
	Scopes    []string
	Timeout   time.Duration
	MaxOutput int

	Handler    Handler
	Middleware []Middleware
}
