package workspaces

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// The two invariants (SPLASH -> *Two invariants*): the traffic log is what
// *happened*; the isotope records are what *should have happened*. They are
// deliberately separated, and a log record without an isotope — or the reverse,
// or a digest that disagrees — is a discrepancy. They live in the session's
// state.
const (
	DirLogs     = "/state/logs"
	DirIsotopes = "/state/isotopes"
)

func logPath(id string) string { return DirLogs + "/" + id + ".jsonl" }
func isoPath(id string) string { return DirIsotopes + "/" + id }

// Op is one workspace operation in a run.
type Op struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Run is one program's operations, in order.
type Run struct {
	ID  string
	Ops []Op
}

// Canonical is the run's content address (RFC 6920): a normalisation of its
// ops, order-independent. This is the isotope — the expectation.
func (r Run) Canonical() string { return canonical(r.Ops) }

func canonical(ops []Op) string {
	lines := make([]string, len(ops))
	for i, o := range ops {
		lines[i] = o.Kind + " " + o.Path
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

type recorder struct{ ops []Op }

func (r *recorder) add(kind, p string) { r.ops = append(r.ops, Op{Kind: kind, Path: p}) }

// Client is the Phase-1 spike: ONE writer, TWO stores — the conjugate pair
// (register, the 42i frame). It writes each workspace to both hosts and records
// its own isotope tracking: the traffic log on the fast host, the isotope
// record on the mirror.
type Client struct {
	tmp    Driver // the fast host (the shortcut) — carries the traffic log
	mirror Driver // the slower host (the mirror) — carries the isotope records
}

// NewClient returns a dual-write client over the two hosts.
func NewClient(tmp, mirror Driver) *Client { return &Client{tmp: tmp, mirror: mirror} }

// Run applies prog to a store on both hosts, then records the run in both
// invariants. One writer produced both stores, so the pair is conjugate: it
// exists, but it is not yet independent.
func (c *Client) Run(ctx context.Context, session string, prog func(*Store) error) (*Run, error) {
	rec := &recorder{}
	if err := prog(New(c.tmp, session).withRecorder(rec)); err != nil {
		return nil, err
	}
	if err := prog(New(c.mirror, session)); err != nil {
		return nil, err
	}

	run := &Run{Ops: rec.ops}
	run.ID = canonical(rec.ops)

	// The traffic log (what happened) -> the fast host.
	var b strings.Builder
	for _, o := range run.Ops {
		enc, err := json.Marshal(o)
		if err != nil {
			return nil, err
		}
		b.Write(enc)
		b.WriteByte('\n')
	}
	if err := c.tmp.Put(ctx, logPath(run.ID), []byte(b.String())); err != nil {
		return nil, err
	}

	// The isotope record (what should have happened) -> the mirror.
	if err := c.mirror.Put(ctx, isoPath(run.ID), []byte(run.Canonical()+"\n")); err != nil {
		return nil, err
	}
	return run, nil
}

// Report is the cross-check: the writer's own claim (the traffic log against the
// isotope records) plus the independent observation (the mirror read back
// through the protocol against the spec).
type Report struct {
	Runs               int
	LogsMissingIsotope []string
	IsotopesMissingLog []string
	DigestMismatch     []string
	SpecMismatch       []string // surfaces where the observation disagrees with the spec
}

// Agrees reports whether every log has an isotope and every isotope a log, with
// matching digests, and the independent observation matches the spec.
func (r Report) Agrees() bool {
	return len(r.LogsMissingIsotope) == 0 &&
		len(r.IsotopesMissingLog) == 0 &&
		len(r.DigestMismatch) == 0 &&
		len(r.SpecMismatch) == 0
}

// Torn reports a discrepancy: the two invariants do not corroborate.
func (r Report) Torn() bool { return !r.Agrees() }

// Check cross-checks the traffic log against the isotope records.
func (c *Client) Check(ctx context.Context) (Report, error) {
	var rep Report

	logs, err := c.tmp.List(ctx, DirLogs)
	if err != nil {
		return rep, err
	}
	isos, err := c.mirror.List(ctx, DirIsotopes)
	if err != nil {
		return rep, err
	}

	isoByID := map[string]bool{}
	for _, p := range isos {
		if strings.HasSuffix(p, "/") {
			continue
		}
		isoByID[path.Base(p)] = true
	}

	for _, p := range logs {
		if strings.HasSuffix(p, "/") {
			continue
		}
		id := strings.TrimSuffix(path.Base(p), ".jsonl")
		rep.Runs++

		body, err := c.tmp.Get(ctx, p)
		if err != nil {
			return rep, err
		}
		ops, err := parseLog(body)
		if err != nil {
			return rep, fmt.Errorf("traffic log %s: %w", id, err)
		}

		iso, err := c.mirror.Get(ctx, isoPath(id))
		if err != nil {
			rep.LogsMissingIsotope = append(rep.LogsMissingIsotope, id)
			continue
		}
		delete(isoByID, id)
		if want := strings.TrimSpace(string(iso)); want != canonical(ops) {
			rep.DigestMismatch = append(rep.DigestMismatch, id)
		}
	}

	for id := range isoByID {
		rep.IsotopesMissingLog = append(rep.IsotopesMissingLog, id)
	}
	sort.Strings(rep.LogsMissingIsotope)
	sort.Strings(rep.IsotopesMissingLog)
	sort.Strings(rep.DigestMismatch)
	return rep, nil
}

func parseLog(body []byte) ([]Op, error) {
	var ops []Op
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}
		var o Op
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			return nil, err
		}
		ops = append(ops, o)
	}
	return ops, nil
}

