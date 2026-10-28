package workspaces

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/james-gibson/isotope"
)

// SheetPath is the surface's safety data sheet — its executable face, relative to
// this module. The sheet declares *this* surface, so it is an artifact of this
// package: a sheet that loses a section, or that the parser the lab canonicalises
// with cannot read, fails here rather than going quiet (sheets/README.md — a
// declaration the surface does not satisfy is a failing test).
const SheetPath = "../../sheets/features/workspace-mcp-surface.feature"

// TestTheSheetIsGherkin guards the sheet's shape, not its truth: the scenarios'
// truth is the rest of this suite (and the instruments). What must not happen is
// a sheet that parses to nothing, a section quietly dropped, or a scenario with
// no steps — each of those reads as "declared" while declaring nothing.
func TestTheSheetIsGherkin(t *testing.T) {
	b, err := os.ReadFile(SheetPath)
	if err != nil {
		t.Fatalf("read the sheet %s: %v", SheetPath, err)
	}
	text := string(b)

	ast, err := isotope.ParseGherkin(text)
	if err != nil {
		t.Fatalf("the sheet is not Gherkin: %v", err)
	}
	if len(ast.Features) != 1 {
		t.Fatalf("features = %d, want 1", len(ast.Features))
	}
	if strings.TrimSpace(ast.Features[0].Name) == "" {
		t.Error("the feature has no name")
	}
	if len(ast.Features[0].Scenarios) == 0 {
		t.Fatal("the sheet parses to no scenarios — the vacuity this test exists to catch")
	}
	for _, sc := range ast.Features[0].Scenarios {
		if len(sc.Steps) == 0 {
			t.Errorf("scenario %q has no steps", sc.Name)
		}
	}

	// One Rule per SDS section: the section map is the sheet's spine, so a
	// missing section is a part of the surface that is no longer declared.
	got := map[int]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*Rule:\s*§(\d+)`).FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("Rule section %q: %v", m[1], err)
		}
		got[n] = true
	}
	for n := 1; n <= 16; n++ {
		if !got[n] {
			t.Errorf("no Rule: §%d — the sheet dropped a section", n)
		}
	}
}
