package argon

import (
	"context"
	"encoding/json"
	"time"
)

// Request is a single tool call as seen by middleware (see Middleware).
// Typed handlers registered with Tool never see it: they receive their
// decoded input instead.
type Request struct {
	// Tool is the name of the invoked tool.
	Tool string
	// Arguments are the raw JSON arguments sent by the client, before
	// validation and decoding.
	Arguments json.RawMessage
	// Meta describes the invoked tool with its effective policies (server
	// defaults applied). Middleware must treat it as read-only.
	Meta *ToolMeta
}

// ToolMeta describes the invoked tool to middleware, e.g. to log it or to
// gate tools marked Destructive.
type ToolMeta struct {
	Name        string
	Title       string
	Annotations Annotations
	// Scopes the caller must hold (see Scopes).
	Scopes []string
	// Timeout and MaxOutput are the effective limits for this call; zero
	// means disabled.
	Timeout   time.Duration
	MaxOutput int
}

// Content is one content block of a tool result or prompt message:
// TextContent, ImageContent or ResourceLink. The set is closed; it cannot be
// implemented outside this package.
type Content interface {
	// size is the block's contribution to MaxOutput accounting.
	size() int
}

// TextContent is a plain-text block.
type TextContent struct {
	Text string
}

// ImageContent is an image, such as a chart or a screenshot.
type ImageContent struct {
	// MIMEType is the image type, e.g. "image/png".
	MIMEType string
	// Data holds the raw image bytes; Argon base64-encodes them on the wire.
	Data []byte
}

// ResourceLink points the client at a resource it can read separately
// (see Resource), instead of embedding the content.
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

// Result is the outcome of a tool call. Typed handlers may return a *Result
// to send several content blocks (see NewResult); middleware sees every
// call's Result.
type Result struct {
	Content []Content
	// Structured is the structured output, sent as structuredContent. It is
	// set for tools whose output type is a struct.
	Structured any
	// IsError marks a tool-level error the model can react to, as opposed to
	// a protocol failure.
	IsError bool
}

// Handler executes a tool call at the middleware level (see Middleware).
// A returned error becomes an isError result.
type Handler func(ctx context.Context, req *Request) (*Result, error)

// Annotations are behaviour hints about a tool, exposed to clients and to
// middleware via ToolMeta. Nil means "unspecified". Set them with ReadOnly,
// Destructive, Idempotent and OpenWorld.
type Annotations struct {
	ReadOnly    *bool
	Destructive *bool
	Idempotent  *bool
	OpenWorld   *bool
}

// toolDef is a registered tool with everything needed to expose and run it.
type toolDef struct {
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
