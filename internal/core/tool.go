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
	// Meta describes the invoked tool with effective policies (server
	// defaults applied). Set by Build; middleware must treat it as read-only.
	Meta *ToolMeta
}

// ToolMeta is the read-only view of a tool that middleware can inspect.
type ToolMeta struct {
	Name        string
	Title       string
	Annotations Annotations
	Scopes      []string
	Timeout     time.Duration
	MaxOutput   int
}

// Content is one content block of a tool result: TextContent, ImageContent
// or ResourceLink. The set is closed; new kinds are added here and in the
// SDK adapter together.
type Content interface {
	// size is the block's contribution to MaxOutput accounting.
	size() int
}

// TextContent is a plain-text block.
type TextContent struct {
	Text string
}

// ImageContent is an image. Data holds the raw bytes (not base64).
type ImageContent struct {
	MIMEType string
	Data     []byte
}

// ResourceLink points the client at a resource it can fetch separately.
type ResourceLink struct {
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string
}

func (c TextContent) size() int  { return len(c.Text) }
func (c ImageContent) size() int { return len(c.Data) }
func (c ResourceLink) size() int {
	return len(c.URI) + len(c.Name) + len(c.Title) + len(c.Description) + len(c.MIMEType)
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
