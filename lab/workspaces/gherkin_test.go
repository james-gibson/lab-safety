package workspaces

import (
	"os"
	"testing"
)

// testKey is the session's seed — the material the spec's isotope is signed with.
var testKey = []byte("seed-42")

// gateRef is the Phase-0 gate as a spec reference.
func gateRef(t *testing.T) SpecRef {
	t.Helper()
	spec, err := GateRef(testKey)
	if err != nil {
		t.Fatalf("read the gate %s: %v", GatePath, err)
	}
	return spec
}

// TestSpecComesFromTheGherkin is the guard on the whole idea: if the reading
// silently yields nothing, this fails rather than the expectation quietly
// becoming empty and the check passing for the wrong reason.
func TestSpecComesFromTheGherkin(t *testing.T) {
	want, err := gateRef(t).Expectation()
	if err != nil {
		t.Fatal(err)
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

// The spec's isotope is deterministic: the same source signs to the same thing.
func TestSpecIsotopeIsDeterministic(t *testing.T) {
	spec := gateRef(t)
	one, err := spec.Isotope()
	if err != nil {
		t.Fatal(err)
	}
	two, err := spec.Isotope()
	if err != nil {
		t.Fatal(err)
	}
	if one.Signature == "" {
		t.Fatal("empty signature")
	}
	if one.Signature != two.Signature {
		t.Errorf("signature is not deterministic: %s vs %s", one.Signature, two.Signature)
	}
	if one.Family != "workspace-vfs" || one.Version != 1 {
		t.Errorf("isotope identity = %s v%d", one.Family, one.Version)
	}
	// a different seed is a different isotope
	other, err := LoadSpec(GatePath, []byte("another-seed"), "workspace-vfs", 1)
	if err != nil {
		t.Fatal(err)
	}
	otherIso, err := other.Isotope()
	if err != nil {
		t.Fatal(err)
	}
	if otherIso.Signature == one.Signature {
		t.Error("a different key produced the same signature")
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
	spec, err := LoadSpec(p, testKey, "x", 1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := spec.Expectation()
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
