package survey_test

// The broken fixtures.
//
// Every check in the accuracy contract has one fixture that breaks exactly it,
// and the test asserts (a) the run fails and (b) the failure says why. A check
// whose input cannot be corrupted cannot be tested, so a fixture is a *mutation
// of the real survey* rather than a hand-copied tree: fixtures/broken/<check>/
// case.json names the one thing to break, and the test materialises the real
// root, applies it, and runs the check.

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	survey "github.com/james-gibson/lab-safety/survey/internal/survey"
)

// root is the survey root: the directory holding this test.
const root = "."

type brokenCase struct {
	Args     []string `json:"args"`
	Want     string   `json:"want"`
	IDColumn string   `json:"id_column"`
	Ops      []op     `json:"ops"`
}

type surveyCase struct {
	name     string
	root     string
	ds       *survey.Dataset
	fixtures string
}

func datasets() []surveyCase {
	return []surveyCase{
		{"model", ".", survey.Model(), filepath.Join("fixtures", "broken")},
		{"tool", "tool", survey.Tool(), filepath.Join("tool", "fixtures", "broken")},
	}
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

func TestTheRealSurveyPasses(t *testing.T) {
	for _, d := range datasets() {
		d := d
		t.Run(d.name, func(t *testing.T) {
			fails, err := survey.CheckDataset(d.ds, d.root)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if len(fails) > 0 {
				t.Fatalf("the committed %s survey fails its own check:\n%s", d.name, strings.Join(fails, "\n"))
			}
		})
	}
}

func TestBrokenFixtures(t *testing.T) {
	for _, d := range datasets() {
		d := d
		t.Run(d.name, func(t *testing.T) {
			dirs, err := filepath.Glob(filepath.Join(d.fixtures, "*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(dirs) == 0 {
				t.Fatalf("no %s fixtures found", d.fixtures)
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

					tmp := materialize(t, d.ds, d.root)
					idCol := c.IDColumn
					if idCol == "" {
						idCol = d.ds.IDField
					}
					for _, o := range c.Ops {
						if err := apply(tmp, o, idCol); err != nil {
							t.Fatalf("applying %s: %v", o.Kind, err)
						}
					}

					args := c.Args
					if len(args) == 0 {
						args = []string{"--check"}
					}
					var fails []string
					switch args[0] {
					case "--check":
						fails, err = survey.CheckDataset(d.ds, tmp)
					case "--stale":
						as := time.Now()
						if len(args) > 2 {
							as, err = time.Parse("2006-01-02", args[2])
							if err != nil {
								t.Fatalf("bad --as-of in fixture: %v", err)
							}
						}
						fails, err = survey.StaleDataset(d.ds, tmp, as)
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

// materialize copies the real survey into a temp dir so a fixture can break a
// copy without touching the working tree.
func materialize(t *testing.T, ds *survey.Dataset, root string) string {
	t.Helper()
	tmp := t.TempDir()
	for _, f := range []string{ds.CSVPath, ds.FeaturePath, ds.ReadmePath} {
		copyFile(t, filepath.Join(root, f), filepath.Join(tmp, f))
	}
	for _, d := range []string{"profiles", "schemas", "sources"} {
		copyDir(t, filepath.Join(root, d), filepath.Join(tmp, d))
	}
	return tmp
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		copyFile(t, filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
	}
}

// apply performs one mutation on the materialised root.
func apply(tmp string, o op, idColumn string) error {
	switch o.Kind {
	case "copy":
		body, err := os.ReadFile(filepath.Join(tmp, o.From))
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(tmp, o.To), body, 0o644)

	case "jsonSet", "jsonDelete":
		path := filepath.Join(tmp, o.File)
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
		path := filepath.Join(tmp, o.File)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(raw, []byte(o.Find)) {
			return fmt.Errorf("%s: %q not found", o.File, o.Find)
		}
		return os.WriteFile(path, bytes.ReplaceAll(raw, []byte(o.Find), []byte(o.Text)), 0o644)

	case "append":
		path := filepath.Join(tmp, o.File)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(o.Text)
		return err

	case "csvSetCell", "csvDuplicateRow":
		path := filepath.Join(tmp, o.File)
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
		if len(records) == 0 {
			return fmt.Errorf("%s is empty", o.File)
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
		idIdx, err := colIndex(idColumn)
		if err != nil {
			return err
		}
		var target []string
		found := -1
		for i, rec := range records[1:] {
			if rec[idIdx] == o.Row {
				target, found = rec, i+1
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("%s: no row %q", o.File, o.Row)
		}
		if o.Kind == "csvSetCell" {
			ci, err := colIndex(o.Column)
			if err != nil {
				return err
			}
			target[ci] = o.Text
		} else {
			dup := append([]string(nil), target...)
			for col, val := range o.Set {
				ci, err := colIndex(col)
				if err != nil {
					return err
				}
				dup[ci] = val
			}
			records = append(records, dup)
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
