# The canon logic — hand-executable

> **Source and authority.** Transcribed from
> `github.com/james-gibson/isotope v0.4.2-0.20261017004323-9cae21124db1`
> (commit `9cae211`), `canonicalization.go` + `steps.go`. That code is the
> authority; this file is a transcript of it. If the module moves, **this file is
> stale** — re-read the canon and re-derive, then bump the version. Do not
> silently reconcile a newer canon here.

This is **the logic**, written so you can apply it with a pen and a shell, with no
access to the canon code. It is deliberately a description of behaviour, not a
copy of any file. Reproduce it exactly — including the parts that look like bugs.

Everything below is in the direction *spec → declarations → isotope*.

---

## 1. Parse

Split the source on `\n`. For each raw line, `trim` both ends. Then:

- **skip** the line if it is empty or starts with `#`.
- **`Feature:` start** — a line beginning with `Feature:` (or the localized forms
  `Característica:`, `Fonctionnalité:`, `フィーチャー:`). Its *name* is everything
  after the **first** `:`, trimmed. Starting a feature **clears** the current
  scenario.
- **`Scenario:` start** — a line beginning with `Scenario:` (or `Escenario:`,
  `Scénario:`, `シナリオ:`). Its name is everything after the first `:`, trimmed.
  Begins a new scenario under the current feature.
- **step line** — only if a scenario is currently open. A step line begins with a
  step keyword (see §2). The keyword is normalized to canonical
  `GIVEN/WHEN/THEN/AND/BUT`; the *text* is the remainder, trimmed.
- **anything else** — ignored. In particular `Background:` and `Rule:` lines are
  not feature/scenario starts and are dropped, and **steps under them are not
  attributed to any scenario** unless a scenario is already open above them.

> Caveat that matters: because only `Feature:` clears the current scenario, a step
> line that appears after one scenario and before the next `Scenario:` line would
> attach to the *previous* scenario. Confirm the spec has no such step before
> trusting the count.

## 2. Keywords

Canonical step keywords: `GIVEN`, `WHEN`, `THEN`, `AND`, `BUT`. The English
prefixes `Given`, `When`, `Then`, `And`, `But` normalize to those. (Localized
prefixes exist — French, Spanish, German, Japanese — and normalize the same way;
the workspace gate is English.)

A step line matches the **first** keyword prefix found. The prefix must be at the
start of the trimmed line; the rest is the step text.

## 3. The workspace step-definition table

A definition is a pattern + a kind + a value function. A pattern uses `{name}`
placeholders; each placeholder captures **any run of non-`"` characters**, and the
whole pattern is anchored (`^…$`) and case-sensitive. Match against the trimmed
step text.

| pattern (exact) | kind | value |
| --- | --- | --- |
| `the resource file "{path}"` | `resource_file` | `{path}` |
| `the resource file "{path}"` | `resource` | `ResourceURI({path})` |
| `an executable endpoint "{path}"` | `tool_file` | `{path}` |
| `an executable endpoint "{path}"` | `tool` | `ToolName({path})` |
| `the template "{path}"` | `prompt_file` | `{path}` |
| `the template "{path}"` | `prompt` | `PromptName({path})` |

**Every matching definition applies** — one step declares both the file and the
surface name. A definition whose value is the empty string is skipped. Walk the
spec's scenarios and steps **in source order**, and within a step walk the table
**top to bottom**; that fixes the order of `declarations`.

### Name maps

```
ResourceURI(p):
    rel := trimPrefix(p, "/resources/")
    rel := trimSuffix(rel, ext(rel))        # ext() is the final ".xxx" ("" if none)
    return "resource://" + rel

ToolName(p):
    return base(trimSuffix(p, "@"))         # base() is the last path segment

PromptName(p):
    return base(trimSuffix(p, ".tmpl"))
```

Worked: `/resources/workspace/config.json` → `resource://workspace/config`;
`/tools/echo@` → `echo`; `/prompts/summarize.tmpl` → `summarize`.

## 4. Expectation — `ByKind`

For a kind, take its values **in declaration order**, keep the first occurrence of
each non-empty value (dedupe, first-wins), then **sort ascending**. That sorted
list is that kind's expectation (`tools`, `resources`, `prompts`).

## 5. Capability families → `min_capabilities`

This mapping belongs to the *caller*, not the canon. For each surface kind that has
a non-empty expectation, emit its family:

| kind | family |
| --- | --- |
| `tool` | `tools/list` |
| `resource` | `resources/list` |
| `prompt` | `prompts/list` |

Sort the families. (The `*_file` kinds are on-disk conventions and produce no
family.)

The harness agrees on the spelling: in smoke-alarm's `capabilityGap`, a
min-capability is a **method name** (`tools/list`) and the family it is compared
against is "the part before `/`" — the key an MCP surface uses under
`capabilities`. So the value is `tools/list`, not `tools`.

## 6. Normalize step/name text

Applied to feature names, scenario names, **and step text** alike:

1. **lowercase** the text.
2. **remove articles** — for each word in the list below, delete every
   word-boundary occurrence (`\b<word>\b`). Order is the list's order.

   ```
   the a an        un une des     el la los las un unos unas
   le la l' les    ein eine einen der die das     の を に は
   ```

3. **remove punctuation** — delete every character that is not a word character
   (`\w` = `[A-Za-z0-9_]`), whitespace, or hyphen (`[^\w\s-]` → `""`). This
   deletes `"`, `/`, `.`, `:`, `@`, `(`, `)`, `—`, …
4. **hyphenate** — split on whitespace (`strings.Fields`), join the pieces with
   `-`.

> Consequence to reproduce, not repair: because step 3 deletes `/` and `.` without
> inserting a space, a path *squashes into one token*.
> `the resource file "/resources/workspace/config.json"` →
> `resource-file-resourcesworkspaceconfigjson`.

## 7. Hash a normalized text

```
hashText(s) = first 8 bytes of SHA256(s), as 16 lowercase hex chars
```

`hashText` is applied to the **normalized** text (step 6), and its 16-hex output is
what the canonical token carries for a step. It is **not** the HMAC (that is §9).

## 8. Canonical token stream

Walk the parsed tree in source order and emit one token per line:

```
FEATURE:name:<normalize(feature name)>
SCENARIO:name:<normalize(scenario name)>
<KEYWORD>:hash:<hashText(normalize(step text))>       # KEYWORD ∈ GIVEN WHEN THEN AND BUT
```

Concatenate the lines with `\n`, with **no trailing newline**. That string is the
isotope's `raw` (also called the canonical token string).

## 9. Sign

```
signature = hex( HMAC-SHA256( key = signingKeyBytes, message = raw ) )

isotope = { family, version, signature, raw }
```

`raw` is byte-exact (§8). A trailing newline changes the signature. `key` is the
session's seed bytes; report only its name.

---

## Checkpoints (catch a normalization bug early)

Apply §6–§7 to these three step texts; if any mismatches, stop and fix the
derivation before going further:

| step text | normalized | hash |
| --- | --- | --- |
| `the resource file "/resources/workspace/config.json"` | `resource-file-resourcesworkspaceconfigjson` | `a05ae99dae3b41c5` |
| `an executable endpoint "/tools/echo@"` | `executable-endpoint-toolsecho` | `f9315b8edd9f3f79` |
| `the template "/prompts/summarize.tmpl"` | `template-promptssummarizetmpl` | `2ac7704877961ba3` |

These were derived by hand from the rules above and independently re-derived in a
throwaway script (SHA-256 + word-boundary article strip + `[^\w\s-]` strip) — no
canon code.
