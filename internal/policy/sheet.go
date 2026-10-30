// Package policy turns the lab's safety data sheets into the machine-readable
// policy object lab-safety publishes.
//
// A sheet (sheets/<surface>.md) is the readable, 16-section declaration of one
// MCP surface. It is also the only place the lab records what a surface
// actually serves, so the manifest is derived from it rather than authored
// twice: a sheet that stops matching the manifest, or a manifest that names a
// tool no sheet serves, is a finding instead of a stale sentence
// (sheets/README.md, adhd#24).
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Hazard classes from the sheet legend (sheets/README.md). E, X and H are the
// classes that make a surface hazardous beyond the lab.
const hazardClasses = "RCWEXH"

// Tool is one tool a sheet declares, with the hazard classes it carries.
type Tool struct {
	Name    string   `json:"name"`
	Classes []string `json:"classes"`
	// Dynamic is true for tools registered at runtime (e.g. by
	// adhd.cluster.join) rather than present in a cold tools/list.
	Dynamic bool `json:"dynamic,omitempty"`
}

// Control is one containment measure from §5, with the status the sheet gives
// it. Enforced is false for anything the sheet calls "declared", "not
// implemented", or "not enforced".
type Control struct {
	Name     string `json:"name"`
	Owner    string `json:"owner,omitempty"`
	Status   string `json:"status"`
	Enforced bool   `json:"enforced"`
}

// Sheet is the part of a safety data sheet the manifest needs: identification
// (§1), composition (§3), containment (§5) and rung gating (§7).
type Sheet struct {
	File      string `json:"file"`
	ID        string `json:"id"`
	IDText    string `json:"id_text,omitempty"`
	Owner     string `json:"owner"`
	Endpoint  string `json:"endpoint"`
	Transport string `json:"transport"`
	Protocol  string `json:"protocol,omitempty"`

	Tools    []Tool    `json:"tools"`
	Controls []Control `json:"controls"`

	// RungByClass is the declared minimum rung per hazard class, from §7.
	RungByClass map[string]int `json:"min_rung_by_class,omitempty"`
	// RungStatus is §7's own wording; RungEnforced is false unless the sheet
	// says the gate is enforced by the surface.
	RungStatus   string `json:"rung_status,omitempty"`
	RungEnforced bool   `json:"rung_enforced"`
}

// Classes returns the distinct hazard classes the sheet's tools carry, sorted.
func (s Sheet) Classes() []string {
	seen := map[string]bool{}
	for _, t := range s.Tools {
		for _, c := range t.Classes {
			seen[c] = true
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// LoadSheets parses every safety data sheet in dir. README.md is the legend,
// not a sheet, and is skipped.
func LoadSheets(dir string) ([]Sheet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read sheets dir: %w", err)
	}

	var sheets []Sheet
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path) //nolint:gosec // G304: the sheets directory is the operator's own declaration
		if err != nil {
			return nil, fmt.Errorf("read sheet %s: %w", path, err)
		}
		sheet, err := ParseSheet(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse sheet %s: %w", path, err)
		}
		// The label is the sheet's own name, not the path it was read through:
		// the manifest is a published artifact and must not change with the
		// directory it was generated from.
		sheet.File = e.Name()
		if sheet.ID == "" {
			sheet.ID = strings.TrimSuffix(e.Name(), ".md")
		}
		sheets = append(sheets, sheet)
	}

	sort.Slice(sheets, func(i, j int) bool { return sheets[i].ID < sheets[j].ID })
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no safety data sheets in %s", dir)
	}
	return sheets, nil
}

// sectionRe splits a sheet on its section headings ("## §5 Fire-fighting …").
var sectionRe = regexp.MustCompile(`(?m)^## §(\d+)`)

