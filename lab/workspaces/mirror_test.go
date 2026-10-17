package workspaces

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func newClient(t *testing.T) (*Client, *TmpDir, *SQLite) {
	t.Helper()
	td, err := NewTmpDir(filepath.Join(t.TempDir(), "42"))
	if err != nil {
		t.Fatal(err)
	}
	sq, err := NewSQLite(filepath.Join(t.TempDir(), "42.wsdb"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = td.Close(); _ = sq.Close() })
	return NewClient(td, sq), td, sq
}

// prog is one workspace program: the operations the client performs.
func prog(ctx context.Context, s *Store) error {
	if err := s.Init(ctx); err != nil {
		return err
	}
	if err := s.AddTool(ctx, "echo", []byte("#!/bin/sh\n")); err != nil {
		return err
	}
	if err := s.AddResource(ctx, "workspace/config.json", []byte("{}\n")); err != nil {
		return err
	}
	if err := s.AddPrompt(ctx, "summarize", []byte("Summarize: {{text}}")); err != nil {
		return err
	}
	if _, err := s.CallTool(ctx, "echo", []byte("hi"), func(in []byte) []byte { return in }); err != nil {
		return err
	}
	if err := s.WriteMessage(ctx, 1, "user", []byte(`{}`)); err != nil {
		return err
	}
	return s.FinalizeArtifact(ctx, "a1")
}

func decodeRecord(t *testing.T, raw []byte) Record {
	t.Helper()
	var rec Record
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("isotope record is not a record: %v", err)
	}
	return rec
}

func TestDualWriteRecordsAnIsotopePerRun(t *testing.T) {
	ctx := context.Background()
	c, td, sq := newClient(t)
	spec := gateRef(t)

	run, err := c.Run(ctx, "42", spec, func(s *Store) error { return prog(ctx, s) })
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Ops) == 0 {
		t.Fatal("run recorded no operations")
	}

	// the sqlite mirror holds the isotope record for the run
	raw, err := sq.Get(ctx, isoPath(run.ID))
	if err != nil {
		t.Fatalf("mirror holds no isotope record: %v", err)
	}
	rec := decodeRecord(t, raw)
	if rec.Run != run.ID {
		t.Errorf("record.run = %q, want %q", rec.Run, run.ID)
	}
	if rec.Spec != GatePath {
		t.Errorf("record.spec = %q, want %q", rec.Spec, GatePath)
	}

	// rung 3: the record is the *spec's* signed isotope, and it re-canonicalizes
	wantIso, err := spec.Isotope()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Signature != wantIso.Signature {
		t.Errorf("record signature = %q, want the spec's %q", rec.Signature, wantIso.Signature)
	}
	if rec.Raw != wantIso.Raw {
		t.Error("record raw does not equal the spec's canonical form")
	}

	// the traffic log hashes to its own name
	lb, err := td.Get(ctx, logPath(run.ID))
	if err != nil {
		t.Fatalf("fast host holds no traffic log: %v", err)
	}
	ops, err := parseLog(lb)
	if err != nil {
		t.Fatal(err)
	}
	if canonical(ops) != run.ID {
		t.Error("the traffic log does not hash to its own name")
	}

	// the writer's own two invariants corroborate
	rep, err := c.Check(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Agrees() {
		t.Fatalf("expected agreement, got %+v", rep)
	}

	// conjugate: one writer produced both stores, so the workspace residual is
	// empty — identity at a point is the shared plane
	res, err := Residual(ctx, td, sq, DirLogs+"/", DirIsotopes+"/")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expected no residual (identity), got %v", res)
	}

	// exists, but not independent -> -42i (yellow)
	p := Assess(rep, true, false)
	if p != PlaneConj {
		t.Errorf("plane = %s, want %s", p, PlaneConj)
	}
	if p.Light() != "yellow" {
		t.Errorf("light = %s, want yellow", p.Light())
	}
}

func TestDissentIsTorn(t *testing.T) {
	ctx := context.Background()
	c, td, _ := newClient(t)
	spec := gateRef(t)

	run, err := c.Run(ctx, "42", spec, func(s *Store) error { return prog(ctx, s) })
	if err != nil {
		t.Fatal(err)
	}

	// seed a discrepancy: the traffic log no longer hashes to its own name
	tampered := `{"kind":"message","path":"/data/messages/99999999.user.json"}` + "\n"
	if err := td.Put(ctx, logPath(run.ID), []byte(tampered)); err != nil {
		t.Fatal(err)
	}

	rep, err := c.Check(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Torn() {
		t.Fatalf("expected a torn report, got %+v", rep)
	}
	if len(rep.LogNotContentAddressed) != 1 || rep.LogNotContentAddressed[0] != run.ID {
		t.Errorf("log mismatch = %v, want [%s]", rep.LogNotContentAddressed, run.ID)
	}

	// a discrepancy is torn -> red, whatever the conjugate/independent split
	p := Assess(rep, true, false)
	if p != PlaneTorn {
		t.Errorf("plane = %s, want %s", p, PlaneTorn)
	}
	if p.Light() != "red" {
		t.Errorf("light = %s, want red", p.Light())
	}
}

// The record is bound to the *spec's* isotope: checking against a differently
// signed spec catches it. This is the whole point of rung 3 — the record is
// re-derived, not trusted.
func TestRecordIsBoundToTheSpecsIsotope(t *testing.T) {
	ctx := context.Background()
	c, _, _ := newClient(t)
	spec := gateRef(t)

	if _, err := c.Run(ctx, "42", spec, func(s *Store) error { return prog(ctx, s) }); err != nil {
		t.Fatal(err)
	}

	other, err := LoadSpec(GatePath, []byte("a-different-seed"), "workspace-vfs", 1)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := c.Check(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.RecordMismatch) != 1 {
		t.Fatalf("record mismatch = %v, want one", rep.RecordMismatch)
	}
	if !rep.Torn() {
		t.Error("a record that does not re-canonicalize must be a discrepancy")
	}
}

func TestIsotopeWithoutLogIsADiscrepancy(t *testing.T) {
	ctx := context.Background()
	c, _, sq := newClient(t)
	spec := gateRef(t)

	if _, err := c.Run(ctx, "42", spec, func(s *Store) error { return prog(ctx, s) }); err != nil {
		t.Fatal(err)
	}
	// a record with no traffic log
	if err := sq.Put(ctx, isoPath("orphan"), []byte("{}\n")); err != nil {
		t.Fatal(err)
	}

	rep, err := c.Check(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(rep.IsotopesMissingLog, "orphan") {
		t.Errorf("expected an orphan record to be a discrepancy, got %+v", rep)
	}
}

func TestPlaneAndLightMapping(t *testing.T) {
	clean := Report{}
	torn := Report{RecordMismatch: []string{"x"}}

	cases := []struct {
		rep         Report
		exists      bool
		independent bool
		plane       Plane
		light       string
	}{
		{clean, false, false, PlaneConcept, "dark"},
		{clean, true, false, PlaneConj, "yellow"},
		{clean, true, true, PlaneWork, "green"},
		{torn, true, true, PlaneTorn, "red"},
		{torn, false, false, PlaneTorn, "red"},
	}
	for _, tc := range cases {
		p := Assess(tc.rep, tc.exists, tc.independent)
		if p != tc.plane {
			t.Errorf("Assess(exists=%v, independent=%v, torn=%v) = %s, want %s",
				tc.exists, tc.independent, tc.rep.Torn(), p, tc.plane)
		}
		if p.Light() != tc.light {
			t.Errorf("plane %s light = %s, want %s", p, p.Light(), tc.light)
		}
	}
}
