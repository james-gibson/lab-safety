# Tool survey — what tools need from the lab before it may trust them

`lab-safety#3` carries **two** datasets. The first is the model survey
(`lab/survey/`): how agents want to be integrated. This is the second: how
**tools** answer the lab's fitness questions — `lab-safety#14`.

The interesting field is `labSafetyFitnessRequirements`: it is where a tool says
what it needs from the lab *before the lab may trust it*. That is a claim about
the lab's own gate, not about the tool's API ergonomics, and it is why the tool
answer shape is not the agent answer shape.

The arrow points the same way as the model survey, and the opposite way to a
surface sheet:

| | origin | projection | if they disagree |
| --- | --- | --- | --- |
| **surface** (`sheets/**/*.feature`) | the Gherkin | the declaration the harness reads | the surface fails its test |
| **survey** (`tool/profiles/*.json`) | the observations | the Gherkin | `--check` fails |

## Layout

```
lab/survey/
  cmd/survey (shared)         --dataset tool --check | --render | --stale
  internal/survey (shared)    schema, projection, drift, canonicalisation
  check_test.go (shared)      both datasets + their fixtures
  tool/
    schemas/flat.json         the answer columns + 5 provenance + 4 derived (46)
    schemas/nested.json       the issue's tool answer shape
    profiles/<tool_id>.json   the ONLY authored data — source of truth
    sources/issue-3.json      verbatim export of the thread
    sources/issue-3.sha256    its digest (checked)
    tools.csv                 GENERATED
    tools.feature             GENERATED
    README.md                 this file; the summary below is GENERATED
    testdata/issue-profiles.json  the 13 raw profiles (the golden fixture)
    golden_test.go            lossless projection + identity + mapping coverage
    fixtures/broken/<check>/  one mutation per failing check
```

The engine is **shared** with the model survey. This directory is a dataset, not
a second tool: the schema loader, the projector, the drift check and the Gherkin
renderer are `internal/survey`'s; only the vocabulary and the artifact names
differ. Adding a third survey is a `Dataset` value, not a fork.

## The profiles are the only authored data

