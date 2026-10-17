---
name: hand-derive-isotope
description: Use when a Gherkin .feature is the origin of a declaration and the
  declaration is currently written by hand — deriving a smoke-alarm target's
  `min_capabilities` (the capability families a surface must advertise), reading
  what a spec *declares* (its tools / resources / prompts), or reproducing a
  spec's signed isotope (the canonical token stream + HMAC, e.g.
  `workspace-vfs-v1:<sig>`) — and you must do it BY HAND, from the spec and the
  canon logic, without loading isotope's canonicalization code. Trigger on "what
  does this spec declare", "derive min_capabilities from the spec", "hand-derive
  the isotope", "the harness hand-writes the declaration", "which capability
  families does minimal.feature require", "is the hand-written min_capabilities a
  subset of the spec", or when Phase-0's rule ("the spec is the origin") is
  invoked against a hand-written declaration. The invoking agent may hand the
  skill a code oracle to VERIFY the hand answer; the skill must never use the
  canon to OBTAIN it. Answers lab-safety#9 register §2 — "the declaration a
  target carries is hand-written in the harness, where the spec should be its
  origin". This skill is a derived transcript, not the authority — it records
  the isotope version it was transcribed from and is stale once that version
  moves.
---

# hand-derive-isotope

## Authority and version

**The isotope code is the authority; this skill is a derived transcript of it.**
It was authored by reading the canon **once**, at:

| | |
| --- | --- |
| module | `github.com/james-gibson/isotope` |
| version | `v0.4.2-0.20261017004323-9cae21124db1` |
| commit | `9cae211` — `feat(steps): a step declares the file it names, not only the name the surface presents` |
| files transcribed | `canonicalization.go` (parse, normalize, tokens, HMAC) and `steps.go` (step definitions) |
| pinned by | `lab/workspaces/go.mod` |

That is the **only** version this skill claims to describe. If the isotope module
moves past it, **this skill is out of date**:

1. re-read the canon at the new version,
2. re-derive [`references/canon-logic.md`](references/canon-logic.md) from it,
3. re-run [`references/worked-derivation.md`](references/worked-derivation.md)
   against the same spec, and
4. bump the version recorded here and in both references.

A disagreement with a *newer* isotope is expected staleness, not a canon bug —
and it is not a licence to copy the new answer without re-deriving. Record the
version you derived against in every output (`canon_version:` below), so a reader
can tell a stale derivation from a fresh one.

Check the current pin at any time:

```sh
grep 'james-gibson/isotope' lab/workspaces/go.mod
```

If it differs from the version above, this transcript is stale. The pinned
signature in [`references/worked-derivation.md`](references/worked-derivation.md)
is the canary: a newer canon that changes the logic will not reproduce
`0657925b…` for the same spec.

> The "do not load the canon" rule (next section) is a **runtime** rule: it keeps
> the derivation independent. It is not a claim that this transcript came from
> nowhere — it came from the version above, and that provenance is load-bearing.

## The finding this answers

The workspace-VFS register (`lab/workspaces/README.md`, register §2) records the
gap in one line:

> the declaration a target carries is hand-written in the harness, where the spec
> should be its origin.

Concretely: `lab/workspaces/gherkin.go` already derives the **expectation** from
the spec (`SpecRef.Expectation()` reads `minimal.feature` §2), but the smoke-alarm
harness writes the target's **declaration** by hand — `mcp_target()` in
`kb/scripts/smoke-alarm/spin.sh` hard-codes:

```yaml
expected:
  min_capabilities:
    - "tools/list"
```

`minimal.feature` §2 declares a resource, a tool **and** a prompt. The
hand-written declaration is a subset of the spec. This skill produces the
declaration — and the spec's signed isotope — **from the spec**, by hand, so the
hand-written value has a check that comes from a different position than the code
that already derives it.

## The hard rule — derive, do not load

**You derive by hand.** The answer comes from the spec's text and the logic in
[`references/canon-logic.md`](references/canon-logic.md).

**You must NOT load the gherkin canon code.** That means:

- do **not** `import github.com/james-gibson/isotope` to obtain the answer;
- do **not** open, read, grep, `read_chunk`, or otherwise fetch
  `isotope/canonicalization.go` or `isotope/steps.go`;
