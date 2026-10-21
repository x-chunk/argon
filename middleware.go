package argon

import "go.xchunk.org/argon/internal/core"

// Low-level types for writing middleware.
type (
	// Request is a single tool invocation.
	Request = core.Request
	// Result is the outcome of a tool call.
	Result = core.Result
	// Content is one content block of a Result: TextContent, ImageContent
	// or ResourceLink.
	Content = core.Content
	// TextContent is a plain-text block.
	TextContent = core.TextContent
	// ImageContent is an image; Data holds raw bytes.
	ImageContent = core.ImageContent
	// ResourceLink points the client at a resource it can fetch separately.
	ResourceLink = core.ResourceLink
	// Handler executes a tool call.
	Handler = core.Handler
	// Middleware wraps a Handler with cross-cutting behaviour.
	Middleware = core.Middleware
	// ToolMeta describes the invoked tool; available to middleware as
	// Request.Meta.
	ToolMeta = core.ToolMeta
	// Annotations are behavioural hints about a tool.
	Annotations = core.Annotations
)
