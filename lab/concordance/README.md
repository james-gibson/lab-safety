# Concordance — do we agree with a third party's claim?

A third party lists MCP servers and tracks things adjacent to what this lab
tracks. The proposal that started this directory was: **record a single field,
`doWeAgree`.**

This directory is that question, done in a way that survives being read closely.

## Why not a column on the model survey

Three reasons, and the first two are fatal on their own:

1. **The subjects do not overlap.** `lab/survey`'s subjects are *callers* (two
   agents). A directory's subjects are *servers*. Our only surface declarations
   are our own three (`sheets/`: adhd, ocd-smoke-alarm, workspaces). A
   `doWeAgree` column there would be empty by construction.
2. **A bare boolean has no truth conditions.** `doWeAgree` is a question, not a
   field. It needs a **subject** both sides name, a **predicate** (agree about
   *what*), and **our own independent observation**. Without those, two
   reasonable agents fill it differently, and `--check` would have to reject it
   as unauthored derived data — which is the one thing the survey's contract
   forbids.
3. **"We copy no data" does not follow from a paid listing.** Paying to be listed
   is the directory's arrangement with a server operator; it is not a licence
   granted to us, and a curated, paid listing is if anything *more* protected.
   Reading a claim in order to agree with it is the act, and that is a terms
   question independent of copyright.

So this is a **separate dataset with its own instrument**. It shares the *shape*
of `lab/survey` — records are the only authored data, everything else is a
projection, `--check` proves no drift — and deliberately shares none of its
*code*: the two datasets must be able to disagree, and a shared checker that
broke would make both lie.

> **Not legal advice.** This directory records the terms a source is under and
> *implements* the obligations it can; it does not adjudicate them. The one thing
> it enforces is that we store the least we can and credit what we must.

## Sources and their terms

