package concordance

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The generated artifacts. concordance.csv and concordance.feature are wholly
// generated; README.md is hand-written around one generated region.
const (
	CSVPath     = "concordance.csv"
	FeaturePath = "concordance.feature"
	ReadmePath  = "README.md"

	readmeBegin = "<!-- BEGIN GENERATED: concordance-summary -->"
	readmeEnd   = "<!-- END GENERATED: concordance-summary -->"
)

// SummaryColumns are the columns of the README summary table.
var SummaryColumns = []string{
	"source_id", "subject_id", "predicate", "their_claim", "our_claim",
	"agrees", "our_basis", "checked_at", "review_by",
}

// field is one flat column and the record path it projects from.
type field struct {
	Col  string
	Path string
}

// mapping is the projection, as data: every flat column except the computed
// `agrees`. Check asserts that mapping + [agrees] == flat.Order, so the
// projection and the schema cannot drift apart.
var mapping = []field{
	{"source_id", "source.id"},
	{"source_url", "source.url"},
	{"retrieved_at", "source.retrieved_at"},
	{"subject_id", "subject.id"},
	{"subject_kind", "subject.kind"},
	{"listing_url", "subject.listing_url"},
	{"predicate", "predicate"},
	{"their_claim", "their_claim.value"},
	{"our_claim", "our_claim.value"},
	{"our_basis", "our_claim.basis"},
	{"checked_at", "provenance.checked_at"},
	{"review_by", "provenance.review_by"},
	{"method", "provenance.method"},
}

// computed are the columns the tool fills in and a record cannot author:
// `agrees` from the two claims, `credit` from the source's licence.
var computed = []string{"agrees", "credit"}

// Row is one projected flat row.
type Row struct {
	Key    string
	Values map[string]any
}

// Agrees is the whole computation, in one line, so it cannot be smuggled in:
// agreement is equality of the two normalised tokens.
func Agrees(their, our string) bool { return their == our }

// Project turns a record into a flat row and fills the computed columns:
// `agrees`, and `credit` — the attribution the source's licence requires, so
// that the credit travels with the data instead of living on a credits page.
func Project(d *Dataset, r Record) (Row, error) {
	vals := map[string]any{}
	for _, f := range mapping {
		v, ok := lookup(r, f.Path)
		if !ok {
			return Row{}, fmt.Errorf("%s: no %s (for %s)", r.Key(), f.Path, f.Col)
		}
		vals[f.Col] = v
	}
	vals["agrees"] = Agrees(r.TheirClaim.Value, r.OurClaim.Value)
	vals["credit"] = d.CreditFor(r.SourceID)
	return Row{Key: r.Key(), Values: vals}, nil
}

// CreditFor renders a source's required credit as one string, or "" when the
// source requires none.
func (d *Dataset) CreditFor(sourceID string) string {
	src, ok := d.Sources[sourceID]
	if !ok || src.Credit == nil {
		return ""
	}
	return src.Credit.Text + " — " + src.Credit.URL
}

func lookup(r Record, path string) (any, bool) {
	switch path {
	case "source.id":
		return r.Source.ID, true
	case "source.url":
		return r.Source.URL, true
	case "source.retrieved_at":
		return r.Source.RetrievedAt, true
	case "subject.id":
		return r.Subject.ID, true
	case "subject.kind":
		return r.Subject.Kind, true
	case "subject.listing_url":
		return r.Subject.ListingURL, true
	case "predicate":
		return r.Predicate, true
	case "their_claim.value":
		return r.TheirClaim.Value, true
	case "our_claim.value":
		return r.OurClaim.Value, true
	case "our_claim.basis":
		return r.OurClaim.Basis, true
	case "provenance.checked_at":
		return r.Provenance.CheckedAt, true
	case "provenance.review_by":
		return r.Provenance.ReviewBy, true
	case "provenance.method":
		return r.Provenance.Method, true
	}
	return nil, false
}

