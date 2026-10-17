package survey

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
)

// The generated artifacts. models.csv and survey.feature are wholly generated;
// README.md is hand-written around one generated region.
const (
	CSVPath     = "models.csv"
	FeaturePath = "survey.feature"
	ReadmePath  = "README.md"

	readmeBegin = "<!-- BEGIN GENERATED: model-survey-summary -->"
	readmeEnd   = "<!-- END GENERATED: model-survey-summary -->"
)

// SummaryColumns are the columns of the README summary table. The full row is
// models.csv; the table is the readable face of the same projection.
var SummaryColumns = []string{
	"agent_id", "agent_vendor", "runtime", "model_family", "deployment_type",
	"preferred_interface", "auth_preferred", "auth_requires_human",
	"mcp_ready", "autonomous_ready", "production_grade", "strictness_score", "review_by",
}

// sortRows returns the rows in the canonical order: by agent_id.
func sortRows(rows []Row) []Row {
	out := append([]Row(nil), rows...)
	sort.Slice(out, func(i, j int) bool { return out[i].AgentID < out[j].AgentID })
	return out
}

// RenderCSV projects the rows onto the flat schema's 100 columns.
func RenderCSV(s *Schemas, rows []Row) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(s.Flat.Order); err != nil {
		return nil, err
	}
	for _, r := range sortRows(rows) {
		if err := w.Write(r.CSVOf(s)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderSummary renders the model survey's generated README region (markers
// included).
func RenderSummary(s *Schemas, rows []Row) ([]byte, error) {
	return RenderSummaryWith(Model(), s, rows)
}

// RenderSummaryWith renders a dataset's generated README region.
func RenderSummaryWith(ds *Dataset, s *Schemas, rows []Row) ([]byte, error) {
	var b strings.Builder
	b.WriteString(ds.ReadmeBegin + "\n")
	b.WriteString("| " + strings.Join(ds.SummaryColumns, " | ") + " |\n")
	b.WriteString("| " + strings.Join(repeat("---", len(ds.SummaryColumns)), " | ") + " |\n")
	for _, r := range sortRows(rows) {
		cs := r.CSVOf(s)
		val := map[string]string{}
		for i, col := range s.Flat.Order {
			val[col] = cs[i]
		}
		cells := make([]string, 0, len(ds.SummaryColumns))
		for _, col := range ds.SummaryColumns {
			cells = append(cells, val[col])
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	b.WriteString(ds.ReadmeEnd + "\n")
	return []byte(b.String()), nil
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

// ApplySummary swaps the model survey's generated region of README.md for
// summary.
func ApplySummary(readme, summary []byte) ([]byte, error) {
	return ApplySummaryWith(Model(), readme, summary)
}

// ApplySummaryWith swaps a dataset's generated README region for summary. It
// fails if the markers are missing, so the region cannot be quietly dropped.
func ApplySummaryWith(ds *Dataset, readme, summary []byte) ([]byte, error) {
	i := bytes.Index(readme, []byte(ds.ReadmeBegin))
	j := bytes.Index(readme, []byte(ds.ReadmeEnd))
	if i < 0 || j < 0 || j < i {
		return nil, fmt.Errorf("README.md has no %s … %s region", ds.ReadmeBegin, ds.ReadmeEnd)
	}
	j += len(ds.ReadmeEnd)
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

// RenderFeature projects the model survey's rows onto Gherkin.
func RenderFeature(s *Schemas, rows []Row) ([]byte, error) {
	return RenderFeatureWith(Model(), s, rows)
}

// RenderFeatureWith projects a dataset's rows onto Gherkin.
//
// The arrow here points the opposite way to a surface sheet (sheets/): for a
// surface, the Gherkin is the origin and the declaration is derived from it.
// For a survey, the observations are the origin and this file is their
// projection. Every data-bearing field is a step — never a table, because
// isotope's canonicaliser hashes step text and ignores table rows, so a table
// would make the spec's isotope blind to the data it claims to carry. The
// Background carries no data for the same reason: the canonicaliser does not
// parse Background steps into scenarios.
func RenderFeatureWith(ds *Dataset, s *Schemas, rows []Row) ([]byte, error) {
	var b strings.Builder
	b.WriteString(ds.FeatureHeader)
	for _, r := range sortRows(rows) {
		cs := r.CSVOf(s)
		fmt.Fprintf(&b, "  Scenario: %s\n", r.AgentID)
		for i, col := range s.Flat.Order {
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

// Artifacts renders the model survey's generated artifacts for a set of rows.
func Artifacts(s *Schemas, rows []Row) (map[string][]byte, error) {
	return ArtifactsWith(Model(), s, rows)
}

// ArtifactsWith renders a dataset's generated artifacts for a set of rows.
func ArtifactsWith(ds *Dataset, s *Schemas, rows []Row) (map[string][]byte, error) {
	csvBytes, err := RenderCSV(s, rows)
	if err != nil {
		return nil, err
	}
	feature, err := RenderFeatureWith(ds, s, rows)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{ds.CSVPath: csvBytes, ds.FeaturePath: feature}, nil
}
