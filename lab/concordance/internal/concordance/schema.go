// Package concordance is the instrument for one question: where a third party
// claims something about a subject we also hold a claim about, do we agree?
//
// It is deliberately small, and deliberately *not* part of lab/survey. The
// survey's subjects are callers; a marketplace's subjects are servers. They
// share no subjects, and they must be able to disagree: a bug in one checker
// must not be able to make the other lie, which is why the generic machinery is
// re-derived here rather than shared.
//
// The two rules that keep this from becoming a copy of someone else's catalogue:
//
//   - a record may not exist without our own claim, anchored by
//     `our_claim.basis` to a file in this repo; and
//   - a third party's claim is stored as ONE normalised token, never prose.
//
// Agreement is computed, never authored.
package concordance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema is a compiled schema plus the shape facts the projector needs.
type Schema struct {
	Name  string
	Raw   []byte
	Order []string
	Props map[string]Prop
	js    *jsonschema.Schema
}

// Prop is the part of a property's schema the tool consults.
type Prop struct {
	Type     string
	Nullable bool
}

// Dataset is an opened concordance: the module root, the repo root it reaches
// out to for `basis`, the compiled schemas, the declared sources and the
// records on disk.
type Dataset struct {
	Root     string // the module root (holds records/, sources/, schemas/)
	RepoRoot string // the repo root, for resolving our_claim.basis
	Record   *Schema
	Flat     *Schema
	Sources  map[string]Source
	Records  []Record
	problems []string
}

// Open loads the schemas, the declared sources and the records.
func Open(root, repoRoot string) (*Dataset, error) {
	rec, err := loadSchema(filepath.Join(root, "schemas", "record.json"))
	if err != nil {
		return nil, err
	}
	flat, err := loadSchema(filepath.Join(root, "schemas", "flat.json"))
	if err != nil {
		return nil, err
	}
	d := &Dataset{Root: root, RepoRoot: repoRoot, Record: rec, Flat: flat}
	if err := d.loadSources(); err != nil {
		return nil, err
	}
	if err := d.loadRecords(); err != nil {
		return nil, err
	}
	return d, nil
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
	return &Schema{Name: filepath.Base(path), Raw: raw, Order: order, Props: props, js: sch}, nil
}

func mustUnmarshal(raw []byte) any {
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	return v
}

// unmarshal parses JSON the way the validator expects it (numbers as
// json.Number), so a Go value round-tripped through it validates identically to
// one read off disk.
func unmarshal(raw []byte) (any, error) {
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// Validate checks an instance (which must have come from jsonschema.UnmarshalJSON).
func (s *Schema) Validate(v any) error {
	if s == nil || s.js == nil {
		return fmt.Errorf("schema %s not compiled", s.Name)
	}
	return s.js.Validate(v)
}

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
	return out
}

// propertyNames returns the keys of the top-level "properties" object in
// document order — the order is the CSV header, and encoding/json's map decode
// would lose it.
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
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return keys, nil
}

func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if d != '{' && d != '[' {
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
