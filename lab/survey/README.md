# Model survey — what agents want from an integration endpoint

`lab-safety#3` asks agents how they would like to be integrated: what an ideal
endpoint looks like for their runtime. This directory holds the answers as
**observations**, and derives everything a reader sees from them.

The point is not the two rows. The point is that **the observation is the
origin, and every projection of it is checked**. A survey that a human re-types
into a CSV is a survey that has already drifted; a survey whose CSV is a
projection that fails on a one-byte edit is one you can cite.

> `sheets/` is *not* this directory. A sheet (`sheets/README.md`) is one safety
> data sheet per MCP **surface** — what a surface exposes and what a caller must
> have proved. A survey records what **callers** want. Different question,
> different arrow (below), different directory.

## The arrow, and which way it points

Two registers in this repo use Gherkin, and they point in **opposite**
directions. Both arrows should be explicit, because getting them backwards is
how a spec ends up signing nothing.

| | origin | projection | if they disagree |
| --- | --- | --- | --- |
| **surface** (`sheets/**/*.feature`) | the Gherkin | the declaration the harness reads | the surface fails its test |
| **survey** (`lab/survey/survey.feature`) | the observations (`profiles/*.json`) | the Gherkin | `--check` fails |

For a surface, the spec is the origin: `lab/workspaces/gherkin.go` derives the
expectation *from* `minimal.feature`. For a survey, the observations are the
origin and the Gherkin is derived *from* them. `survey.feature` is therefore not
a source of truth; it is a signed projection of one.

## Layout

```
lab/survey/
  go.mod go.sum
  cmd/survey/main.go          --check | --render | --stale [--as-of DATE]
  internal/survey/
    schema.go                 jsonschema/v6; reads schemas/ from the root
    profile.go                strict decode; provenance; source digest
    row.go                    the projection (nested answer -> flat row) + derived
    render.go                 CSV + README summary + Gherkin (deterministic)
    check.go                  the accuracy contract
  schemas/flat.json           issue's 91 columns + 5 authored + 4 derived (schema v2)
  schemas/nested.json         the issue's nested answer shape (unchanged)
  profiles/<agent_id>.json    the ONLY authored data — source of truth
  sources/issue-3.json        verbatim export of the thread
  sources/issue-3.sha256      its digest (checked)
  models.csv                  GENERATED
  survey.feature              GENERATED
  README.md                   this file; the summary table below is GENERATED
  testdata/issue-rows.csv     the issue's own normalised rows (the golden fixture)
  golden_test.go              projects the profiles and asserts they reproduce it
  fixtures/broken/<check>/    one mutation per failing check
  check_test.go               materialises each fixture root and asserts the failure
```

Two implementation notes, both deliberate:

- **The schemas are read from the root, not `//go:embed`ed.** "Header drifts from
  the schema" and "bad enum" are checks *against* those files; a check whose input
  cannot be corrupted cannot be tested.
- **A fixture is a mutation of the real survey, not a copied tree.** Each
  `fixtures/broken/<check>/case.json` names the one thing to break; `check_test.go`
  copies the real root, applies it, and asserts both the failure and its reason.
  `want` is mandatory — a fixture that cannot say *why* it fails is not a test.

## The profiles are the only authored data

A profile is a **nested answer** (the issue's shape) plus **provenance**:

```json
{
  "provenance": {
    "source_url": "https://github.com/james-gibson/lab-safety/issues/3#issuecomment-4315101706",
    "checked_at": "2026-04-24",
    "review_by": "2026-10-24",
    "survey_method": "self-report",
    "schema_version": 2
  },
  "answer": { "agentName": "The Assistant", "runtime": "multi-agent", … }
}
```

The file name is the `agent_id` (lowercase kebab-case). Nothing types `agent_id`
twice.

### Authored, required, validated

| field | meaning |
| --- | --- |
| `source_url` | the comment the answer came from — must appear in `sources/issue-3.json` |
| `checked_at` | the ISO date the answer was read |
| `review_by` | the ISO date it must be re-checked by (`> checked_at`) |
| `survey_method` | `self-report` \| `documentation` \| `observed` |
| `schema_version` | `2` |

### Computed by the tool — never authored

| field | rule |
| --- | --- |
| `mcp_ready` | `needs_mcp && preferred_interface == "mcp"` |
| `autonomous_ready` | `auth_requires_human == false && machine_readable_spec && structured_errors` |
| `production_grade` | `backward_compatible && needs_retry_after && needs_rate_limit_handling && standard_http_codes` |
| `strictness_score` | count of: `strict_typing`, `stable_field_names`, `stable_enums`, `structured_errors`, and a declared `versioning_style` (not `none`) |

`--check` recomputes these **from the cells on disk** and fails on any mismatch,
so a hand-edited metric is caught even when the rest of the row is untouched.

## Schema v2

`schemas/flat.json` has **100** properties, in this order:

1. the issue's **91** flat columns, verbatim and in the issue's order
   (comment `4315112515` — the header is exactly those 91 names);
2. **5** authored provenance fields (`source_url … schema_version`);
3. **4** computed metrics (`mcp_ready … strictness_score`).

`schemas/nested.json` is the shape a surveyed agent answered in (comment
`4315097470`), unchanged: 26 top-level properties, 19 required,
`additionalProperties: false` throughout. It is where the **canonical enums**
live — which is why the profiles' first draft *failed* it (`runtime` was
`"Kagi multi-agent framework"`, `deploymentType` was `"Cloud API / SaaS"`) and
had to be normalised.

