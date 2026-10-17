package survey

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/james-gibson/isotope"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// oneLine flattens a (possibly multi-line) validation error for a report line.
func oneLine(err error) string {
	return strings.Join(strings.Split(err.Error(), "\n"), " | ")
}

// roundTrip puts a Go value through JSON the way the validator expects it.
func roundTrip(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// Check runs the model survey's accuracy contract.
func Check(root string) ([]string, error) {
	return CheckDataset(Model(), root)
}

// CheckWith is Check against already-loaded schemas (used by tests).
func CheckWith(sc *Schemas, root string) ([]string, error) {
	return checkWith(Model(), sc, root)
}

// CheckDataset runs a dataset's accuracy contract and returns one line per
// failure, sorted. An empty result means the survey is consistent: every
// profile is valid, every projection matches disk, and every derived metric
// recomputes.
func CheckDataset(ds *Dataset, root string) ([]string, error) {
	sc, err := LoadSchemas(root)
	if err != nil {
		return nil, err
	}
	return checkWith(ds, sc, root)
}

func checkWith(ds *Dataset, sc *Schemas, root string) ([]string, error) {
	var fails []string
	add := func(format string, a ...any) { fails = append(fails, fmt.Sprintf(format, a...)) }

	// (1) The flat schema is the answer columns, then the authored provenance,
	// then the computed metrics; and the projector covers exactly the answer
	// columns. If these disagree, every later check is meaningless.
	want := append([]string{ds.IDField}, mappingColumns(ds.Mapping)...)
	if len(sc.Flat.Order) != ds.FlatCount {
		add("schema: %s has %d properties, want %d (answer + 5 provenance + %d derived)",
			sc.Flat.Name, len(sc.Flat.Order), ds.FlatCount, len(ds.DerivedKeys))
	} else if !equalStrings(sc.Flat.Order[:len(want)], want) {
		add("schema: %s properties[:%d] are not [%s] + the projection; the schema and the projector disagree",
			sc.Flat.Name, len(want), ds.IDField)
	}

	// (2) The raw export the profiles were normalised from, and its digest.
	knownSources, err := SourceURLsAt(root, ds.SourceJSON)
	if err != nil {
		add("source: %v", err)
		knownSources = nil
	}
	if err := SourceDigestAt(root, ds.SourceJSON, ds.SourceSHA); err != nil {
		add("source: %v", err)
	}

	// (3) Every profile: strict decode, provenance, nested schema, projection,
	// and the flat schema over the projected row.
	profiles, problems, err := LoadProfilesWith(ds, root)
	if err != nil {
		return fails, err
	}
	fails = append(fails, problems...)

	var rows []Row
	for _, p := range profiles {
		fails = append(fails, p.CheckProvenanceWith(ds, knownSources)...)

		inst, err := roundTrip(p.Answer)
		if err != nil {
			add("schema: %s answer is not JSON: %v", p.AgentID, err)
			continue
		}
		if err := sc.Nested.Validate(inst); err != nil {
			add("schema: %s answer fails %s: %s", p.AgentID, sc.Nested.Name, oneLine(err))
			continue
		}
		row, err := ProjectWith(ds, sc, p)
		if err != nil {
			add("projection: %v", err)
			continue
		}
		if err := validateFlat(sc, row); err != nil {
			add("schema: %s row fails %s: %s", p.AgentID, sc.Flat.Name, oneLine(err))
		}
		rows = append(rows, row)
	}

	// (4) The artifacts on disk: header, duplicates, orphans, encoding, and an
	// independent recompute of the derived metrics from the CSV itself.
	disk, header, err := readCSV(filepath.Join(root, ds.CSVPath))
	if err != nil {
		add("drift: %s: %v", ds.CSVPath, err)
	} else {
		if !equalStrings(header, sc.Flat.Order) {
			add("header: %s header is not %s's properties in order", ds.CSVPath, sc.Flat.Name)
		}
		seen := map[string]bool{}
		for _, r := range disk {
			id := r[ds.IDField]
			if seen[id] {
				add("duplicate %s: %s has %q twice", ds.IDField, ds.CSVPath, id)
			}
			seen[id] = true
			fails = append(fails, checkEncoding(ds, sc, id, r)...)
			d := ds.Derive(r)
			for _, k := range ds.DerivedKeys {
				if r[k] != d[k] {
					add("derived: %s %s is %q, recomputed from the row: %q", id, k, r[k], d[k])
				}
			}
		}
		profIDs := map[string]bool{}
		for _, p := range profiles {
			profIDs[p.AgentID] = true
			if !seen[p.AgentID] {
				add("orphan profile: %s has no row in %s", p.AgentID, ds.CSVPath)
			}
		}
		for id := range seen {
			if !profIDs[id] {
				add("orphan row: %s has %q with no profile", ds.CSVPath, id)
			}
		}
	}

	// (5) The generated artifacts must equal their projections, byte for byte.
	// This is what makes the profiles the source of truth.
	art, err := ArtifactsWith(ds, sc, rows)
	if err != nil {
		add("render: %v", err)
	}
	for _, path := range []string{ds.CSVPath, ds.FeaturePath} {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			add("drift: %s: %v", path, err)
			continue
		}
		if wantBytes, ok := art[path]; ok && !bytes.Equal(got, wantBytes) {
			add("drift: %s differs from the projection — run `survey --render`", path)
		}
	}
	summary, err := RenderSummaryWith(ds, sc, rows)
	if err != nil {
		add("render: %v", err)
	} else if readme, err := os.ReadFile(filepath.Join(root, ds.ReadmePath)); err != nil {
		add("drift: %s: %v", ds.ReadmePath, err)
	} else if wantReadme, err := ApplySummaryWith(ds, readme, summary); err != nil {
		add("drift: %s: %v", ds.ReadmePath, err)
	} else if !bytes.Equal(readme, wantReadme) {
		add("drift: %s summary region differs from the projection — run `survey --render`", ds.ReadmePath)
	}

	// (6) The generated spec must be a spec the canonicaliser can read: a
	// projection that canonicalises to nothing would sign nothing.
	if feat, err := os.ReadFile(filepath.Join(root, ds.FeaturePath)); err != nil {
		add("feature: %v", err)
	} else if ast, err := isotope.ParseGherkin(string(feat)); err != nil {
		add("feature: %s does not parse: %v", ds.FeaturePath, err)
	} else if n := len(ast.Canonicalize()); n == 0 {
		add("feature: %s canonicalises to zero tokens (the projection carries no data)", ds.FeaturePath)
	}

	sort.Strings(fails)
	return fails, nil
}

