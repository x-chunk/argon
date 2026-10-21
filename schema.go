package argon

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// schema is a JSON Schema inferred from a Go type, kept both in wire form
// (for tools/list) and resolved form (for validation).
type schema struct {
	raw      json.RawMessage
	resolved *jsonschema.Resolved
}

func schemaFor[T any]() (*schema, error) {
	sc, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(sc)
	if err != nil {
		return nil, err
	}
	res, err := sc.Resolve(nil)
	if err != nil {
		return nil, err
	}
	return &schema{raw: raw, resolved: res}, nil
}

// decode validates args against the schema, applies defaults and decodes
// the result into dst. Missing arguments are treated as an empty object.
func (s *schema) decode(args json.RawMessage, dst any) error {
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if err := s.resolved.ApplyDefaults(&m); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if err := s.resolved.Validate(m); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}