func sortRows(rows []Row) []Row {
	out := append([]Row(nil), rows...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// CSVOf renders a row's cells in the flat schema's column order.
func (r Row) CSVOf(d *Dataset) []string {
	out := make([]string, 0, len(d.Flat.Order))
	for _, col := range d.Flat.Order {
		out = append(out, cell(r.Values[col]))
	}
	return out
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

// RenderCSV projects the rows onto the flat schema's columns.
func RenderCSV(d *Dataset, rows []Row) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(d.Flat.Order); err != nil {
		return nil, err
	}
	for _, r := range sortRows(rows) {
		if err := w.Write(r.CSVOf(d)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderSummary renders the generated README region (markers included).
func RenderSummary(d *Dataset, rows []Row) ([]byte, error) {
	var b strings.Builder
	b.WriteString(readmeBegin + "\n")
	b.WriteString("| " + strings.Join(SummaryColumns, " | ") + " |\n")
	b.WriteString("| " + strings.Join(repeat("---", len(SummaryColumns)), " | ") + " |\n")
	for _, r := range sortRows(rows) {
		cs := r.CSVOf(d)
		val := map[string]string{}
		for i, col := range d.Flat.Order {
			val[col] = cs[i]
		}
		cells := make([]string, 0, len(SummaryColumns))
		for _, col := range SummaryColumns {
			cells = append(cells, val[col])
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	if credits := creditsOf(d, rows); len(credits) > 0 {
		b.WriteString("\n")
		for _, c := range credits {
			b.WriteString("_Data credit required by the source's licence:_ " + c + "\n")
		}
	}
	b.WriteString(readmeEnd + "\n")
	return []byte(b.String()), nil
}

// creditsOf returns the distinct credits the rows carry, sorted. A view that
// shows the data must show these; rendering them into the region is how the
// obligation is kept structural rather than remembered.
func creditsOf(d *Dataset, rows []Row) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range sortRows(rows) {
		c, _ := r.Values["credit"].(string)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// ApplySummary swaps the generated region of README.md for summary.
func ApplySummary(readme, summary []byte) ([]byte, error) {
	i := bytes.Index(readme, []byte(readmeBegin))
	j := bytes.Index(readme, []byte(readmeEnd))
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("README.md has no %s … %s region", readmeBegin, readmeEnd)
	}
	j += len(readmeEnd)
	tail := readme[j:]
	if len(tail) > 0 && tail[0] == '\n' {
		tail = tail[1:]
	}
	var out bytes.Buffer
	out.Write(readme[:i])
	out.Write(summary)
	out.Write(tail)
	return out.Bytes(), nil
}

// RenderFeature projects the rows onto Gherkin. The arrow points the same way
// as the survey's: the records are the origin, this file is the projection.
// Every data-bearing field is a step, never a table — isotope's canonicaliser
// hashes step text and ignores table rows.
func RenderFeature(d *Dataset, rows []Row) ([]byte, error) {
	var b strings.Builder
	b.WriteString("@lab-safety\n@concordance\n@z0-physical\n@domain-concordance\n")
	b.WriteString("Feature: Concordance — a third party's claims against our own\n\n")
	b.WriteString("  Each record names a subject both sides name, a predicate, and the two\n")
	b.WriteString("  claims. `agrees` is computed from them and is not a field of a record, so\n")
	b.WriteString("  it cannot be asserted — only recomputed.\n\n")
	b.WriteString("  A record may not exist without our own claim, anchored by `our_claim.basis`\n")
	b.WriteString("  to a file in this repo; and a third party's claim is stored as one\n")
	b.WriteString("  normalised token, never their prose. Those two rules are what stop this\n")
	b.WriteString("  file being a copy of somebody's catalogue.\n\n")
	b.WriteString("  Background:\n")
	b.WriteString("    Given the records under \"records/\" are the concordance's source of truth\n\n")
	for _, r := range sortRows(rows) {
		cs := r.CSVOf(d)
		fmt.Fprintf(&b, "  Scenario: %s\n", r.Key)
		for i, col := range d.Flat.Order {
			kw := "Then"
			if i > 0 {
				kw = "And"
			}
			fmt.Fprintf(&b, "    %s its %s is %q\n", kw, col, cs[i])
		}
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}

// Artifacts renders the generated artifacts for a set of rows.
func Artifacts(d *Dataset, rows []Row) (map[string][]byte, error) {
	csvBytes, err := RenderCSV(d, rows)
	if err != nil {
		return nil, err
	}
	feature, err := RenderFeature(d, rows)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{CSVPath: csvBytes, FeaturePath: feature}, nil
}
