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
| `mirror.go` | the two invariants, the dual-write client, the cross-check, the plane/light mapping, `Residual` |
| `verify.go` | the independent measurement: the expectation from the spec, the observation through the protocol |
| `*_test.go` | the gate: Rules §1–§3 (and §5) on **both** hosts, unchanged |

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

## The finding this spike lives beside (register §2)

The lab's **headless** `adhd` renders a smoke-alarm target only **green**/*red*;
`dark` is the no-light default and **yellow has no producer** on that path. So
`-42i` currently *reads* green. The dissent this spike manufactures is asserted here
(rung 0) and must be surfaced as a yellow light only once the instrument has a rung
for it — see [`james-gibson/smoke-alarm#34`](https://github.com/james-gibson/smoke-alarm/issues/34).

## The conjugate, before independence

Before `Verification`, both invariants were the writer's own record (the isotope was
a content address of the *run*), so `Residual` — the *abuse*: reflect one store to get
the other, read the asymmetry — was empty by construction, and the pair read `-42i`.
Identity at a point *is* the shared plane. That was the honest starting state; the
step above supplies the independence the cross-check needs.

## Not in this spike

`isotope.CanonicalizeGherkinToIsotope` is **rung 3**: the isotope record is a content
address of the *run* (RFC 6920), and `Spec()` is a hand-written declaration standing
in for the Gherkin parser. Consensus (`N ≥ 3F+1`) and the `ResultLedger` (rung 4) are
later too.
