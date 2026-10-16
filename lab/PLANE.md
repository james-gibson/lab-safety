# The measure: a plane in local-time

**Status:** informational. This document defines the quantity that the figures in
`MATH.md` measure, and states why it is a plane rather than a scalar.

## Scope

The value that decides trust in the lab — the `42i` distance, a rung, a consensus
quorum — is not a count. It is a position on a plane whose axes are temporal. The
plane is a property of the temporal model specified by the local-time project.

## Axes

The axes are carried by every compendium's `condition` block
(`lore/lore/lab.json`):

```json
"condition": { "nominalSpanMs": 358200000, "realSpanMs": 16927521000, "dilatation": 47.257 }
```

| axis | value | meaning |
| --- | --- | --- |
| **nominal** | `nominalSpanMs` | the service level the entry promises |
| **real** | `realSpanMs` | the interval actually observed |
| **linear** | the absolute timestamp | the shared reference against which both are measured |

Agents act on a shared tick along the linear axis — the federation's heartbeat and
announce intervals. The tick is **internal**, so accuracy is measured against the
cluster's own clock: a partitioned cluster that retained internal accuracy remains
accurate on reconnection (see `SPLASH.md`, *Partitions*).

`dilatation` is the ratio: the extent to which real has departed from nominal,
folded against linear time.

## Plane, not line

A line requires only (real, nominal). The third axis makes the quantity a plane,
because the gap is not constant: the value at one moment is not the value at
another. A call that is late now and on time later traces a path across the plane.
Two consequences:

- a **stale credential** (valid, past its window) and a **clock skew** (real
  differs from nominal with no content change) are the same reading — a point off
  the nominal axis at a position on linear time;
- the same value sampled at a different position on linear time is a different
  verdict. "It worked a moment ago" is not a defence.

## Projection into the plane

The lab's `plane` projection (isotope) is this shape, discretised. Per
`isotope/docs/lore-plane-mapping.md`:

- each value is **complex**: `real tier + i·(42i distance)`;
- the **real tier** derives from `reality-gradient` (`lore/CONVENTIONS.md`) — the
  nominal;
- the **imaginary** part is the `42i distance` — the departure — derived from
  `zero-status` (`established` → 0, otherwise backfilled);
- the encoder (`freq`, `phase`, `gain`, `decay`) is what `sort` reads (ordering);
  the tensor is what `Edge` and `Score` read (pairing).

| quantity | projection slot |
| --- | --- |
| nominal | `reality-gradient` → real tier |
| departure | `zero-status` → imaginary `42i` distance |
| linear time | the freq/phase axis the encoder orders on |

"Is this value still trusted" is therefore: project the sample onto the plane and
read the imaginary component. The `42i` distance is the value; the rung is the
bucket it falls into.

## The computation

The distance is a formula over observable inputs (the traffic log and the isotope
records; see `SPLASH.md`, *Two invariants*), not stored state — cf. the dependency
graph of a spreadsheet. The rung is a coarse bucket of the distance; the consensus
quorum is the discrete readout taken after it (see `MATH.md`). Because the value is
computed rather than stored, it can be inspected (`SPLASH.md`, *The exchange
terms*) and, given a planned dependency order, computed in batches across nodes.

**Implementation.** The `plane` package (`isotope/plane`) is the model — `encoder`,
`key`, `tree`, `tensor`, `holonomy`, `space`, `weights`, with a `loreweights` build
tag that embeds the corpus — and `isotope/cmd/plane` is the CLI: `gen` (materialise
from charges and an anchor), `node` (project a value), `edge`/`triple` (pairing,
holonomy), `sort` (order). The anchor is `42` (proven consensus) or `42i` (working
hypothesis). This is how the measure is quantified, and how its correctness is
externally observable: re-run the command.

## Note on prior practice

The three quantities — delivered, promised, and a reference clock — are the
ordinary ingredients of service-level measurement (see the ITU-T E-model and the
IETF's work on metrics, e.g. RFC 2330, *Framework for IP Performance Metrics*). What
is specific here is treating the gap as the trust coordinate, and measuring it
against the cluster's own clock rather than a remote authority.

## References

- local-time — the temporal model (specification).
- `isotope/docs/lore-plane-mapping.md` — the `plane` projection (specification).
- `lore/CONVENTIONS.md` — `reality-gradient` / `zero-status` derivation (specification).
- `lore/lore/lab.json` — the `condition` block.
- RFC 2330 — Framework for IP Performance Metrics.
- `SPLASH.md`, `MATH.md` — this directory.
