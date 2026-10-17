package concordance_test

// The broken fixtures.
//
// Every check in the contract has one fixture that breaks exactly it, and the
// test asserts (a) the run fails and (b) the failure says why.
//
// The real dataset ships with NO records — a record may only exist where we hold
// our own claim, and no claim of a third party's has been read (see README.md). So a
// fixture supplies its own record: it copies fixtures/base/... into records/ and
// mutates it. fixtures/base is not under fixtures/broken/, so the glob below
// does not pick it up as a case, and a synthetic record never sits in records/
// where it could be mistaken for an observation.

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/james-gibson/lab-safety/concordance/internal/concordance"
)

// moduleRoot is the concordance root: the directory holding this test.
const moduleRoot = "."

const baseRecord = "fixtures/base/records/mcpregistry/acme-weather/exposes-tool.json"

type brokenCase struct {
	Args []string `json:"args"`
	Want string   `json:"want"`
	// Render regenerates the artifacts after Ops and before Post. Default true;
	// a fixture that breaks a generated artifact sets it false (or uses Post).
	Render *bool `json:"render"`
	Ops    []op  `json:"ops"`
	Post   []op  `json:"post"`
}

type op struct {
	Kind   string            `json:"kind"`
	File   string            `json:"file"`
	From   string            `json:"from"`
	To     string            `json:"to"`
	Path   string            `json:"path"`
	Row    string            `json:"row"`
	Column string            `json:"column"`
	Find   string            `json:"find"`
	Text   string            `json:"text"`
	Value  json.RawMessage   `json:"value"`
	Set    map[string]string `json:"set"`
}

func TestTheRealConcordancePasses(t *testing.T) {
	d, err := concordance.Open(moduleRoot, "../..")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	fails, err := d.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(fails) > 0 {
		t.Fatalf("the committed concordance fails its own check:\n%s", strings.Join(fails, "\n"))
	}
}

// A disagreement is representable and is not a failure: the two claims differ,
// `agrees` is false, and --check is still green. That is the whole point of the
// field — so it needs a test of its own, not a broken fixture.
func TestADisagreementIsRepresentable(t *testing.T) {
	tmp, root := materialize(t)
	_ = tmp
	if err := apply(root, op{Kind: "copy", From: baseRecord, To: "records/mcpregistry/acme-weather/exposes-tool.json"}); err != nil {
		t.Fatal(err)
	}
	if err := apply(root, op{Kind: "jsonSet", File: "records/mcpregistry/acme-weather/exposes-tool.json",
		Path: "our_claim.value", Value: json.RawMessage(`"weather.put"`)}); err != nil {
		t.Fatal(err)
	}
	d, err := concordance.Open(root, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if fails, err := d.Render(); err != nil || len(fails) > 0 {
		t.Fatalf("render: %v %v", fails, err)
	}
	fails, err := d.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(fails) > 0 {
		t.Fatalf("a disagreement should not fail the check:\n%s", strings.Join(fails, "\n"))
	}
	raw, err := os.ReadFile(filepath.Join(root, concordance.CSVPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "weather.get,weather.put") {
		t.Fatalf("the projection does not show the two differing claims:\n%s", raw)
	}
	if !strings.Contains(string(raw), "false") {
		t.Fatalf("agrees should be false in the projection:\n%s", raw)
	}
}

func TestBrokenFixtures(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("fixtures", "broken", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("no fixtures/broken/* fixtures found")
	}
	sort.Strings(dirs)

	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "case.json"))
			if err != nil {
				t.Fatal(err)
			}
			var c brokenCase
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			if c.Want == "" {
				t.Fatal("fixture has no \"want\" — a fixture that cannot say why it fails is not a test")
			}
			render := c.Render == nil || *c.Render

			tmp, root := materialize(t)
			for _, o := range c.Ops {
				if err := apply(root, o); err != nil {
					t.Fatalf("applying %s: %v", o.Kind, err)
				}
			}
			if render {
				d, err := concordance.Open(root, tmp)
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				if _, err := d.Render(); err != nil {
					t.Fatalf("render: %v", err)
				}
			}
			for _, o := range c.Post {
				if err := apply(root, o); err != nil {
					t.Fatalf("applying post %s: %v", o.Kind, err)
				}
			}

			d, err := concordance.Open(root, tmp)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			args := c.Args
			if len(args) == 0 {
				args = []string{"--check"}
			}
			var fails []string
			switch args[0] {
			case "--check":
				fails, err = d.Check()
			case "--stale":
				as := time.Now()
				if len(args) > 2 {
					as, err = time.Parse("2006-01-02", args[2])
					if err != nil {
						t.Fatalf("bad --as-of in fixture: %v", err)
					}
				}
				fails, err = d.Stale(as)
			default:
				t.Fatalf("unknown fixture args %v", args)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(fails) == 0 {
				t.Fatalf("%v reported no failure; the fixture was supposed to break %q", args, c.Want)
			}
			if !anyContains(fails, c.Want) {
				t.Fatalf("no failure mentions %q; failures were:\n%s", c.Want, strings.Join(fails, "\n"))
			}
		})
	}
}