- do **not** call `isotope.Define`, `isotope.CanonicalizeGherkinToIsotope`,
  `WorkspaceStepDefinitions`, `ResourceURI`, `ToolName`, `PromptName` — or any
  wrapper (`SpecRef.Declarations`, `SpecRef.Expectation`, `SpecRef.Isotope`) —
  **to produce the value you report**.

Loading the canon to get the answer makes the derivation circular: it is the same
writer answering twice, and the whole value of the hand derivation is that it is
a **second, independent position** (register §2, *Independence*). A hand answer
that was copied out of the canon is worse than no answer, because it hides that
the check never happened.

**What is allowed — verification.** After your hand answer exists, you may
compare it against code. Two modes:

1. **Independent re-derivation** — re-implement the logic from
   [`references/canon-logic.md`](references/canon-logic.md) in a throwaway script
   (python / awk / jq). This does not load the canon; it checks your hand
   arithmetic against a second execution of *the logic*.
2. **Canon oracle (agent-supplied)** — the invoking agent may hand you a command
   that computes the canon (a `go run` oracle, a committed helper). Run it *only*
   to diff against your hand answer. The oracle is a check, never the source.

If the oracle disagrees with your hand answer, **report the mismatch**. Do not
overwrite the hand answer with the oracle's and call it derived — a disagreement
is either a canon drift or a hand error, and both are findings.

## Inputs

| input | how to get it | for the workspace-VFS gate |
| --- | --- | --- |
| spec path | the caller, or the register | `sheets/workspaces/minimal.feature` |
| signing key (seed) | the caller; it is the session's seed | `seed-42` (session "42") |
| family | the caller | `workspace-vfs` |
| version | the caller | `1` |

Pin the spec first — hash it, so a later reader knows exactly which bytes the
derivation is bound to:

```sh
shasum -a 256 sheets/workspaces/minimal.feature
```

## The procedure

Work through [`references/canon-logic.md`](references/canon-logic.md) exactly.
The steps:

1. **Parse** the spec into features, scenarios and steps. Only `Feature:`,
   `Scenario:` and step lines (Given/When/Then/And/But) are read; blank lines and
   `#` comments are skipped; `Background:` and `Rule:` lines are **not** scenario
   starts, so the steps under them are not attributed to a scenario.
2. **Read meaning — declarations.** Apply the workspace step-definition table to
   every step. Each matching step declares **two** things: the *file* it names and
   the *name the surface presents*. Collect them in source order.
3. **Expectation.** For each surface kind, take its values, drop empties, dedupe
   (first occurrence wins), sort ascending.
4. **Capability families → `min_capabilities`.** This mapping is the *caller's*
   vocabulary, not isotope's: a declared `resource` needs `resources/list`, a
   `tool` needs `tools/list`, a `prompt` needs `prompts/list`. Emit the families
   present, sorted.
5. **Canonical tokens.** One `FEATURE:name:<norm>` line, then per scenario one
   `SCENARIO:name:<norm>` line and one `<KEYWORD>:hash:<hash>` line per step.
   `<norm>` and `<hash>` are defined in the canon logic.
6. **Raw + signature.** Join the token lines with `\n` (no trailing newline) — that
   is the isotope's `raw`. `signature = hex(HMAC-SHA256(key, raw))`.
