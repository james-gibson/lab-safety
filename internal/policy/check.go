package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Finding is one thing the checker has to say about a declaration. Severity
// "error" means the declaration contradicts itself or the surface it describes;
// "note" means the declaration is honest but not enforced, which the lab allows
// as long as it is labelled.
type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Subject  string `json:"subject,omitempty"`
	Detail   string `json:"detail"`
}

// Options configures a check.
type Options struct {
	Sheets []Sheet
	Policy *Policy
	// ProbeURL, when set, is a live MCP endpoint to compare against the
	// declared tools of ProbeSurface.
	ProbeURL     string
	ProbeSurface string
	// Client is used for the probe; nil means a 5s-timeout default.
	Client *http.Client
}

// Check validates the declaration. It returns findings rather than an error:
// a declaration that is merely unenforced is not an error, and only a
// contradiction or a mismatched surface fails the check.
func Check(opts Options) ([]Finding, error) {
	var findings []Finding

	if opts.Policy == nil {
		return nil, fmt.Errorf("no policy loaded")
	}

	for _, skill := range opts.Policy.Skills {
		sheet, tool, ok := findTool(opts.Sheets, skill.Tool)
		switch {
		case !ok:
			findings = append(findings, Finding{
				Severity: "error",
				Code:     "SKILL-TOOL-UNKNOWN",
				Subject:  skill.Scope,
				Detail: fmt.Sprintf("names tool %q, which no safety data sheet declares: a skill "+
					"cannot be gated for a surface nobody has declared", skill.Tool),
			})
			continue
		}

		// Every class the tool carries needs a declared rung, or the skill's
		// min_rung is a number with nothing behind it.
		var missing []string
		for _, class := range tool.Classes {
			if _, ok := sheet.RungByClass[class]; !ok {
				missing = append(missing, class)
			}
		}
		if len(missing) > 0 {
			findings = append(findings, Finding{
				Severity: "error",
				Code:     "SKILL-RUNG-UNDECLARED",
				Subject:  skill.Scope,
				Detail: fmt.Sprintf("%s/%s carries class %s with no declared minimum rung in §7 of %s",
					sheet.ID, tool.Name, strings.Join(missing, ", "), sheet.File),
			})
		}

		if skill.Price.Amount == 0 {
			findings = append(findings, Finding{
				Severity: "note",
				Code:     "SKILL-PRICE-ZERO",
				Subject:  skill.Scope,
				Detail:   "priced at zero, so the token is free and the skill is gated by nothing",
			})
		}
	}

	// Token gates. One note for the set: the manifest publishes a price for
	// every skill, and no surface checks a token yet.
	if gated := len(opts.Policy.Skills); gated > 0 {
		findings = append(findings, Finding{
			Severity: "note",
			Code:     "GATE-NOT-ENFORCED",
			Subject:  fmt.Sprintf("%d skill(s)", gated),
			Detail: "a token is required by declaration, but no surface reads one: " +
				"the price is declared, so a token currently costs nothing",
		})
	}

	// The issuance endpoints are a surface too. By sheets/README.md a surface
	// cannot be gated until it is declared, so an undeclared issuer is a hole in
	// the model rather than a cosmetic gap.
	for _, endpoint := range []struct {
		kind, url string
	}{
		{"authorization", opts.Policy.AuthorizationEndpoint},
		{"token", opts.Policy.TokenEndpoint},
		{"registration", opts.Policy.RegistrationEndpoint},
	} {
		if endpoint.url == "" {
			continue
		}
		findings = append(findings, Finding{
			Severity: "note",
			Code:     "ISSUER-UNDECLARED",
			Subject:  endpoint.kind,
			Detail: fmt.Sprintf("%s is published as an issuance endpoint, but no safety data sheet "+
				"declares the surface that serves it", endpoint.url),
		})
	}

	// The temporal cloak cannot be enforced from the server side, and the lab's
	// own lattice makes timing a thing to be independent of, not a credential.
	if opts.Policy.Timing != nil {
		findings = append(findings, Finding{
			Severity: "note",
			Code:     "TIMING-UNENFORCEABLE",
			Subject:  fmt.Sprintf("%dms", opts.Policy.Timing.StandardizedLatencyMS),
			Detail: "a client-side beat is not verifiable by the surface being called; a caller that " +
				"reads the manifest satisfies it, and rung 4 of the 42i lattice requires timing to be " +
				"uncorrelated",
		})
	}

	for _, sheet := range opts.Sheets {
		if !sheet.RungEnforced {
			detail := "§7 declares no rung status: whether the surface reads rungs is unknown"
			if sheet.RungStatus != "" {
				detail = fmt.Sprintf("§7 status %q: the declared rungs are not read by the surface", sheet.RungStatus)
			}
			findings = append(findings, Finding{
				Severity: "note",
				Code:     "RUNG-NOT-ENFORCED",
				Subject:  sheet.ID,
				Detail:   detail,
			})
		}

		// Split "declared only" from "partly enforced": adhd's kill switch is
		// enforced for leave and declared for the gate, and collapsing the two
		// would overstate the gap in one direction and understate it in the
		// other.
		var declared, partial []string
		for _, c := range sheet.Controls {
			if c.Enforced {
				continue
			}
			if strings.Contains(c.Status, "enforced") {
				partial = append(partial, c.Name)
				continue
			}
			declared = append(declared, c.Name)
		}
		if len(declared) > 0 || len(partial) > 0 {
			detail := ""
			if len(declared) > 0 {
				detail = fmt.Sprintf("%d of %d §5 controls are declared only: %s",
					len(declared), len(sheet.Controls), strings.Join(declared, "; "))
			}
			if len(partial) > 0 {
				if detail != "" {
					detail += " — "
				}
				detail += fmt.Sprintf("partly enforced: %s", strings.Join(partial, "; "))
			}
			findings = append(findings, Finding{
				Severity: "note",
				Code:     "CONTROL-NOT-ENFORCED",
				Subject:  sheet.ID,
				Detail:   detail,
			})
		}
	}

	if opts.ProbeURL != "" {
		probe, err := probeTools(opts)
		if err != nil {
			findings = append(findings, Finding{
				Severity: "error",
				Code:     "PROBE-FAILED",
				Subject:  opts.ProbeSurface,
				Detail:   err.Error(),
			})
		} else {
			findings = append(findings, compareTools(opts.Sheets, opts.ProbeSurface, probe)...)
		}
	}

	sortFindings(findings)
	return findings, nil
}

