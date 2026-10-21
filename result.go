package argon

import (
	"encoding/json"
	"fmt"

	"go.xchunk.org/argon/internal/core"
)

// Text returns a result consisting of a single text block.
func Text(s string) *Result { return NewResult(TextContent{Text: s}) }

// Image returns a result consisting of a single image.
func Image(mimeType string, data []byte) *Result {
	return NewResult(ImageContent{MIMEType: mimeType, Data: data})
}

// NewResult returns a result made of the given content blocks, e.g. text
// followed by an image and a resource link.
func NewResult(blocks ...Content) *Result { return &Result{Content: blocks} }

// pack converts a handler's typed output into a Result:
//   - *Result is returned as is;
//   - string becomes a text block;
//   - anything else becomes structured content plus its JSON as text.
func pack(out any) (*core.Result, error) {
	switch v := out.(type) {
	case *Result:
		return v, nil
	case string:
		return Text(v), nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("argon: encode output: %w", err)
	}
	return &Result{Content: []Content{TextContent{Text: string(b)}}, Structured: out}, nil
}
