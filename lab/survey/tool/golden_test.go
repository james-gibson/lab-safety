package tool_test

// The golden fixture.
//
// The thread has no tool CSV rows, so the model survey's "reproduce the issue's
// rows" golden cannot be reused. The independent expected output here is the
// raw material itself: testdata/issue-profiles.json is the 13 profile objects
// from the three comments, verbatim. The test asserts the normalized profiles
// are those objects — no answer is rewritten — and that the projection is total
// over the authored vocabulary.

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	survey "github.com/james-gibson/lab-safety/survey/internal/survey"
)

func TestToolProfilesAreTheRawAnswers(t *testing.T) {
	raw, err := readRawProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 13 {
		t.Fatalf("testdata/issue-profiles.json has %d profiles, want 13", len(raw))
	}
	want := map[string]map[string]any{}
	for _, p := range raw {
		name, _ := p["toolName"].(string)
		if name == "" {
			t.Fatal("a raw profile has no toolName")
		}
		if _, dup := want[name]; dup {
			t.Fatalf("the golden fixture has a duplicate toolName %q", name)
		}
		want[name] = p
	}

	ds := survey.Tool()
	profiles, problems, err := survey.LoadProfilesWith(ds, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("profiles: %v", problems)
	}
	if len(profiles) != 13 {
		t.Fatalf("have %d profiles, want 13", len(profiles))
	}

	for _, p := range profiles {
		name, _ := p.Answer["toolName"].(string)
		r, ok := want[name]
		if !ok {
			t.Fatalf("%s: toolName %q is not in the golden fixture", p.AgentID, name)
		}
		if !reflect.DeepEqual(p.Answer, r) {
			t.Errorf("%s: the normalized answer is not the raw profile (the raw answer must be preserved verbatim)", p.AgentID)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		names := make([]string, 0, len(want))
		for k := range want {
			names = append(names, k)
		}
		t.Fatalf("golden fixture profiles with no normalized counterpart: %v", names)
	}
}

func TestToolProjectionIsTotal(t *testing.T) {
	ds := survey.Tool()
	sc, err := survey.LoadSchemas(".")
	if err != nil {
		t.Fatal(err)
	}
	profiles, problems, err := survey.LoadProfilesWith(ds, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("profiles: %v", problems)
	}
	for _, p := range profiles {
		if _, err := survey.ProjectWith(ds, sc, p); err != nil {
			t.Errorf("%s: projection failed: %v (a freeform answer the vocabulary does not cover?)", p.AgentID, err)
		}
	}
}

func readRawProfiles() ([]map[string]any, error) {
	body, err := os.ReadFile("testdata/issue-profiles.json")
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
