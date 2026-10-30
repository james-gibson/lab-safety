package policy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A sheet with both table layouts the repo uses, a prose cell that must not be
// read as hazard classes, a dynamic-tools table, and wrapped status text.
const fixtureSheet = `# Safety Data Sheet — fixture surface

## §1 Identification

| field | value |
| --- | --- |
| surface id | ` + "`fixture`" + ` (test surface) |
| owner repo | ` + "`github.com/james-gibson/fixture`" + ` |
| endpoint | ` + "`http://<host>:9999/mcp`" + ` |
| transport | ` + "`POST /mcp`" + ` |
| protocol version | ` + "`2025-01-01`" + ` |

## §2 Hazard identification

Classes present: every class.

## §3 Composition

| # | tool | class | inputs | returns |
| --- | --- | --- | --- | --- |
| 1 | ` + "`fixture.read`" + ` | ` + "`R`" + ` | — | state |
| 2 | ` + "`fixture.exec`" + ` | ` + "`X`" + ` | body | output |
| 3 | ` + "`fixture.proxy`" + ` | ` + "`R`,`E`" + ` | target | proxied result |

**Dynamic tools** (registered at runtime):

| tool | class | note |
| --- | --- | --- |
| ` + "`fixture.project.<name>.call`" + ` | ` + "`E`" + ` | installed at join |

Non-tool surface:

| kind | entry |
| --- | --- |
| ` + "`resource`" + ` | ` + "`hosted://status`" + ` |

## §5 Fire-fighting measures

| control | value | status |
| --- | --- | --- |
| auth | none | **declared, not implemented** |
| bind | loopback | enforced — note the port is random |
| kill switch | remove | enforced (leave); declared (gate) |

## §7 Handling and storage

- **Rung:** ` + "`R`" + ` → rung 0; ` + "`E`" + ` → rung 4; ` + "`X`" + ` → rung 6. Status: not
  enforced (the surface does not read rungs).

## §8 Exposure controls

| rung | may read |
| --- | --- |
| 0+ | reads |
`

func TestParseSheet(t *testing.T) {
	sheet, err := ParseSheet(fixtureSheet)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if sheet.ID != "fixture" {
		t.Errorf("ID = %q, want fixture", sheet.ID)
	}
	if sheet.Owner != "`github.com/james-gibson/fixture`" {
		t.Errorf("Owner = %q", sheet.Owner)
	}

	tools := map[string]Tool{}
	for _, tool := range sheet.Tools {
		tools[tool.Name] = tool
	}
	if got := tools["fixture.read"].Classes; len(got) != 1 || got[0] != "R" {
		t.Errorf("fixture.read classes = %v", got)
	}
	if got := tools["fixture.proxy"].Classes; len(got) != 2 || got[0] != "E" || got[1] != "R" {
		t.Errorf("fixture.proxy classes = %v", got)
	}
	if !tools["fixture.project.<name>.call"].Dynamic {
		t.Error("the runtime tool was not marked dynamic")
	}
	// The prose cell "proxied result" and the non-tool table must not become
	// tools: ParseClasses rejects anything that is not legend letters.
	for _, bogus := range []string{"resource", "kind", "returns", "inputs"} {
		if _, ok := tools[bogus]; ok {
			t.Errorf("prose table read as an inventory: %q became a tool", bogus)
		}
	}

	if got := sheet.RungByClass["X"]; got != 6 {
		t.Errorf("rung for X = %d, want 6", got)
	}
	if got := sheet.RungByClass["E"]; got != 4 {
		t.Errorf("rung for E = %d, want 4", got)
	}
	if got := sheet.RungByClass["R"]; got != 0 {
		t.Errorf("rung for R = %d, want 0", got)
	}
	if sheet.RungEnforced {
		t.Error("a wrapped \"Status: not enforced\" was read as enforced")
	}
	if !strings.Contains(sheet.RungStatus, "surface does not read rungs") {
		t.Errorf("wrapped status truncated: %q", sheet.RungStatus)
	}

	controls := map[string]Control{}
	for _, c := range sheet.Controls {
		controls[c.Name] = c
	}
	if controls["auth"].Enforced {
		t.Error("declared, not implemented was read as enforced")
	}
	if !controls["bind"].Enforced {
		t.Error("enforced-with-a-note was read as not enforced")
	}
	if controls["kill switch"].Enforced {
		t.Error("enforced (leave); declared (gate) was read as fully enforced")
	}
}

