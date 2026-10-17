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
   granted to us. Reading a claim in order to agree with it is the act, and that
   is a terms question independent of copyright.

So this is a **separate dataset with its own instrument**. It shares the *shape*
of `lab/survey` — records are the only authored data, everything else is a
projection, `--check` proves no drift — and deliberately shares none of its
*code*: the two datasets must be able to disagree, and a shared checker that
broke would make both lie.

> **Not legal advice.** This directory records the terms a source is under and
> *implements* the obligations it can; it does not adjudicate them.

## The finding that matters most: there is one operator

| source | operator | licence | readable? |
| --- | --- | --- | --- |
| `glama` — [MCP Directory API](https://glama.ai/mcp/reference) | `frank-fiegel` | attribution-required | yes, with a key |
| `awesome-mcp-servers` — [punkpeye/awesome-mcp-servers](https://github.com/punkpeye/awesome-mcp-servers) | `frank-fiegel` | MIT | yes |
| `awesome-remote-mcp-servers` — [punkpeye/awesome-remote-mcp-servers](https://github.com/punkpeye/awesome-remote-mcp-servers) | `frank-fiegel` | MIT | yes |
| `mcpregistry` — [Official MCP Registry](https://registry.modelcontextprotocol.io) | `modelcontextprotocol` | permissive | yes, no key |
| `mcpmarket` — the source the proposal named | `mcpmarket` | all-rights-reserved | **no** |

Both "awesome" lists are `Copyright © Frank Fiegel (frank@glama.ai)` — the same
operator as the Glama directory — and **every entry in `awesome-mcp-servers`
carries a `glama.ai` score badge and a link to that server's Glama listing.**
Glama's own data-sources note lists the awesome list as one of its inputs. So
"Glama ingests awesome-mcp-servers" is **one operator ingesting its own list**,
and a concordance between them is self-comparison.

That is this lab's own thesis, turned on its own inputs. The lab says a node is
not trusted to report on itself and a verification chain needs an *independent*
position. The MCP directory ecosystem violates exactly that: **agreement between
these directories is not evidence.**

So it is a check. `--check` refuses to let it count:

```
independence: acme-weather/exposes-tool is compared across glama and
awesome-mcp-servers, which share the operator "frank-fiegel" — one operator
agreeing with itself is not evidence
```

`source_operators` is a projected column, so every row says who stands behind it.

**Of the five sources, only the Official MCP Registry is both independent and
readable.** That is the answer to "do we agree with a directory": pick the one
that is not the same operator.

## What we may hold: `pointer` vs `ingest`

A source's `policy` is what we *do*, not what we *may*:

| policy | means | when |
| --- | --- | --- |
| `pointer` | one normalised value token per subject, plus the listing URL | the licence does not permit copying |
| `ingest` | we may hold the source's content | the licence permits it — and then a `notice` is required |

`ingest` is refused on a licence that does not permit holding content, and a
source that declares it must carry the **notice** every copy has to reproduce.
`--check` then requires that notice to match the declaration, so the copy and the
declaration cannot drift apart.

### What we ingest is a census, not a mirror

`ingest/<source>.json` holds the **denominator, the operator and the notice** —
not the entries:

```json
{
  "source": "awesome-mcp-servers",
  "notice": "MIT License — Copyright © 2024 Frank Fiegel (frank@glama.ai)",
  "credit": "MCP server list from punkpeye/awesome-mcp-servers (MIT)",
  "retrieved_at": "2026-10-17",
  "entries": 4259,
  "entry_shape": "one `- [name](repo) [badge](glama listing) <flags> - <description>` line per server",
  "notes": "…"
}
```

The licence permits copying these lists. Mirroring them is still the wrong move,
and the numbers say why: 4,259 entries in a 1.4 MB README, pushed daily — a copy
is stale on arrival, and it is the copying this dataset exists to avoid. A census
gives the **denominator** (how many servers the ecosystem claims) and the
**operator** (so a family of properties can never again be read as independent
sources), and `entry_shape` is what turns it into a bounded slice later.

## The third URL is not a source — it is the other arrow

`agentclientprotocol.com/rfds/mcp-over-acp` is a **specification** (a draft RFD,
by `nikomatsakis`, targeting MCP 2026-07-28). It is not a directory of subjects,
so it does not belong in `sources/`.

It belongs to the *other* arrow. In this repo a spec is the **origin** of a
declaration — that is `sheets/`, where the Gherkin is the origin and a surface
that does not satisfy it is a failing test. The RFD is exactly that shape: it
declares that an MCP server may be declared with `"type": "acp"`, and that
independent MCP requests then travel over the ACP connection with no second
process and no network endpoint.

Two concrete consequences, neither of which needs new machinery here:

- **`acp` is a `transport` value.** `transport` is already a predicate, so a
  record about a server's transport can carry `acp` today.
- **A surface that speaks ACP is a `sheets/` declaration to derive**, not a
  concordance record — with the same rule as any other sheet: the spec is the
  origin, and a surface that does not satisfy it is a failing test.

Filed as a finding rather than ingested, because ingesting a spec into a claims
dataset would put an origin where a claim belongs.

## Whose data is whose

| | |
| --- | --- |
| **Ours** | our own claim, the subject, the predicate, the dates, `agrees`, the credit, the operators |
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

### Attribution

Glama's terms say the data "is licensed, not public domain", that every view must
credit Glama, and that every server presented must link to **its own listing** —
"the url the API returns for it, not a copy of the listing you host". So:

- `credit` is a **projected column**: `concordance.csv` and `concordance.feature`
  carry it per row, and the README region carries it. A view cannot be built
  without it.
- a record from a source that requires it must carry `subject.listing_url`, **on
  the source's own host**;
- an attribution-required source must record its `waiver` route — and Glama names
  our case: "an internal tool, **a research dataset**, or a voice or CLI surface"
  can have attribution waived under a commercial licence. A human decision.

### Secrets

**A key belongs in the environment, never in this repo.** `--check` scans every
file and fails on anything shaped like a credential (`glm_…`, `sk-…`, `ghp_…`,
`AKIA…`, private keys, assigned secrets). The fixture that breaks this check
*assembles* its fake key at test time, so the repository stays clean while the
check stays honest.

## A record

`records/<source>/<subject>/<predicate>.json` — **the path is the key**, and
`--check` asserts all three levels equal the three fields.

```json
{
  "source":     { "id": "mcpregistry", "url": "https://registry.modelcontextprotocol.io", "retrieved_at": "2026-10-17" },
  "subject":    { "id": "playwright-mcp", "kind": "mcp-server",
                  "listing_url": "https://registry.modelcontextprotocol.io/v0/servers?search=playwright-mcp" },
  "predicate":  "transport",
  "their_claim": { "value": "streamable-http" },
  "our_claim":   { "value": "streamable-http", "basis": "sheets/adhd.md" },
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
    record.go                   strict decode; sources; policy; path-is-the-key
    render.go                   the projection + `agrees` + `credit` + operators
    check.go                    the contract: provenance, attribution, ingest, independence, secrets
  schemas/record.json           the authored record (no `agrees`)
  schemas/flat.json             the projection (has `agrees`, `credit`, `source_operators`)
  sources/<id>.json             a source's location, terms, licence, policy, operators, credit
  ingest/<id>.json              a census of what a permissive source holds (never a mirror)
  records/<source>/<subject>/<predicate>.json    the authored data
  concordance.csv               GENERATED
  concordance.feature           GENERATED
  README.md                     this file; the table below is GENERATED
  fixtures/base/                the synthetic record the fixtures start from
  fixtures/broken/<check>/      one mutation per failing check
  check_test.go                 materialises each fixture root and asserts the failure
```

## What `--check` proves — and what it does not

Proves: every record is strictly typed and schema-valid; the path is the key; the
source is declared and its URL is the source's own; `our_claim.basis` exists under
the repo root; both claims are normalised tokens; dates parse, `review_by >
checked_at`, `retrieved_at <= checked_at`; the predicate is in vocabulary;
`agrees` recomputes from the two tokens on disk; the credit the source's licence
requires is on every row, in the README region and in the feature; every listing
link a source requires is present and on the source's host; an `ingest` source has
a permissive licence, a notice, and a census whose notice and credit match the
declaration; **no two sources sharing an operator are treated as two positions**;
no file contains a credential; `concordance.csv`'s header is the flat schema's
properties in order; no duplicate key; the three artifacts are byte for byte what
the records project to; and the feature canonicalises to a non-zero token count.

Does **not** prove: that a third party's claim is *true*, that our own claim is
*right*, or that a source's terms permit what we store. It proves that we stored
the least we could, credited what we must, and that nothing drifted.

## Adding a record

1. Decide you may read the source — a human's call, not the tool's. Its location,
   terms, licence, policy and operators go in `sources/<id>.json`.
2. Pick a subject **you already hold a claim about**, and name the file that holds
   it in `our_claim.basis`.
3. Reduce their claim to one token. If you cannot, stop — it is not well-typed.
4. Write `records/<source>/<subject>/<predicate>.json`, then
   `go run ./cmd/concordance --render` and `--check`.

### Why there are no records yet

Five sources are declared; none yields a record, and the reason is the same each
time: **a record needs *our* claim, and we hold none about a third-party server.**
Every source here supplies the *their* side for free. Our own surfaces (`sheets/`)
are the only subjects we hold claims about, and no public directory lists them.
Inventing our side is the defect rule 1 exists to prevent.

An empty concordance that passes `--check` is a legitimate state.

**The missing piece is an instrument, not a licence.** The lab already owns one:
`smoke-alarm` probes an MCP surface and produces an *observation*, which is a
stronger `our_claim.basis` than a survey answer. And
`awesome-remote-mcp-servers` is the right place to point it: every one of its
1,071 entries names a **live endpoint**, so a bounded slice can be *observed*
rather than copied. That is the next step, and it is the lab's own thesis — a
claim verified by an independent instrument, not by another directory.

## Findings

1. **The directory ecosystem is one operator.** `glama`, `awesome-mcp-servers`
   and `awesome-remote-mcp-servers` are all Frank Fiegel (`frank@glama.ai`), and
   the server list carries Glama badges on every entry. Agreement between them is
   self-comparison, so `--check` now refuses to count it as evidence. Only the
   Official MCP Registry is both independent and readable.
2. **`mcpmarket` cannot be read under its own terms.** Client-side rendered, "all
   rights reserved", with a `/terms` page. Kept declared, because it is the source
   the question was asked about and "we looked and could not read it" is part of
   the answer.
3. **The registry's names are not vendors.** Searching it for `github`, `slack`,
   `netlify` or `postgres` returns almost entirely re-packagings
   (`ai.smithery/*`, `com.mcparmory/*`, `ai.waystation/*`). An entry's reverse-DNS
   prefix identifies **who published the entry**, not who makes the platform — so
   "the registry says the GitHub MCP server is by smithery" is true of the
   registry and false of GitHub. That is exactly the disagreement class this
   dataset exists to catch, and why `vendor` is the first predicate worth having.
4. **A permissive licence is not a reason to mirror.** The two lists are MIT and
   we may copy them; they are still 4,259 and 1,071 entries, pushed daily. A
   census is the honest ingest.
5. **Glama anticipates our case.** Attribution is waivable for "a research
   dataset" or a CLI surface under a commercial licence.
6. **MCP-over-ACP is a spec, not a source.** It declares `"type": "acp"` as a way
   to carry MCP over an ACP connection with no second endpoint — so it is a
   `transport` value here and a `sheets/` declaration there, and it belongs to the
   other arrow.

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
| source_id | source_operators | subject_id | predicate | their_claim | our_claim | agrees | our_basis | checked_at | review_by |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
<!-- END GENERATED: concordance-summary -->

Generated from `records/**` by `concordance --render`; checked by
`concordance --check`. The full projection of each row is in `concordance.csv`.