// ParseSheet extracts the manifest-relevant sections from one sheet. Sections
// are addressed by number so a reworded heading does not break the parse.
func ParseSheet(text string) (Sheet, error) {
	sections := splitSections(text)
	var sheet Sheet

	if s1, ok := sections[1]; ok {
		fields := firstTable(s1)
		sheet.IDText = fields["surface id"]
		sheet.Owner = fields["owner repo"]
		sheet.Endpoint = fields["endpoint"]
		sheet.Transport = fields["transport"]
		sheet.Protocol = fields["protocol version"]
		sheet.ID = quotedToken(sheet.IDText)
	}

	if s3, ok := sections[3]; ok {
		sheet.Tools = parseTools(s3)
	}
	if s5, ok := sections[5]; ok {
		sheet.Controls = parseControls(s5)
	}
	if s7, ok := sections[7]; ok {
		sheet.RungByClass = parseRungs(s7, sheet.Classes())
		sheet.RungStatus, sheet.RungEnforced = statusLine(s7)
	}

	if len(sheet.Tools) == 0 {
		return sheet, fmt.Errorf("no tools declared in §3")
	}
	return sheet, nil
}

// splitSections returns the body of each numbered section, keyed by number.
func splitSections(text string) map[int]string {
	out := map[int]string{}
	matches := sectionRe.FindAllStringSubmatchIndex(text, -1)
	for i, m := range matches {
		n, err := strconv.Atoi(text[m[2]:m[3]])
		if err != nil {
			continue
		}
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		out[n] = text[m[1]:end]
	}
	return out
}

// firstTable parses the first two-column table in a section into a map. Values
// keep their markdown; keys are trimmed and unquoted.
func firstTable(section string) map[string]string {
	out := map[string]string{}
	for _, row := range tableRows(section) {
		if len(row) < 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(unquote(row[0])))
		if key == "" || key == "field" {
			continue
		}
		out[key] = strings.TrimSpace(row[1])
	}
	return out
}

// parseTools reads every inventory row in §3: the static table and, when the
// sheet has one, the dynamic-tools table that follows the "**Dynamic tools**"
// line. A row is recognised by a class cell, so column layouts may differ.
func parseTools(section string) []Tool {
	var tools []Tool
	dynamic := false

	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, "**Dynamic tools**") {
			dynamic = true
			continue
		}
		row, ok := parseRow(line)
		if !ok {
			continue
		}
		for i, cell := range row {
			classes := ParseClasses(cell)
			if len(classes) == 0 || i == 0 {
				continue
			}
			name := strings.TrimSpace(unquote(row[i-1]))
			if name == "" || name == "#" || isNumeric(name) {
				continue
			}
			tools = append(tools, Tool{Name: name, Classes: classes, Dynamic: dynamic})
			break
		}
	}
	return tools
}

// parseControls reads §5's table. The status is the last cell; both "control |
// value | status" (the alarm) and "control | who owns it | status" (adhd) are
// accepted.
func parseControls(section string) []Control {
	var controls []Control
	for _, row := range tableRows(section) {
		if len(row) < 2 {
			continue
		}
		name := strings.TrimSpace(unquote(row[0]))
		if strings.EqualFold(name, "control") {
			continue
		}
		status := strings.TrimSpace(row[len(row)-1])
		owner := ""
		if len(row) >= 3 {
			owner = strings.TrimSpace(unquote(row[1]))
		}
		controls = append(controls, Control{
			Name:     name,
			Owner:    owner,
			Status:   normaliseStatus(status),
			Enforced: isEnforced(status),
		})
	}
	return controls
}

// A status sentence may wrap across two lines in the markdown, so the
// paragraph is flattened before it is matched and the capture stops at the
// first full stop.
var statusInTextRe = regexp.MustCompile(`(?i)Status:\s*(.+?)(?:\.\s|\.$|$)`)

