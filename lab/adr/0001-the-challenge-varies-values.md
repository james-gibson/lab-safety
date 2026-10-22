# ADR 0001 — the challenge varies values, never structure

**Status:** proposed, 2026-10-21
**Amends:** `lab/CHALLENGE.md` §*the challenge varies fixtures, never expected outcomes*
**Depends on:** `isotope/docs/adr/0001` (identity is wording), `0002` (tables are
arguments), `0003` (outline rows are parts)

## Context

`CHALLENGE.md` requires a re-attestation to be a run, not a token, and states two rules:
*the challenge varies fixtures, never expected outcomes*, and *silence is failure*. Both
are right and neither is checkable yet, because the canon constrains which variation is
even possible without changing the spec's identity.

Measured against the canon:

| variation | in the identity? | usable by a challenge? |
| --- | --- | --- |
| a value in a data-table cell | no (`base` → `admin`, same `eOBhPmeUtD6x`) | **yes** |
| a value substituted for a `<placeholder>` | no (`<x>` → `x`, value never hashed) | **yes** |
| a value spelled into a step line | **yes** | no — it is an edit, not a variation |
| a header or column name | no | yes, but it changes meaning invisibly |
| the number of rows | no | yes, and invisibly — the dangerous one |
| an `Examples:` row | **collapsed** — outline steps attach to the previous scenario, all rows become one case | broken today |

So "vary the fixture" has a narrow, canon-shaped meaning: **vary values in cells or
placeholder substitutions, and nothing else.** Everything else is either an edit that
changes identity or an invisible change that no receipt can catch.

## Decision

**The challenge varies values only. Structure is fixed and is part of the promise.**

Fixed, and asserted by the run: step text, headers, row count, number of steps and
scenarios, and every expected outcome. Varied: input cells and `<placeholder>`
substitutions.

Two consequences for `CHALLENGE.md`:

1. **"Never expected outcomes" needs the cell split.** In a table-driven case the outcome
   is often a cell. The rule is therefore: an *input* cell may vary, an *expected-outcome*
   cell may not. That distinction has to be declared per table, since the canon cannot
   see the table at all.
2. **The run must prove its identity held.** Before the run, assert that the substituted
   spec's isotope equals the declared spec's isotope. If it does not, the challenge
   edited the promise and the result must not be attested — *silence is failure* applies
   to a failed identity check as much as to a missing receipt.

This is a check the canon makes cheap precisely because it ignores tables: substitution
into a cell cannot change the token, so an identity change means the harness did
something structural rather than varying a fixture.

## Consequences

| act | identity | admissible |
| --- | --- | --- |
| swap `base` for `admin` in a cell | unchanged | yes, if the cell is an input |
| substitute a fresh nonce for `<port>` | unchanged | yes |
| rewrite a step to take a different value | changed | no — a new promise, declarable separately |
| add a row | unchanged | no — invisible, so the receipt cannot show it |
| add a column | unchanged | no — invisible, same reason |

The asymmetry is deliberate: anything the canon cannot see is forbidden to the challenge,
because a receipt cannot prove it happened. Unverifiable variation is how a verifier
becomes a free attack-surface report (`CHALLENGE.md`).

## Known limits

- **The identity check is necessary, not sufficient.** A harness could vary a value in a
  cell *and* nothing else, and still run a different binary; the transcript, not the
  token, is what binds the two.
- **Row and column counts are unverifiable by the canon** and are therefore enforced only
  by the harness. A dishonest harness can add a case and produce a valid run for a
  smaller promise.
- **Placeholder names are identity, values are not.** `<port>` and `<port>` with different
  values are the same promise — which is the point — but two different meanings sharing
  one placeholder name collide silently.
- **`Examples:` rows are unusable until `isotope/docs/adr/0003` lands.** A challenge built
  on an outline today varies cases that the canon has merged into the previous scenario.
- **Input-versus-outcome is a declaration, not a derivation.** Nothing in the file marks
  a cell as expected output; that reading lives in the harness's config and can drift
  from the spec it claims to describe.

## What would reverse this

A canon that hashes table structure (ADR `isotope/0002`'s composition rule) would make
headers and row counts visible, at which point a challenge could vary them and a receipt
could show that it did — but a hashed cell would then stop being a fixture slot, so
input values would have to move to placeholders only.
