package argon

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

// stringArgs decodes string-valued arguments — URI template variables and
// prompt arguments — into a struct. Values are converted to the field types
// from the struct's JSON Schema (integer, number, boolean, otherwise string)
// and then validated and decoded like tool input, so unknown names, missing
// required fields and bad values are rejected.
type stringArgs struct {
	*schema
	order []string           // property names in struct field order
	props map[string]argProp // property name → metadata
}

type argProp struct {
	typ         string // JSON Schema type
	description string
	required    bool
}

func stringArgsSchema[T any]() (*stringArgs, error) {
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		return nil, fmt.Errorf("argument type %s must be a struct", reflect.TypeFor[T]())
	}
	sc, err := schemaFor[T]()
	if err != nil {
		return nil, err
	}
	root := sc.resolved.Schema()
	sa := &stringArgs{schema: sc, props: make(map[string]argProp, len(root.Properties))}
	for name, p := range root.Properties {
		sa.props[name] = argProp{
			typ:         p.Type,
			description: p.Description,
			required:    slices.Contains(root.Required, name),
		}
	}
	// Struct field order first, then anything else (sorted) for determinism.
	for _, name := range root.PropertyOrder {
		if _, ok := sa.props[name]; ok && !slices.Contains(sa.order, name) {
			sa.order = append(sa.order, name)
		}
	}
	var rest []string
	for name := range sa.props {
		if !slices.Contains(sa.order, name) {
			rest = append(rest, name)
		}
	}
	slices.Sort(rest)
	sa.order = append(sa.order, rest...)
	return sa, nil
}

// has reports whether the struct has a property called name.
func (sa *stringArgs) has(name string) bool {
	_, ok := sa.props[name]
	return ok
}

// decodeStrings converts values to the property types, then validates and
// decodes them into dst.
func (sa *stringArgs) decodeStrings(values map[string]string, dst any) error {
	m := make(map[string]any, len(values))
	for k, v := range values {
		val, err := convertString(v, sa.props[k].typ)
		if err != nil {
			return fmt.Errorf("%q: %w", k, err)
		}
		m[k] = val
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return sa.decode(b, dst)
}

func convertString(v, typ string) (any, error) {
	switch typ {
	case "integer":
		return strconv.ParseInt(v, 10, 64)
	case "number":
		return strconv.ParseFloat(v, 64)
	case "boolean":
		return strconv.ParseBool(v)
	}
	return v, nil
}