func validateFlat(sc *Schemas, row Row) error {
	inst, err := roundTrip(row.Values)
	if err != nil {
		return err
	}
	return sc.Flat.Validate(inst)
}

// checkEncoding enforces the cell conventions the schema cannot: no spaces
// around a pipe, pipe-lists of vocabulary values, and integer code lists.
func checkEncoding(ds *Dataset, sc *Schemas, id string, row map[string]string) []string {
	var out []string
	kinds := map[string]kind{}
	for _, f := range ds.Mapping {
		kinds[f.Col] = f.Kind
	}
	for col, cell := range row {
		if strings.Contains(cell, " |") || strings.Contains(cell, "| ") {
			out = append(out, fmt.Sprintf("encoding: %s %s=%q has a space around a pipe", id, col, cell))
		}
		switch kinds[col] {
		case kList, kMappedList:
			if info := sc.Flat.Props[col]; len(info.Enum) > 0 {
				for _, part := range strings.Split(cell, "|") {
					if part == "" {
						continue
					}
					if !contains(info.Enum, part) {
						out = append(out, fmt.Sprintf("encoding: %s %s has %q, not in %v", id, col, part, info.Enum))
					}
				}
			}
		case kIntList:
			for _, part := range strings.Split(cell, "|") {
				if part == "" {
					continue
				}
				if _, err := strconv.Atoi(part); err != nil {
					out = append(out, fmt.Sprintf("encoding: %s %s has %q, not an integer", id, col, part))
				}
			}
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// readCSV reads a generated CSV into rows keyed by column, with the header.
func readCSV(path string) ([]map[string]string, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("empty file")
	}
	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) == 1 && rec[0] == "" {
			continue
		}
		m := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(rec) {
				m[col] = rec[i]
			}
		}
		rows = append(rows, m)
	}
	return rows, header, nil
}

