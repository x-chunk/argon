package argon

import (
	"encoding/json"
	"fmt"
	"reflect"
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
//   - anything else becomes its JSON as text; it is also sent as structured
//     content when structured is set, i.e. when the tool declares an output
//     schema (struct outputs). Older spec revisions require structured
//     content to be an object, so slices and scalars are text-only.
//
// Nil slices and maps are encoded as [] and {} rather than null.
func pack(out any, structured bool) (*Result, error) {
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
	if string(b) == "null" {
		switch reflect.ValueOf(out).Kind() {
		case reflect.Slice:
			b = []byte("[]")
		case reflect.Map:
			b = []byte("{}")
		}
	}
	res := &Result{Content: []Content{TextContent{Text: string(b)}}}
	if structured {
		res.Structured = out
	}
	return res, nil
}
