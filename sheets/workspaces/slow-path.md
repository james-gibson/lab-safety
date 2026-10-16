# Slow path — crossing transcripts (fox-hen-cabbage)

> The **fast path** is the prototype's own run (rung 0, self-report). The **slow
> path** is worked here, by hand, exhaustively. Each transcript is a
> *fox-hen-cabbage*: a small state space whose constraints **force** one legal
> crossing, with a single forbidden move. A fast run is validated by whether it
> **reproduces the transcript** — the slower route checked against the shortcut
> (SPLASH → *Shortcut validation*, *Two invariants*).

All three transcripts run in the prototype's **single session, id `42`** (register
→ §2): the id is the name; the plane (`42 → -42i → -42`) is the store's state.

## How the agent uses these

For every prototype run, extract the observed `(state, move, light)` sequence from
its plane/rotation trace (rung 0–1) and **diff it against the canonical sequence**
below. Three outcomes:

- **match** → the fast path agrees with the slow path; the run is corroborated.
- **deviation** → either the fast path is wrong (a defect), or the transcript is
  wrong (a **finding** — the slow path is not above correction).
- **forbidden move taken** → the invariant was violated; stop and record it.

Played against rungs 3–4: an `isotope` canonicalization of the matching scenario,
and a `ResultLedger` record of the comparison. The transcript is the *reference*;
the trace is the *claim*; the isotope is the evidence they were compared.

---

## T1 — the crossing (the quarter-turn)

**Puzzle.** Move the pair of stores from *nonexistent* to *working*.

- a store may not read **works** (green) unless it first read **exists** (yellow);
- a store may not read **exists** unless it has been written;
- **independent** may not be claimed while **one writer** produces both stores;
- every move is a quarter-turn (`×(-i)`).

| # | move | tmp | sqlite | light | allowed? |
| --- | --- | --- | --- | --- | --- |
| 0 | — | absent | absent | dark | — |
| 1 | build one store, dual-write | present | present | yellow | yes — exists, but conjugate (one writer) |
| 2 | claim independent | present | present | green | **NO** — one writer is identity, not independence |
| 2′ | split the writers | present | present | green | yes — the conjugate separates |

**Canonical:** `dark → yellow → green` in **exactly two quarter-turns**.
**Fast-path check:** the rotation trace reads `42 → -42i → -42`, and no light turns
green while a single writer produced both stores.
**Forbidden:** green reached without yellow; independence claimed at step 1.

---

## T2 — the mirror (the two invariants)

**Puzzle.** Keep the traffic log (tmp) and the isotope records (sqlite) such that
neither is ever the sole authority.

- a log trace without a matching isotope record is a discrepancy;
- an isotope record without a matching log trace is a discrepancy;
- to be a *check*, the two must be **independent** (different write positions);
- one writer may produce both only as a **transcript**, never as a **cross-check**.

| # | move | log | mirror | authority | allowed? |
| --- | --- | --- | --- | --- | --- |
| 0 | — | — | — | none | — |
| 1 | dual-write one run | entry | record | none | yes — shape only; flagged *not-independent* |
| 2 | declare the pair a cross-check | entry | record | independent? | **NO** — same writer ⇒ same error in both |
| 2′ | write log and mirror from separate positions | entry | record | independent | yes |
| 3 | delete one side | entry | — | — | **NO** — a one-sided state is a discrepancy, not silent |

**Canonical:** shape at step 1; independence only after step 2′; a one-sided state
surfaces as a discrepancy.
**Fast-path check:** the prototype never reports *independent* while one writer
wrote both; a one-sided state raises a discrepancy (an alarm), never a pass.
**Forbidden:** independence declared by the dual-write spike.

---

## T3 — the tool file (write-to-invoke, dynamic)

**Puzzle.** A `@` tool file: invoke by writing, read the result; add and remove
tools at runtime.

- a tool listed by `tools/list` must be **callable** (listed-but-uncallable is a
  defect — cf. adhd#24, smoke-alarm §16);
- a result must correspond to a request (**no orphan result**);
- removing a tool removes it from the list (**no ghost**);
- add/remove take effect **without a restart**.

| # | move | `context/tools/echo@` | tools/list | callable | allowed? |
| --- | --- | --- | --- | --- | --- |
| 0 | — | absent | — | no | — |
| 1 | write `echo@` | present | includes `echo` | yes | yes |
| 1′ | skip: list `echo`, no callable file | absent | includes `echo` | no | **NO** — listed but not callable |
| 2 | call: write request, read result | present | includes `echo` | yes | yes — result pairs with the request |
| 2′ | skip: result with no request | present | — | — | **NO** — orphan result |
| 3 | remove `echo@` | absent | excludes `echo` | no | yes |
| 3′ | skip: remove file, keep listing | absent | includes `echo` | no | **NO** — ghost |

**Canonical:** add ⇒ listed **and** callable; call ⇒ result pairs; remove ⇒ delisted
**and** uncallable; no restart.
**Fast-path check:** `tools/list` ⊆ callable; every result has a request; no ghosts;
add/remove take effect without restart.
**Forbidden:** listed-but-uncallable; orphan result; ghost listing.

---

## Why these three

Each is a **constraint-forced crossing**, so the slow path is decidable by hand and
the fast path cannot "mostly work" around it: T1 pins the **rotation** (`minimal.feature`
Rule §8), T2 pins the **two invariants** (Rule §6), T3 pins the **MCP mapping +
dynamic commands** (Rules §2, §5). Together they cover the quarter-turn end to end —
concept → exist → work — with one forbidden move apiece.

The slow path is not above correction: a **deviation** is either a defect in the fast
path or a **finding** against the transcript. Both are results.
