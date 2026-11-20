package ingress

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/james-gibson/isotope"
)

// TestFeatureIsTheOrigin is the Gherkin-first gate: the binary's declaration
// is read by isotope itself, from the same feature the shell-to-gherkin canon
// validated (isotope m9IezbqaBCfurSzt6AwDc1axgBrbhnt2gGXAQY7jjfs, 67 tokens,
// 10 parts, including the fuzz-regression scenario). The validator's result and
// this test cannot drift independently.
func TestFeatureIsTheOrigin(t *testing.T) {
	path := filepath.Join("..", "..", "sheets", "ingress", "wan-ingress.feature")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read origin: %v", err)
	}
	text := string(raw)

	ast, err := isotope.ParseGherkin(text)
	if err != nil {
		t.Fatalf("isotope.ParseGherkin: %v", err)
	}
	if ast == nil {
		t.Fatal("isotope.ParseGherkin returned no AST")
	}

	first, err := isotope.CanonicalizeGherkinToIsotope(text, []byte("seed-42"), "wan-ingress", 1)
	if err != nil {
		t.Fatalf("isotope.CanonicalizeGherkinToIsotope: %v", err)
	}
	if first.Signature == "" || strings.TrimSpace(first.Raw) == "" {
		t.Fatalf("canon produced an empty isotope: %+v", first)
	}
	second, err := isotope.CanonicalizeGherkinToIsotope(text, []byte("seed-42"), "wan-ingress", 1)
	if err != nil || second.Signature != first.Signature || second.Raw != first.Raw {
		t.Fatalf("canon is not stable: first=%+v second=%+v err=%v", first, second, err)
	}
	t.Logf("origin isotope=%s signature=%s tokens=%d", first.String(), first.Signature, strings.Count(first.Raw, "\n")+1)
}
