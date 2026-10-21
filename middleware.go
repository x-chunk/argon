package argon

import "go.xchunk.org/argon/internal/core"

// Low-level types for writing middleware.
type (
	// Request is a single tool invocation.
	Request = core.Request
	// Result is the outcome of a tool call.
	Result = core.Result
	// Content is one content block of a Result.
	Content = core.Content
	// Handler executes a tool call.
	Handler = core.Handler
	// Middleware wraps a Handler with cross-cutting behaviour.
	Middleware = core.Middleware
)
