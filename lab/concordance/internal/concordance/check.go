package concordance

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/james-gibson/isotope"
)

// Check runs the whole contract and returns one line per failure, sorted. An
// empty result means: every record is well-typed and anchored to one of our own
// artefacts, every source is declared, every agreement recomputes, and the
// generated artifacts have not drifted.
func (d *Dataset) Check() ([]string, error) {
	var fails []string
	add := func(format string, a ...any) { fails = append(fails, fmt.Sprintf(format, a...)) }

	// (1) The flat schema is the projection: mapping + the computed columns.
	want := []string{}
	for _, f := range mapping {
		want = append(want, f.Col)
	}
	want = append(want, computed...)
	if !equalStrings(d.Flat.Order, want) {
		add("schema: %s properties are not the projection + %v; the schema and the projector disagree", d.Flat.Name, computed)
	}

	// (2) Problems found while loading (bad paths, strict decode, schema).
	fails = append(fails, d.problems...)

	// (2b) No credential in the repo. A key belongs in the environment: this
	// dataset reads a licensed directory, and a key committed here would be a
	// leak, not a convenience.
	fails = append(fails, d.checkNoSecrets()...)

	// (3) Every record.
	var rows []Row
	for _, r := range d.Records {
		fails = append(fails, d.checkRecord(r)...)
		row, err := Project(d, r)
		if err != nil {
			add("projection: %v", err)
			continue
		}
		if err := d.validateFlat(row); err != nil {
			add("schema: %s row fails %s: %s", r.Key(), d.Flat.Name, oneLine(err))
		}
		rows = append(rows, row)
	}

	// (3b) Attribution: a source that declares a credit must have it, and a
	// source that requires a per-listing link must have one per record. These
	// are licence obligations, so they are checks and not notes.
	fails = append(fails, d.checkAttribution(rows)...)

	// (4) The artifact on disk: header, recomputed agreement, duplicate keys.
	disk, header, err := readCSV(filepath.Join(d.Root, CSVPath))
	if err != nil {
		add("drift: %s: %v", CSVPath, err)
	} else {
		if !equalStrings(header, d.Flat.Order) {
			add("header: %s header is not %s's properties in order", CSVPath, d.Flat.Name)
		}
		seen := map[string]bool{}
		for _, r := range disk {
			key := r["source_id"] + "/" + r["subject_id"] + "/" + r["predicate"]
			if seen[key] {
				add("duplicate record: %s has %q twice", CSVPath, key)
			}
			seen[key] = true
			want := Agrees(r["their_claim"], r["our_claim"])
			if r["agrees"] != fmt.Sprint(want) {
				add("agrees: %s is %q, recomputed from the two claims: %v", key, r["agrees"], want)
			}
		}
	}

	// (5) The generated artifacts must equal their projections, byte for byte.
	art, err := Artifacts(d, rows)
	if err != nil {
		add("render: %v", err)
	}
	for _, path := range []string{CSVPath, FeaturePath} {
		got, err := os.ReadFile(filepath.Join(d.Root, path))
		if err != nil {
			add("drift: %s: %v", path, err)
			continue
		}
		if wantBytes, ok := art[path]; ok && !bytes.Equal(got, wantBytes) {
			add("drift: %s differs from the projection — run `concordance --render`", path)
		}
	}
	summary, err := RenderSummary(d, rows)
	if err != nil {
		add("render: %v", err)
	} else if readme, err := os.ReadFile(filepath.Join(d.Root, ReadmePath)); err != nil {
		add("drift: %s: %v", ReadmePath, err)
	} else if wantReadme, err := ApplySummary(readme, summary); err != nil {
		add("drift: %s: %v", ReadmePath, err)
	} else if !bytes.Equal(readme, wantReadme) {
		add("drift: %s summary region differs from the projection — run `concordance --render`", ReadmePath)
	}

	// (6) The projection must be a spec the canonicaliser can read.
	if feat, err := os.ReadFile(filepath.Join(d.Root, FeaturePath)); err != nil {
		add("feature: %v", err)
	} else if ast, err := isotope.ParseGherkin(string(feat)); err != nil {
		add("feature: %s does not parse: %v", FeaturePath, err)
	} else if n := len(ast.Canonicalize()); n == 0 {
		add("feature: %s canonicalises to zero tokens (the projection carries no data)", FeaturePath)
	}

	sort.Strings(fails)
	return dedupe(fails), nil
}

