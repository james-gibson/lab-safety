// Command lab-safety publishes and checks the lab's policy manifest.
//
// The manifest is the machine-readable projection of the safety data sheets
// (sheets/): what each MCP surface serves, which classes it carries, the
// minimum rung per class, and which containment controls are enforced. The
// authored half (policy/skills.yaml) adds what the resulting tokens cost —
// a price per credential, not per request.
//
//	lab-safety manifest            emit x_lab_safety_manifest
//	lab-safety check               find contradictions and declared-not-enforced
//	lab-safety check --probe=URL --surface=ID
//	                               compare a live surface's tools/list to its sheet
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"runtime/debug"
	"strings"

	"github.com/james-gibson/lab-safety/internal/policy"
)

var buildVersion string

func version() string {
	if buildVersion != "" {
		return buildVersion
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 8 {
			return s.Value[:8]
		}
	}
	return "dev"
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "manifest":
		cmdManifest(args[1:])
	case "check":
		cmdCheck(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("lab-safety %s\n", version())
	case "--help", "-h", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: lab-safety <command> [flags]

Commands:
  manifest   Emit the policy manifest (x_lab_safety_manifest)
  check      Check the declaration against itself, and optionally a live surface
  version    Print version

Flags:
  --sheets=<dir>       Safety data sheets (default "sheets")
  --policy=<path>      Priced skills and issuer endpoints (default "policy/skills.yaml")
  --out=<path>         manifest: write here instead of stdout
  --probe=<url>        check: POST tools/list here and compare with --surface
  --surface=<id>       check: the sheet the probed endpoint belongs to
  --json               check: emit findings as JSON
`)
}

func cmdManifest(args []string) {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	sheetsDir := fs.String("sheets", "sheets", "safety data sheet directory")
	policyPath := fs.String("policy", "policy/skills.yaml", "policy file")
	outPath := fs.String("out", "", "write the manifest to this path instead of stdout")
	_ = fs.Parse(args)

	sheets, pol := load(*sheetsDir, *policyPath)
	m := policy.Build(sheets, pol)

	if *outPath == "" || *outPath == "-" {
		if err := m.WriteJSON(os.Stdout); err != nil {
			fatal("%v", err)
		}
		return
	}
	f, err := os.Create(*outPath)
	if err != nil {
		fatal("write manifest: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := m.WriteJSON(f); err != nil {
		fatal("%v", err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (status: %s, %d surface(s), %d skill(s))\n",
		*outPath, m.Body.Status, len(m.Body.Surfaces), len(m.Body.Skills))
}

func cmdCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	sheetsDir := fs.String("sheets", "sheets", "safety data sheet directory")
	policyPath := fs.String("policy", "policy/skills.yaml", "policy file")
	probeURL := fs.String("probe", "", "live MCP endpoint to compare against --surface")
	surface := fs.String("surface", "", "the sheet the probed endpoint belongs to")
	asJSON := fs.Bool("json", false, "emit findings as JSON")
	_ = fs.Parse(args)

	sheets, pol := load(*sheetsDir, *policyPath)

	findings, err := policy.Check(policy.Options{
		Sheets:       sheets,
		Policy:       pol,
		ProbeURL:     *probeURL,
		ProbeSurface: *surface,
	})
	if err != nil {
		fatal("%v", err)
	}

	errors := 0
	for _, f := range findings {
		if f.Severity == "error" {
			errors++
		}
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(findings); err != nil {
			fatal("encode findings: %v", err)
		}
	} else {
		fmt.Printf("lab-safety check: %d sheet(s), %d skill(s)\n", len(sheets), len(pol.Skills))
		width := 0
		for _, f := range findings {
			if len(f.Code) > width {
				width = len(f.Code)
			}
		}
		for _, f := range findings {
			fmt.Printf("  %-5s %-*s  %-22s %s\n", f.Severity, width, f.Code, truncate(f.Subject, 22), f.Detail)
		}
		fmt.Printf("\n%d error(s), %d note(s)\n", errors, len(findings)-errors)
	}

	if errors > 0 {
		os.Exit(1)
	}
}

func load(sheetsDir, policyPath string) ([]policy.Sheet, *policy.Policy) {
	sheets, err := policy.LoadSheets(sheetsDir)
	if err != nil {
		fatal("%v", err)
	}
	pol, err := policy.LoadPolicy(policyPath)
	if err != nil {
		// A missing policy file is not fatal: the manifest still describes the
		// sheets, and check still reports what it can.
		if !errors.Is(err, fs.ErrNotExist) {
			fatal("%v", err)
		}
		fmt.Fprintf(os.Stderr, "lab-safety: no policy at %s (manifest will carry no skills)\n", policyPath)
		pol = &policy.Policy{}
	}
	return sheets, pol
}

// truncate shortens a finding subject for the table, counting runes so a
// multi-byte surface id is not cut in half.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return strings.TrimSuffix(string(r[:n-1]), " ") + "…"
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