// statusLine returns the "Status: …" sentence of the paragraph that declares
// the rung gate, and whether it says enforced. It is scoped to that paragraph
// because §7 also carries statuses for other things (adhd's outbound auth says
// "enforced-with-caveat" before the rung sentence, and must not be read as the
// gate's status).
func statusLine(section string) (string, bool) {
	for _, paragraph := range strings.Split(section, "\n\n") {
		if !strings.Contains(strings.ToLower(paragraph), "rung") {
			continue
		}
		flat := strings.Join(strings.Fields(paragraph), " ")
		if m := statusInTextRe.FindStringSubmatch(flat); m != nil {
			status := strings.TrimSpace(m[1])
			return normaliseStatus(status), isEnforced(status)
		}
	}
	return "", false
}

var rungArrowRe = regexp.MustCompile("([RCWEXH][RCWEXH\\s,/\x60]*?)\\s*(?:→|->)\\s*rung\\s*(\\d+)")

var rungFlatRe = regexp.MustCompile(`minimum rung\s*(\d+)`)

// parseRungs reads §7's declared minimum rung per class. Two shapes exist in
// the sheets: an explicit "`R`/`C` → rung 0; `W` → rung 3" list, and a flat
// "declared minimum rung 0 (`R` only)" sentence that applies to every class the
// surface serves.
func parseRungs(section string, classes []string) map[string]int {
	out := map[string]int{}
	for _, m := range rungArrowRe.FindAllStringSubmatch(section, -1) {
		rung, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		for _, class := range ParseClasses(m[1]) {
			out[class] = rung
		}
	}
	if len(out) > 0 {
		return out
	}
	if m := rungFlatRe.FindStringSubmatch(section); m != nil {
		rung, err := strconv.Atoi(m[1])
		if err == nil {
			for _, class := range classes {
				out[class] = rung
			}
		}
	}
	return out
}

// ParseClasses normalises a class cell ("`R`,`E`" or "R/C") into letters. A
// cell that contains anything other than legend letters and the separators the
// sheets use is rejected outright: otherwise a prose cell such as "hosted
// service health summary" would be read as the classes E and H, and a
// non-tool table would be mistaken for an inventory.
func ParseClasses(cell string) []string {
	s := strings.ToUpper(cell)
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	for _, r := range s {
		if strings.ContainsRune(hazardClasses, r) {
			c := string(r)
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
			continue
		}
		switch r {
		case ' ', ',', '/', '|', '+', '&', '.':
			continue
		default:
			return nil
		}
	}
	sort.Strings(out)
	return out
}

// tableRows returns the parsed cells of every table row in a section, skipping
// header separators.
func tableRows(section string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(section, "\n") {
		row, ok := parseRow(line)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// parseRow splits "| a | b | c |" into cells, reporting false for lines that
// are not table rows or are the header separator.
func parseRow(line string) ([]string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") || len(trimmed) < 2 {
		return nil, false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(trimmed, "|"), "|")
	cells := strings.Split(body, "|")
	for i, c := range cells {
		cells[i] = strings.TrimSpace(c)
	}
	if isSeparator(cells) {
		return nil, false
	}
	return cells, true
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if c == "" {
			continue
		}
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil
}

// unquote strips the markdown code ticks and emphasis a sheet wraps values in.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	s = strings.ReplaceAll(s, "**", "")
	return strings.TrimSpace(s)
}

var tokenRe = regexp.MustCompile("`([^`]+)`")

// quotedToken returns the first code-quoted token, which is how a sheet names
// its surface in §1 ("`adhd` (dashboard & MCP server)"). The file name is the
// fallback when the sheet has no such token.
func quotedToken(s string) string {
	if m := tokenRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// normaliseStatus lowercases and strips emphasis so statuses compare.
func normaliseStatus(s string) string {
	s = strings.ToLower(unquote(s))
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".")
	return s
}

// isEnforced reports whether a status cell claims the surface enforces the
// control. "enforced — note …" counts; "enforced (leave); declared (gate)",
// "declared, not implemented" and "not enforced" do not: a control that is only
// declared is exactly what the manifest must not overstate.
func isEnforced(status string) bool {
	s := normaliseStatus(status)
	if !strings.HasPrefix(s, "enforced") {
		return false
	}
	return !strings.Contains(s, "declared") && !strings.Contains(s, "not implemented")
}