// Stale lists the model survey's profiles whose review_by falls before as-of.
func Stale(root string, asOf time.Time) ([]string, error) {
	return StaleDataset(Model(), root, asOf)
}

// StaleDataset lists a dataset's profiles whose review_by falls before as-of.
func StaleDataset(ds *Dataset, root string, asOf time.Time) ([]string, error) {
	profiles, _, err := LoadProfilesWith(ds, root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range profiles {
		d, err := time.Parse("2006-01-02", p.Provenance.ReviewBy)
		if err != nil {
			out = append(out, fmt.Sprintf("stale: %s has no valid review_by", p.AgentID))
			continue
		}
		if d.Before(asOf) {
			out = append(out, fmt.Sprintf("stale: %s review_by %s is before %s — re-survey it",
				p.AgentID, p.Provenance.ReviewBy, asOf.Format("2006-01-02")))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Render validates the model survey's profiles, then writes its artifacts.
func Render(root string) ([]string, error) {
	return RenderDataset(Model(), root)
}

// RenderDataset validates every profile, then writes models.csv/tools.csv, the
// feature and the README summary region atomically. It refuses to write if any
// profile is invalid: a projection of an invalid observation is not worth
// committing.
func RenderDataset(ds *Dataset, root string) ([]string, error) {
	sc, err := LoadSchemas(root)
	if err != nil {
		return nil, err
	}
	profiles, problems, err := LoadProfilesWith(ds, root)
	if err != nil {
		return nil, err
	}
	fails := append([]string(nil), problems...)
	var rows []Row
	for _, p := range profiles {
		fails = append(fails, p.CheckProvenanceWith(ds, nil)...)
		inst, err := roundTrip(p.Answer)
		if err != nil {
			fails = append(fails, fmt.Sprintf("schema: %s answer is not JSON: %v", p.AgentID, err))
			continue
		}
		if err := sc.Nested.Validate(inst); err != nil {
			fails = append(fails, fmt.Sprintf("schema: %s answer fails %s: %s", p.AgentID, sc.Nested.Name, oneLine(err)))
			continue
		}
		row, err := ProjectWith(ds, sc, p)
		if err != nil {
			fails = append(fails, fmt.Sprintf("projection: %v", err))
			continue
		}
		if err := validateFlat(sc, row); err != nil {
			fails = append(fails, fmt.Sprintf("schema: %s row fails %s: %s", p.AgentID, sc.Flat.Name, oneLine(err)))
			continue
		}
		rows = append(rows, row)
	}
	if len(fails) > 0 {
		sort.Strings(fails)
		return fails, nil
	}

	csvBytes, err := RenderCSV(sc, rows)
	if err != nil {
		return nil, err
	}
	feature, err := RenderFeatureWith(ds, sc, rows)
	if err != nil {
		return nil, err
	}
	summary, err := RenderSummaryWith(ds, sc, rows)
	if err != nil {
		return nil, err
	}
	readmePath := filepath.Join(root, ds.ReadmePath)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, err
	}
	newReadme, err := ApplySummaryWith(ds, readme, summary)
	if err != nil {
		return nil, err
	}
	for path, body := range map[string][]byte{
		filepath.Join(root, ds.CSVPath):     csvBytes,
		filepath.Join(root, ds.FeaturePath): feature,
		readmePath:                          newReadme,
	} {
		if err := writeAtomic(path, body); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// writeAtomic writes body to path via a sibling temp file, so a failed render
// never leaves a half-written artifact.
func writeAtomic(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