// dedupe removes repeated lines: several checks can notice the same missing
// artifact, and a report that says it twice reads like two problems.
func dedupe(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i > 0 && s == in[i-1] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// checkAttribution enforces the licence obligations that attach to a source.
//
// A source that declares a credit has it carried by every row (so the CSV
// cannot be shown without it), by the README region, and by the feature. A
// source that requires a per-listing link must have one on every record, and it
// must point at the source's own host — "the url the API returns for it, not a
// copy of the listing you host".
func (d *Dataset) checkAttribution(rows []Row) []string {
	var out []string

	// declarations must be usable
	for _, id := range sortedSourceIDs(d.Sources) {
		src := d.Sources[id]
		if src.Licence == "attribution-required" && src.Credit == nil {
			out = append(out, fmt.Sprintf("attribution: sources/%s.json is attribution-required but declares no credit", id))
		}
		if src.Credit != nil {
			if src.Credit.Text == "" || src.Credit.URL == "" {
				out = append(out, fmt.Sprintf("attribution: sources/%s.json credit needs both text and url", id))
			}
			if src.Waiver == "" && src.Licence == "attribution-required" {
				out = append(out, fmt.Sprintf("attribution: sources/%s.json requires attribution but records no waiver route — a research dataset or CLI surface may not be able to display it", id))
			}
		}
	}

	// per-record obligations
	for _, r := range d.Records {
		src, ok := d.Sources[r.SourceID]
		if !ok {
			continue // already reported as an orphan record
		}
		if src.LinkPerRow {
			if r.Subject.ListingURL == "" {
				out = append(out, fmt.Sprintf("attribution: %s has no subject.listing_url, and sources/%s.json requires a link to every listing", r.Key(), src.ID))
			} else if host := hostOf(r.Subject.ListingURL); host != hostOf(src.URL) {
				out = append(out, fmt.Sprintf("attribution: %s listing_url %q is on %q, not the source's %q",
					r.Key(), r.Subject.ListingURL, host, hostOf(src.URL)))
			}
		}
	}

	// the views we generate must carry the credits
	credits := creditsOf(d, rows)
	if len(credits) == 0 {
		return out
	}
	readme, err := os.ReadFile(filepath.Join(d.Root, ReadmePath))
	if err == nil {
		for _, c := range credits {
			if !bytes.Contains(readme, []byte(c)) {
				out = append(out, fmt.Sprintf("attribution: %s does not carry the required credit %q", ReadmePath, c))
			}
		}
	}
	feat, err := os.ReadFile(filepath.Join(d.Root, FeaturePath))
	if err == nil {
		for _, c := range credits {
			if !bytes.Contains(feat, []byte(c)) {
				out = append(out, fmt.Sprintf("attribution: %s does not carry the required credit %q", FeaturePath, c))
			}
		}
	}
	return out
}

func sortedSourceIDs(m map[string]Source) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func hostOf(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}
	return u.Host
}

// secretPatterns are the credential shapes we refuse to see in the repo. A key
// belongs in the environment; a key in a file is a leak with a long tail.
var secretPatterns = []struct {
	Name string
	Re   *regexp.Regexp
}{
	{"glama api key", regexp.MustCompile(`glm_[A-Za-z0-9_]{20,}`)},
	{"openai-style key", regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)},
	{"github token", regexp.MustCompile(`(ghp_|gho_|ghs_|ghu_|github_pat_)[A-Za-z0-9_]{20,}`)},
	{"aws access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"assigned secret", regexp.MustCompile(`(?i)(api[_-]?key|client[_-]?secret|access[_-]?token|bearer)["']?\s*[:=]\s*["'][A-Za-z0-9_\-]{24,}["']`)},
}

// checkNoSecrets walks the whole module — records, sources, fixtures, docs — and
// refuses to let a credential sit in it. The fixture that breaks this check
// appends its fake key at test time, so the repository itself stays clean.
func (d *Dataset) checkNoSecrets() []string {
	var out []string
	err := filepath.Walk(d.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(d.Root, path)
		if err != nil {
			return err
		}
		for _, p := range secretPatterns {
			if loc := p.Re.Find(raw); loc != nil {
				out = append(out, fmt.Sprintf("secret: %s looks like it contains a %s — keys belong in the environment, never in the repo", rel, p.Name))
			}
		}
		return nil
	})
	if err != nil {
		out = append(out, fmt.Sprintf("secret: could not scan the module: %v", err))
	}
	return out
}