func anyContains(fails []string, want string) bool {
	for _, f := range fails {
		if strings.Contains(f, want) {
			return true
		}
	}
	return false
}

// materialize builds a temp tree that mirrors the repo's shape, so
// `our_claim.basis` (which is repo-relative) resolves:
//
//	<tmp>/lab/concordance/...   the module, copied
//	<tmp>/sheets/...            our artefacts the basis points at
func materialize(t *testing.T) (tmp, root string) {
	t.Helper()
	tmp = t.TempDir()
	root = filepath.Join(tmp, "lab", "concordance")
	if err := copyTree(moduleRoot, root); err != nil {
		t.Fatal(err)
	}
	if err := copyTree("../../sheets", filepath.Join(tmp, "sheets")); err != nil {
		t.Fatal(err)
	}
	return tmp, root
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// apply performs one mutation on the materialised root.
func apply(root string, o op) error {
	switch o.Kind {
	case "copy":
		body, err := os.ReadFile(filepath.Join(root, o.From))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, o.To)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, o.To), body, 0o644)

	case "jsonSet", "jsonDelete":
		path := filepath.Join(root, o.File)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		parts := strings.Split(o.Path, ".")
		cur := doc
		for _, k := range parts[:len(parts)-1] {
			next, ok := cur[k].(map[string]any)
			if !ok {
				return fmt.Errorf("%s: %s is not an object", o.File, k)
			}
			cur = next
		}
		last := parts[len(parts)-1]
		if o.Kind == "jsonDelete" {
			delete(cur, last)
		} else {
			var v any
			if err := json.Unmarshal(o.Value, &v); err != nil {
				return fmt.Errorf("value: %w", err)
			}
			cur[last] = v
		}
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(out, '\n'), 0o644)

	case "rawReplace":
		path := filepath.Join(root, o.File)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(raw, []byte(o.Find)) {
			return fmt.Errorf("%s: %q not found", o.File, o.Find)
		}
		return os.WriteFile(path, bytes.ReplaceAll(raw, []byte(o.Find), []byte(o.Text)), 0o644)

	case "append":
		path := filepath.Join(root, o.File)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(o.Text)
		return err

	case "csvSetCell":
		path := filepath.Join(root, o.File)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		r := csv.NewReader(bytes.NewReader(raw))
		r.FieldsPerRecord = -1
		records, err := r.ReadAll()
		if err != nil {
			return err
		}
		if len(records) < 2 {
			return fmt.Errorf("%s has no data rows to edit", o.File)
		}
		header := records[0]
		colIndex := func(name string) (int, error) {
			for i, c := range header {
				if c == name {
					return i, nil
				}
			}
			return 0, fmt.Errorf("%s: no column %q", o.File, name)
		}
		idIdx, err := colIndex("subject_id")
		if err != nil {
			return err
		}
		ci, err := colIndex(o.Column)
		if err != nil {
			return err
		}
		for _, rec := range records[1:] {
			if rec[idIdx] == o.Row {
				rec[ci] = o.Text
			}
		}
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if err := w.WriteAll(records); err != nil {
			return err
		}
		w.Flush()
		return os.WriteFile(path, buf.Bytes(), 0o644)

	default:
		return fmt.Errorf("unknown op %q", o.Kind)
	}
}
