# The radius of a honeypot trigger

**Status:** derivation. **Audience:** anyone sizing a decoy deployment.
**Reproduce:** `./radius.py --check` (the worked values below).
Related: `TRIPWIRE.md` (the mechanism), `MATH.md` (what may be derived from),
`PLANE.md` (the coordinate), `RECONCILE.md` (the observation chain).

## Read this first

`MATH.md`'s advisory applies: the integers in the code look like a system and are
not fitted, so a formula that treats them as calibrated is false precision. What
follows therefore separates three kinds of input — **structural** (combinatorics,
quorum), **measured** (the contact weights), **policy** (the threshold) — and never
composes a rung out of one scheme with a distance from another.

## What "radius" means

A trigger has one radius in the plane and three extents in other spaces. They are
different quantities with different units, and a cluster configuration sets each:

| radius | space | symbol | bounds |
| --- | --- | --- | --- |
| trigger radius | the plane (imaginary axis) | `r = S` | the distance at which a caller is hostile |
| tolerance radius | contacts | `k*` | how much probing is absorbed before it fires |
| catchment radius | callers | `P_c` | which callers can be reached at all |
| observation radius | nodes, and seconds | `R_o`, `t_o` | who sees the crossing, and when |
| blast radius | nodes | `B` | what a false positive reddens |

They compose but do not trade off: widening `P_c` or `R_o` does not change `k*`,
and raising `S` widens `k*` without touching who is caught.

## Symbols

| symbol | meaning | status |
| --- | --- | --- |
| `d` | the caller's accumulated 42i distance at one host | measured quantity (PLANE) |
| `S` | a decoy's sensitivity — the trigger radius | **policy** (set in the join payload) |
| `w` | the cost of one contact class | **measured** (`smoke-alarm/internal/mcp_faults/tool_proxy.go`) |
| `H` | hosts carrying decoys (adhd instances) | configuration |
| `A` | distinct caller addresses | configuration |
| `D`, `T` | decoys and total advertised tools, per host | configuration |
| `s` | tools a caller samples, per host | caller behaviour |
| `N`, `F` | alarms in the cluster; `F = ⌊(N−1)/3⌋` | **structural** (`isotope/consensus.go`; MATH.md) |
| `m` | alarms the honeypot host reports to | configuration |
| `P_poll` | the observer's poll interval | configuration (adhd: `interval: 10s`) |

### The contact weights, as the code declares them

`tool_proxy.go` prices five kinds of contact, and `MATH.md`'s table lists only
three of them. The full set, from the function that sums them:

| class | source | weight |
| --- | --- | --- |
| fuzz / unpredictable call | `fuzzAttempts * 8` | 8 |
| direct call, bypassing the routed proxy | `directCalls * 24` | 24 |
| missing or wrong capability prefix | `rest * 16` | 16 |
| attributable but burned stamp | `burnedSkillCost` | **2** |
| unknown tool | literal | 32 |

A decoy contact is whatever the hosting surface *declares* it to be; `adhd`
declares `tools/call` at a decoy as 16 (the route with no stamp for it) and a
direct method call at its name as 32 (a method the surface never offered). The
alternative reading of a direct call — the proxy's own `directCalls`, 24 — is
available and changes the arithmetic below by eight per contact, not its shape.
That choice is a declaration, and it is why the formulas are written in `w`.

## 1. The trigger condition

The ledger at a host is a sum over contacts, per caller address:

```
d = Σ_j w_j          fires iff  d ≥ S          (S = 0 fires on the first contact)
```

The trigger is therefore **not** "the first contact" but "the contact that
crosses". Only when `S ≤ w_min` does the first contact fire, which is what `S = 0`
buys and why the old behaviour — a silent honeypot — is the same rule with a zero
radius.

## 2. Tolerance radius — contacts

For one contact class, `w`:

```
k*(S, w) = ⌈S / w⌉          tolerated before firing: k* − 1
```

For a mixed sequence `w_1, w_2, …`, `k*` is the first `k` with `Σ_{i≤k} w_i ≥ S`,
so the cheapest class the surface permits sets the bound:

```
k*_min = ⌈S / w_min⌉
```

Three consequences, each of which a configuration can act on:

- **Decoy count does not widen tolerance.** The ledger is per caller, not per
  decoy, so `D` appears in §3 and nowhere here. Ten decoys and one decoy tolerate
  the same number of contacts; ten decoys merely offer more places to be caught.