// checkRecord enforces the two rules that keep this from being a mirror of a
// catalogue, plus the address and provenance rules.
func (d *Dataset) checkRecord(r Record) []string {
	var out []string
	key := r.Key()

	// the path is the key
	want := filepath.Join("records", r.SourceID, r.SubjectID, r.Predicate+".json")
	if r.Path != want {
		out = append(out, fmt.Sprintf("address: %s says %s/%s/%s — the path is the key", r.Path, r.SourceID, r.SubjectID, r.Predicate))
	}

	// the source must be declared, and its URL is the source's, not ours to vary
	src, ok := d.Sources[r.SourceID]
	if !ok {
		out = append(out, fmt.Sprintf("orphan record: %s names source %q, which is not declared in sources/", key, r.SourceID))
	} else if r.Source.URL != src.URL {
		out = append(out, fmt.Sprintf("source_url: %s says %q, but sources/%s.json declares %q",
			key, r.Source.URL, r.SourceID, src.URL))
	}

	// Rule 2: our claim must be anchored to one of our own artefacts.
	out = append(out, d.checkBasis(key, r.OurClaim.Basis)...)

	out = append(out, r.CheckProvenance()...)
	return out
}

// checkBasis resolves our_claim.basis against the repo root. A record whose
// basis does not exist is a record we cannot justify, and the point of the rule
// is that such a record cannot exist at all.
func (d *Dataset) checkBasis(key, basis string) []string {
	if basis == "" {
		return []string{fmt.Sprintf("basis: %s has no our_claim.basis — a record with no claim of ours is a copy of theirs", key)}
	}
	if filepath.IsAbs(basis) {
		return []string{fmt.Sprintf("basis: %s basis %q is absolute; it must be relative to the repo root", key, basis)}
	}
	clean := filepath.Clean(basis)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return []string{fmt.Sprintf("basis: %s basis %q escapes the repo root", key, basis)}
	}
	full := filepath.Join(d.RepoRoot, clean)
	info, err := os.Stat(full)
	if err != nil {
		return []string{fmt.Sprintf("basis: %s basis %q does not exist under the repo root (%s)", key, basis, d.RepoRoot)}
	}
	if info.IsDir() {
		return []string{fmt.Sprintf("basis: %s basis %q is a directory, not one of our artefacts", key, basis)}
	}
	return nil
}

func (d *Dataset) validateFlat(row Row) error {
	inst, err := roundTrip(row.Values)
	if err != nil {
		return err
	}
	return d.Flat.Validate(inst)
}

// readCSV reads concordance.csv into rows keyed by column, with the header.
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

// Stale lists records whose review_by falls before as-of.
func (d *Dataset) Stale(asOf time.Time) ([]string, error) {
	var out []string
	for _, r := range d.Records {
		review, err := time.Parse("2006-01-02", r.Provenance.ReviewBy)
		if err != nil {
			out = append(out, fmt.Sprintf("stale: %s has no valid review_by", r.Key()))
			continue
		}
		if review.Before(asOf) {
			out = append(out, fmt.Sprintf("stale: %s review_by %s is before %s — re-check it",
				r.Key(), r.Provenance.ReviewBy, asOf.Format("2006-01-02")))
		}
	}
	sort.Strings(out)
	return out, nil
}

// Render validates every record, then writes concordance.csv, concordance.feature
// and the README summary region atomically. It refuses to write while any record
// is invalid — a projection of an unjustified record is not worth committing.
func (d *Dataset) Render() ([]string, error) {
	var fails []string
	var rows []Row
	for _, r := range d.Records {
		fails = append(fails, d.checkRecord(r)...)
		row, err := Project(d, r)
		if err != nil {
			fails = append(fails, fmt.Sprintf("projection: %v", err))
			continue
		}
		if err := d.validateFlat(row); err != nil {
			fails = append(fails, fmt.Sprintf("schema: %s row fails %s: %s", r.Key(), d.Flat.Name, oneLine(err)))
			continue
		}
		rows = append(rows, row)
	}
	fails = append(fails, d.problems...)
	if len(fails) > 0 {
		sort.Strings(fails)
		return fails, nil
	}

	csvBytes, err := RenderCSV(d, rows)
	if err != nil {
		return nil, err
	}
	feature, err := RenderFeature(d, rows)
	if err != nil {
		return nil, err
	}
	summary, err := RenderSummary(d, rows)
	if err != nil {
		return nil, err
	}
	readmePath := filepath.Join(d.Root, ReadmePath)
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return nil, err
	}
	newReadme, err := ApplySummary(readme, summary)
	if err != nil {
		return nil, err
	}
	for path, body := range map[string][]byte{
		filepath.Join(d.Root, CSVPath):     csvBytes,
		filepath.Join(d.Root, FeaturePath): feature,
		readmePath:                         newReadme,
	} {
		if err := writeAtomic(path, body); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func writeAtomic(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

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
