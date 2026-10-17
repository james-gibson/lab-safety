package workspaces

import (
	"os"
	"path"
	"sort"
	"strings"
)

// GatePath is the Phase-0 gate, relative to this module.
const GatePath = "../../sheets/workspaces/minimal.feature"

// ---- a small Gherkin reader ----

// Step is one Gherkin step: its keyword and its text.
type Step struct {
	Keyword string // Given, When, Then, And, But, *
	Text    string
}

// Scenario is one scenario (or the Background) and its steps.
type Scenario struct {
	Name  string
	Steps []Step
}

// Rule is one Rule, or the anonymous rule a feature opens with.
type Rule struct {
	Name      string
	Scenarios []Scenario
}

// Feature is a parsed .feature file.
type Feature struct {
	Name  string
	Rules []Rule
}

var stepKeywords = []string{"Given", "When", "Then", "And", "But", "*"}

func splitStep(line string) (string, string, bool) {
	for _, kw := range stepKeywords {
		if strings.HasPrefix(line, kw+" ") {
			return kw, strings.TrimSpace(strings.TrimPrefix(line, kw)), true
		}
	}
	return "", "", false
}

// ParseGherkin reads the structure of a .feature source — rules, scenarios and
// steps. Just enough of the language to read the spec: tags, comments, doc
// strings and Examples tables are skipped.
func ParseGherkin(src string) Feature {
	var f Feature
	cur, curSc := -1, -1
	openRule := func() {
		if cur < 0 {
			f.Rules = append(f.Rules, Rule{})
			cur = len(f.Rules) - 1
		}
	}
	openScenario := func(name string) {
		openRule()
		f.Rules[cur].Scenarios = append(f.Rules[cur].Scenarios, Scenario{Name: name})
		curSc = len(f.Rules[cur].Scenarios) - 1
	}

	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "", strings.HasPrefix(line, "#"), strings.HasPrefix(line, "@"):
			continue
		case strings.HasPrefix(line, "Feature:"):
			f.Name = strings.TrimSpace(strings.TrimPrefix(line, "Feature:"))
		case strings.HasPrefix(line, "Rule:"):
			f.Rules = append(f.Rules, Rule{Name: strings.TrimSpace(strings.TrimPrefix(line, "Rule:"))})
			cur, curSc = len(f.Rules)-1, -1
		case strings.HasPrefix(line, "Scenario Outline:"):
			openScenario(strings.TrimSpace(strings.TrimPrefix(line, "Scenario Outline:")))
		case strings.HasPrefix(line, "Scenario:"):
			openScenario(strings.TrimSpace(strings.TrimPrefix(line, "Scenario:")))
		case strings.HasPrefix(line, "Background:"):
			openScenario("Background")
		default:
			if kw, text, ok := splitStep(line); ok {
				openRule()
				if curSc < 0 {
					openScenario("")
				}
				f.Rules[cur].Scenarios[curSc].Steps = append(f.Rules[cur].Scenarios[curSc].Steps, Step{Keyword: kw, Text: text})
			}
			// anything else (Examples header, table rows, …) is skipped
		}
	}
	return f
}

// ParseFeatureFile reads and parses a .feature file.
func ParseFeatureFile(p string) (Feature, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return Feature{}, err
	}
	return ParseGherkin(string(b)), nil
}

// ---- step definitions: the spec -> the expectation ----

// The mapping lives here, once, and the store's own listing uses it too — so the
// spec and the observation cannot drift apart.
func resourceURI(p string) string {
	rel := strings.TrimPrefix(p, DirResources+"/")
	rel = strings.TrimSuffix(rel, path.Ext(rel))
	return "resource://" + rel
}

func toolName(p string) string { return path.Base(strings.TrimSuffix(p, "@")) }

func promptName(p string) string { return path.Base(strings.TrimSuffix(p, ".tmpl")) }

func quoted(s string) string {
	i := strings.Index(s, `"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i+1:], `"`)
	if j < 0 {
		return ""
	}
	return s[i+1 : i+1+j]
}

func sortedUnique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Spec reads the expectation from the Gherkin. The spec is the origin (register,
// the 42i frame): the expectation is not a hand-written declaration, it is what
// the gate declares — the resource files, the executable endpoints and the
// templates that its scenarios name.
func Spec(featurePath string) (Expectation, error) {
	f, err := ParseFeatureFile(featurePath)
	if err != nil {
		return Expectation{}, err
	}
	var e Expectation
	for _, r := range f.Rules {
		for _, sc := range r.Scenarios {
			for _, st := range sc.Steps {
				p := quoted(st.Text)
				switch {
				case strings.HasPrefix(st.Text, "the resource file "):
					e.Resources = append(e.Resources, resourceURI(p))
				case strings.HasPrefix(st.Text, "an executable endpoint "):
					e.Tools = append(e.Tools, toolName(p))
				case strings.HasPrefix(st.Text, "the template "):
					e.Prompts = append(e.Prompts, promptName(p))
				}
			}
		}
	}
	e.Tools = sortedUnique(e.Tools)
	e.Resources = sortedUnique(e.Resources)
	e.Prompts = sortedUnique(e.Prompts)
	return e, nil
}

// GateSpec reads the expectation from the Phase-0 gate.
func GateSpec() (Expectation, error) { return Spec(GatePath) }