// compareTools reports the two directions of a false declaration: tools the
// sheet claims but the surface does not serve (adhd#24), and tools the surface
// serves that no sheet declared.
func compareTools(sheets []Sheet, surfaceID string, served []string) []Finding {
	sheet, ok := findSheet(sheets, surfaceID)
	if !ok {
		return []Finding{{
			Severity: "error",
			Code:     "PROBE-SURFACE-UNKNOWN",
			Subject:  surfaceID,
			Detail:   "the probed surface has no safety data sheet",
		}}
	}

	var findings []Finding
	declared := map[string]bool{}
	for _, tool := range sheet.Tools {
		// A dynamic tool is registered at runtime and absent from a cold
		// tools/list, so it is neither expected nor a surprise.
		if tool.Dynamic || strings.Contains(tool.Name, "<") {
			continue
		}
		declared[tool.Name] = true
	}

	servedSet := map[string]bool{}
	for _, name := range served {
		servedSet[name] = true
	}

	var missing, extra []string
	for name := range declared {
		if !servedSet[name] {
			missing = append(missing, name)
		}
	}
	for _, name := range served {
		if !declared[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		findings = append(findings, Finding{
			Severity: "error",
			Code:     "PROBE-TOOLS-MISSING",
			Subject:  surfaceID,
			Detail: fmt.Sprintf("declared but not served: %s — a false declaration (§16, adhd#24)",
				strings.Join(missing, ", ")),
		})
	}
	if len(extra) > 0 {
		findings = append(findings, Finding{
			Severity: "error",
			Code:     "PROBE-TOOLS-EXTRA",
			Subject:  surfaceID,
			Detail: fmt.Sprintf("served but undeclared: %s — the surface exposes more than its sheet allows",
				strings.Join(extra, ", ")),
		})
	}
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Severity: "note",
			Code:     "PROBE-OK",
			Subject:  surfaceID,
			Detail:   fmt.Sprintf("%d declared tools served, none undeclared", len(declared)),
		})
	}
	return findings
}

// probeTools performs a tools/list against a live streamable-HTTP MCP endpoint.
func probeTools(opts Options) ([]string, error) {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, opts.ProbeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("probe %s: %w", opts.ProbeURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A streamable-HTTP server may answer with either JSON or an SSE stream.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read probe response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("probe %s: %s", opts.ProbeURL, resp.Status)
	}

	payload := jsonPayload(raw)
	var decoded struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("decode tools/list: %w", err)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("tools/list: %s", decoded.Error.Message)
	}

	names := make([]string, 0, len(decoded.Result.Tools))
	for _, tool := range decoded.Result.Tools {
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("tools/list returned no tools")
	}
	return names, nil
}

// jsonPayload returns the JSON document from a plain body or from an SSE
// stream ("data: {...}").
func jsonPayload(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return trimmed
	}
	for _, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			candidate := bytes.TrimSpace([]byte(after))
			if len(candidate) > 0 && candidate[0] == '{' {
				return candidate
			}
		}
	}
	return trimmed
}

func findSheet(sheets []Sheet, id string) (Sheet, bool) {
	for _, s := range sheets {
		if s.ID == id {
			return s, true
		}
	}
	return Sheet{}, false
}

func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity == "error"
		}
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Subject < findings[j].Subject
	})
}
