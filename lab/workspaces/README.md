# workspaces — the focused dual-write client (Phase 1 spike)

> Plan: [lab-safety#9](https://github.com/james-gibson/lab-safety/issues/9) ·
> Register + gate: [`sheets/workspaces/`](../../sheets/workspaces/) ·
> Theory: [`lab/SPLASH.md`](../SPLASH.md)

This is the **Phase 1 spike** from the register (§5): the smallest honest thing
that makes the file model real on a *real filesystem*, and makes the **two
invariants** exist so they can later dissent. It is disposable — the choices it
tests are what survives, not the code.

```sh
go test ./...
```

## What it is

| file | what |
| --- | --- |
| `driver.go` | the `Driver` host interface + two hosts: `TmpDir` (a real directory) and `SQLite` (one `.wsdb` file, pure-Go `modernc.org/sqlite`) |
| `store.go` | the VFS conventions — the tree, the state machine, and the MCP mapping (`resources/*`, `tools/*@`, `prompts/*.tmpl`) as *file* operations |
| `mirror.go` | the two invariants, the dual-write client, the cross-check, the plane/light mapping, the residual |
| `*_test.go` | the gate: Rules §1–§3 (and §5) run on **both** hosts, unchanged |

## The gate, as the plan states it

- **`minimal.feature` Rules §1–§3 are green on a real temp dir.** ✅ (`store_test.go`)
- **The same suite passes *unchanged* on the sqlite host.** ✅ — each Rule runs as a
  subtest on `tmpdir` *and* `sqlite`; nothing is host-conditional. The one place it
  *did* have to change was a real finding: a directory on a real filesystem has its
  parent directories *implicitly*, sqlite does not, so the sqlite host now records
  parents to keep the two trees identical.
- **The sqlite mirror holds an isotope record per run.** ✅ (`mirror_test.go`)

## The model, in one paragraph

`Client.Run` applies one program to a store on **both** hosts — one writer, two
stores, the *conjugate* pair — writes the **traffic log** (what happened) to the
fast host and the **isotope record** (what should have happened, a content address
of the run) to the mirror, then `Check` cross-checks them. The plane follows:
`42` dark (nothing exists) → `-42i` yellow (exists, one writer, conjugate) →
`-42` green (independent, the pair agrees); a discrepancy is **torn** → red.

## The finding this spike lives beside (register §2)

The lab's **headless** `adhd` renders a smoke-alarm target only **green**/*red*;
`dark` is the no-light default and **yellow has no producer** on that path. So
`-42i` currently *reads* green. The dissent this spike manufactures is asserted here
(rung 0) and must be surfaced as a yellow light only once the instrument has a rung
for it — see [`james-gibson/smoke-alarm#34`](https://github.com/james-gibson/smoke-alarm/issues/34).

## Independence is Phase 2, not here

Both stores are written by **one writer**, so `Residual` (the "abuse": reflect one
store to get the other, read the asymmetry) is empty by construction — identity at a
point *is* the shared plane. `Assess(..., independent: false)` is therefore `-42i`.
Phase 2 introduces separate writers; then a non-empty residual, or agreement across
truly independent writers, is the evidence.

## Not in this spike

`isotope.CanonicalizeGherkinToIsotope` is **rung 3**, not here: the isotope record is
a content address of the *run* (RFC 6920), not yet a canonicalization of the
*Gherkin*. Consensus (`N ≥ 3F+1`) and the `ResultLedger` (rung 4) are later too.
