# Register of work

**Status:** informational. A register of the work described in this directory: each
item, its state, and where it is recorded.

**States:** *implemented* (a runnable artifact) · *modified* (an uncommitted change
in the working tree) · *filed* (a tracked issue) · *design* (decided, not
implemented).

## Components and tooling

| item | state | artifacts | issue |
| --- | --- | --- | --- |
| `adhd` as a launchd service (`dev.kb.adhd`) | implemented | `kb/scripts/adhd/{dev.kb.adhd.plist,adhd.yaml,install.sh,README.md}` | — |
| `adhd` idle profile starts no MCP server | filed | — | lezz.go#11 |
| headless config does not drive isotope registration | filed | — | adhd#15 |
| alarms SOPS-backed (`--secure`, `run-secure.sh`) | implemented | `kb/scripts/smoke-alarm/run-secure.sh`, `secure/smoke-alarm.enc.env` | — |
| `secrets-rotation.yaml` runnable on `env://` | implemented | `kb/scripts/smoke-alarm/configs/secrets-rotation.yaml` | — |
| tuner spinner | implemented | `kb/scripts/tuner/spin.sh` | — |
| smoke-alarm spinner (`--tuner`, `--secure`, `--guard-lab`) | implemented | `kb/scripts/smoke-alarm/spin.sh` | — |
| adhd spinner (`adopt`, `register`) | implemented | `kb/scripts/adhd/spin.sh` | — |

## Discovery and publication

| item | state | artifacts | issue |
| --- | --- | --- | --- |
| the vault publishes a dynamic `llms.txt` that smoke-alarm consumes | implemented | `kb/scripts/adhd/llms-txt.sh` | — |
| post-connection trust summary for library READMEs | implemented | `lab/SPLASH.md` (+ 10 READMEs) | — |
| guard should express refusals in `42i` coordinates | filed (proposal) | — | lab-safety#6 |
| alarm's bare `/` returns 404 | filed | — | smoke-alarm#22 |
| tuner does not enumerate `_smoke-alarm._tcp`; discovery should key on registered isotopes and dedupe endpoints | filed | — | tuner#3 |
| lifecycle events and rotation schedule visible to the indexer | implemented | `secure/scripts/{sops-op.sh,rotation-report.sh}`, `personal/ops/*` | — |

## Trust chain

| item | state | artifacts | issue |
| --- | --- | --- | --- |
| tuner ⇄ smoke-alarm ⇄ adhd round-trip | implemented (exercise) | `lab/{FLOW.md,exercise.sh}` | — |
| honeypot tool; trusted vs probing caller | implemented (exercise) | `lab/{TRIPWIRE.md,tripwire.sh}` | — |
| alarm merges the reported security component | modified | `smoke-alarm/internal/health/server.go` | smoke-alarm#23 |
| observer adhd renders alarm components as indicators | modified | `adhd/internal/smokelink/watcher.go` | — |
| node honeypot: weights → distance → rung → eviction | design (unit-tested only; `internal/engine`, `internal/mcp_faults`) | — | — |
| verification chain (ingress → verifier → monitor) | implemented (exercise) | `lab/{RECONCILE.md,accounts.sh}` | — |
| alert traversing the chain (A→B→C) | not implemented | — | smoke-alarm#24 |
| enactor of a disagreement | design — resolved toward a self-monitoring triad | `RECONCILE.md` | smoke-alarm#25 |
| consensus tolerance `F = ⌊(N−1)/3⌋` | standard, tested | `isotope/consensus.go` | — |
| devmode inflates `N` and `F` | filed | — | isotope#9 |
| client-side consensus via renamed proxies | tested | `adhd/internal/mcpserver/proxy_consensus_test.go` | — |
| chaos: ordinal reassembled under perturbation (partials, duplicates, disorder, gap) | tested | `adhd/internal/mcpserver/ordinal_reassembly_test.go` | — |
| chaos suite as capability flags / membership predicate | tested | `TestOrdinalRequiresTheCluster` | — |
| the plane as a local-time measure | filed (proposal) | `PLANE.md` | local-time#132 |

## Findings

| document | content |
| --- | --- |
| `MATH.md` | which figures are grounded; which are placeholders |
| `PLANE.md` | the measure: a plane in local-time |
| `SPLASH.md` | the model, with references |
| `TRUST-FLOWS.md` | the flows as diagrams |
| `SYNAPSE.md` | the code index as a declared state: readiness vs reachability, what it ignores, and its measured cost profile |
| `PLAIN.md` | the design without the vocabulary, and the two objections it has to survive |
| `CHIMERA.md` | the identifier family, records, policy encryption, receipts, and the good-enough query threshold |
| `TEMPORAL.md` | the temporal reference spec prompt, with four worked reference chains |
| `TOPOLOGY.md` | `do / know / see` reporting, and the typed system-description model |
| `ADDRESSING.md` | bind vs destination vs identity; reserved TLDs; three local failures |
| `SCENARIOS.md` | the `innkeeper` world model as a test harness |
| `HISTORY.md` | the register of eighteen historical sources and their disposition |
| `README.md` | index and summary |

## Uncommitted changes

- `smoke-alarm/internal/health/server.go` — merges upstream components.
- `adhd/internal/smokelink/watcher.go` — emits an indicator per component.
- `adhd/internal/mcpserver/proxy_consensus_test.go`,
  `ordinal_reassembly_test.go` — the tests above.
- README summaries (SPLASH) in the ten registries.
- `lab/{CHIMERA.md,TEMPORAL.md,PLAIN.md,SYNAPSE.md,TOPOLOGY.md,ADDRESSING.md,SCENARIOS.md,HISTORY.md}`
  — new, untracked. Eight documents, none of which existed on disk before:
  `CHIMERA`/`TEMPORAL` recovered from two chat exports (the identifier algebra, and the
  temporal reference spec); `SYNAPSE` from measurements taken in this working tree;
  `PLAIN` written to answer the "why not bare MCP" objection; `TOPOLOGY`/`ADDRESSING`/
  `SCENARIOS`/`HISTORY` from seven further historical exports processed 2026-10-19.
  See `HISTORY.md` for the source register.
- `lab/SPLASH.md` — extended with the measured proxy section and the concurrency
  figures; the index material moved out to `SYNAPSE.md`.
- `lab/{README.md,THREADS.md}` — index and register updated for the eight documents.
- The eighteen historical sources are **not** in this repository. They were moved to the
  knowledge base at `kb/personal/sources/{json,ps}` — this repo is public and the
  material includes an auth design and internal addresses. See `HISTORY.md`.
- `kb/personal/sources/ps` — added to the kb index excludes (unreadable subset-font
  printouts); `json/` is left indexed so the exports stay searchable.
- `AGENTS.md` at `personal/`, `kb/`, `mcp-lab/`, `open-knowledge/`, `openttd/`, and
  `~/.config/opencode/AGENTS.md` — Synapse call signature and registered repos.

The two code changes are required for the alert paths in the exercises.

## Running state

Only the launchd agent `dev.kb.adhd` runs, bound to `127.0.0.1:9090`.

## Summary

Each item above is one of three things: a component that observes another; a place
from which a defect can propagate (an unguarded endpoint, a manifest, an alert that
does not traverse); or a figure that decides trust. The first two are engineering;
the third is the measure in `PLANE.md`.