// Plane is a position on the shared plane (register, the 42i frame).
type Plane string

const (
	PlaneConcept Plane = "42"   // the concept: nothing exists        -> dark
	PlaneConj    Plane = "-42i" // exists, one writer (conjugate)     -> yellow
	PlaneWork    Plane = "-42"  // independent, and the pair agrees   -> green
	PlaneTorn    Plane = "torn" // the two invariants disagree         -> red
)

// Light maps a plane to the lab's adhd light vocabulary (register §2).
func (p Plane) Light() string {
	switch p {
	case PlaneConcept:
		return "dark"
	case PlaneConj:
		return "yellow"
	case PlaneWork:
		return "green"
	default:
		return "red"
	}
}

// Assess reads the situation from the cross-check and the conjugate/independent
// split. Independence is manufactured by the system (Phase 2) — the spike's
// dual-write is one writer, so it is conjugate by construction.
func Assess(rep Report, exists, independent bool) Plane {
	switch {
	case rep.Torn():
		return PlaneTorn
	case !exists:
		return PlaneConcept
	case !independent:
		return PlaneConj
	default:
		return PlaneWork
	}
}

// Residual is the "abuse" (register, the 42i frame): reflect one store to get
// the other, and read where they differ. Paths under exclude are ignored (the
// invariant artifacts live on different hosts by design).
func Residual(ctx context.Context, a, b Driver, exclude ...string) ([]string, error) {
	skip := func(p string) bool {
		for _, e := range exclude {
			if strings.HasPrefix(p, e) {
				return true
			}
		}
		return false
	}
	load := func(d Driver) (map[string]string, error) {
		ps, err := d.List(ctx, "")
		if err != nil {
			return nil, err
		}
		m := map[string]string{}
		for _, p := range ps {
			if skip(p) {
				continue
			}
			if strings.HasSuffix(p, "/") {
				m[p] = ""
				continue
			}
			body, err := d.Get(ctx, p)
			if err != nil {
				return nil, err
			}
			m[p] = string(body)
		}
		return m, nil
	}
	am, err := load(a)
	if err != nil {
		return nil, err
	}
	bm, err := load(b)
	if err != nil {
		return nil, err
	}
	var out []string
	for p, av := range am {
		bv, ok := bm[p]
		if !ok || bv != av {
			out = append(out, p)
		}
	}
	for p := range bm {
		if _, ok := am[p]; !ok {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}
