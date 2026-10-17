package workspaces

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/james-gibson/isotope"
)

// GatePath is the Phase-0 gate, relative to this module.
const GatePath = "../../sheets/workspaces/minimal.feature"

// The parse is NOT ours. `isotope.ParseGherkin` already exists, and the register
// (§3) already names `isotope.CanonicalizeGherkinToIsotope` as the standard for
// command identity / content address — so the module depends on it rather than
// hand-rolling a reader (A9: standard tooling suffices).
//
// What we add is only the *step definitions*: what the gate's steps declare.

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
// the 42i frame): the expectation is what the gate *declares* — the resource
// files, the executable endpoints and the templates its scenarios name.
func Spec(featurePath string) (Expectation, error) {
	b, err := os.ReadFile(featurePath)
	if err != nil {
		return Expectation{}, err
	}
	ast, err := isotope.ParseGherkin(string(b))
	if err != nil {
		return Expectation{}, err
	}

	var e Expectation
	for _, f := range ast.Features {
		for _, sc := range f.Scenarios {
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
