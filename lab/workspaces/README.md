# workspaces — the focused client (Phase 1 + independence)

> Plan: [lab-safety#9](https://github.com/james-gibson/lab-safety/issues/9) ·
> Register + gate: [`sheets/workspaces/`](../../sheets/workspaces/) ·
> Theory: [`lab/SPLASH.md`](../SPLASH.md)

## The architecture — one writer, two drivers, an MCP protocol

This is the shape, and it is deliberate:

- **one writer** — the client. There is no second writer to add; independence is
  *not* more writers.
- **two drivers** — `TmpDir` (a real directory) and `SQLite` (one `.wsdb` file):
  the same store on two hosts, swappable.
- **an MCP protocol** — the surface over the store: resources, tools and prompts,
  spoken as JSON-RPC 2.0.

```sh
go test ./...
```

| file | what |
| --- | --- |
| `driver.go` | the `Driver` host interface + `TmpDir` and `SQLite` |
| `store.go` | the VFS conventions — the tree, the state machine — and the MCP mapping as *file* operations |
| `protocol.go` | the MCP surface (JSON-RPC 2.0) over a store: `initialize`, `tools/*`, `resources/*`, `prompts/*` |
| `http.go` | the surface over the wire: `Server` (`http.Handler`) — the MCP surface from a separate process, plus `/healthz` |
| `seed.go` | the spec, inverted: `Seed` materializes what the spec declares, `BindTools`, `CheckLayout` |
| `advertise.go` | the session as a *node*: `SessionName`, `Register` — the lab's own registry, by name |
| `cmd/workspace-mcp` | the process: serves a session's surface on `-addr`, seeded from `-spec` |
| `mirror.go` | the two invariants, the dual-write client, the cross-check, the plane/light mapping, `Residual` |
| `gherkin.go` | the spec reference: `SpecRef`, what the spec *declares* (isotope's step definitions), and the spec's signed isotope |
| `verify.go` | the independent measurement: the expectation from the spec, the observation through the protocol |
| `*_test.go` | the gate: Rules §1–§3 (and §5) on **both** hosts, unchanged |

## The roots — the session IS the root

[`filesystem.feature`](../../sheets/workspaces/filesystem.feature) is the origin; this
is its inversion. Paths are absolute in the session's namespace, and `..` refuses — the
isolation is **structural**, not a permission check.

```
/                      the session IS the root
├── root/              ~/  — the home, UNDER the root
├── data/              task.json, messages/, artifacts/
├── state/             current, transitions.log, logs/, isotopes/
├── tools/             */@  — the MCP tools
├── resources/         **   — the MCP resources
└── prompts/           *.tmpl
```

A session may **join** another session's path at a mount point (`Store.Join`): the
sharing is a join, not inheritance — nothing is copied, and a read follows the join.

## The gate, as the plan states it

- **`minimal.feature` Rules §1–§3 are green on a real temp dir.** ✅
- **The same suite passes *unchanged* on the sqlite host.** ✅ — each Rule runs as a
  subtest on `tmpdir` *and* `sqlite`; nothing is host-conditional. The one place it
  *had* to change was a real finding: a directory on a real filesystem has its
  parent directories *implicitly*; sqlite does not. The sqlite host now records
  parents so the two trees are identical.
- **The sqlite mirror holds an isotope record per run.** ✅

## Independence — verification from a different position

With one writer, independence cannot come from a second writer. It comes from
**where the second measurement is taken**:

- the **expectation** (what should have happened) is declared by the **spec**
  (`Spec()`), *before* the run and independently of it — a writer bug cannot move it;
- the **observation** (what the mirror holds) is read back **through the MCP
  protocol** (`Observe`), not through the writer's driver calls.

`Report` runs both cross-checks: the writer's own claim (`Check`: the traffic log
against the isotope records) and the independent observation (`Verification`: the
mirror against the spec). When both agree the pair is `-42` — **green**.

**What it buys, and what it does not.** This is *positional* independence: the check
no longer trusts the writer's own bookkeeping, so a divergence between the write path
and the read path is caught (`TestVerificationCatchesADivergentMirror`). It is not
*measurable* independence: one writer still corrupts both stores identically, and a
consistent writer bug is invisible. The register (§2 → *Two invariants*) says exactly
this; the step makes the position independent, not the writer.

## Rung 2 — the store becomes a service

`go test` is rung 0: the writer's own report. Rung 2 puts the surface in its **own
process** and lets the lab's own instrument decide:

```sh
./spike.sh                 # the tmpdir host
DRIVER=sqlite ./spike.sh   # the sqlite host
```

The script builds `cmd/workspace-mcp`, seeds session `42` from the gate, and registers
it as a smoke-alarm `mcp` target (the alarm's `strict` handshake). The observed
sequence, identical on both hosts:

```
dark ──▶ green ──▶ red
(no session)   (the session exists)   (the session goes away, server still alive)
```

The wire contract is the **prober's**, not this package's: `initialize` + `tools/list`,
each a JSON-RPC 2.0 POST that must answer 2xx with no `error`. `http_test.go` replays
that exact request shape, so a disagreement between the test and the live alarm would
be a defect in the surface, not in the guess about what the prober sends.

The surface answers for its **session**, not for its process: with no session behind it
it refuses (`-32001`), which is why phase 3 shows red while the server is still up. A
process being alive is not a session existing — and plane `42`, *not present*, would
otherwise be unreachable on the wire.

What the alarm asserts is the **handshake**, not the declarations: it never compares
the surface to the spec (`min_capabilities` is unchecked). That the surface serves the
spec is asserted at rung 0 (`TestSeedIsTheSpecInverted`, `TestTheSurfaceServesTheSpec`)
and read on the wire by the eye.

## Rung 2b — the session is a node, not a port

`workspace-mcp -smoke-alarm <registry>` registers the session **by name** in the lab's
own registry (smoke-alarm's `POST /isotope/register`, through `isotope.Client`), and the
monitor resolves it by name:

```sh
smoke-alarm/spin.sh up ws42mon  --target-session workspace-42 --registry http://127.0.0.1:64192
adhd/spin.sh        up ws42dash --smoke-alarm-instance ws42mon
```

So `spike.sh` no longer knows the session's port, nor the monitor's. That is the lab's
model (`kb/scripts/lab/TRUST-FLOWS.md` §2: A = session, B = surface, C = authority; §5:
the wiring is decided out of band), and it is the frame's reading of §2's premise: the
session is a **user in the root's namespace**, and the *registry* judges its trust rung —
not us.

Two things worth knowing:

- **a name is not a heartbeat.** Nothing deregisters, so a stale name outlives the
  session; the probe is what notices, not the registry.
- **no mDNS.** adhd's peer view reads the registry, and announcing under
  `_adhd-isotope._tcp` would claim to *be* an adhd. LAN discovery is a later step, with
  its own service type.

## The finding this spike lives beside (register §2)

The lab's **headless** `adhd` renders a smoke-alarm target only **green**/*red*;
`dark` is the no-light default and **yellow has no producer** on that path. So
`-42i` currently *reads* green. The dissent this spike manufactures is asserted here
(rung 0) and must be surfaced as a yellow light only once the instrument has a rung
for it — see [`james-gibson/smoke-alarm#34`](https://github.com/james-gibson/smoke-alarm/issues/34).

**Rung 2 located it.** `adhd` *does* map `degraded` → yellow
(`internal/smokelink/watcher.go`, `mapHealthState`); the gap is smoke-alarm's **prober**,
where `healthy` → green and *anything else* → red, and a capability mismatch is never
examined. So the ticket is a prober fix in the alarm, not a light fix in adhd — and what
the alarm must be able to say is *"answering, but not what was declared"*, which is
exactly the dissent.

## The conjugate, before independence

Before `Verification`, both invariants were the writer's own record (the isotope was
a content address of the *run*), so `Residual` — the *abuse*: reflect one store to get
the other, read the asymmetry — was empty by construction, and the pair read `-42i`.
Identity at a point *is* the shared plane. That was the honest starting state; the
step above supplies the independence the cross-check needs.

## Not in this spike

**Rung 3 is closed for canonicalization.** The isotope **record** is the *spec's* signed
isotope (`isotope.CanonicalizeGherkinToIsotope`), **re-derived** by `Check` rather than
trusted; the run's traffic log is named by a content address of its own operations. The
whole reading of the spec is isotope's — parse, meaning (step definitions), identity.
Still open: **consensus** (`isotope.ConsensusReporter`, N ≥ 3F+1) — separate instances
agreeing — and the `ResultLedger` (rung 4).

**Rung 2 has landed** (above). What the alarm still cannot do is the **spec check** —
it never compares the surface to the spec — so the authority on "the surface serves what
the spec declares" is rung 0 and the eye, not the instrument. Same ticket as the yellow.
