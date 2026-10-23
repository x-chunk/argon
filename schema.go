package argon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
)

// schema is a JSON Schema inferred from a Go type, kept both in wire form
// (for tools/list) and resolved form (for validation).
type schema struct {
	raw      json.RawMessage
	resolved *jsonschema.Resolved
}

func schemaFor[T any]() (*schema, error) {
	return schemaForType(reflect.TypeFor[T]())
}

func schemaForType(t reflect.Type) (*schema, error) {
	sc, err := jsonschema.ForType(t, nil)
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
// Errors wrap ErrInvalidArguments.
//
// The validator needs float64 numbers, but decoding into dst from those
// would round integers beyond 2^53. So dst is decoded from a copy that keeps
// numbers as json.Number, with only the applied defaults merged in.
func (s *schema) decode(args json.RawMessage, dst any) error {
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	if err := s.resolved.ApplyDefaults(&m); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	if err := s.resolved.Validate(m); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	var exact map[string]any
	d := json.NewDecoder(bytes.NewReader(args))
	d.UseNumber()
	if err := d.Decode(&exact); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	if exact == nil {
		exact = map[string]any{}
	}
	mergeMissing(exact, m)
	b, err := json.Marshal(exact)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	return nil
}

// mergeMissing copies into dst the entries of src it lacks, recursing into
// objects present in both. It carries applied defaults over to dst.
func mergeMissing(dst, src map[string]any) {
	for k, v := range src {
		cur, ok := dst[k]
		if !ok {
			dst[k] = v
			continue
		}
		if dm, ok := cur.(map[string]any); ok {
			if sm, ok := v.(map[string]any); ok {
				mergeMissing(dm, sm)
			}
		}
	}
}
