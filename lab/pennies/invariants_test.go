package pennies_test

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/james-gibson/lab-safety/internal/policy"
	"github.com/james-gibson/lab-safety/lab/pennies"
)

// The invariant suite: the harness is checked by the checker, in the states it
// is meant to be in and the states it is meant to be caught in.
func load(t *testing.T) ([]policy.Sheet, *policy.Policy) {
	t.Helper()
	root := filepath.Join("..", "..")
	sheets, err := policy.LoadSheets(filepath.Join(root, "sheets"))
	if err != nil {
		t.Fatalf("load sheets: %v", err)
	}
	pol, err := policy.LoadPolicy(filepath.Join(root, "policy", "skills.yaml"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	return sheets, pol
}

func check(t *testing.T, srv *httptest.Server, gate bool) []policy.Finding {
	t.Helper()
	sheets, pol := load(t)
	findings, err := policy.Check(policy.Options{
		Sheets:       sheets,
		Policy:       pol,
		ProbeURL:     srv.URL,
		ProbeSurface: "pennies",
		GateProbe:    gate,
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return findings
}

func find(findings []policy.Finding, code string) (policy.Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return policy.Finding{}, false
}

func errors(findings []policy.Finding) []policy.Finding {
	var out []policy.Finding
	for _, f := range findings {
		if f.Severity == "error" {
			out = append(out, f)
		}
	}
	return out
}

// The declared surface, with the gate closed and the tool set matching its
// sheet, passes every invariant the checker knows.
func TestInvariantsHold(t *testing.T) {
	srv := httptest.NewServer(pennies.New(pennies.Options{Enforce: true}))
	defer srv.Close()

	findings := check(t, srv, true)

	if errs := errors(findings); len(errs) > 0 {
		t.Fatalf("the declared harness failed its own declaration: %+v", errs)
	}
	if _, ok := find(findings, "PROBE-OK"); !ok {
		t.Errorf("tools/list did not match the sheet: %+v", findings)
	}
	if f, ok := find(findings, "GATE-VERIFIED"); !ok {
		t.Errorf("the penny gate was not verified: %+v", findings)
	} else if f.Subject != "barista:path" {
		t.Errorf("GATE-VERIFIED subject = %q", f.Subject)
	}
	// The other four skills are still declared-only, and the checker says so
	// rather than letting the one enforced skill speak for the rest.
	if _, ok := find(findings, "GATE-NOT-ENFORCED"); !ok {
		t.Error("the declared-only skills were not reported")
	}
}

// The gate invariant catches an open gate: --enforce=false serves the priced
// tool without a token, and that is an error, not a note.
func TestInvariantCatchesOpenGate(t *testing.T) {
	srv := httptest.NewServer(pennies.New(pennies.Options{Enforce: false}))
	defer srv.Close()

	findings := check(t, srv, true)

	f, ok := find(findings, "GATE-OPEN")
	if !ok {
		t.Fatalf("an open gate was not caught: %+v", findings)
	}
	if f.Severity != "error" || f.Subject != "barista:path" {
		t.Errorf("GATE-OPEN = %+v", f)
	}
	if _, ok := find(findings, "GATE-VERIFIED"); ok {
		t.Error("an open gate was reported as verified")
	}
}

// The tool-set invariant catches an undeclared tool: the harness installs one
// that no sheet mentions.
func TestInvariantCatchesUndeclaredTool(t *testing.T) {
	srv := httptest.NewServer(pennies.New(pennies.Options{Enforce: true, Undeclared: true}))
	defer srv.Close()

	findings := check(t, srv, true)

	f, ok := find(findings, "PROBE-TOOLS-EXTRA")
	if !ok {
		t.Fatalf("an undeclared tool was not caught: %+v", findings)
	}
	if f.Severity != "error" {
		t.Errorf("PROBE-TOOLS-EXTRA severity = %q", f.Severity)
	}
}

// A claim of enforcement with no probe is not silently trusted.
func TestInvariantReportsUnverifiedGate(t *testing.T) {
	srv := httptest.NewServer(pennies.New(pennies.Options{Enforce: true}))
	defer srv.Close()

	findings := check(t, srv, false)

	if _, ok := find(findings, "GATE-UNVERIFIED"); !ok {
		t.Fatalf("an enforced claim with no probe was not reported: %+v", findings)
	}
	if _, ok := find(findings, "GATE-VERIFIED"); ok {
		t.Error("GATE-VERIFIED without running the probe")
	}
}
