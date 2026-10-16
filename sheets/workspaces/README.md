# Workspace VFS — Standards & Validation Register

> Prototype: [lab-safety#9](https://github.com/james-gibson/lab-safety/issues/9) ·
> Parent RFC: [#5](https://github.com/james-gibson/lab-safety/issues/5) ·
> Artifact style: [../README.md](../README.md) ·
> Linked scenarios: [standards.feature](standards.feature)

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
them from scratch, or worse, invents a fourth way.

## §1 The 42i premise — there is almost always a slower way to validate

42i is premised on the existence of a slower validator. The fastest claim is a
self-report; the slowest is ground truth. A "mostly works" is worth exactly what
the **slowest validator you actually ran** says.

```
fastest ───────────────────────────────────────────────────────────► slowest (ground truth)

  prototype green      same Gherkin on     smoke-alarm        isotope canonicalization
  on tmpdir      →     sqlite driver   →   MCP spec probe →   + Byzantine consensus   →  seed-42
  (self-report)        (2nd impl)          (wire-level)       (N ≥ 3F+1)                (42 + 0i)
```

| rung | validator | independence it buys | how |
| --- | --- | --- | --- |
| 0 | the prototype's own suite (tmpdir) | none — self-report | `go test .../workspaces` |
| 1 | the **sqlite** driver, same Gherkin **unchanged** | a second implementation | driver swap; any Gherkin edit = leak |
| 2 | `ocd-smoke-alarm` on the live surface | wire-level, separate process | `ProtocolMCP` target + spec check |
| 3 | `isotope` canonicalization + consensus | separate *instances* must agree | `CanonicalizeGherkinToIsotope`, `ConsensusReporter`, N ≥ 3F+1 |
| 4 | `ResultLedger` history | agreement over *time* | `DetectFlakiness`, `DetectRegression`, `VerifyFix` |
| — | ground truth `42 + 0i` | the seeds themselves | `pickled-onions` |

The rungs are **additive**: rung 3 does not replace rung 0; it *audits* it. The
prototype must be able to say, for every assumption, which rung currently supports
it and which rung still does not.

## §2 Standards register

Every layer adopts an **existing** standard. `status` ∈ {chosen, available,
adopted, verified} — the vocabulary from `../README.md`, so intent is never
written as fact.

| layer | standard adopted | existing tooling | status |
| --- | --- | --- | --- |
| filesystem semantics | POSIX | Go stdlib `io/fs`, `os.DirFS`, `testing/fstest` | chosen · available |
| MCP | MCP spec (`tools/*`, `resources/*`, `prompts/*`), JSON-RPC 2.0, protocol `2024-11-05` | an existing Go MCP SDK (lab currently **hand-rolls** it: adhd `internal/mcpclient`, `internal/mcpserver`) | chosen · available |
| fs → HTTP | WebDAV | `golang.org/x/net/webdav` | chosen · available (harness only) |
| subscribe | file-watch | `fsnotify` | chosen · available |
| second backend | SQLite | `modernc.org/sqlite` (pure-Go, no cgo) | chosen · available |
| telemetry | OTLP | `go.opentelemetry.io/otel` — already a dep in smoke-alarm | chosen · available |
| formats | JSON / JSONL / JSONC | stdlib `encoding/json` | chosen · available |
| slow validation | isotope canonicalization + Byzantine consensus | `github.com/james-gibson/isotope` (`CanonicalizeGherkinToIsotope`, `ConsensusReporter`, `ResultLedger`) | chosen · available |
| seeds (ground truth) | Gherkin seeds | `pickled-onions` | chosen · available |

Nothing is `adopted` or `verified` yet — the prototype has not been built. This
register is deliberately ahead of the code so the standards are fixed before the
first commit, not reverse-engineered from it.

## §3 Assumption → validator map

Each assumption from #9 names the rung that supports it.

| # | assumption | fast (rung 0–1) | slow (rung 2+) |
| --- | --- | --- | --- |
| A1 | path conventions are backend-independent | same suite, tmpdir then sqlite | isotope consensus on the shared suite |
| A2 | resources are files | `resources/list` == `find` | smoke-alarm spec probe |
| A3 | tools are files | `tools/list` == `ls *@` | smoke-alarm spec probe |
| A4 | `tools/call` = file write + result read | correctness + latency | smoke-alarm spec probe |
| A5 | the state machine is driven by file writes | `messages/*` → `.state/current` | `ResultLedger` over many runs |
| A6 | `.wsdb`-per-session access is downward-only | access scenarios | consensus across alarm instances |
| A7 | `subscribe` ⇄ file-watch | both drivers | `DetectFlakiness` (watch races) |
| A8 | a VFS-backed surface passes MCP spec compliance | local probe | smoke-alarm verdict (authoritative) |
| A9 | standard tooling suffices — no bespoke standard | the §2 table compiles | — (a *design* claim; audited by review) |

## §4 The plan (condensed, so it does not get lost)

1. **Phase 0 — Gherkin first.** The minimal VFS *is* its scenarios. No code until
   they exist. If it cannot be expressed in Gherkin, it is not in the minimal VFS.
   → **the gate: [minimal.feature](minimal.feature)** (each Rule names its slower
   validator).
2. **Phase 1 — `tmpdir` driver + focused MCP client.** A real temp directory tree;
   the experiment itself.
3. **Phase 2 — `sqlite` driver swap.** The Phase-0 suite passes **unchanged**; an
   edit to make it pass is a leak, and a leak is evidence.

## §5 Finding rule

If any layer needs a **new standard**, or a hand-written protocol where a standard
already exists, record it as **evidence against** the hypothesis — or at minimum a
named cost. Do not invent around a gap; report the gap. See `standards.feature`
→ *Rule: adopting a bespoke standard is evidence against the hypothesis*.

## §6 Refs

- Plan + falsification criteria: #9 · RFC: #5 · sheet pattern: [#8](https://github.com/james-gibson/lab-safety/pull/8)
- `gateway` symbols: `isotope/canonicalization.go`, `consensus.go`, `results.go`, `fortytwointegration.go`, `trust.go`
- Lab: smoke-alarm `internal/telemetry`, `internal/targets`; adhd `internal/proxy`, `internal/mcpclient`, `internal/telemetry` (stub)