| source | licence | key | obligations we implement |
| --- | --- | --- | --- |
| `mcpregistry` — [Official MCP Registry](https://registry.modelcontextprotocol.io) | permissive (Apache-2.0; docs CC-BY-4.0) | none — `/v0/servers` is unauthenticated | credit (CC-BY-4.0 asks; a declared credit is enforced) |
| `glama` — [Glama MCP Directory API](https://glama.ai/mcp/reference) | **attribution-required** | `Authorization: Bearer glm_…` | credit **and** a link to every listing |
| `mcpmarket` — the source the proposal named | all-rights-reserved | none available | — (declared, but unreadable: see *Why there are no records yet*) |

### Attribution is a check, not a note

Glama's terms are explicit: the data "is licensed, not public domain", and every
view that shows it must carry a visible credit to Glama, with **every server
presented linked to its own listing on Glama** — "the url the API returns for it,
not a copy of the listing you host".

That is a property of our *outputs*, so it is enforced there:

- a source declares `credit` (`text` + `url`) and `listing_link_required`;
- `--check` requires every row to carry the credit, and every record to carry
  `subject.listing_url`, **on the source's own host**;
- the credit is a **projected column**, so `concordance.csv` and
  `concordance.feature` carry it per row, and the README region carries it too.

A view of this data therefore cannot be built without the credit and the links —
not because someone remembered, but because the projection has no other shape.

Glama also names the case we are actually in. Its terms say attribution can be
**waived** under a commercial licence for "an internal tool, **a research
dataset**, or a voice or CLI surface" — which is what this is. That waiver is a
decision for a human; until it exists, the credit travels with the data. The
route is recorded in `sources/glama.json` → `waiver`.

## Whose data is whose

| | |
| --- | --- |
| **Ours** | our own claim, the subject, the predicate, the dates, and `agrees` |
| **Theirs** | a pointer: the listing URL, and **one normalised value token** |
| **Never** | their prose, their selection, their arrangement, their catalogue |

Two rules carry that, and `--check` enforces both:

1. **A record cannot exist without our own claim.** `our_claim.basis` must name a
   file that exists under the repo root. You cannot add a record merely because
   they list something — only for a subject you already hold a claim about. A
   mirror of their catalogue is therefore *not representable*.
2. **Their claim is one normalised token, not prose.** `their_claim.value` must
   match `^[a-z0-9][a-z0-9.+:/@_-]*$` (≤128 chars). If a claim cannot be reduced
   to a token, the comparison is not well-typed yet and `--check` says so.

### Secrets

**A key belongs in the environment, never in this repo.** `--check` scans every
file and fails on anything shaped like a credential (`glm_…`, `sk-…`, `ghp_…`,
`AKIA…`, private keys, assigned secrets). The fixture that breaks this check
*assembles* its fake key at test time, so the repository itself stays clean while
the check stays honest.

## A record

`records/<source>/<subject>/<predicate>.json` — **the path is the key**, and
`--check` asserts all three levels equal the three fields.

```json
{
  "source":     { "id": "glama", "url": "https://glama.ai/mcp/reference", "retrieved_at": "2026-10-17" },
  "subject":    { "id": "playwright-mcp", "kind": "mcp-server",
                  "listing_url": "https://glama.ai/mcp/servers/microsoft/playwright-mcp" },
  "predicate":  "exposes-tool",
  "their_claim": { "value": "browser.navigate" },
  "our_claim":   { "value": "browser.navigate", "basis": "sheets/adhd.md" },
  "provenance": { "checked_at": "2026-10-17", "review_by": "2026-11-17", "method": "read" }
}
```

(The example above is illustrative — it is not an observation, and it is not in
`records/`.)

Predicates: `listed`, `vendor`, `endpoint`, `exposes-tool`, `auth-method`,
`transport`. Values are normalised tokens; for `listed` they are `true` / `false`.

**`agrees` is computed and is not a field of a record.** `schemas/record.json`
has no such property and `additionalProperties: false`, so authoring it is a
strict-decode failure. It appears only in the projection, where `--check`
recomputes it from the two tokens and fails on a mismatch.

## Layout

```
lab/concordance/
  go.mod go.sum
  cmd/concordance/main.go       --check | --render | --stale [--as-of]
  internal/concordance/
    schema.go                   jsonschema/v6; reads schemas/ from the root
    record.go                   strict decode; sources; path-is-the-key
    render.go                   the projection + `agrees` + `credit`
    check.go                    the contract: provenance, attribution, secrets
  schemas/record.json           the authored record (no `agrees`)
  schemas/flat.json             the projection (has `agrees`, `credit`)
  sources/<id>.json             a source's location, terms and credit
  records/<source>/<subject>/<predicate>.json    the authored data
  concordance.csv               GENERATED
  concordance.feature           GENERATED
  README.md                     this file; the table below is GENERATED
  fixtures/base/                the synthetic record the fixtures start from
  fixtures/broken/<check>/      one mutation per failing check
  check_test.go                 materialises each fixture root and asserts the failure
```

## What `--check` proves — and what it does not

Proves: every record is strictly typed and schema-valid; the path is the key;
the source is declared and its URL is the source's own; `our_claim.basis` exists
under the repo root; both claims are normalised tokens; dates parse,
`review_by > checked_at`, `retrieved_at <= checked_at`; the predicate is in
vocabulary; `agrees` recomputes from the two tokens on disk; the credit the
source's licence requires is on every row, in the README region and in the
feature; every listing link a source requires is present and on the source's
host; no file contains a credential; `concordance.csv`'s header is the flat
schema's properties in order; no duplicate key; the three artifacts are byte for
byte what the records project to; and the feature canonicalises to a non-zero
token count.

Does **not** prove: that a third party's claim is *true*, that our own claim is
*right*, or that the source's terms permit what we store. It proves that we
stored the least we could, credited what we must, and that nothing drifted.

## Adding a record

1. Decide you may read the source — a human's call, not the tool's. Its location,
   terms and credit go in `sources/<id>.json` with `policy: "pointer"`.
2. Pick a subject **you already hold a claim about**, and name the file that
   holds it in `our_claim.basis`.
3. Reduce their claim to one token. If you cannot, stop — it is not well-typed.
4. Write `records/<source>/<subject>/<predicate>.json`, then
   `go run ./cmd/concordance --render` and `--check`.

### Why there are no records yet

Both sources are declared. Neither yields a record, for two different reasons:

- **`mcpregistry`** answers without a key and its licence is permissive, so the
  *their* side is free. What is missing is **our** side: we hold no claim about a
  third-party MCP server. Every record needs one, and inventing it is the defect
  this rule exists to prevent. Our own surfaces (`sheets/`) are the only subjects
  we hold claims about, and no public directory lists them.
- **`glama`** needs a working key. The key supplied to this session returns
  **401 unauthorized** from `/api/mcp/v1/servers`; a replacement is at
  `https://glama.ai/settings/api-keys`. Even with a key, the same gap applies:
  glama supplies *their* claim, not ours.

An empty concordance that passes `--check` is a legitimate state.

**The missing piece is an instrument, not a licence.** The lab already owns one:
`smoke-alarm` probes an MCP surface and produces an *observation*. A probe result
is exactly the "our own claim" a record needs, and it is a stronger basis than a
survey answer. The next step is therefore: pick a server from the registry, probe
it with the lab's own instrument, write the observation into the repo, and point
`our_claim.basis` at it.

## Findings

Four things this work established, none of which were known before:

1. **Glama ingests the Official MCP Registry.** Its own data-sources note lists
   community submissions, automated discovery, *the Official MCP Registry*, and
   the `awesome-mcp-servers` list. So a registry-vs-glama concordance would
   mostly measure **ingestion, not independent curation** — a comparison between
   them is worth less than it looks, and any future record should say which of
   the two is the independent position.
2. **The registry is a firehose, and its names are not vendors.** Searching it
   for `github`, `slack`, `netlify` or `postgres` returns almost entirely
   third-party re-packagings (`ai.smithery/*`, `com.mcparmory/*`,
   `ai.waystation/*`). An entry's reverse-DNS prefix identifies **who published
   the entry**, not who makes the platform — so "the registry says the GitHub MCP
   server is by smithery" is a true statement about the registry and a false one
   about GitHub. That is precisely the class of disagreement this dataset exists
   to catch, and it is why `vendor` is the interesting first predicate.
3. **A marketplace's licence is an output constraint, not a footnote.** Glama's
   obligation attaches to every *view* of the data, so it belongs in the
   projection — which is why `credit` and `listing_url` are columns.
4. **Glama anticipates our case and offers a way out.** "A research dataset" and
   "a CLI surface" are named as things attribution does not fit, waivable under a
   commercial licence. That is the clean path for this directory, and it is a
   decision for a human.

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
