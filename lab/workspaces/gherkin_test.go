package workspaces

import (
	"os"
	"testing"
)

// TestSpecComesFromTheGherkin is the guard on the whole idea: if the parse
// silently yields nothing, this fails rather than the expectation quietly
// becoming empty and the check passing for the wrong reason.
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
