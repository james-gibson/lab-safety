# Workspace VFS — Standards & Validation Register

> Prototype: [lab-safety#9](https://github.com/james-gibson/lab-safety/issues/9) ·
> Parent RFC: [#5](https://github.com/james-gibson/lab-safety/issues/5) ·
> Theory: [lab/SPLASH.md](../../lab/SPLASH.md) ·
> Artifact style: [../README.md](../README.md) ·
> Gate: [minimal.feature](minimal.feature) · Punctuation: [standards.feature](standards.feature)

This is a **register**, not a surface safety data sheet. A surface sheet declares
one MCP surface; a register records a **plan and the standards it commits to**,
so the expensive thing — *which standard we chose, and the slower validation that
keeps the choice honest* — survives context loss.

## §0 The hypothesis (why a register exists at all)

> **If the convergence is a bad idea, using an existing filesystem will not work.
> If it is not a bad idea, it will probably mostly work.**

Full plan in #9. The thing worth protecting is not the prototype's code — that is
disposable — it is the **choices**: every layer lands on an existing standard, and
every fast claim has a slower validator. Lose those and the next agent re-derives
them, or worse, invents a fourth way.

**Theory is imagination; the prototype is reality.** SPLASH can hold two invariants
independent by *describing* them so; reality cannot. Independence is not a property a
diagram has — it is one the running system **manufactures**, in custom code, once it
exists. And the prototype, **once it exists, is its own virtual space to exist**: it
is the ground that produces the comparison the theory presupposes. So the theory's
ideal is the *destination*, never a precondition for starting. Read every "not yet"
below that way.

## The 42i frame — the shared plane and the quarter-turn

The two solutions (tmp filesystem, sqlite) share **exactly one property today:
they do not exist.** That coincidence is the ground state, and the ground state is
a *plane*: at `42` the pair is **identical** — one concept, not yet two things.

Apply the symmetry 42i was built for, a quarter-turn `×(-i)`:

| stage | anchor | reads as | the pair |
| --- | --- | --- | --- |
| now | `42` | the concept, on the real axis | identical (neither exists) |
| `×(-i)` | `-42i` | exist, on the imaginary axis | a conjugate pair (still one writer) |
| `×(-i)` | `-42` | work, real, opposite | independent (separate writers) |

So the phases are not a checklist; they are **rotations of one situation**:

- **Phase 0/1 — the spike** = `42 → -42i`: the pair *exists*, but as a conjugate —
  the dual-write holds both at the same point. That is exactly why it is not yet
  independent: **identity at a point *is* the shared plane.**
- **Phase 2 — the split** = `-42i → -42`: the conjugate separates into two real,
  working halves. Reflection is the operation, and **the residual asymmetry is the
  evidence** — where the reflection fails is where the two stores genuinely differ.

This is the "abuse": implement **one**, reflect it across the axis to get the
other, and read the residual — instead of building both in parallel.

**The falsification.** If 42i fits this situation, the plan must be *expressible*
this way **and** the quarter-turn must *do work* — predict the order, cut the
build, name the failure mode. If relating the phases needs numbers, then 42i was
wrong for this situation, and the numbers belong to the **embedder project**, not
here. We hold the *situation*; the embedder holds the *measure*.

**The Gherkin is the origin point on the plane.** The minimal contract is `42` —
the concept, the origin; existing (the prototype) is its **inversion**
(`42 → -42i → -42`). So the gate is not a checklist to clear before building; it
is the **pole the build inverts**. Two consequences, and they are requirements:

- **Qualitative fit is a gate.** *Every* feature of the prototype must fit the
  frame qualitatively. A feature that needs numbers to fit is not yet in the frame
  — it is a finding, not a build item.
- **Every operation carries its plane — and the rotation is *surfaced*.** Trace
  every operation with its 42i **plane** or **rotation**; then the running system
  is the plane's own record. But the *evidence* is not a private log. The rotation
  must be visible on the lab's own instruments: as an **adhd status light** that
  moves as the quarter-turn proceeds, and as a **smoke-alarm test** that asserts
  the movement. **Rotation tracing is the feature**; OTel is for a human
  debugging, not for the verdict. A trace no light and no alarm test can see is
  not evidence.

  **The light is the plane:**

  | plane | reads as | adhd light |
  | --- | --- | --- |
  | `42` | concept / not present | **dark** |
  | `-42i` | exists | **yellow** |
  | `-42` | works | **green** |
  | torn | falsified | **red** |

  A **stand-in scratch script** may prime the loop — drive the first transitions
  end to end before the machinery is real. It is scaffolding, not a deliverable,
  and the lights and alarm tests outlive it. The priming script is
  [`lab/workspaces/spike.sh`](../../lab/workspaces/spike.sh) (§7).

  **Finding — the instrument's vocabulary is narrower than the plane's.** Probed
  against the live lab (2026-10-16, via `spike.sh`): headless `adhd` renders a
  smoke-alarm target as **green** (healthy) or **red**
  (unhealthy/outage/regression), and **dark** only as the no-light default. There
  is **no producer of yellow** on that path — `degraded` comes only from the 42i
  distance route (the MCP server manager), and the *local vs remote disagree* rule
  lives in the TUI's `localprobe`, which headless does not run. `min_capabilities`
  mismatch is not checked (still healthy); a partial handshake is `outage` (red).

  So the store reaching `-42i` reads **green**, not yellow — the instrument cannot
  yet say *"exists but not yet independent"*. Two consequences:

  - the **existence** transition (`42 → -42i`) and the **independence** transition
    (`-42i → -42`) both read green today; only **torn** (red) separates them;
  - to *see* `-42i` as yellow, the lab must first express **dissent** — the
    two-invariant disagreement — which is exactly what the Phase-1 mirror supplies.

  This is a finding against the **instrument**, not the hypothesis: the plane still
  describes the situation; the lab's lights do not yet have a rung for it. Recorded
  as a symptom, not a blocker — the frame is unchanged, only the reading changes.

## §1 The 42i premise — there is almost always a slower way to validate

42i is premised on the existence of a slower validator. The fastest claim is a
self-report; the slowest is ground truth. A "mostly works" is worth exactly what
the **slowest validator you actually ran** says.

```
fastest ───────────────────────────────────────────────────────────► slowest (ground truth)

  prototype green      same Gherkin on     smoke-alarm        isotope canonicalization
  on tmpdir      →     sqlite driver   →   MCP spec probe →   + Byzantine consensus   →  seed-42
  (self-report)        (mirror host)       (wire-level)       (N ≥ 3F+1)                (42 + 0i)
```

| rung | validator | independence it buys | how |
| --- | --- | --- | --- |
| 0 | the prototype's own suite (tmpdir) | none — self-report | `go test .../workspaces` |
| 1 | the **sqlite** host, same Gherkin | a second host **and the mirror** (see §2) | `CanonicalizeGherkinToIsotope` + mirrored store |
| 2 | `ocd-smoke-alarm` on the live surface | wire-level, separate process | `ProtocolMCP` target + spec check |
| 3 | `isotope` canonicalization + consensus | separate *instances* must agree | `ConsensusReporter`, N ≥ 3F+1 |
| 4 | `ResultLedger` history | agreement over *time* | `DetectFlakiness`, `DetectRegression` |
| — | ground truth `42 + 0i` | the seeds themselves | `pickled-onions` |

The rungs are **additive**: rung 3 audits rung 0; it does not replace it. The
prototype must be able, for every assumption, to name the rung that supports it
and the rung that still does not.

**The slow path is worked out by hand:** [slow-path.md](slow-path.md) holds three
crossing transcripts (*fox-hen-cabbage*) — the constraint-forced sequences a fast
run must reproduce, with one forbidden move each. A deviation is either a defect in
the fast path or a finding against the transcript.

## §2 Command replication and dynamic commands

This is not a preference of the prototype — it is what the lab already requires
(SPLASH), and the prototype must **exercise** it rather than assume it.

**Commands replicate across hosts.** The same command must be runnable on more than
one host, and the two runs **compared**:

- *consensus* — "the same capability can be reached under two names through two
  proxies … run one task through both and compare: agreement corroborates"
  (SPLASH → *The proxy*);
- *shortcut validation* — the fast, cheap route is periodically checked against a
  **slower route**, and "the **isotope** records that the comparison occurred and
  what it returned" (SPLASH → *Shortcut validation*);
- *failover* — the failover host "already holds the **mirrored traffic** and the
  pre-negotiated material; the transfer is a role swap, not a cold start"
  (SPLASH → *Failover*).

**Commands are dynamic.** A command is registered at runtime and appears or
disappears without a restart. In the VFS this is already the file form —
`context/tools/*@` is enumerated at call time, so dropping a file adds a tool and
removing it takes one away. The lab's precedent is adhd's dynamic tools
(`adhd.project.<name>.call`, a renamed upstream tool) and its honeypot decoys.

**The in-task example — a tmp dir mirrored by a sqlite store for isotope
tracking.** This is the concrete instance the prototype must build:

- the **tmpdir** is the fast host (the shortcut);
- the **sqlite** store is the **mirror** — the slower, offline-capable host — and
  it carries **isotope records**;
- the pair is SPLASH's **two invariants**: the traffic log (what *happened*) and
  the isotope records (what *should have happened*), "deliberately independent
  measurements" that cross-check — "a log record without a corresponding isotope,
  or an isotope without a log trace, is a discrepancy."

**A single session, id `42`.** The prototype runs in exactly one session — the id
`42` the frame anchors on. The id is a *name*; the plane (`42 → -42i → -42`) is the
*state* the store passes through. `.wsdb` is per session, so the mirror is
`42.wsdb`, and one session keeps the pair of stores the only pair — the quarter-turn
is not diluted across sessions.

So "tmpdir → sqlite" is not a backend swap with a personality change. It is the
prototype of **replicated commands across hosts, with the mirror holding the
second invariant** — the same shape the cluster uses for consensus, routing and
failover. If the VFS cannot express it, the VFS cannot be a cluster node.

**The spike that starts it.** Both later phases need the MCP client, so Phase 1
begins *with the client* — and has it write tmp and sqlite **together**, recording
its **own isotope tracking** (each run canonicalized to an isotope record in
sqlite). That is the fastest way to create the two-invariant *shape*.

But one writer producing both stores is **not** two independent measurements:
a bug in the writer corrupts both identically (SPLASH → *Two invariants*). That is a
fact, not a blocker. Independence cannot be *imported* from the theory either — it is
**manufactured** by the system, in custom code, once the system exists, and it cannot
exist before the system does. So the dual-write starts with the *shape* and grows the
independence (Phase 2); the sqlite side is a transcript until the stores are actually
separate, and only then a cross-check. The prototype being its own virtual space is
the point: the comparison lives there, not in the diagram. Do not gate Phase 1 on the
ideal — the ideal is what Phase 1 makes reachable.

## §3 Standards register

Every layer adopts an **existing** standard. `status` ∈ {chosen, available,
adopted, verified}.

| layer | standard adopted | existing tooling | status |
| --- | --- | --- | --- |
| filesystem semantics | POSIX | Go stdlib `io/fs`, `os.DirFS`, `testing/fstest` | chosen · available |
| MCP | MCP spec (`tools/*`, `resources/*`, `prompts/*`), JSON-RPC 2.0, protocol `2024-11-05` | an existing Go MCP SDK (lab currently **hand-rolls** it: adhd `internal/mcpclient`, `internal/mcpserver`) | chosen · available |
| fs → HTTP | WebDAV | `golang.org/x/net/webdav` | chosen · available (harness only) |
| subscribe | file-watch | `fsnotify` | chosen · available |
| second host / mirror | SQLite | `modernc.org/sqlite` (pure-Go, no cgo) | chosen · available |
| command identity | content address (RFC 6920) | `isotope.CanonicalizeGherkinToIsotope` | chosen · available |
| telemetry | OTLP; **every span carries its 42i plane/rotation** | `go.opentelemetry.io/otel` — already a dep in smoke-alarm | chosen · available |
| formats | JSON / JSONL / JSONC | stdlib `encoding/json` | chosen · available |
| slow validation | canonicalization + Byzantine consensus | `github.com/james-gibson/isotope` (`ConsensusReporter`, `ResultLedger`) | chosen · available |
| seeds (ground truth) | Gherkin seeds | `pickled-onions` | chosen · available |

## §4 Assumption → validator map

| # | assumption | fast (rung 0–1) | slow (rung 2+) |
| --- | --- | --- | --- |
| A1 | path conventions are backend-independent | same suite, tmpdir then sqlite | isotope consensus on the shared suite |
| A2 | resources are files | `resources/list` == `find` | smoke-alarm spec probe |
| A3 | tools are files | `tools/list` == `ls *@` | smoke-alarm spec probe |
| A4 | `tools/call` = file write + result read | correctness + latency | smoke-alarm spec probe |
| A5 | the state machine is driven by file writes | `messages/*` → `.state/current` | `ResultLedger` over many runs |
| A6 | `.wsdb`-per-session access is downward-only | access scenarios | consensus across alarm instances |
| A7 | `subscribe` ⇄ file-watch | both hosts | `DetectFlakiness` (watch races) |
| A8 | a VFS-backed surface passes MCP spec compliance | local probe | smoke-alarm verdict (authoritative) |
| A9 | standard tooling suffices — no bespoke standard | the §3 table compiles | — (design claim; audited by review) |
| A10 | a command can be **replicated across hosts** and compared | same command on tmpdir + sqlite | consensus / `ResultLedger` |
| A11 | a command can be **dynamic** (registered at runtime) | drop `context/tools/x@`, re-list | smoke-alarm re-probe |
| A12 | the sqlite **mirror** holds isotope records | dual-write spike (one writer, both stores) | two-invariant cross-check — **independence is Phase 2, not the spike** |
| A13 | the prototype runs a **single session** (`42`) | the run opens `42.wsdb` once | a second session's isolation (rung 3) — **deferred; multi-session is out of scope** |

## §5 The plan (condensed, so it does not get lost)

1. **Phase 0 — Gherkin first.** The minimal VFS *is* its scenarios. No code until
   they exist. If it cannot be expressed in Gherkin, it is not in the minimal VFS.
   → **the gate: [minimal.feature](minimal.feature)** (each Rule names its slower
   validator).
2. **Phase 1 — the focused MCP client, dual-writing (the spike).** Start with the
   client (both later phases need it). It writes each workspace **to tmp and to
   sqlite together**, and records **its own isotope tracking** — the two-invariant
   *shape* in one artifact, on a real filesystem. The experiment itself. One
   session (`42`); the mirror is `42.wsdb` (A13).
3. **Phase 2 — split the stores; the mirror becomes independent.** Introduce the
   driver interface so tmp and sqlite are swappable hosts, and the sqlite store is
   written/verified from a **different position** — the *independence* the
   cross-check needs. The Phase-0 suite passes **unchanged**; an edit to make it
   pass is a leak, and a leak is evidence.

## §6 Finding rule

If any layer needs a **new standard**, or a hand-written protocol where a standard
already exists, record it as **evidence against** the hypothesis — or at minimum a
named cost. Do not invent around a gap; report the gap. See `standards.feature`
→ *Rule: a bespoke standard is evidence against the hypothesis*.

## §7 Refs

- Plan + falsification criteria: #9 · RFC: #5 · sheet pattern: #8
- Theory: `lab/SPLASH.md` → *The proxy*, *Shortcut validation*, *Two invariants*,
  *Failover*, *Decomposition by feature*, *Derived values*.
- Symbols: `isotope/canonicalization.go`, `consensus.go`, `results.go`,
  `fortytwointegration.go`, `trust.go`; `isotope/plane`.
- Lab: smoke-alarm `internal/telemetry`, `internal/targets`; adhd `internal/proxy`,
  `internal/mcpclient`, `internal/telemetry` (stub).
- Instrument wiring: `adhd/internal/smokelink/watcher.go` (`mapHealthState`,
  `drainWatcherUpdates` — the `old_status:"dark"` first-transition),
  `adhd/internal/headless/server.go`; smoke-alarm `internal/knownstate/store.go`,
  `internal/engine/engine.go` (`toKnownStateStatus`, `httpHandshakeMethods`).
- Priming script (step 1): [`lab/workspaces/spike.sh`](../../lab/workspaces/spike.sh).