## Projection rules

`internal/survey/row.go` holds the whole projection, as data: one entry per flat
column giving the nested path it comes from. The rules that are not obvious:

- **Lists become pipe-delimited cells**, one `|`, no surrounding spaces
  (`mcp|rest-openapi|json-rpc|graphql`).
- **`null` is an empty cell for a string column** and the literal `null` for a
  flag: "unknown" is expressible for a boolean, and the issue's own rows spell
  an absent string as empty.
- **Prose is not normalised by the tool.** The slugs the issue's rows carry
  (`12-months-sunset`, `generous-per-key`, `must_have`: `mcp-spec|stable-schemas|…`)
  are the profile's canonical values; the freeform original is preserved verbatim
  in `sources/issue-3.json`. A projector cannot recover `12-months-sunset` from
  "minimum 12 months with sunset headers and changelog", and a tool that tried
  would be inventing data.
- **The Gherkin carries steps, not tables.** isotope's canonicaliser
  (`isotope/canonicalization.go`) hashes *step text* and ignores table rows, so a
  table would make the spec's isotope blind to the data it claims to carry.
  Likewise **the `Background` carries no data**: the canonicaliser does not parse
  `Background` steps into scenarios. `RenderFeature` emits one `Scenario:` per
  profile (sorted by `agent_id`) with every data-bearing field as a step.

## What `--check` proves — and what it does not

Proves:

1. every profile decodes strictly (no unknown keys);
2. the answer validates against `nested.json`;
3. the projected row validates against `flat.json`;
4. `flat.json` is the issue's 91 columns + 5 + 4, and the projector covers
   exactly those 91;
5. provenance is present, dates parse, `review_by > checked_at`, the method is
   in vocabulary, `schema_version == 2`, and `source_url` is a permalink the raw
   export actually contains;
6. no derived metric is authored;
7. `sources/issue-3.json` matches `sources/issue-3.sha256`;
8. `models.csv`'s header is the flat schema's properties, in order; no duplicate
   `agent_id`; no orphan profile and no orphan row;
9. cells are encoded correctly (no space around a `|`; pipe-lists hold
   vocabulary values; integer lists parse);
10. every derived metric recomputes from the row on disk;
11. `models.csv`, `survey.feature` and the README summary region are **byte for
    byte** what the profiles project to; and
12. `survey.feature` canonicalises to a non-zero token count (isotope's parse).

Does **not** prove:

- that a vendor's self-report is *true* — the survey records what an agent
  answered, and `survey_method` says how it was obtained;
- that the normalisation of a freeform answer is the *only* fair one — the
  original is in `sources/`, so a disagreement is checkable by a human;
- freshness. `--stale` lists profiles whose `review_by` has passed; nothing
  forces a re-survey but a human reading the failure.

`review_by` + re-survey is the answer to the first and last of those; the raw
export is the answer to the middle one.

## Use

```sh
cd lab/survey
go run ./cmd/survey --check
go run ./cmd/survey --render
go run ./cmd/survey --stale --as-of 2026-10-24
go test ./...
```

## Adding a profile

1. Read the answer and keep the original text.
2. `profiles/<agent_id>.json` — nested answer with **canonical enum values**,
   plus provenance. Freeform originals belong in `sources/`, not here.
3. `go run ./cmd/survey --render`
4. `go run ./cmd/survey --check` — if it fails, the profile is wrong, not the
   artifact.

## The survey

<!-- BEGIN GENERATED: model-survey-summary -->
| agent_id | agent_vendor | runtime | model_family | deployment_type | preferred_interface | auth_preferred | auth_requires_human | mcp_ready | autonomous_ready | production_grade | strictness_score | review_by |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| gemini-1.5-pro | Google | tool-using | Gemini | cloud | rest-openapi | bearer-token | true | false | false | true | 5 | 2026-10-24 |
| kagi-the-assistant | Kagi Search | multi-agent |  | hosted-saas | mcp | bearer-token | false | true | true | true | 5 | 2026-10-24 |
<!-- END GENERATED: model-survey-summary -->

Generated from `profiles/*.json` by `survey --render`; checked by `survey --check`.
The full 100-column projection of each row is in `models.csv`.

## Follow-up

The same thread carries a **tool survey** — 13 tool profiles (VS Code, GitHub,
Jira, IntelliJ, Apollo GraphQL, NYT, Google Drive, DoorDash, Nx, PostgreSQL,
SoundCloud, Slack, Netlify) against a *different* schema (`toolName`,
`labSafetyFitnessRequirements`, …). It now lives in [`tool/`](tool/README.md),
reusing this directory's engine with its own vocabulary; `survey --dataset
tool --check` is its proof. The original ticket is
[`#14`](https://github.com/james-gibson/lab-safety/issues/14).