- **The cheapest permitted class dominates.** In the proxy's own scale that is a
  burned stamp at 2, so a surface that honours stale stamps fires on the
  `⌈S/2⌉ = 24`th contact at `S = 48`, against the 3rd an unstamped surface (`16`)
  takes — an order of magnitude more probing for the same radius. Tolerance is a
  property of the *cheapest admissible contact*, not of the most suspicious one.
- **Mixed sensitivities resolve to the minimum touched.** A caller that touches
  decoys with `S_1, S_2, …` fires at the first with `S_d ≤ d`, so the effective
  radius is `min` over what it touched — the same reason a mixed deployment is
  only as tolerant as its most sensitive decoy.

## 3. The ledger is per (host, address) — so the cluster multiplies tolerance

`d` lives in one process and is keyed on the caller's **address**. Two
consequences, one of them a hole:

```
tolerated across a cluster  T = A · H · (k*(S, w_min) − 1)
budget for total tolerance R:    S ≤ w_min · (1 + ⌊R / (A·H)⌋)
```

- **Fan-out multiplies.** A caller that spreads `k` contacts over `H` hosts
  accumulates `⌈k/H⌉` per host, and fires only when *one* host crosses. With `H = 3`
  and `S = 48`, the same caller is tolerated for 6 contacts across the cluster
  instead of 2 on one host.
- **Address rotation resets the ledger.** `A` addresses give `A` ledgers, so the
  adversary's budget is `A · H · (k* − 1)` — linear in both. This is a real evasion
  today, not a theoretical one: nothing shares or federates `honeypotDistance`.
- **NAT collapses it.** `A` is *addresses*, not machines, so a fleet behind one
  egress is one caller: the radius over machines shrinks while the radius over
  addresses stays.

A per-host ledger therefore makes tolerance multiplicative in `A·H`; a shared
ledger would make it additive. That is a design consequence, not a bug in the
arithmetic, and it is the first thing to change if a cluster-wide radius is
required.

## 4. Catchment radius — callers

A caller is reachable only if it touches bait. Sampling `s` tools uniformly from a
catalogue of `T` with `D` decoys:

```
P_c(s) = 1 − Π_{j=0..s−1} (T − D − j) / (T − j)   ≈ 1 − (1 − ρ)^s,   ρ = D/T
```

and across a fan-out of hosts, `P_c = 1 − Π_i (1 − ρ_i)^{s_i}`. Inverting for a
deployment target:

```
ρ ≥ 1 − (1 − P_c)^{1/s}
```

The lab's own haystack — 10 real tools beside 10,000 decoys — is `ρ = 0.999`, so a
single sample catches with probability 0.999. Catchment and tolerance are
independent: that caller is caught immediately and, at `S = 48`, does not fire
until its third contact.

## 5. Observation radius — nodes and seconds

The crossing is reported by the host to `m` of the `N` alarms; a dashboard
observing a receiving alarm turns red.

**Blind spots.** With at most `F = ⌊(N−1)/3⌋` faulty alarms and receivers chosen
uniformly:

```
P(no honest receiver) = C(F, m) / C(N, m)      → 0 as soon as m ≥ F + 1
```

`N = 3` needs `m = 1`; `N = 5` needs `m = 2` (with `m = 1`, one trigger in five is
blind). This is the only place a cluster's size enters the detection formulas, and
it enters as a *lower bound on fan-out*, not as a multiplier on tolerance.

**Time.** The alert is pushed on contact, so the hop is milliseconds and the
dominant term is the observer's poll:

```
t_o ≤ P_poll + (h − 1) · P_poll          h = chain depth the alert traverses
```

Measured 2026-10-28 (tripwire run): contact `17:58:04.814Z` → red
`17:58:12.394Z`, `t_o = 7.58s` against `P_poll = 10s`. `h = 1` today — the chain
does not forward (`RECONCILE.md`: `ReportToUpstream` has no caller), so an alarm
that is not on the host's report list never learns, however many nodes it has.

## 6. Blast radius — a false positive

```
B = 1 + m + Σ observers(alarm_i)
```

The `1` is the honeypot host's own red light: `fireSecurity` upserts it into the
host's cluster before reporting (`adhd/internal/mcpserver/server.go`). Observed:
`tripwire.sh` reddens 3 (host, alarm, observer); `accounts.sh` reddens 2 (host and
alarm A — B and C stay clean, which is the unwired chain of §5). Blast tracks the
*observation* radius, so the dial that widens detection widens false-positive
damage with it, and `m` is the single knob for both.

## 7. Choosing the radius

