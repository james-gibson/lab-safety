# The code index as a declared state

**Companion to [`SPLASH.md`](SPLASH.md). That document keeps the trust and proxy
model; this one holds everything specific to the code index — what it claims, what
it actually sees, and what it costs.**

The index is one participant in the chain, and it answers questions about itself.
Those answers are declarations. They are useful, and they are not authority, for the
reason *Relation, not transport* gives in general form: a declared state is
information. This document records what the declaration does and does not cover,
measured on the built chain.

## Readiness is a self-report

On 2026-10-19 the index over the main working tree reported **fully indexed**:
embeddings complete, one hundred percent of files processed, no provisional state.
In the same response it reported **124,978 of 179,626 graph edges unresolved** —
roughly seventy percent of its edges pointing at targets it cannot name.

*Indexed* and *connected* are different claims, and only the first was made. The
second is the one callers depend on: `explore`-style operations that walk from a
symbol to its callers use those edges, and a resolver that skips an unresolved edge
does so silently. An impact analysis on an affected symbol under-reports, and the
response still says fully indexed.

The gap narrows to a question a probe can answer: not "did you index it", but "do
the links resolve". Until the probe exists, treat readiness as a statement about
processing and never about reachability.

## What the index does not see

Three behaviours, each of which makes an index look more complete than it is.

**Ignore rules are not inherited from version control.** A file matched by
`.gitignore` was written into a working tree and picked up by the indexer within
about twelve seconds. Exclusions are the index's own: a fixed list of build outputs,
dependency directories and artefacts (`node_modules`, `dist`, `build`, `out`,
`coverage`, `.next`, `vendor`, `_build`, `deps`, `venv`, `__pycache__`,
`priv/static`, `artifacts`, `.git`), plus, in places, a generated-or-minified-content
heuristic. The heuristic is what silently dropped auto-generated modules and hashed
webpack bundles. Whether a path is in the index is therefore not a question version
control can answer, and a `.gitignore` line is not a way to keep something out.

**A path can leave the index without an error.** Registering a nested repository as
its own target removed it from the parent's view. Queries scoped to the parent path
then returned nothing — at full readiness, with no diagnostic, and no change to the
parent's reported file count that a caller would notice. The scope a caller believes
it is searching is not always the scope that answers.

**Ranking is not authority.** Three stale clones of one repository out-ranked the
live copy for every query that touched them — nineteen of twenty hits for one
identifier — until they were explicitly excluded. The index was not wrong about what
existed; it had no way to prefer what was current. Removing the clones cut the index
from 4,092 to 3,002 files and from 25,680 to 17,760 chunks, about thirty percent of
it being duplicates.

A feature id attached to application code does not help any of this. It is a comment:
it adds a declaration, not an edge. Traceability and reachability are separate
measures, and annotating the first does not compute the second.

## Cost profile

Measured 2026-10-19, loopback, warm index, one machine (10 cores, 8 performance).

**There is a fixed floor, and it dominates.** Searching 26 files costs 129 ms;
searching 3,002 files costs 287 ms. A hundredfold more code buys a 2.2× cost. The
`limit` matters little — five results cost 287 ms, a hundred results 326 ms. So the
cost of a query is mostly the cost of *asking*, not the cost of *looking*.

That has a direct consequence for sharding: splitting one query into four pays the
floor four times unless the shards run concurrently.

**The service is genuinely concurrent.** Identical searches at increasing
concurrency, wall time and per-request median:

| concurrency | wall | p50 | throughput |
| --- | --- | --- | --- |
| 1 | 144 ms | 144 ms | 7 req/s |
| 2 | 142 ms | 140 ms | 14 req/s |
| 4 | 145 ms | 144 ms | 28 req/s |
| 8 | 169 ms | 153 ms | 47 req/s |
| 16 | 323 ms | 278 ms | 50 req/s |

Scaling is near-perfect to four, the knee is at eight, and throughput saturates
around fifty requests per second. Past the knee, adding callers buys queueing rather
than throughput: at sixteen, mean latency doubles and throughput does not move.

**Fan-out works, and the slowest shard bounds it.** Four path-scoped shards over the
same ground: 758 ms serially, 206 ms concurrently — a 3.7× speedup against a 4×
ceiling, so the parallel path is nearly free. The bound is the slowest shard, not
the sum, which is the whole argument for fanning out.

**A query that finds nothing costs more than one that finds plenty.** The same
search returning hits costs 292 ms; returning nothing costs 629 ms. An empty result
is more than twice as expensive as a productive one. This is the opposite of the
intuitive model, and it changes how a fan-out should be designed: the shard that
bounds the whole call is likely to be the one with no hits. Budget for it.

## Operational consequences

- Keep something out of the index with the index's own exclusion mechanism, not with
  version control.
- Do not infer reachability from readiness. If an answer depends on graph edges, ask
  what fraction of them resolve.
- Prefer one query over several. Shard only when the shards are genuinely
  independent and can be issued concurrently; otherwise each shard pays the floor.
- Expect the empty shard to be the slow one, and do not read its latency as evidence
  that it is doing more work.

## References

- [`SPLASH.md`](SPLASH.md) — the trust and proxy model this index sits inside.
- [`SLA.md`](SLA.md) — service levels and the graded-trust reasoning.
- [`CHIMERA.md`](CHIMERA.md) — the wider design family: composite application ids,
  the good-enough query threshold, and receipts that outlive access.
