// Package survey is the model survey's accuracy tool.
//
// The survey observes what agents want from an integration endpoint. The
// observations are the profiles under profiles/<agent_id>.json; everything a
// reader sees — models.csv, the README summary, survey.feature — is a
// deterministic projection of them. This package is the only writer, and Check
// proves none of the projections drifted.
//
// Projection rules, the schema version, and exactly what is and is not checked
// live in README.md beside this directory.
package survey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema is one compiled JSON Schema plus the shape facts the projector needs:
// the property order (JSON object order matters — it is the CSV header), and a
// per-property type summary.
type Schema struct {
	Name  string
	Raw   []byte
	Order []string
	Props map[string]Prop
	js    *jsonschema.Schema
}

// Prop is the part of a property's schema the tool consults.
type Prop struct {
	Type     string // "string" | "boolean" | "integer"
	Nullable bool   // the declared type union includes "null"
	Enum     []string
}

// Schemas are the two schemas the survey is defined by.
type Schemas struct {
	Flat   *Schema
	Nested *Schema
}

// LoadSchemas reads schemas/flat.json and schemas/nested.json from root.
//
// The schemas are read from the survey root rather than embedded so that a
// broken fixture can overlay one: "header drifts from the schema" and "bad
// enum" are checks against these files, and a check whose input cannot be
// corrupted cannot be tested.
func LoadSchemas(root string) (*Schemas, error) {
	flat, err := loadSchema(filepath.Join(root, "schemas", "flat.json"))
	if err != nil {
		return nil, err
	}
	nested, err := loadSchema(filepath.Join(root, "schemas", "nested.json"))
	if err != nil {
		return nil, err
	}
	return &Schemas{Flat: flat, Nested: nested}, nil
}

func loadSchema(path string) (*Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	order, err := propertyNames(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var doc struct {
		Properties map[string]map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	props := make(map[string]Prop, len(order))
	for _, name := range order {
		p, ok := doc.Properties[name]
		if !ok {
			return nil, fmt.Errorf("%s: property %q missing from properties", path, name)
		}
		props[name] = prop(p)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(path, mustUnmarshal(raw)); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	sch, err := c.Compile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Schema{
		Name:  filepath.Base(path),
		Raw:   raw,
		Order: order,
		Props: props,
		js:    sch,
	}, nil
}

func mustUnmarshal(raw []byte) any {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		panic(err) // a schema we just parsed as JSON cannot fail the second parse
	}
	return v
}

// Validate checks an instance against the schema. instance must have come from
// jsonschema.UnmarshalJSON (or an equivalent JSON round trip) so its Go types
// are the ones the validator recognises.
func (s *Schema) Validate(v any) error {
	if s == nil || s.js == nil {
		return fmt.Errorf("schema %s not compiled", s.Name)
	}
	return s.js.Validate(v)
}

// prop summarises a property schema: its scalar type, whether it may be null,
// and the enum values when it declares them.
func prop(p map[string]any) Prop {
	out := Prop{}
	switch t := p["type"].(type) {
	case string:
		out.Type = t
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				if s == "null" {
					out.Nullable = true
				} else if out.Type == "" {
					out.Type = s
				}
			}
		}
	}
	if _, ok := p["const"]; ok {
		out.Type = "integer"
	}
	if vals, ok := p["enum"].([]any); ok {
		for _, e := range vals {
			if e == nil {
				out.Nullable = true
				continue
			}
			if s, ok := e.(string); ok {
				out.Enum = append(out.Enum, s)
			}
		}
	}
	if vals, ok := p["x-list-enum"].([]any); ok {
		for _, e := range vals {
			if s, ok := e.(string); ok {
				out.Enum = append(out.Enum, s)
			}
		}
	}
	return out
}

// propertyNames returns the keys of the top-level "properties" object in
// document order. encoding/json's map decode would lose that order, and the
// order is the CSV header.
func propertyNames(raw []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("root is not a JSON object")
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if keyTok == "properties" {
			return readObjectKeys(dec)
		}
		if err := skipValue(dec); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf(`no "properties" object`)
}

func readObjectKeys(dec *json.Decoder) ([]string, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf(`"properties" is not a JSON object`)
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			return nil, err
		}
		s, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("object key is not a string")
		}
		keys = append(keys, s)
		if err := skipValue(dec); err != nil {
			return nil, err
		}
	}
	if _, err := dec.Token(); err != nil { // consume '}'
		return nil, err
	}
	return keys, nil
}

// skipValue consumes exactly one JSON value from dec.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{', '[':
	default:
		return nil
	}
	depth := 1
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if dd, ok := t.(json.Delim); ok {
			switch dd {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

// joinPath renders a dotted path for messages.
func joinPath(parts ...string) string { return strings.Join(parts, ".") }