A profile is the **raw answer** (the issue's shape, freeform strings and all)
plus **provenance**:

```json
{
  "provenance": {
    "source_url": "https://github.com/james-gibson/lab-safety/issues/3#issuecomment-4315175145",
    "checked_at": "2026-04-24",
    "review_by": "2026-10-24",
    "survey_method": "self-report",
    "schema_version": 1
  },
  "answer": { "toolName": "VS Code Extension API", … }
}
```

The file name is the `tool_id` (lowercase kebab-case). Nothing types it twice.

### Authored, required, validated

| field | meaning |
| --- | --- |
| `source_url` | the comment the answer came from — must appear in `sources/issue-3.json` |
| `checked_at` | the ISO date the answer was read |
| `review_by` | the ISO date it must be re-checked by (`> checked_at`) |
| `survey_method` | `self-report` \| `documentation` \| `observed` |
| `schema_version` | `1` |

### Computed by the tool — never authored

| field | rule |
| --- | --- |
| `mcp_ready` | `supports_mcp` |
| `dry_run_ready` | `supports_dry_run` |
| `autonomous_ready` | `human_in_the_loop_required == false && is_idempotent == true` |
| `risk_score` | mutability weight + blast weight + complexity weight (0–13) |

`--check` recomputes these **from the cells on disk** and fails on any mismatch.

## The vocabulary

The raw answers are preserved verbatim. The flat row adds **classes** derived by
a documented, total mapping; an answer no mapping covers fails `--check`.

| derived column | values |
| --- | --- |
| `mutability_class` | `none`, `low`, `moderate`, `high`, `critical`, `variable` (leading word) |
| `blast_class` | `none`, `local`, `project`, `org`, `production`, `physical`, `data` (table) |
| `complexity_class` | `low`, `moderate`, `high`, `extreme` (leading word) |
| `payload_class` | `low`, `moderate`, `high` (leading word) |
| `interface_families` | `mcp`, `rest`, `graphql`, `grpc`, `json-rpc`, `sdk`, `cli`, `sql`, `events`, `custom` (table, pipe-list) |
| `schema_families` | `openapi`, `graphql`, `json-schema`, `typescript`, `xml`, `sql`, `custom`, `other` (table, pipe-list) |
| `auth_families` | `none`, `api-key`, `pat`, `oauth2`, `jwt`, `iam`, `account`, `custom` (table, pipe-list) |
| `model_families` | `all`, `claude`, `gpt`, `gemini`, `codex`, `other` (table, pipe-list) |

The mapping lives in `internal/survey/tool.go`. It is deliberately **not** the
model survey's vocabulary: a tool's `stateMutability` is not an agent's
`runtime`, and "Claude Code" (a product) is not "Claude" (a family) except where
the mapping says so.

## Projection rules

- **Lists become pipe-delimited cells**, one `|`, no surrounding spaces.
- **`null` is an empty cell for a string column** and the literal `null` for a
  flag.
- **The raw prose is not rewritten.** `stateMutability` keeps
  `"High (Direct local filesystem and process mutation)"`; the class next to it
  is the normalized comparison value. A disagreement is checkable against the raw
  answer.
- **The Gherkin carries steps, not tables.** isotope's canonicaliser hashes step
  text and ignores table rows, so a table would make the spec's isotope blind to
  the data. The `Background` carries no data for the same reason.

## What `--check` proves — and what it does not

Proves:

1. every profile decodes strictly;
2. the answer validates against `nested.json`;
3. the projected row validates against `flat.json`;
4. `flat.json` is the answer columns + 5 + 4, and the projector covers exactly
   the answer columns;
5. provenance is present, dates parse, `review_by > checked_at`, the method is
   in vocabulary, `schema_version == 1`, and `source_url` is in the raw export;
6. no derived metric is authored;
7. `sources/issue-3.json` matches its sha256;
8. `tools.csv`'s header is the flat schema's properties, in order; no duplicate
   `tool_id`; no orphan profile and no orphan row;
9. cells are encoded correctly (no space around a `|`; pipe-lists hold
   vocabulary values; integer lists parse);
10. every derived metric recomputes from the row on disk;
11. `tools.csv`, `tools.feature` and the README summary region are **byte for
    byte** what the profiles project to; and
12. `tools.feature` canonicalises to a non-zero token count.

Does **not** prove:

- that a tool's self-report is *true* — `survey_method` says how it was
  obtained;
- that the class mapping is the *only* fair one — the raw answer is beside it;
- freshness. `--stale` lists profiles past `review_by`; a human re-surveys.

## The golden fixture

The model survey's golden is the issue's own CSV rows. The tool survey has **no
CSV rows in the thread** — only 13 nested profiles. So the independent golden is
the raw profiles themselves (`testdata/issue-profiles.json`): the test asserts
13 / identity, a lossless projection of every mirrored field, and total mapping
coverage. It does not invent an expected row.

## Use

```sh
cd lab/survey
go run ./cmd/survey --dataset tool --check
go run ./cmd/survey --dataset tool --render
go run ./cmd/survey --dataset tool --stale --as-of 2026-10-24
go test ./...
```

## Adding a profile

1. Read the answer and keep the original text.
2. `profiles/<tool_id>.json` — the raw answer plus provenance.
3. `go run ./cmd/survey --dataset tool --render`
4. `go run ./cmd/survey --dataset tool --check` — if it fails, the profile (or
   the vocabulary) is wrong, not the artifact.

## The survey

<!-- BEGIN GENERATED: tool-survey-summary -->
| tool_id | tool_vendor | primary_interface_type | mutability_class | blast_class | complexity_class | payload_class | minimum_capability_rung | max_acceptable_imaginary_distance | supports_mcp | is_idempotent | human_in_the_loop_required | risk_score | review_by |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| apollo-graphql | Apollo GraphQL | MCP / GraphQL | variable | data | high | moderate | 3 | 15 | true | null | true | 8 | 2026-10-24 |
| doordash-drive | DoorDash | REST | critical | physical | high | low | 5 | 5 | false | true | true | 12 | 2026-10-24 |
| github-rest-graphql-api | GitHub (Microsoft) | REST / GraphQL | critical | org | moderate | high | 4 | 8 | true | true | true | 9 | 2026-10-24 |
| google-drive-api | Google Cloud | REST | high | org | moderate | high | 4 | 10 | true | false | true | 8 | 2026-10-24 |
| intellij-platform-sdk | JetBrains | Java/Kotlin API | high | local | extreme | low | 4 | 10 | false | false | true | 8 | 2026-10-24 |
| jira-cloud-rest-api | Atlassian | REST | moderate | project | high | moderate | 3 | 20 | true | true | false | 7 | 2026-10-24 |
| netlify-api | Netlify | REST API / CLI | critical | production | moderate | high | 5 | 8 | true | true | true | 10 | 2026-10-24 |
| new-york-times-api | The New York Times | REST | none | none | low | low | 1 | 42 | false | true | false | 1 | 2026-10-24 |
| nx-build-system | Nrwl / Nx | CLI / MCP Server | high | org | high | moderate | 4 | 15 | true | false | true | 9 | 2026-10-24 |
| postgresql | PostgreSQL Global Development Group / Azure / Supabase | Wire Protocol (SQL) / MCP Tooling | critical | data | moderate | high | 3 | 20 | true | null | true | 9 | 2026-10-24 |
| slack-api | Salesforce / Slack | Web API / Events API / MCP | moderate | org | moderate | low | 3 | 25 | true | false | true | 7 | 2026-10-24 |
| soundcloud-api | SoundCloud | REST | low | data | low | moderate | 1 | 42 | false | true | false | 5 | 2026-10-24 |
| vs-code-extension-api | Microsoft | VS Code Extension API (TS/JS) | high | local | high | moderate | 4 | 12.5 | true | false | true | 7 | 2026-10-24 |
<!-- END GENERATED: tool-survey-summary -->

Generated from `profiles/*.json` by `survey --dataset tool --render`; checked by
`survey --dataset tool --check`. The full 46-column projection of each row is in
`tools.csv`.
