package ingress

import (
	"strings"
	"testing"
)

const (
	testToken     = "wan-token"
	testKey       = "border-key"
	testBorder    = "http://127.0.0.1:19100/cluster"
	testAgreement = `{"agreements":[{"id":"workspace-42","namespace":"session-42","border":"` + testBorder + `","aliases":["42","workspace-42","ws42"],"rung":2}]}`
)

func sampleResolver(t *testing.T) *Resolver {
	t.Helper()
	res, err := LoadAgreements(strings.NewReader(testAgreement))
	if err != nil {
		t.Fatalf("load sample agreements: %v", err)
	}
	return res
}

func TestParseName(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		fail bool
	}{
		{raw: "42", want: "42"},
		{raw: "WS42", want: "ws42"},
		{raw: "workspace-42.", want: "workspace-42"},
		{raw: "%34%32", want: "42"},
		{raw: "0 .", want: "0"}, // fuzz regression: trim to a fixed point, not one pass
		{raw: "0%20.", want: "0"},
		{raw: "", fail: true},
		{raw: "%00", fail: true},
		{raw: "a/../b", fail: true},
		{raw: "..", fail: true},
		{raw: "%2e%2e", fail: true},
		{raw: strings.Repeat("a", MaxNameBytes+1), fail: true},
		{raw: string([]byte{0xff}), fail: true},
	}
	for _, tc := range cases {
		got, err := ParseName(tc.raw)
		if tc.fail {
			if err == nil {
				t.Errorf("ParseName(%q) = %q, want refusal", tc.raw, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseName(%q) = %q, %v; want %q", tc.raw, got, err, tc.want)
		}
	}
}

func TestResolveByAgreementOnly(t *testing.T) {
	resolver := sampleResolver(t)

	got := resolver.Resolve("42")
	if got.Decision != "forward" || got.Route == nil {
		t.Fatalf("known name = %+v, want forward", got)
	}
	if got.Route.Address != testBorder || got.Route.Namespace != "session-42" || got.Route.AgreementID != "workspace-42" {
		t.Fatalf("route was not copied from the agreement: %+v", got.Route)
	}
	if again := resolver.Resolve("42"); again.Decision != got.Decision || again.Reason != got.Reason || again.Route.Address != got.Route.Address {
		t.Fatalf("resolution is not deterministic: %+v / %+v", got, again)
	}

	for _, tc := range []struct{ name, reason string }{
		{name: "nowhere", reason: "unknown"},
		{name: "%00", reason: "malformed"},
	} {
		refusal := resolver.Resolve(tc.name)
		if refusal.Decision != "quarantine" || refusal.Reason != tc.reason || refusal.Route != nil {
			t.Errorf("Resolve(%q) = %+v, want quarantine/%s with no route", tc.name, refusal, tc.reason)
		}
	}
}

func TestAmbiguousNameIsQuarantined(t *testing.T) {
	raw := `{"agreements":[` +
		`{"id":"a","namespace":"ns-a","border":"http://a.invalid/cluster","aliases":["42"]},` +
		`{"id":"b","namespace":"ns-b","border":"http://b.invalid/cluster","aliases":["42"]}]}`
	resolver, err := LoadAgreements(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("load ambiguous agreements: %v", err)
	}
	got := resolver.Resolve("42")
	if got.Decision != "quarantine" || got.Reason != "ambiguous" || got.Route != nil {
		t.Fatalf("ambiguous name = %+v, want quarantine/ambiguous with no route", got)
	}
}

func TestAgreementValidation(t *testing.T) {
	for name, raw := range map[string]string{
		"malformed json": `{`,
		"no border":      `{"agreements":[{"id":"a","namespace":"n","aliases":["42"]}]}`,
		"bad border":     `{"agreements":[{"id":"a","namespace":"n","border":"file:///etc/passwd","aliases":["42"]}]}`,
		"bad alias":      `{"agreements":[{"id":"a","namespace":"n","border":"http://a.invalid","aliases":["a/b"]}]}`,
	} {
		if _, err := LoadAgreements(strings.NewReader(raw)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAdmit(t *testing.T) {
	if !Admit("Bearer "+testToken, testToken) {
		t.Fatal("valid bearer was refused")
	}
	for _, header := range []string{"", "Bearer", "bearer wrong", "Basic " + testToken, "Bearer " + testToken + "x"} {
		if Admit(header, testToken) {
			t.Errorf("header %q was admitted", header)
		}
	}
	if Admit("Bearer ", "") {
		t.Error("empty token admitted a caller")
	}
}
