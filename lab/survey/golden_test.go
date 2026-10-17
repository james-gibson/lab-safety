package survey_test

// The golden fixture.
//
// james-gibson/lab-safety#3 comment 4315112515 carries two *normalised* CSV rows
// — the issue's own answer to "project the nested profiles onto the flat
// schema". They are the expected output of the projector, and they cost nothing
// to keep: if a later edit changes how a nested field flattens, this test says
// so. testdata/issue-rows.csv is that comment verbatim.

import (
	"bytes"
	"encoding/csv"
	"os"
	"testing"

	survey "github.com/james-gibson/lab-safety/survey/internal/survey"
)

func TestProjectionReproducesTheIssueRows(t *testing.T) {
	sc, err := survey.LoadSchemas(root)
	if err != nil {
		t.Fatal(err)
	}
	profiles, problems, err := survey.LoadProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("profiles: %v", problems)
	}
	rows := make([]survey.Row, 0, len(profiles))
	for _, p := range profiles {
		row, err := survey.Project(sc, p)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	got, err := survey.RenderCSV(sc, rows)
	if err != nil {
		t.Fatal(err)
	}

	wantRaw, err := os.ReadFile("testdata/issue-rows.csv")
	if err != nil {
		t.Fatal(err)
	}
	want := parseCSV(t, wantRaw, "testdata/issue-rows.csv")
	have := parseCSV(t, got, "models.csv")

	if len(want) != 3 {
		t.Fatalf("testdata/issue-rows.csv has %d records, want 3 (header + 2 rows)", len(want))
	}
	if len(want[0]) != 91 {
		t.Fatalf("testdata/issue-rows.csv header has %d columns, want the issue's 91", len(want[0]))
	}
	if len(have) != 3 {
		t.Fatalf("models.csv has %d records, want 3", len(have))
	}
	if len(have[0]) != 100 {
		t.Fatalf("models.csv header has %d columns, want 100", len(have[0]))
	}
	if !equalSlice(have[0][:91], want[0]) {
		t.Fatalf("models.csv's first 91 columns are not the issue's header\n got: %v\nwant: %v", have[0][:91], want[0])
	}

	// Match by agent_id: the tool sorts by agent_id, the issue's rows are in
	// the order the thread happened to produce them.
	index := map[string][]string{}
	for _, rec := range have[1:] {
		index[rec[0]] = rec
	}
	for _, rec := range want[1:] {
		id := rec[0]
		got, ok := index[id]
		if !ok {
			t.Fatalf("models.csv has no row for %q", id)
		}
		for i, wantCell := range rec {
			if got[i] != wantCell {
				t.Errorf("%s %s: got %q, want %q (the issue's comment 4315112515)", id, want[0][i], got[i], wantCell)
			}
		}
	}
}

func parseCSV(t *testing.T, raw []byte, name string) [][]string {
	t.Helper()
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return records
}

func equalSlice(a, b []string) bool {
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
