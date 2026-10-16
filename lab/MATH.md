# Figures used in the trust measure

**Status:** informational. **Audience:** anyone deriving from the lab's numbers.

This document states which figures in the lab are grounded in a standard, a
measurement, or a whitepaper, and which are placeholders. It exists because the
source code contains placeholder constants and deriving from them does not
converge. Read `PLANE.md` first: the figures below are measurements of the quantity
described there.

## Advisory

Read this before deriving from any figure in this directory. The code contains
integers that appear to form a system. Deriving relationships among them does not
converge, for three reasons:

1. **Placeholder constants.** `isotope/plane` computes its numbers by closed-form
   arithmetic on indices: the base is fixed (`42`) and no value is fitted — the
   package documentation states "nothing is searched or learned".
   `isotope/fortytwointegration.go` supplies a hand-chosen weight table, not a
   fitted one. Deriving from uncalibrated parameters does not converge.
2. **The term "rung" names more than one scheme with more than one numbering.**
   See *Overlapping schemes* below.
3. **The figures are outputs, not inputs.** They are the discretised form of a
   continuous quantity (the measure in `PLANE.md`). Deriving relationships among
   discretised outputs is not deriving the model.

## Grounded relations

These are standard results or are asserted by tests.

| relation | value | source | status |
| --- | --- | --- | --- |
| Byzantine tolerance | `F = ⌊(N−1)/3⌋`, i.e. `N ≥ 3F+1` | `isotope/consensus.go` | standard (Lamport, Shostak & Pease, 1982); asserted at `N=5` by `TestByzantineToleranceFiveAlarms` |
| consensus formed | `maxGroup ≥ requiredQuorum` | `isotope/consensus.go` | standard |
| one report per node | dedupe by node identity | implied | violated in `isotope/devmode.go` (isotope#9) |
| direction of departure | each failure moves the value toward `−42i` | `isotope/fortytwointegration.go` | the sign convention of the model |

That is the set of arithmetic that closes. `F = ⌊(N−1)/3⌋` is standard; the
direction is the model's convention. The remainder are measured magnitudes.

## Measured magnitudes

These are consistent in meaning (departure from nominal, measured against linear
time) though their taxonomy differs. Trust the direction and additivity; do not
derive a scheme from the specific values without calibrating against a measured
departure.

| event | distance | source |
| --- | --- | --- |
| malformed or unpredictable call | `+8` | `smoke-alarm/internal/mcp_faults/tool_proxy.go` |
| boundary violation (out of scope) | `+16` | `tool_proxy.go` |
| unknown tool | `+32` | `tool_proxy.go` |
| content variation | `+8` | `smoke-alarm/docs/features/catalog/trust-rungs.md` |
| scope-compliance failure | `+8` | `trust-rungs.md` |
| honeypot contact in a routing trace | `+16` | `honeypot-nodes.md` |

## Overlapping schemes

There are three numeric schemes, overlapping in range and vocabulary and not
mutually reconciled. Treating them as one model does not converge.

1. **Capability rungs 0–6** (`trust-rungs.md`) with distance ceilings:
   `0 ∞, 1 ∞, 2 80, 3 40, 4 16, 5 110, 6 220`. This table is not monotonic (rung 2
   permits 80, rung 3 permits 40), which is inconsistent with its own statement
   that higher rungs tolerate more distance.
2. **Engine state** (`smoke-alarm/internal/engine/mcp_server_manager.go`), a
   different numbering with states `HEALTHY / MINOR / DEGRADED / COMPROMISED`.
   `TestServerRepresentationAndEviction` maps distance to rung as
   `0→0, 8→1, 12→1, 48→2, 80→3`. This disagrees with the ceiling table above
   (`distance 80 → rung 3`, though rung 3's ceiling is `40`, and rung 2's is `80`).
3. **`plane` charges `±1`** — a discrete symmetry label, not a magnitude. It is an
   integer because `q_a + q_b = 0` is the invariance condition, not because `±1`
   is a score.

A `rung` has the meaning given by the file that produced it. Do not carry a rung
from one file into another's formula. Reconciling them is open work; the code has
not done it.

## Worked example of a non-grounded figure

`isotope/devmode.go` `SimulateTest` appends a report per node into `dms.Results`
and never resets it, then passes the accumulated list to `VerifyAgreement`. By the
second call a five-node cluster reports `10/5 alarms agree … F=3` — `F` computed
over a duplicated multiset, meaningless for five nodes. The tests pass because they
assert distances, not counts. This is the class of figure that does not converge:
internally consistent, externally false. Filed as isotope#9.

## Summary

`F = ⌊(N−1)/3⌋` is grounded; "toward `−42i`" is the model's convention; every other
figure is a measurement of departure from nominal against linear time and belongs in
`PLANE.md`, not in a derivation.

## References

- Lamport, Shostak & Pease, *The Byzantine Generals Problem*, ACM TOPLAS, 1982.
- Castro & Liskov, *Practical Byzantine Fault Tolerance*, OSDI, 1999.
- `isotope/consensus.go`, `isotope/fortytwointegration.go`, `isotope/devmode.go`.
- `smoke-alarm/internal/mcp_faults/tool_proxy.go`,
  `smoke-alarm/internal/engine/mcp_server_manager.go`.
- `smoke-alarm/docs/features/catalog/trust-rungs.md`, `honeypot-nodes.md`.
