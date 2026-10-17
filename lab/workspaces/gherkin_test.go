package workspaces

import (
	"os"
	"strings"
	"testing"
)

// TestSpecComesFromTheGherkin is the guard on the whole idea: if the parser
// silently reads nothing, this fails rather than the expectation quietly
// becoming empty.
func TestSpecComesFromTheGherkin(t *testing.T) {
	want, err := GateSpec()
	if err != nil {
		t.Fatalf("read the gate %s: %v", GatePath, err)
	}
	if !equalStrings(want.Tools, []string{"echo"}) {
		t.Errorf("tools = %v, want [echo]", want.Tools)
	}
	if !equalStrings(want.Resources, []string{"resource://workspace/config"}) {
		t.Errorf("resources = %v, want [resource://workspace/config]", want.Resources)
	}
	if !equalStrings(want.Prompts, []string{"summarize"}) {
		t.Errorf("prompts = %v, want [summarize]", want.Prompts)
	}
}

func TestParseGherkin(t *testing.T) {
	src := `@lab-safety
Feature: a session filesystem

  # a comment
  Rule: §1 the conventions

    Background:
      Given a session "42"

    Scenario: the layout
      Given the resource file "/resources/a/b.json"
      And an executable endpoint "/tools/echo@"
      Then the tree contains "/data/task.json"

    Scenario Outline: the hosts
      Given the template "/prompts/summarize.tmpl"
      Then the session is "/"

      Examples:
        | host   |
        | tmpdir |
`
	f := ParseGherkin(src)
	if f.Name != "a session filesystem" {
		t.Errorf("feature name = %q", f.Name)
	}
	if len(f.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(f.Rules))
	}
	if f.Rules[0].Name != "§1 the conventions" {
		t.Errorf("rule name = %q", f.Rules[0].Name)
	}
	scs := f.Rules[0].Scenarios
	if len(scs) != 3 {
		t.Fatalf("scenarios = %d, want 3 (background + 2)", len(scs))
	}
	if scs[0].Name != "Background" {
		t.Errorf("scenario 0 = %q, want Background", scs[0].Name)
	}
	if got := scs[1].Steps[0]; got.Keyword != "Given" || got.Text != `the resource file "/resources/a/b.json"` {
		t.Errorf("step = %+v", got)
	}
	// the Examples table is not steps
	if len(scs[2].Steps) != 2 {
		t.Fatalf("outline steps = %+v", scs[2].Steps)
	}
	for _, st := range scs[2].Steps {
		if strings.Contains(st.Text, "|") {
			t.Errorf("a table row parsed as a step: %+v", st)
		}
	}
}

func TestSpecStepDefinitions(t *testing.T) {
	dir := t.TempDir()
	src := "Feature: x\n" +
		"  Rule: r\n" +
		"    Scenario: s\n" +
		"      Given the resource file \"/resources/one/two.json\"\n" +
		"      And an executable endpoint \"/tools/echo@\"\n" +
		"      And the template \"/prompts/summarize.tmpl\"\n" +
		"      And the resource file \"/resources/one/two.json\"\n"
	p := dir + "/spec.feature"
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Spec(p)
	if err != nil {
		t.Fatal(err)
	}
	if !equalStrings(got.Resources, []string{"resource://one/two"}) {
		t.Errorf("resources = %v (duplicates must collapse)", got.Resources)
	}
	if !equalStrings(got.Tools, []string{"echo"}) {
		t.Errorf("tools = %v", got.Tools)
	}
	if !equalStrings(got.Prompts, []string{"summarize"}) {
		t.Errorf("prompts = %v", got.Prompts)
	}
}
