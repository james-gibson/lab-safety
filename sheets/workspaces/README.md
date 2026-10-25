# Workspace VFS — Standards & Validation Register

> Prototype: [lab-safety#9](https://github.com/james-gibson/lab-safety/issues/9) ·
> Parent RFC: [#5](https://github.com/james-gibson/lab-safety/issues/5) ·
> Theory: [lab/SPLASH.md](../../lab/SPLASH.md) ·
> Artifact style: [../README.md](../README.md) ·
> Gate: [minimal.feature](minimal.feature) · Roots: [filesystem.feature](filesystem.feature) ·
> Punctuation: [standards.feature](standards.feature)

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

  **Rung 2 pinned it to one component.** `adhd` *does* map `degraded` → yellow
  (`internal/smokelink/watcher.go`, `mapHealthState`), so the missing producer is
  **smoke-alarm's own prober** — any failure there is `unhealthy`/`outage` → red, and
  a capability mismatch is never examined. That sharpens
  [smoke-alarm#34](https://github.com/james-gibson/smoke-alarm/issues/34) from *"the
  instrument has no yellow"* to *"the prober never yields `degraded`"*, and puts the
  fix in the alarm rather than in adhd.

  So the store reaching `-42i` **read green**, not yellow — the instrument could not
  yet say *"exists but not yet independent"*. Two consequences:

  - the **existence** transition (`42 → -42i`) and the **independence** transition
    (`-42i → -42`) both read green, and only **torn** (red) separated them;
  - to *see* `-42i` as yellow, the lab had to first express **dissent** — the
    two-invariant disagreement — which is exactly what the Phase-1 mirror supplies.

  A finding against the **instrument**, not the hypothesis: the plane still described
  the situation; the lab's lights did not yet have a rung for it. Recorded as a
  symptom, not a blocker — the frame unchanged, only the reading changing.

  **Closed — the instrument has the rung** (smoke-alarm
  [#34](https://github.com/james-gibson/smoke-alarm/issues/34) → PR
  [#36](https://github.com/james-gibson/smoke-alarm/pull/36), 2026-10-17). The fix
  needed **both** halves, and the second was not in the ticket:

  - **the prober** now records every method's outcome instead of returning on the
    first failure, and classifies *some answered* — or *all answered but a declared
    `expected.min_capabilities` family not advertised* — as `degraded`. That field
    was declared in every sample config and read **nowhere** in the engine;
  - **the engine** was rewriting it back: `classifyWithPolicy` turned any failure on
    an `EverHealthy` target into `StateRegression`, and `knownstate.IsFailure(degraded)`
    is `true` *by spec* (`features/known-state.feature`: *"IsFailure returns true for
    degraded, outage, and failed"*). The **rendering** was the wrong layer —
    `targets.CheckResult.IsFailure` already excluded degraded — so a departure is now
    neither rewritten as a regression nor escalated to an outage. The streak
    accounting and the spec are untouched, and every existing regression/outage
    scenario feeds an *unhealthy* result, so the change is a no-op until a degraded
    result exists.

  Our half is the **surface**: it advertises only what the session can serve, so a
  session that has lost part of itself says so on the wire. Verified end to end, on
  both hosts, with `spike.sh`:

  ```
  dark ──▶ green ──▶ yellow ──▶ red
  (no session)  (it exists)  (it lost what the spec declares)  (it goes away)
  ```

  So the plane table above is now literally readable: `-42i` is yellow because a
  **departure is its own rung**, and red is reserved for *not answering*.

  One gap remains, and it is ours, not the instrument's: the declaration a target
  carries (`min_capabilities`) is written by hand in the harness, where Phase 0's
  rule — *the spec is the origin* — says it should be **derived** from the Gherkin,
  as the store's expectation already is.

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

**What each rung buys, in the mathematics of loss:** [math.md](math.md) states the
model behind "an event reaches the node that wants it" — a **lost bit** is RAID's
problem, a **lost RPC** is RPC's, and a **wrong bit** is neither, so it needs
verification rather than parity. It is also where the ladder's additivity stops being
a slogan: each rung moves a *different factor* of that probability (§6 there), and the
low parity this spike runs at is a **decision** that document justifies rather than an
omission.

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
`/tools/*@` is enumerated at call time, so dropping a file adds a tool and
removing it takes one away. The lab's precedent is adhd's dynamic tools
(`adhd.project.<name>.call`, a renamed upstream tool) and its honeypot decoys.

**The architecture is one writer, two drivers, and an MCP protocol — not two
writers.** The client is the *sole* writer; the two hosts are the *drivers*; the
surface over the store is *MCP*. Independence therefore cannot come from a second
writer; it comes from **where the second measurement is taken** (§5, Phase 2).

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
exist before the system does. So the dual-write starts with the *shape*, and Phase 2
supplies the independence **positionally**: the expectation is declared by the spec
(independent of the run) and the observation is read back through the MCP protocol
from the mirror — a different position than the write path. The writer is never
duplicated. The prototype being its own virtual space is the point: the comparison
lives there, not in the diagram. Do not gate Phase 1 on the ideal — the ideal is what
Phase 1 makes reachable.

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
| A5 | the state machine is driven by file writes | `/data/messages/*` → `/state/current` | `ResultLedger` over many runs |
| A6 | `.wsdb`-per-session access is downward-only | access scenarios | consensus across alarm instances |
| A7 | `subscribe` ⇄ file-watch | both hosts | `DetectFlakiness` (watch races) |
| A8 | a VFS-backed surface passes MCP spec compliance | local probe | smoke-alarm verdict (authoritative) |
| A9 | standard tooling suffices — no bespoke standard | the §3 table compiles | — (design claim; audited by review) |
| A10 | a command can be **replicated across hosts** and compared | same command on tmpdir + sqlite | consensus / `ResultLedger` |
| A11 | a command can be **dynamic** (registered at runtime) | drop `/tools/x@`, re-list | smoke-alarm re-probe |
| A12 | the sqlite **mirror** holds isotope records | dual-write spike (one writer, both stores) | two-invariant cross-check; independence is **positional** (spec + protocol read), not a second writer |
| A13 | the prototype runs a **single session** (`42`) | the run opens `42.wsdb` once | a second session's isolation (rung 3) — **deferred; multi-session is out of scope** |

**Note — the roots vs. Rule §4.** [`filesystem.feature`](filesystem.feature): a session
**is** the root (`/`); `~/` is a home directory **under** it, not `/` itself; and — unix
assumptions — **sessions may share** a path by an explicit join. That resolves §4: a
*descendant* (an agent added beneath a session) **gaining that session's access** is a
*join*, not inheritance; and a child session sees nothing of its parent *by default*. If
a child session is instead meant to *inherit* the parent's paths automatically, the two
conflict and one must move.

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
3. **Phase 2 — the mirror is verified from a different position.** The driver
   interface (swappable hosts) is **done**; the second measurement is then taken
   from a *different position*: the expectation is declared by the **spec**
   (independent of the run) and the observation is read back **through the MCP
   protocol** from the mirror. The writer is **not** duplicated. The Phase-0 suite
   passes **unchanged**; an edit to make it pass is a leak, and a leak is evidence.

**Phase 1 landed** — [`lab/workspaces/`](../../lab/workspaces/) (a Go module, the
spike). Gate met: Rules §1–§3 (and §5) pass on the tmpdir host **and unchanged** on
the sqlite host (`42.wsdb`, pure-Go `modernc.org/sqlite`); the mirror holds an
isotope record per run; the two invariants cross-check, and a seeded dissent reads
**torn** (red). One host-equivalence finding: a real directory has its parents
*implicitly*, sqlite does not — the sqlite host now records parents so the two trees
are identical.

**Phase 2 landed (positional independence)** — `protocol.go` (the MCP surface as
JSON-RPC 2.0) + `verify.go` (the expectation read from the Gherkin, the observation via
`Observe` through the protocol, `Verify`). `Report` runs both the writer's own claim
and the independent observation; a divergent mirror is caught
(`TestVerificationCatchesADivergentMirror`) and the pair then reads **green**. This
is *positional* independence — the check no longer trusts the writer's bookkeeping —
**not** measurable independence: one writer still corrupts both stores identically.
The dissent is asserted at rung 0, and it can now be **rendered** as well: §2's finding
is closed and the middle rung belongs to the instrument (§5, *the middle rung landed*).

**Rung 2 landed — the store becomes a service.** `cmd/workspace-mcp` serves the
store's MCP surface over HTTP from its **own process**, and `seed.go` inverts the
spec into it (the Gherkin is the origin: what the spec declares is what must exist,
at the path the spec names). `spike.sh` now drives the real server — step 1's python
stand-in is **retired** — and the lab's own `ocd-smoke-alarm` probes it with its
`strict` handshake (`initialize` + `tools/list`). On **both** hosts:

```
dark ──▶ green ──▶ red
(no session)   (the session exists)   (the session goes away, server still alive)
```

So the verdict is the instrument's, not this package's suite: **A8's slow validator**,
exercised. Three things this step had to decide, recorded rather than assumed:

- **the wire contract is the prober's, not ours.** The required methods come from
  smoke-alarm's `httpHandshakeMethods` (profile `strict`); the surface is written to
  answer *that*, and `http_test.go` replays its exact request shape (A8 at rung 0).
  Where the MCP spec leaves the method list open, the lab's prober closes it — a
  named cost, not a bespoke standard.
- **the surface reports its *session*, not its process.** With no session behind it
  the surface refuses (`-32001`), so in phase 3 the server is *still running* and the
  light goes red. Otherwise a live process would stand in for an existing session and
  the plane `42` — *not present* — would be unreachable on the wire.
- **what the alarm asserts is now the declarations, not only the handshake.** This
  step originally recorded that the prober never compares the surface to what was
  declared (`min_capabilities` unchecked); [smoke-alarm#34](https://github.com/james-gibson/smoke-alarm/issues/34)
  closed that, and the prober reads an unadvertised declared family as `degraded`.
  What it does **not** do is *derive* that declaration from the spec — the harness
  writes it by hand today — so "the surface serves the spec" is still asserted at
  rung 0 (`TestSeedIsTheSpecInverted`, `TestTheSurfaceServesTheSpec`) and is now also
  readable on the wire.

**Rung 2b landed — the session is a node, not a port.** The session registers
*itself* by name in the lab's own registry (smoke-alarm `POST /isotope/register`,
through `isotope.Client`), and the monitor resolves it **by name**:
`--target-session workspace-42 --registry <url>`. No caller carries the session's
address. That is the lab's model, not an invention — `TRUST-FLOWS.md` §2 already calls
the session *A* and the surface *B* ("a harness's automatic `tools/list` reaches B and
stops … names, not authority"), §5 already puts the wiring out of band, and adhd's peer
discovery reads the same registry (`isotope_discovery.go` → `DiscoverIsotopes`), not
mDNS. Live, before anything probes it:

```
registry now: workspace-42=127.0.0.1:64194 rung 2
```

This is §2's premise made concrete: **the session is a user in the root's namespace.**
It never holds the root's material — the wire contract is the prober's — and the root
reflects it: the trust rung is the registry's judgement, not ours. Two costs recorded
rather than hidden:

- **a name is not a heartbeat — and the registry now enforces it.** When this was
  first observed, deleting the session's store left the registry listing its endpoint
  until the alarm process restarted; only the probe noticed. smoke-alarm now treats a
  registration as a heartbeat: the session renews while it serves, deregisters when it
  stops, and the registry sweeps a name it has not heard from (smoke-alarm#35). A hard
  kill still leaves the name until the TTL (5m default) — a backstop, not a substitute
  for the arrival/departure signal.
- **mDNS is deliberately unused.** adhd's peer view comes from the registry, and
  announcing under `_adhd-isotope._tcp` would claim to *be* an adhd. A session that
  needs LAN discovery announces under its own service type — a later step.

**The middle rung landed — `-42i` is readable** (smoke-alarm#34 → PR #36). The
surface advertises only what the session can serve, and the monitor now reads a
partial handshake, or an unadvertised *declared* capability, as `degraded` — so the
light moves `dark → green → yellow → red` on both hosts, with yellow meaning exactly
*"exists, but not as declared"*. §2's finding is closed; the detail is there. One gap
remains and it is ours: the declaration the target carries is hand-written in the
harness, where it should be derived from the spec.

**Rung 3 closed — canonicalization.** The isotope **record** is now the *spec's* signed
isotope: `Client.Run` signs the gate with `isotope.CanonicalizeGherkinToIsotope` (family
`workspace-vfs`, v1, key = the session seed), and the mirror stores
`{run, spec, family, version, signature, raw}`. `Check` **re-derives** it, so the record
is *verified*, not trusted (`TestRecordIsBoundToTheSpecsIsotope`). The reading of the spec
is isotope's throughout — parse (`ParseGherkin`), meaning (step definitions), identity
(`CanonicalizeGherkinToIsotope`) — so the register's §3 row is now literally used, and
`lab/workspaces` hand-writes none of it. What remains open at rung 3 is its other half:
**consensus** (`ConsensusReporter`, N ≥ 3F+1) — separate instances agreeing.

## §6 Finding rule

If any layer needs a **new standard**, or a hand-written protocol where a standard
already exists, record it as **evidence against** the hypothesis — or at minimum a
named cost. Do not invent around a gap; report the gap. See `standards.feature`
→ *Rule: a bespoke standard is evidence against the hypothesis*.

**Instance — 2026-10-16: the rule caught one.** A **Gherkin parser was hand-written**
in `lab/workspaces` while `isotope.ParseGherkin` already existed — and §3 already named
`isotope.CanonicalizeGherkinToIsotope` as the standard for *command identity / content
address*. It was caught in review, deleted, and the module now depends on isotope
(`v0.4.2-0.…`, the step-definition branch — awaiting a release): `gherkin.go` keeps
only the **step definitions**, over isotope's AST.

**Named cost:** one bespoke standard, briefly — exactly the code A9 claims we will not
write. Note where it happened: in *our* layer, not the filesystem's. The hypothesis was
not damaged; our own discipline was. Which is the point of recording it.

**Instance — 2026-10-16 (rung 2): a named cost, not a new standard.** The HTTP
transport for the MCP surface ([`http.go`](../../lab/workspaces/http.go)) is
hand-written — about thirty lines over `net/http` — where §3 names *an existing Go MCP
SDK* as the standard. This adds **no new standard** and touches no wire format: it is
the JSON-RPC surface (`protocol.go`) carried over HTTP, and the contract it must meet
is the lab's own prober, not a choice of ours. §3 already records the lab as
hand-rolling MCP (`adhd/internal/mcpclient`, `internal/mcpserver`), so the second
instance has the same shape as the first — again in *our* layer — and the SDK remains
the slower, shared route.

## §7 Refs

- Plan + falsification criteria: #9 · RFC: #5 · sheet pattern: #8
- Theory: `lab/SPLASH.md` → *The proxy*, *Shortcut validation*, *Two invariants*,
  *Failover*, *Decomposition by feature*, *Derived values*.
- Loss model — the mathematics behind *"reaches the node that wants it"*:
  [math.md](math.md) — RAID/k-out-of-n for a lost **bit**, RPC semantics for a lost
  **RPC**, verification for a **wrong bit**; and why the parity here is low on purpose
  (Patterson et al. 1988; Chen et al. 1994; Birrell & Nelson 1984; Lamport et al. 1982).
- Symbols: `isotope/canonicalization.go`, `consensus.go`, `results.go`,
  `fortytwointegration.go`, `trust.go`; `isotope/plane`.
- Lab: smoke-alarm `internal/telemetry`, `internal/targets`; adhd `internal/proxy`,
  `internal/mcpclient`, `internal/telemetry` (stub).
- Instrument wiring: `adhd/internal/smokelink/watcher.go` (`mapHealthState`,
  `drainWatcherUpdates` — the `old_status:"dark"` first-transition),
  `adhd/internal/headless/server.go`; smoke-alarm `internal/knownstate/store.go`,
  `internal/engine/engine.go` (`toKnownStateStatus`, `httpHandshakeMethods`,
  `probeJSONRPCHandshakeHTTP` — the prober that now produces the middle rung).
- The middle rung, in the alarm:
  [smoke-alarm#34](https://github.com/james-gibson/smoke-alarm/issues/34) →
  [PR #36](https://github.com/james-gibson/smoke-alarm/pull/36) — the fix belongs to
  the instrument, not to this prototype.
- Live driver (rungs 1–2b, and the middle rung):
  [`lab/workspaces/spike.sh`](../../lab/workspaces/spike.sh) — builds
  `cmd/workspace-mcp`, seeds a session from the gate, registers it in the lab's
  registry, and puts it on the lab's own instruments (adhd light + smoke-alarm
  target), on either host (`DRIVER=sqlite`). It never assembles a session URL: the
  monitor is given `--target-session` and resolves it by name.
- Harness flags added for that: `kb/scripts/smoke-alarm/spin.sh`
  (`--target-session NAME --registry URL`) and `kb/scripts/adhd/spin.sh`
  (`--smoke-alarm-instance NAME`).
- Dual-write client (Phase 1): [`lab/workspaces/`](../../lab/workspaces/) — a Go
  module (`go test ./...`).
- Roots: [`filesystem.feature`](filesystem.feature) — a session **is** the root (`/`),
  its home `~/` is under it, and sessions may **share** a path by an explicit join.