// The parse must survive the sheets the repository actually ships, because a
// manifest built from a stale parse is the false declaration the sheets exist
// to prevent.
func TestParseRealSheets(t *testing.T) {
	dir := filepath.Join("..", "..", "sheets")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("sheets directory not present")
	}
	sheets, err := LoadSheets(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(sheets) < 2 {
		t.Fatalf("parsed %d sheets, want at least 2", len(sheets))
	}

	byID := map[string]Sheet{}
	for _, s := range sheets {
		byID[s.ID] = s
	}

	adhd, ok := byID["adhd"]
	if !ok {
		t.Fatalf("adhd sheet not found, have %v", keys(byID))
	}
	if adhd.RungByClass["X"] != 6 || adhd.RungByClass["E"] != 4 || adhd.RungByClass["W"] != 3 {
		t.Errorf("adhd rungs = %v", adhd.RungByClass)
	}
	if !hasToolClass(adhd, "adhd.proxy", "E") {
		t.Error("adhd.proxy does not carry E")
	}
	if !hasDynamic(adhd, "adhd.project.<name>.call") {
		t.Error("adhd's runtime project tool is not marked dynamic")
	}

	smoke, ok := byID["ocd-smoke-alarm"]
	if !ok {
		t.Fatalf("ocd-smoke-alarm sheet not found, have %v", keys(byID))
	}
	// The alarm uses the flat form ("declared minimum rung 0 (`R` only)").
	if smoke.RungByClass["R"] != 0 {
		t.Errorf("smoke rung for R = %d, want 0", smoke.RungByClass["R"])
	}
	for _, tool := range smoke.Tools {
		if len(tool.Classes) != 1 || tool.Classes[0] != "R" {
			t.Errorf("%s carries %v, want R only", tool.Name, tool.Classes)
		}
	}
}

func TestBuildManifest(t *testing.T) {
	sheet, err := ParseSheet(fixtureSheet)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	sheet.ID = "fixture"

	pol := &Policy{
		Issuer:                "https://issuer.example",
		RegistrationEndpoint:  "https://issuer.example/gate/{{token_id}}/register",
		AuthorizationEndpoint: "https://issuer.example/v1/auth",
		TokenEndpoint:         "https://issuer.example/v1/token",
		Grants:                Grants{Provider: "token-issuance", Required: true, UnitValue: 0.01, Currency: "USD"},
		Skills: []SkillDecl{
			{Scope: "exec:run", Tool: "fixture.exec", Price: Price{Amount: 0.01, Currency: "USD", Unit: "token"}},
			{Scope: "proxy:call", Tool: "fixture.proxy", Price: Price{Amount: 0.01, Currency: "USD", Unit: "token"}},
			{Scope: "project:call", Tool: "fixture.project.<name>.call", Price: Price{Amount: 0.01, Currency: "USD", Unit: "token"}},
		},
	}

	m := Build([]Sheet{sheet}, pol)

	if m.Body.Status != Declared {
		t.Errorf("status = %q, want declared while no gate is enforced", m.Body.Status)
	}
	if len(m.Body.Surfaces) != 1 || m.Body.Surfaces[0].ID != "fixture" {
		t.Fatalf("surfaces = %+v", m.Body.Surfaces)
	}

	skills := map[string]Skill{}
	for _, s := range m.Body.Skills {
		skills[s.Scope] = s
		if s.Gate.Enforcement != Declared {
			t.Errorf("%s gate = %q, want declared", s.Scope, s.Gate.Enforcement)
		}
	}

	if got := skills["exec:run"].MinRung; got != 6 {
		t.Errorf("exec:run min rung = %d, want 6", got)
	}
	if got := skills["proxy:call"].MinRung; got != 4 {
		t.Errorf("proxy:call min rung = %d, want 4 (highest of its classes)", got)
	}
	if !skills["project:call"].Dynamic {
		t.Error("a priced runtime tool is not marked dynamic")
	}
	if skills["project:call"].Surface != "fixture" {
		t.Errorf("dynamic skill surface = %q", skills["project:call"].Surface)
	}
}

// The published manifest is a committed artifact, so it is checked against the
// sheets and policy it claims to come from: a drift between the two is the same
// false declaration the sheets exist to prevent, one layer out.
func TestCommittedManifestMatches(t *testing.T) {
	root := filepath.Join("..", "..")
	committed := filepath.Join(root, ".well-known", "lab-safety.json")
	raw, err := os.ReadFile(committed)
	if err != nil {
		t.Skipf("no published manifest: %v", err)
	}

	sheets, err := LoadSheets(filepath.Join(root, "sheets"))
	if err != nil {
		t.Fatalf("load sheets: %v", err)
	}
	pol, err := LoadPolicy(filepath.Join(root, "policy", "skills.yaml"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}

	var want, got any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("committed manifest is not JSON: %v", err)
	}
	var buf strings.Builder
	if err := Build(sheets, pol).WriteJSON(&buf); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := json.Unmarshal([]byte(buf.String()), &got); err != nil {
		t.Fatalf("generated manifest is not JSON: %v", err)
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s is stale: regenerate it with `lab-safety manifest --out %s`",
			committed, committed)
	}
}

