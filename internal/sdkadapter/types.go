package sdkadapter

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server is an opaque handle to a built SDK server, so callers outside this
// package never name SDK types.
type Server = mcp.Server

// The types below are the adapter's own description of what to expose. They
// are deliberately plain data plus functions: the adapter knows nothing about
// Argon's public API, and the root package converts at the boundary.

// BlockKind identifies the kind of a content block.
type BlockKind int

const (
	BlockText BlockKind = iota
	BlockImage
	BlockResourceLink
)

// Block is one content block: text, an image or a resource link. Only the
// fields of its Kind are meaningful.
type Block struct {
	Kind BlockKind

	Text     string // BlockText
	MIMEType string // BlockImage, BlockResourceLink
	Data     []byte // BlockImage: raw bytes

	URI         string // BlockResourceLink
	Name        string
	Title       string
	Description string
}

// Annotations are tool behaviour hints; nil means unspecified.
type Annotations struct {
	ReadOnly, Destructive, Idempotent, OpenWorld *bool
}

// ToolDef describes a tool for tools/list.
type ToolDef struct {
	Name         string
	Title        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage // nil when the tool has no structured output
	Annotations  Annotations
}

// ToolResult is the outcome of a tool call.
type ToolResult struct {
	Content    []Block
	Structured any
	IsError    bool
}

// Tool is a tool definition plus its call function. A returned error becomes
// an isError result.
type Tool struct {
	Def  ToolDef
	Call func(ctx context.Context, args json.RawMessage) (*ToolResult, error)
}

// ResourceDef describes a resource (URI set) or a resource template
// (URITemplate set). URIs are validated before they reach the adapter.
type ResourceDef struct {
	URI         string
	URITemplate string
	Name        string
	Title       string
	Description string
	MIMEType    string
}

// ResourceData is the content of a resource read.
type ResourceData struct {
	URI      string
	MIMEType string
	Text     string
	Blob     []byte
}

// Resource is a resource definition plus its read function. Read receives
// the requested URI; an error wrapping errs.ResourceNotFound becomes the
// protocol's resource-not-found error.
type Resource struct {
	Def  ResourceDef
	Read func(ctx context.Context, uri string) (*ResourceData, error)
}

// PromptArg describes a prompt argument.
type PromptArg struct {
	Name        string
	Description string
	Required    bool
}

// PromptDef describes a prompt for prompts/list.
type PromptDef struct {
	Name        string
	Title       string
	Description string
	Arguments   []PromptArg
}

// PromptMsg is one rendered prompt message; Role is "user" or "assistant".
type PromptMsg struct {
	Role    string
	Content Block
}

// Prompt is a prompt definition plus its render function. An error wrapping
// errs.InvalidArguments becomes the protocol's invalid-params error.
type Prompt struct {
	Def PromptDef
	Get func(ctx context.Context, args map[string]string) ([]PromptMsg, error)
}