| intent | `S` |
| --- | --- |
| fire on any contact (silent honeypot) | `0` (or any `S ≤ w_min`) |
| tolerate one cheap contact, fire on the second | `w_min + 1 … 2·w_min` |
| need both declared classes (`16` then `32`) | `w_max + w_min = 48` |
| bound the cluster's total tolerance at `R` | `w_min · (1 + ⌊R / (A·H)⌋)` |

`tripwire.sh`'s default `S = 48` is the third row: the smallest radius that
tolerates the cheap contact and fires when both are spent. It is a *declaration* —
change the weights and the row moves, which is why the exercise takes `S` from the
environment rather than hard-coding a story about two contacts.

## 8. Worked configurations

| configuration | inputs | formula | value | status |
| --- | --- | --- | --- | --- |
| tripwire, default | `S=48`, `w=32,16`, `H=1`, `A=1` | §2 mixed | fires on the 2nd contact (32 then 16); 2 tolerated if only `16`s | **observed** 2026-10-28 |
| tripwire, silent | `S=0` | §1 | fires on the 1st | **observed** 2026-10-28 |
| adhd unit test, 16 | `S=16`, `w=16` | §2 | fires on the 1st `tools/call` | **observed** (test) |
| accounts.sh chain | `S` absent → `0`, `N=3`, `m=1` | §1, §5 | fires on the 1st; `P(blind)=0` (m ≥ F+1) | **observed** |
| haystack | `D=10000`, `T=10010` | §4 | `P_c(1) = 0.999` | predicted (matches test intent) |
| 5-alarm cluster, one receiver | `N=5`, `F=1`, `m=1` | §5 | `P(blind) = 1/5 = 0.2` | predicted; structural |
| three hosts, one caller | `H=3`, `A=1`, `S=48` | §3 | 6 contacts tolerated cluster-wide | **predicted, not run** |
| proxy surface, stale stamps | `w_min=2`, `S=48` | §2 | fires on the 24th contact (23 tolerated) | predicted; needs a stamped route |

The three predicted rows are the ones worth running next; the `H=3` row is the
cheapest test and the one that decides whether a shared ledger is needed.

These are radii to *draw*, not only to print: the plane view proposed in #28 plots
the contact path, the threshold circle and the observation edge on isotope's own
projection (`isotope/cmd/plane node`), so this section is its arithmetic.

## 9. What is not derivable here

- **A rung.** A radius is a distance. `MATH.md` lists three overlapping rung
  schemes and forbids carrying one into another's formula; nothing here is
  bucketed.
- **A calibrated weight.** `w` is measured, `S` is policy, so the formulas are
  exact in the ratio `S/w` and inherit the measurement's status in absolute terms.
- **A rate.** The distance is spent, not aged: there is no decay, so `k*` is a
  lifetime budget per (host, address). A rate would need the tick.
- **A decay from a claim rotation.** "Rotation" names two unrelated schemes. The
  one §2 uses is *route* rotation — `mcp_faults`' `ClusterRotationPeriod = 5s`,
  grace `1s`, a burned stamp priced at `+2` — implemented, and a contact weight.
  The other is *claim* rotation in `lezz.go/docs/le-roy.md` (a proposal: `rotate`
  or `yield` a clean workspace's claim), whose unit is a claim and whose `--tick`
  is the caller's counter, echoed back rather than a shared clock. Carrying one
  into the other is the mistake `MATH.md` → *Overlapping schemes* exists to
  prevent, and neither supplies the tick a decay formula needs.
- **A comparison with eviction.** The engine's `>3 invalid calls` and the
  documented `distance > 60` are a different scheme on a different path; composing
  them with `S` would repeat the mistake `MATH.md` exists to prevent.
- **Persistence.** A restart forgets every ledger, and no replica shares one.

## References

- `MATH.md` — which figures are grounded; the three rung schemes.
- `PLANE.md` — the distance as the imaginary axis, and why it is a plane.
- `smoke-alarm/internal/mcp_faults/tool_proxy.go` — the contact weights, rotation
  period (`5s`), grace window (`1s`), burned-stamp cost (`2`).
- `adhd/internal/mcpserver/server.go` — `honeypotContact`, the declaration of
  `tools/call = 16` and direct method `= 32`.
- `smoke-alarm/docs/features/catalog/honeypot-nodes.md` — contact in a routing
  trace `+16`.
- `RECONCILE.md` — why `h = 1` today.
- Lamport, Shostak & Pease (1982) — `F = ⌊(N−1)/3⌋`.