func TestBuildEmptyPolicyIsDeclared(t *testing.T) {
	sheet, _ := ParseSheet(fixtureSheet)
	m := Build([]Sheet{sheet}, &Policy{})
	if m.Body.Status != Declared {
		t.Errorf("status = %q, want declared when nothing is priced", m.Body.Status)
	}
	if m.Body.Grants != nil {
		t.Error("empty grants were emitted")
	}
}

// The endpoints are URLs, so the loader must not accept something that only
// looks like one: a value with no scheme and no host is a typo the manifest
// would otherwise publish as a declaration.
func TestLoadPolicyValidatesEndpoints(t *testing.T) {
	tests := []struct{ name, yaml, wantErr string }{
		{"placeholder", "issuer: https://issuer.example\n", ""},
		{"templated registration",
			`registration_endpoint: "https://issuer.example/api/v1/gate/{{token_id}}/register"`, ""},
		{"no scheme", "token_endpoint: issuer.example/v1/token", "scheme"},
		{"no host", "token_endpoint: https:///v1/token", "no host"},
		{"wrong scheme", "authorization_endpoint: ftp://issuer.example/v1/auth", "scheme"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "skills.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatalf("write policy: %v", err)
			}
			_, err := LoadPolicy(path)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestPlaceholderHosts(t *testing.T) {
	placeholders := []string{
		"example.com", "example.net", "example.org", "auth.example.com",
		"issuer.example", "gate.invalid", "svc.test", "localhost", "api.localhost",
		"EXAMPLE.com.",
	}
	for _, host := range placeholders {
		if !isPlaceholderHost(host) {
			t.Errorf("%q is not recognised as reserved", host)
		}
	}

	live := []string{"lezz.go", "auth.jamesgibson.dev", "127.0.0.1", "notexample.com", "test.example.org.evil"}
	for _, host := range live {
		if isPlaceholderHost(host) {
			t.Errorf("%q was treated as reserved", host)
		}
	}
}

// The repository's own policy must stay on reserved ground while no issuer runs:
// `https://lezz.go` parses, resolves nowhere, and reads as the real domain of the
// Go module next door. That is the mistake this test keeps out of the manifest.
func TestRepoPolicyIssuerIsReserved(t *testing.T) {
	pol, err := LoadPolicy(filepath.Join("..", "..", "policy", "skills.yaml"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	for _, raw := range []string{
		pol.Issuer, pol.AuthorizationEndpoint, pol.TokenEndpoint, pol.RegistrationEndpoint,
	} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("%q does not parse: %v", raw, err)
		}
		if !isPlaceholderHost(u.Hostname()) {
			t.Errorf("%q names a host that is not reserved: the manifest would advertise an issuer "+
				"that does not exist (#32)", raw)
		}
	}
}

// A placeholder and an undeclared live endpoint are different findings: one says
// the manifest names no issuer, the other says an issuer exists with no sheet.
// Reporting the first as the second lets a placeholder pass for a surface.
func TestCheckDistinguishesPlaceholderIssuer(t *testing.T) {
	sheet, _ := ParseSheet(fixtureSheet)
	sheet.ID = "fixture"

	pol := &Policy{
		Issuer:                "https://issuer.example",
		AuthorizationEndpoint: "https://issuer.example/v1/auth",
		TokenEndpoint:         "https://issuer.example/v1/token",
		RegistrationEndpoint:  "https://issuer.example/api/v1/gate/{{token_id}}/register",
		Skills: []SkillDecl{
			{Scope: "exec:run", Tool: "fixture.exec", Price: Price{Amount: 0.01, Currency: "USD", Unit: "token"}},
		},
	}

	findings, err := Check(Options{Sheets: []Sheet{sheet}, Policy: pol})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	placeholders := 0
	for _, f := range findings {
		if f.Severity == "error" {
			t.Errorf("unexpected error: %+v", f)
		}
		if f.Code == "ISSUER-PLACEHOLDER" {
			placeholders++
		}
	}
	if placeholders != 3 {
		t.Errorf("ISSUER-PLACEHOLDER findings = %d, want 3 (auth, token, registration)", placeholders)
	}
	if f, ok := finding("ISSUER-UNDECLARED", findings); ok {
		t.Errorf("a reserved host was reported as an undeclared live surface: %+v", f)
	}

	pol.TokenEndpoint = "https://auth.jamesgibson.dev/v1/token"
	findings, err = Check(Options{Sheets: []Sheet{sheet}, Policy: pol})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, ok := finding("ISSUER-UNDECLARED", findings); !ok {
		t.Errorf("a live endpoint was not reported as undeclared: %+v", findings)
	}
	if f, ok := finding("ISSUER-PLACEHOLDER", findings); ok && f.Subject == "token" {
		t.Errorf("the live endpoint was called a placeholder: %+v", f)
	}
}

func TestCheckFindsUnknownTool(t *testing.T) {
	sheet, _ := ParseSheet(fixtureSheet)
	sheet.ID = "fixture"
	pol := &Policy{Skills: []SkillDecl{
		{Scope: "ghost:call", Tool: "fixture.nonexistent", Price: Price{Amount: 0.01}},
	}}

	findings, err := Check(Options{Sheets: []Sheet{sheet}, Policy: pol})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if f, ok := finding("SKILL-TOOL-UNKNOWN", findings); !ok || f.Severity != "error" {
		t.Fatalf("want a SKILL-TOOL-UNKNOWN error, got %+v", findings)
	}
}

func TestCheckFindsUndeclaredRung(t *testing.T) {
	sheet := Sheet{
		ID:          "bare",
		File:        "bare.md",
		Tools:       []Tool{{Name: "bare.exec", Classes: []string{"X"}}},
		RungByClass: map[string]int{"R": 0},
	}
	pol := &Policy{Skills: []SkillDecl{{Scope: "exec:run", Tool: "bare.exec", Price: Price{Amount: 1}}}}

	findings, err := Check(Options{Sheets: []Sheet{sheet}, Policy: pol})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if f, ok := finding("SKILL-RUNG-UNDECLARED", findings); !ok || f.Severity != "error" {
		t.Fatalf("want SKILL-RUNG-UNDECLARED, got %+v", findings)
	}
}

func TestCheckReportsDeclaredGate(t *testing.T) {
	sheet, _ := ParseSheet(fixtureSheet)
	sheet.ID = "fixture"
	pol := &Policy{Skills: []SkillDecl{
		{Scope: "exec:run", Tool: "fixture.exec", Price: Price{Amount: 0.01}},
	}}

	findings, err := Check(Options{Sheets: []Sheet{sheet}, Policy: pol})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if f, ok := finding("GATE-NOT-ENFORCED", findings); !ok || f.Severity != "note" {
		t.Fatalf("want a GATE-NOT-ENFORCED note, got %+v", findings)
	}
	for _, f := range findings {
		if f.Severity == "error" {
			t.Errorf("unexpected error: %+v", f)
		}
	}
}

func TestCheckProbe(t *testing.T) {
	sheet, _ := ParseSheet(fixtureSheet)
	sheet.ID = "fixture"

	tests := []struct {
		name     string
		served   []string
		wantCode string
	}{
		{"match", []string{"fixture.read", "fixture.exec", "fixture.proxy"}, "PROBE-OK"},
		{"missing", []string{"fixture.read", "fixture.exec"}, "PROBE-TOOLS-MISSING"},
		{"extra", []string{"fixture.read", "fixture.exec", "fixture.proxy", "fixture.undeclared"}, "PROBE-TOOLS-EXTRA"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(toolsListHandler(tc.served, false))
			defer srv.Close()

			findings, err := Check(Options{
				Sheets:       []Sheet{sheet},
				Policy:       &Policy{},
				ProbeURL:     srv.URL,
				ProbeSurface: "fixture",
			})
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			f, ok := finding(tc.wantCode, findings)
			if !ok {
				t.Fatalf("want %s, got %+v", tc.wantCode, findings)
			}
			if tc.wantCode != "PROBE-OK" && f.Severity != "error" {
				t.Errorf("%s severity = %q, want error", tc.wantCode, f.Severity)
			}
		})
	}
}

func TestCheckProbeReadsSSE(t *testing.T) {
	srv := httptest.NewServer(toolsListHandler([]string{"fixture.read", "fixture.exec", "fixture.proxy"}, true))
	defer srv.Close()

	sheet, _ := ParseSheet(fixtureSheet)
	sheet.ID = "fixture"

	findings, err := Check(Options{
		Sheets:       []Sheet{sheet},
		Policy:       &Policy{},
		ProbeURL:     srv.URL,
		ProbeSurface: "fixture",
	})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, ok := finding("PROBE-OK", findings); !ok {
		t.Fatalf("an SSE tools/list was not read: %+v", findings)
	}
}

func toolsListHandler(names []string, sse bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		tools := make([]map[string]string, 0, len(names))
		for _, n := range names {
			tools = append(tools, map[string]string{"name": n})
		}
		payload, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"tools": tools},
		})
		if sse {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}
}

func finding(code string, findings []Finding) (Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func hasToolClass(s Sheet, name, class string) bool {
	for _, t := range s.Tools {
		if t.Name != name {
			continue
		}
		for _, c := range t.Classes {
			if c == class {
				return true
			}
		}
	}
	return false
}

func hasDynamic(s Sheet, name string) bool {
	for _, t := range s.Tools {
		if t.Name == name {
			return t.Dynamic
		}
	}
	return false
}

func keys(m map[string]Sheet) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
