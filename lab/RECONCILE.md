# The verification chain

**Status:** design note and experiment report. **Reproduce:** `./accounts.sh`
(re-run 2026-10-28, unchanged; the script builds the binaries itself and needs no
spin harness). Related: `TRIPWIRE.md`, `SPLASH.md`.

## Topology

Node A takes ingress and sheds load to B on its own decision. Node B is idle but
permitted to check A's work (traffic mirroring). Node B is monitored by C, and so
on. Each node is verified by the next, so no node reports on itself. The triad is
referred to here as the **golden triangle cluster**: verification traverses the
triangle, and three is the smallest number that both closes the loop and gives a
fault a neighbour to fall to.

```
        load                 mirror                monitor            …
  ┌──────┐  shed  ┌──────┐  check  ┌──────┐  check ┌──────┐
  │  A   │───────▶│  B   │◀────────│  C   │◀───────│  D   │
  │ingress│        │ idle │  A work │monitor│  B work │  …   │
  └──────┘        └──────┘         └──────┘         └──────┘
        B verifies A        C verifies B        D verifies C
```

## Verification paths

B holds A's traffic log (the mirrored traffic — what A did) and A's isotope records
(what A should have done); verification is the cross-check between them
(`SPLASH.md`, *Two invariants*). The mirror is not a second log; it is one of the
two invariants. This is also why B can stand in during a failure and verify from
the same artifact.

## Admission layers

A, B and C also serve as the admission layers for anything entering the cluster
(`SPLASH.md`, *Three admission layers*): A holds the session, B the tool surface,
C the credentials. A caller passes all three before a credential is authorized to
enter memory. A harness's automatic `tools/list` reaches B and no further.

| node | verification role | admission layer |
| --- | --- | --- |
| A | ingress | presence — the session |
| B | verifies A | surface — the dynamic tool array |
| C | monitors B | authority — the credentials |

## Failover

The nodes pre-negotiate their isotope before the cycle, so a compromise does not
force re-negotiation under load. If A is compromised, B takes the session (it
already holds the mirrored traffic and the pre-negotiated material; the transfer is
a role swap, not a cold start). C then fills B's slot, or provisions a replacement.
Pre-negotiation provides immediate failover; mirroring provides immediate stand-in.

## Exercise (available today)

`accounts.sh` builds three nodes in the chain — `alarm-a` (ingress, no target),
`alarm-b` (a `verify` target on A), `alarm-c` (a `verify` target on B) — and a
honeypot host reporting to A alone. Observations:

```
B verifies A -> [('verify', 'healthy')]     # the chain checks work
C verifies B -> [('verify', 'healthy')]
prober touches the bait at A's ingress
a: [('security/honeypot_triggered', 'healthy=False')]
b: no security component                     # the alert does not traverse
c: no security component
```

The *checking* half of the chain works: each node observes the one below. The
*alert* half does not: an alert raised at A does not reach B, so B does not observe
it.

## State of the mechanism

| element | state |
| --- | --- |
| chain `verify` targets | implemented |
| honeypot host → its ingress alarm | implemented (component fix, smoke-alarm#23) |
| alert traversing the chain (A → B → C) | **not implemented** — `Poller.ReportToUpstream` has no caller (smoke-alarm#24) |
| load-shed A → B, traffic mirroring | design — no shed/mirror path in the code |
| chaos test / receipt reconciliation | design — specification and isotope receipts; no `chaos` in smoke-alarm's non-test code |

The missing link is specific: `Poller.ReportToUpstream`
(`internal/federation/federation.go:115`) marshals the full status — components
included — to `upstream/federation/report`, which is exactly the "B is informed of
A's alert" path. It is defined and never called. Wiring it on the federation
cadence would carry the alert up the chain; `handleFederationReport` already merges
components (smoke-alarm#23), so the receiving end is ready. Until then,
corroboration requires the reporting node to post to every alarm directly, which is
a fan rather than a chain.

## Open item: the enactor of a disagreement

If B holds the full mirror and disagrees with A, two readings had been considered,
and the design discussion resolved toward a third:

1. **C withholds the commit.** B detects and escalates; C is the authority that
   withholds the rung update. Detection and enforcement stay separate; no single
   verifier can demote. Cost: latency, and dependence on C observing.
2. **B demotes directly.** Immediate, and the decision sits with the evidence. Cost:
   B holds enforcement authority over A, checked only by C's later review.
3. **The cluster forks.** Neither C withholds nor B demotes; the honest cluster
   continues on its own branch and the disagreeing node is orphaned out of the
   index. This is what "removed from the trust index" means: a fork, not a verdict
   (see `SPLASH.md`, *Prior art*, and the proof-of-work row). No enforcement
   authority is required; the branch with more work continues.

If option 2 is taken, the resolution discussed was that B is not a single node but
a **self-monitoring cluster of three** acting on internal agreement, so a single
faulty verifier cannot demote an honest node. Note the caveat: with `N = 3`, the
Byzantine tolerance `⌊(N−1)/3⌋` is `0` — three nodes with quorum 2 tolerate one
crash or silent fault, but have no Byzantine headroom. Strict `N ≥ 3F+1` requires
`N = 4` for `F = 1`. Filed as a proposal: james-gibson/smoke-alarm#25.

## References

- RFC 3161 — Time-Stamp Protocol (via `SPLASH.md`).
- Castro & Liskov, *Practical Byzantine Fault Tolerance*, OSDI, 1999.
- james-gibson/smoke-alarm#23, #24, #25 — component merge, upstream push, enactor.
- `SPLASH.md` — *The measure*, *Failover*, *Partitions*.