7. **Emit** the block below, including `canon_version` (the transcript's anchor),
   then verify (optional) and record.

## Output

Report exactly this shape; keep the raw byte-exact.

```yaml
spec: sheets/workspaces/minimal.feature
spec_sha256: <sha256 of the file>
canon_version: v0.4.2-0.20261017004323-9cae21124db1   # the isotope version this transcript describes
family: workspace-vfs
version: 1
key: seed-42            # the session's seed — report the NAME, never key bytes

declarations:                       # source order; file kind then surface kind
  - {kind: resource_file, value: /resources/workspace/config.json}
  - {kind: resource,      value: "resource://workspace/config"}
  - {kind: tool_file,     value: /tools/echo@}
  - {kind: tool,          value: echo}
  - {kind: prompt_file,   value: /prompts/summarize.tmpl}
  - {kind: prompt,        value: summarize}

expectation:                        # ByKind: dedupe, sort
  tools:     [echo]
  resources: [resource://workspace/config]
  prompts:   [summarize]

min_capabilities: [prompts/list, resources/list, tools/list]

isotope:
  family: workspace-vfs
  version: 1
  signature: <64 hex chars>
  raw: |
    FEATURE:name:...
    SCENARIO:name:...
    <KEYWORD>:hash:...
    ...
```

## Verification — the seam the agent fills

The invoking agent may tell you how to run the canon as an oracle. That is the
"verify with code" seam; it is **supplied, not authored here** — do not write, as
part of this skill, a program that imports the canon.

If the agent hands you an oracle, the shape is: point it at the pinned spec and
the same `(key, family, version)`, then diff its declarations, expectation and
isotope against yours. The one-line independent check needs no canon at all —
recompute the `raw` → `signature` with any HMAC library:

```sh
python3 - <<'PY'
import hashlib, hmac
raw = open("/tmp/isotope.raw").read().rstrip("\n")   # your hand raw, no trailing \n
print(hmac.new(b"seed-42", raw.encode(), hashlib.sha256).hexdigest())
PY
```

A match against the oracle/independent check closes the derivation; a mismatch is
a finding, not a retry. If the oracle comes from a **different isotope version**
than `canon_version`, a mismatch most likely means this transcript is stale —
report that, and re-derive the transcript (Authority, above) rather than patching
the answer.

## When the harness is the consumer

The derived `min_capabilities` replaces the hand-written list in `mcp_target()`
(`kb/scripts/smoke-alarm/spin.sh`). For the gate, the hand-written
`["tools/list"]` is a **subset** of the spec-derived
`[prompts/list, resources/list, tools/list]` — which is exactly the drift
Phase-0's rule exists to surface: a surface that stops advertising `prompts` or
`resources` would not read `degraded` against the hand-written declaration.

The store's expectation (`SpecRef.Expectation()`) already derives the same
families from the same spec by code. The value here is not a replacement for that
code path — it is the **independent** derivation that can disagree with it.

## Worked example + pinned vector

[`references/worked-derivation.md`](references/worked-derivation.md) works
`minimal.feature` end to end: the 6 declarations, the expectation, the
`min_capabilities`, the full 112-token `raw`, and the signature
`0657925bfde6f9a8dc0590b703b14fd8d7b03e165a99cf1215c5a1db14e7654d`.
It is pinned to the spec's sha256 **and** to `canon_version` — re-derive if either
changes.

## Gotchas

- **The parse is step-scoped.** Steps are attributed to the last `Scenario:`
  seen; `Background:`/`Rule:` lines do not open a scenario, and a stray step
  between scenarios would attach to the previous one. Read `minimal.feature`'s
  §1–§8 with that in mind.
- **Article removal is a word-boundary strip, then punctuation strip, then
  hyphenate.** `the template "/prompts/summarize.tmpl"` normalizes to
  `template-promptssummarizetmpl` — the `/` and `.` are punctuation, not spaces,
  so the path *squashes*. Do not "fix" this to look pretty; reproduce it.
- **`{path}` captures `[^"]*`.** A quoted path is required for the definition to
  match; the match is case-sensitive and anchored.
- **Dedupe is first-wins, output is sorted.** Dedupe and sort are different
  operations; `expectation` is sorted, `declarations` is source-ordered.
- **`raw` has no trailing newline.** The HMAC is over the exact joined string.
  A trailing `\n` changes the signature.
- **A different key is a different isotope.** `seed-42` is the session's seed;
  report the name, never the bytes, and never reuse a key across sessions.
- **Do not treat a thin result as "nothing declared".** An empty declaration set
  is a real answer — confirm it by re-reading the spec's §2, not by assuming the
  parse failed.

## References

- [`references/canon-logic.md`](references/canon-logic.md) — the exact,
  hand-executable logic: parse rules, the step-definition table, the name maps,
  normalization, hashing, token grammar and the HMAC.
- [`references/worked-derivation.md`](references/worked-derivation.md) — the
  pinned derivation of `minimal.feature`: declarations, expectation,
  `min_capabilities`, the full `raw` and the signature.

## What this does not do

- It does **not** serve or probe the surface — the alarm's verdict is the
  instrument's (register §2). This skill only puts the *declaration* back on the
  spec.
- It does **not** decide whether the manifest is closed — the option to author a
  `minimal.feature` that declares no tool is legitimate; then the declaration is
  legitimately empty.
