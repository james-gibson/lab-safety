package workspaces

import (
	"os"

	"github.com/james-gibson/isotope"
)

// GatePath is the Phase-0 gate, relative to this module.
const GatePath = "../../sheets/workspaces/minimal.feature"

// SpecRef is the spec a run claims to satisfy: its source, and the material its
// isotope is signed with.
//
// Reading the spec is *isotope's* — the parse, the step definitions, the
// canonicalization and the signature (register §3: "command identity | content
// address | isotope.CanonicalizeGherkinToIsotope"). This only points at it, so
// there is one reading and it cannot drift from the observation.
type SpecRef struct {
	Path    string
	Source  []byte
	Key     []byte
	Family  string
	Version int
}

// LoadSpec reads a spec from disk.
func LoadSpec(p string, key []byte, family string, version int) (SpecRef, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return SpecRef{}, err
	}
	return SpecRef{Path: p, Source: b, Key: key, Family: family, Version: version}, nil
}

// GateRef loads the Phase-0 gate. `key` is the session's seed.
func GateRef(key []byte) (SpecRef, error) {
	return LoadSpec(GatePath, key, "workspace-vfs", 1)
}

// Expectation is what the spec *declares* — isotope's step definitions over the
// parse (isotope.WorkspaceStepDefinitions). The spec is the origin: the
// expectation is not a hand-written list.
func (s SpecRef) Expectation() (Expectation, error) {
	spec, err := isotope.Define(string(s.Source), isotope.WorkspaceStepDefinitions()...)
	if err != nil {
		return Expectation{}, err
	}
	return Expectation{
		Tools:     spec.ByKind("tool"),
		Resources: spec.ByKind("resource"),
		Prompts:   spec.ByKind("prompt"),
	}, nil
}

// Isotope is the spec's signed isotope — rung 3 (register §1).
func (s SpecRef) Isotope() (isotope.Isotope, error) {
	return isotope.CanonicalizeGherkinToIsotope(string(s.Source), s.Key, s.Family, s.Version)
}
