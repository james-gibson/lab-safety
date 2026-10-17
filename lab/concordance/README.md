# Concordance — do we agree with a third party's claim?

A third party, [`mcpmarket.com/server`](https://mcpmarket.com/server), lists MCP
servers and tracks things adjacent to what this lab tracks. The proposal that
started this directory was: **record a single field, `doWeAgree`.**

This directory is that field, done in a way that survives being read closely.

## Why not a column on the model survey

Three reasons, and the first two are fatal on their own:

1. **The subjects do not overlap.** `lab/survey`'s subjects are *callers* (two
   agents). A marketplace's subjects are *servers*. Our only surface
   declarations are our own three (`sheets/`: adhd, ocd-smoke-alarm, workspaces).
   A `doWeAgree` column there would be empty by construction.
2. **A bare boolean has no truth conditions.** `doWeAgree` is a question, not a
   field. It needs a **subject** both sides name, a **predicate** (agree about
   *what*), and **our own independent observation**. Without those, two
   reasonable agents fill it differently, and `--check` would have to reject it
   as unauthored derived data — which is the one thing the survey's contract
   forbids.
3. **"We copy no data" does not follow from a paid listing.** Paying to be listed
   is the marketplace's arrangement with a server operator; it is not a licence
   granted to us, and a paid, curated listing is if anything *more* protected
   than an incidental one. Reading a claim to agree with it is also a terms
   question independent of copyright.

So this is a **separate dataset with its own instrument**. It shares the *shape*
of `lab/survey` — records are the only authored data, everything else is a
projection, `--check` proves no drift — and deliberately shares none of its
*code*: the two datasets must be able to disagree, and a shared checker that
broke would make both lie.

> **Not legal advice.** This directory records the terms a source is under; it
> does not adjudicate them. The one thing it does enforce is that we store the
> *least* we can.

## Whose data is whose

The rule is structural, and `--check` enforces it:

| | |
| --- | --- |
| **Ours** | our own claim, the subject, the predicate, the dates, and `agrees` |
| **Theirs** | a pointer: the listing URL, and **one normalised value token** |
| **Never** | their prose, their selection, their arrangement, their catalogue |

Two rules carry that:

1. **A record cannot exist without our own claim.** `our_claim.basis` must name a
   file that exists under the repo root. You cannot add a record merely because
   they list something — you can only add a record for a subject you already hold
   a claim about. A mirror of their catalogue is therefore *not representable*.
2. **Their claim is one normalised token, not prose.** `their_claim.value` must
   match `^[a-z0-9][a-z0-9.+:/@_-]*$` (≤128 chars). If a claim cannot be reduced
   to a token, the comparison is not well-typed yet and `--check` says so. This
   is the same discipline the model survey uses for prose → slugs.

**Agreement is computed, and is not a field of a record.** `schemas/record.json`
has no `agrees` property and `additionalProperties: false`, so authoring it is a
strict-decode failure. It appears only in the projection
(`schemas/flat.json`, `concordance.csv`, `concordance.feature`), where `--check`
recomputes it from the two tokens and fails on a mismatch.

## A record

`records/<source>/<subject>/<predicate>.json` — **the path is the key**, and
`--check` asserts all three levels equal the three fields.

```json
{
  "source":     { "id": "mcpmarket", "url": "https://mcpmarket.com/server", "retrieved_at": "2026-10-17" },
  "subject":    { "id": "acme-weather", "kind": "mcp-server" },
  "predicate":  "exposes-tool",
  "their_claim": { "value": "weather.get" },
  "our_claim":   { "value": "weather.get", "basis": "sheets/adhd.md" },
  "provenance": { "checked_at": "2026-10-17", "review_by": "2026-11-17", "method": "read" }
}
```

(The example above is illustrative — it is not an observation, and it is not in
`records/`.)

Predicates: `listed`, `vendor`, `endpoint`, `exposes-tool`, `auth-method`,
`transport`. Values are normalised tokens; for `listed` they are `true` / `false`.

## Layout

```
lab/concordance/
  go.mod go.sum
  cmd/concordance/main.go       --check | --render | --stale [--as-of]
  internal/concordance/
    schema.go                   jsonschema/v6; reads schemas/ from the root
    record.go                   strict decode; sources; path-is-the-key
    render.go                   the projection + `agrees`; CSV + summary + Gherkin
    check.go                    the contract
  schemas/record.json           the authored record (no `agrees`)
  schemas/flat.json             the projection (has `agrees`)
  sources/<id>.json             a third party's location and terms
  records/<source>/<subject>/<predicate>.json    the authored data
  concordance.csv               GENERATED
  concordance.feature           GENERATED
  README.md                     this file; the table below is GENERATED
  fixtures/broken/<check>/      one mutation per failing check
  fixtures/base/                the synthetic record the fixtures start from
  check_test.go                 materialises each fixture root and asserts the failure
```

## What `--check` proves — and what it does not

Proves: every record is strictly typed and schema-valid; the path is the key;
the source is declared and its URL is the source's own; `our_claim.basis` exists
under the repo root; both claims are normalised tokens; dates parse,
`review_by > checked_at`, `retrieved_at <= checked_at`; the predicate is in
vocabulary; `agrees` recomputes from the two tokens on disk; `concordance.csv`'s
header is the flat schema's properties in order; no duplicate key; and
`concordance.csv`, `concordance.feature` and the README region are byte for byte
what the records project to; and the feature canonicalises to a non-zero token
count.

Does **not** prove: that a third party's claim is *true*, that our own claim is
*right*, or that the source's terms permit what we store. It proves that we
stored the least we could and that nothing drifted.

## Adding the first record

1. Decide you may read the source — that is a human's call, not the tool's. The
   source's location and terms go in `sources/<id>.json` with
   `policy: "pointer"`.
2. Pick a subject **you already hold a claim about**, and name the file that
   holds it in `our_claim.basis`.
3. Reduce their claim to one token. If you cannot, stop — it is not well-typed.
4. Write `records/<source>/<subject>/<predicate>.json`, then
   `go run ./cmd/concordance --render` and `--check`.

### Why there are no records yet

`sources/mcpmarket.json` is declared. Its catalogue is rendered client-side and
its footer reserves all rights, so **a claim cannot be read out of the HTML** —
it needs their API or a scrape. That is a decision for a human, so nothing is
recorded, and the empty `records/` is a determination rather than an omission.
An empty concordance that passes `--check` is a legitimate state.

## Use

```sh
cd lab/concordance
go run ./cmd/concordance --check
go run ./cmd/concordance --render
go run ./cmd/concordance --stale --as-of 2026-11-17
go test ./...
```

## The concordance

<!-- BEGIN GENERATED: concordance-summary -->
| source_id | subject_id | predicate | their_claim | our_claim | agrees | our_basis | checked_at | review_by |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
<!-- END GENERATED: concordance-summary -->

Generated from `records/**` by `concordance --render`; checked by
`concordance --check`. The full projection of each row is in `concordance.csv`.
