# MCP Surface Safety Data Sheets

> One safety data sheet per MCP surface in the lab, laid out as Gherkin.

Lab-safety's stated job is to **govern what MCP surfaces are permissible**
(README → *the lab*). A surface cannot be gated until it is **declared**: what it
exposes, what each exposed tool can touch, and what a caller has to have proved
before it may touch it. This directory holds those declarations.

A sheet borrows the structure of a **material safety data sheet** — the
16-section format a chemist uses to describe a substance — and applies it to an
MCP surface. The sections keep their order (§1 identification … §16 other
information) so a reader knows where to look without being told.

The difference from ordinary documentation: **every section is also Gherkin.**
The markdown sheet is the readable face; the linked `features/*.feature` file is
the same content as executable, canonical scenarios. The isotope protocol
(`pickled-onions` → `isotope.Canonicalize`) signs exactly that shape, so a
declared sheet and a runnable test are one artifact. A declaration the surface
does not satisfy is therefore a *failing test*, not a stale sentence.

That is the point of [#24](https://github.com/james-gibson/adhd/issues/24): a
suite that asserts tools a surface does not serve is a **false declaration**. An
SDS turns that mismatch into a §16 finding instead of a sentence nobody re-reads.

## Layout

| path | what |
| --- | --- |
| `README.md` | this file — the pattern and the section map |
| `<surface>.md` | one safety data sheet per MCP surface |
| `features/<surface>-mcp-surface.feature` | the linked executable scenarios |

## Hazard classes

Every exposed tool is labelled with one or more classes. The legend is the same
on every sheet.

| class | name | what it means |
| --- | --- | --- |
| `R` | read | returns surface state; no mutation, no egress |
| `C` | compute | pure / derived result from caller input (crypto, receipts) |
| `W` | write | mutates surface state (registry, dynamic tools, paths) |
| `E` | egress | makes the surface contact another endpoint |
| `X` | exec | runs a local process on the surface host |
| `H` | honeypot | decoy tool; invoking it marks the caller untrusted |

`E`, `X` and `H` are the classes that make a surface **hazardous to expose
beyond the lab**, because each lets a caller reach past the surface: `E` at a
third party, `X` at the host's process table, `H` at the surface's own
caller-identity ledger. `R`/`C` surfaces are safe to hand to a low-rung caller; a
surface carrying `X`, or an unrestricted `E`, should not be reachable below a
rung that requires independent verification.

## Rungs

Rungs are the isotope capability lattice (README → *the isotope protocol*),
0 (Absolute Zero) … 6 (Certification). A sheet states the **minimum rung** at
which a tool's class is permissible (§7) and what a caller at each rung may read
(§8).

## Section map (SDS → MCP surface)

| SDS § | section | what it declares for a surface |
| --- | --- | --- |
| 1 | Identification | surface id, owning tool, endpoint, transport, protocol |
| 2 | Hazard identification | which hazard classes the surface exposes |
| 3 | Composition | the tool inventory (name, class, inputs) |
| 4 | First-aid measures | the error taxonomy and how a caller recovers |
| 5 | Fire-fighting measures | containment: rate limits, proxy gate, kill switch |
| 6 | Accidental-release measures | untrusted-caller containment, honeypot handling |
| 7 | Handling and storage | auth model, rung requirements, acknowledgements |
| 8 | Exposure controls | what a caller at each rung may read |
| 9 | Physical and chemical properties | transport facts (address, framing, statelessness) |
| 10 | Stability and reactivity | degradation and chaos behaviour |
| 11 | Toxicological information | the hazardous tools, in detail |
| 12 | Ecological information | egress footprint (what the surface reaches outward) |
| 13 | Disposal considerations | leave / teardown semantics |
| 14 | Transport information | DNS / egress preconditions |
| 15 | Regulatory information | isotope certification and the lab-safety gate |
| 16 | Other information | revision, declared-vs-actual diff, links |

## Declared vs actual (§16)

Every sheet ends with a diff between what the surface's own documentation claims
and what `tools/list` actually returns. Two defects land there:

- **declared, absent** — a tool named by docs / features that the surface does
  not serve (the #24 family: `adhd.features.*`);
- **present, undeclared** — a tool the surface serves that no sheet or seed names.

Both are *findings*, not edits. §16 records them and points at the tracking
issue; the fix belongs to the owning tool.

## Status vocabulary

Because lab-safety is pre-flight and mostly unimplemented, a sheet must not
present intent as fact. Every enforcement claim carries a status:

- **enforced** — the surface actually does this today (with a code reference);
- **declared** — this is the intended contract; not yet enforced (with an issue).

## Running the features

The `features/*.feature` files are canonical scenarios, **not yet wired to a
runner in this repo**. They are written to be run against a *live surface* (point
`tools/list` / `tools/call` at the endpoint in §1) and to be canonicalised by
`pickled-onions` → `isotope.Canonicalize` unmodified. Tag scheme:

```gherkin
@lab-safety @mcp-surface @surface-<tool> @z0-physical @domain-surface-safety
```

`Rule:` blocks correspond 1:1 to the SDS sections, so a failing `Rule: §3
Composition` reads as *"the declared inventory does not match the surface"*.

## Sheets

| surface | owner | tools | sheet | features |
| --- | --- | --- | --- | --- |
| adhd MCP server | `adhd` | 18 static + dynamic | [adhd.md](adhd.md) | [features/adhd-mcp-surface.feature](features/adhd-mcp-surface.feature) |
| hosted service MCP | `ocd-smoke-alarm` | 4 | [ocd-smoke-alarm.md](ocd-smoke-alarm.md) | [features/ocd-smoke-alarm-mcp-surface.feature](features/ocd-smoke-alarm-mcp-surface.feature) |

## Adding a surface

1. Enumerate the surface with `tools/list` against a live instance; do not copy
   from the README.
2. Classify each tool (`R/C/W/E/X/H`) and record the inputs that matter.
3. Fill §1–§16, marking each enforcement claim **enforced** or **declared**.
4. Write `features/<surface>-mcp-surface.feature` with one `Rule:` per section.
5. Record the declared-vs-actual diff in §16.
