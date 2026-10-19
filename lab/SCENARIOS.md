# Scenarios: a natural-language world model as a test harness

**Status:** findings, from the historical design conversation *Bed and Breakfast Testing
Scenario*. This is the most directly reusable artifact in the batch: a way to
integration-test a natural-language tool by giving it a small world and a set of claims
about that world, two of which are true.

## The tool being tested

A CLI world model with a deliberately tiny grammar. Verbs are bound to a target type,
which is the whole type system:

```
enter  → space      leave  → space     watch  → space
see    → item       read   → item      hold   → item
request → item
```

Building a world:

```sh
innkeeper add-space lobby
innkeeper add-space vault --special
innkeeper connect lobby vault --transparent
innkeeper enter innkeeper lobby      # the innkeeper plane enters
innkeeper enter james lobby          # the james plane enters
innkeeper place tray in lobby
innkeeper place chest in vault
```

Three ideas are doing the work here, and they are worth stealing:

- **Planes.** Participants act from their own plane, so the same space holds different
  observers. "James enters the lobby" and "the innkeeper enters the lobby" are different
  events in the same room.
- **Special spaces.** `--special` marks a space whose contents are not ordinary.
- **Transparent connections.** `connect A B --transparent` lets a watcher see into `B`
  without entering it — an observation channel that is not a movement. That is exactly
  the "watch a space" verb made structural.

## The test method: two truths and a lie

A scenario is a world plus **three claims about it, of which two are true**. The tool
under test is asked to evaluate them. This is better than an assertion list because:

- a true/false verdict on prose cannot be pattern-matched from the prompt;
- one false claim per set means a model that always answers "true" scores 0;
- the claims are cheap to write and can be versioned with the world.

Requested at the time: **at least three groups**, so a single lucky guess does not pass
the suite.

## The four scenario families produced

1. **The innkeeper** (the original) — a bed and breakfast, trays, a vault, a towel.
2. **A different time of day and year** — same logic, reusing a name or two and some
   items, to test that the tool does not over-fit to one setting.
3. **A diner** — the original shape transported to another venue.
4. **The merge queue** — software developers landing a release. The interesting one, and
   the best stress case.

## The merge-queue scenario, and why it is the useful one

*Latchkey Labs, 11:20 PM Thursday, five engineers racing a midnight freeze.*

**Spaces (6).** Terminal (the release manager's shell), Review Board (the PR list),
Editor (each developer's IDE), Chat Channel, Staging Server, and **CI Pipeline** — marked
special and transparent, so its streaming log is watchable without being entered.

**Characters.** Wren (authored the feature PR, first in queue), Devon (approver for
security-sensitive PRs), Ash (CSS refactor that conflicts with Wren's), Pria (owns the
deploy config and CI), Nico (junior, first release, shadowing).

**Fixed items — the modelling rule worth keeping.** Some things cannot be moved, and
the reason is always the same: *they are processes or environments, not artifacts.*

| item | space | why fixed |
| --- | --- | --- |
| Merge Queue | Terminal | a serialised system process, not a file you pick up |
| CI Dashboard | CI Pipeline | the pipeline's built-in view, not a portable thing |
| Staging Database | Staging Server | a running Postgres — queryable, not movable |
| Review Guidelines | Review Board | a pinned doc, locked at the top |
| Chat Pin | Chat Channel | a pinned message, admin-locked |

That distinction — movable artifact versus immovable process — is the same one
[`TOPOLOGY.md`](TOPOLOGY.md) draws between a `Service` and its `bindings`, and the same
one the trust model draws between a record and the route that carried it. A world model
that lets you pick up a database is not modelling anything real.

## Where this connects

- The verb/target-type map is a capability check in miniature: `hold` cannot take a
  space, so the grammar itself refuses a whole class of wrong claim. Compare the
  Gherkin decomposition in [`SPLASH.md`](SPLASH.md) §*Decomposition by feature*, where a
  declared feature is the input and the observed receipt is the output.
- Planes are the same idea as the observation/invariant split elsewhere in this
  directory: two observers of one space, each with their own view.
- The "two truths and a lie" set is the cheapest possible harness for a claim, and it
  generalises: three claims, two true, is enough to falsify a model that always agrees.

## Open

- No scoring rule was fixed for a partially correct answer — the suite grades verdicts,
  not reasoning.
- Transparent spaces have no stated limit on nesting. A transparent space inside a
  transparent space was raised and not settled.
