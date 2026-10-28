# The loss model — when a bit, or an RPC, is lost

**Status:** findings. **Audience:** anyone reasoning about whether an event reaches
the node that wants it. Read with the register ([`README.md`](README.md)), which this
document quantifies, and with [`../../lab/MATH.md`](../../lab/MATH.md), whose
conventions it follows: every claim below is marked **standard** (a published result),
**model** (a choice of ours) or **assumption** (not measured). No numbers are fitted —
the 42i *distance* measure lives in the embedder project; this is about **loss**, a
different quantity, and mixing them is the mistake `lab/MATH.md` warns about.

The question: **what is the probability that an event is propagated to the node that
wants it?**

## 1. Three ways an event fails to arrive, not two

The spike is exercised on one machine, one process, at low parity — **deliberately**
(§5). That choice makes sense only if the failure modes are separated, because they are
attacked by different mathematics.

| mode | what happened | RAID's name | what covers it |
| --- | --- | --- | --- |
| **lost bit** | the event is gone from every store that held it | fail-stop | redundancy + parity: RAID, erasure codes |
| **lost RPC** | the event is not lost, but never *delivered* to the node that wants it | fail-stop (the hop stops) | retransmission + idempotence; or a harness |
| **wrong bit** | the event is present, and says something other than what happened | fail-arbitrary / Byzantine | verification: a checksum, a second invariant, consensus |

RAID's mathematics is **fail-stop** mathematics (a disk that stops answering —
Patterson, Gibson & Katz, *A Case for Redundant Arrays of Inexpensive Disks*, 1988).
Our store's dominant failure is the third row, which RAID's maths does **not** cover —
that is the single most important fact in this document, and §5 is where it pays.

## 2. What the protocols actually guarantee

**MCP does not specify delivery.** It is JSON-RPC 2.0 messages; the specification
defines the shape of a call and the shape of an error, not whether a message arrives,
how often, or in what order. The specification says so itself: the [Triggers and Events
Working Group](https://modelcontextprotocol.io/community/working-groups/triggers-events)
charter (*2026-03-24*) puts **"delivery semantics, and event ordering guarantees"** in
scope as *specification work still to be done*, notes that "today, clients learn about
server-side updates by **polling** or holding an SSE connection open", and defers to the
Transports WG because "delivery and ordering guarantees **depend on transport
capabilities**". The existing notification primitive
(`notifications/resources/updated`, which a surface may implement under
`resources/subscribe`) carries no delivery promise of its own.

**So the laws of RPC apply** (Birrell & Nelson 1984; Spector 1982; critiqued in
Tanenbaum & van Renesse 1988). An RPC can fail as: the request is lost, the reply is
lost, the server stops, the network partitions, or — the interesting one — **the call
returns ambiguously**: the caller times out and does not know whether the operation
happened. That forces a choice, and each choice has a price:

| semantics | how | price |
| --- | --- | --- |
| at-most-once | send once; never retry | lost calls stay lost |
| at-least-once | retry until a reply | **duplicates** — and a duplicate `tools/call` is a duplicate *file write* (§4) |
| exactly-once | retry + server-side dedupe/state | requires identity for the operation, and a transaction |

**ACP buys a term rather than deriving it.** Under the Agent Client Protocol the agent
runs as a **subprocess of the harness**, JSON-RPC 2.0 over stdio (HTTP for remote
agents). The harness owns the process and the pipe, so it observes the child's exit and
the delivery term collapses toward 1. That is the cheat, and it is legitimate — but it
is paid for with §5's currency, not removed: the failure domain becomes **shared**, so
whatever takes out the harness takes out the agent. **ACP does not reduce loss; it moves
it from the delivery term to the common-mode term.** A harness is a parity decision
wearing a protocol's clothes.

## 3. The store: RAID's mathematics, exactly

Let an event be held by `n` stores, of which any `m` may be lost before the event is
lost. With independent per-store loss probability `p`:

```
P(lost) = Σ      C(n,k) p^k (1-p)^(n-k)          // standard: k-out-of-n / MDS code
        k>m                                       // (Reed & Solomon 1960)
```

The RAID levels are points in that family, not a separate theory:

| scheme | `(n, m)` | `P(lost)` | tolerates |
| --- | --- | --- | --- |
| RAID 0 (stripe) | `(1, 0)` | `p` | nothing |
| RAID 1 (mirror) | `(2, 1)` | `p²` | any one |
| RAID 5 (`d` data + 1 parity) | `(d+1, 1)` | `≈ C(d+1,2)·p²` | any one |
| RAID 6 (`d` data + 2 parity) | `(d+2, 2)` | `≈ C(d+2,3)·p³` | any two |

Three consequences that matter more than the formulas:

1. **Parity buys the square of `p`, not more.** `p²` only beats `p` when `p` is small
   to begin with — the same trade RAID's paper argues on economic grounds: the benefit
   is the *difference*, and the cost is a whole device.
2. **Bigger stripes are less safe, not more.** `C(d+1,2)` in RAID 5 says that spreading
   parity over more stores *increases* the chance of a second simultaneous loss. Redundancy
   is not monotone in `n`; adding stores without adding parity makes things worse.
3. **The rebuild window is a different failure model.** Restoring parity re-reads every
   surviving store, so the period after a loss is the period of maximum exposure — the
   reason RAID 5's "write hole" and long resilvers are a literature of their own.

And the limit, stated by the RAID literature itself (Chen, Lee, Gibson, Katz & Patterson,
*RAID: High-Performance, Reliable Secondary Storage*, 1994): the `p^k` terms assume
**independence**, and real failures correlate — power, controller, host, and above all
**the process doing the writing**. Correlated failure is not a correction term in the
formula; it is a different term:

```
P(lost) = c + (1 - c)·P(all replicas lost | no common-mode failure)
            where c = P(common-mode failure)
```

If `c ≫ p²`, then `P(lost) ≈ c`, and **no parity whatsoever helps** — the redundancy is
protecting a term the dominant failure does not touch. That inequality is the whole of
§5.

## 4. The path: getting it to the node that wants it

The event reaching an interested node is a *series* system, and series availability
**multiplies** — this is the standard reliability-block-diagram result, and it is the
least intuitive thing in this document:

```
P(reach) = Π qᵢ        over h hops        // a path of h hops, each surviving qᵢ
        = q^h          if the hops are alike
```

So a path of ten hops each surviving at 0.999 delivers `≈0.990` — a 1% loss from parts
that are individually "fine". Loss is dominated by the **number of hops**, not by the
worst hop.

**Retransmission trades loss for duplication.** Over `k` independent attempts:

```
P(≥1 delivered) = 1 - (1-q)^k                       // the loss you remove
P(≥2 delivered) = 1 - (1-q)^k - k·q·(1-q)^(k-1)     // the duplicates you buy
```

which is at-least-once semantics in one line — and in a file-backed model a duplicate is
a **second identical write**. The lab's answer is designed, and it is **not** per-tool
idempotence: the **consumption filter** (`SPLASH.md` → *The consumption filter*;
[`../../lab/SLA.md`](../../lab/SLA.md) §3) makes the isotope single-use, so
a repeat is filtered *before* it becomes an effect and the retry is safe by
construction. That is the model's answer — designed in `SPLASH.md`, **not yet exercised
in the spike**, which has the isotope record but no filter. What it costs depends on
whether the filter is treated as the authority or as an index — a *suppressed call* or a
*lookup* — and that, with the sizing and the window it slides on, is derived next.

### The consumption filter: sizing it, and the window it slides on

A dedupe filter that is **authoritative** makes its false-positive rate a *correctness*
parameter rather than a performance one: a false positive suppresses a legitimate first
use. So the sizing has a standard form, and it has a window — and both have to be right.

**Sizing (standard).** Bloom (1970), with `m` bits, `k` hashes and `n` inserted items:

```
P(bit still 0) = (1 - 1/m)^(kn) ≈ e^(-kn/m)
p = (1 - e^(-kn/m))^k                  // false-positive rate
k* = (m/n)·ln 2                        // optimal k
m/n = 1.4427·log₂(1/p) = ln(1/p)/ln²2  // bits per item, at k*
```

which collapses to a memorable pair at the optimum: **`k* = log₂(1/p)` hashes and
`1.4427·log₂(1/p)` bits per item.** At `p = 0.01` that is 7 hashes and ≈9.6 bits per
item. These are the filter's own standard results, not measurements of ours.

**`n` is per tick — which is why the filter is small.** The filter travels with the tick
and holds the isotopes consumed *in that tick*, so `n` is the number of operations in one
window, not a lifetime total. That is the whole reason a compact artifact can ride every
update: the tick bounds `n`, and `n` bounds `m`.

**But it must be sized on the burst, not the mean.** The tick exists to *synchronise* the
burst (`SPLASH.md` → *A shared tick*), so arrivals cluster at the boundary — exactly when
`n`, and therefore `p`, is largest. Sizing on the average `n` understates the rate
precisely when the most calls are at risk. And `p` degrades **without warning** — the
rate is a *cliff*, not a slope: a filter sized for `p = 0.01` over `n` items answers
"maybe" at ≈0.16 once `2n` have been inserted, and ≈0.68 at `4n` (formula evaluations,
illustrative — §7). Nothing in the filter reports this; in a dedupe filter it simply
means dropping more calls.

**The window, and the inequality that matters.** Each tick replaces the previous filter,
so dedupe covers exactly one window `T`. A retry arriving after `T` is no longer in the
filter and **lands as a duplicate**:

```
dedupe(window T) covers retry delays  Δ < T
required:   T ≥ τ_max      τ_max = the longest retry interval a caller may use
                           (in this lab: the check timeout — 5s by default)
```

`τ_max` is not the filter's parameter; it belongs to the **retry schedule**, and it is
the number the window must be compared against. The lab's own defaults put them **equal**
(`timeout: 5s`, tick 5s), which makes the boundary case the *normal* case rather than an
edge: a retry issued on timeout arrives just as the window has slid. Three routes out,
and they are not equivalent:

| route | effect on `p` | cost |
| --- | --- | --- |
| widen the window (`T > τ_max`) | none | slower to forget a consumed isotope |
| retain `w` windows | `p_eff ≈ w·p` — the union compounds | `w×` the bits to distribute |
| a **rolling** filter (counting Bloom / cuckoo — Fan et al.) | constant `p` | deletion support; more state than a bitmap |

**And the third row points at the real discipline: the filter is an *index*, not the
authority.** Bloom's guarantee is the *absence* of false negatives — "definitely not
present" is exact, "maybe present" is not — so the standard use is to treat "maybe" as a
question for an **exact** structure rather than as an answer. Then a false positive costs
a **lookup**, not a call, and `p` is no longer a lost-call rate. What makes that
affordable here is that the exact structure already exists: the isotope is a **content
address** (RFC 6920), so the used set *is* the isotope records the mirror already holds
as the second invariant. The filter is the compact, distributable index over that set —
the role it was invented for, and the reason the identity had to be a content address
before the filter could be correct.

**Whichever route, both mistakes stay observable** — which is the property an
authoritative filter would destroy. A false positive that suppresses a call leaves a
**log entry with no isotope**; a duplicate that escapes the window leaves **one isotope
with two log entries**. The two invariants tell them apart, so the filter's error rate is
*measurable from the record* rather than asserted from its parameters (§7).

**Fan-out.** If the graph offers `κ` disjoint paths of `h` hops, redundancy is parallel
and the result is the complement:

```
P(reach) = 1 - (1 - q^h)^κ
```

**And the pull term, which is what the spike actually has.** The surface implements no
subscription, so nothing is pushed: an event reaches the interested node only if that
node **re-reads the store**. Then the path is:

```
P(propagated) = P(durably recorded) × P(readable through the surface) × P(caught in time)
```

The third factor is not a probability per call but a **rate against a window**: a reader
polling every `T` catches an event that matters for `W` with probability `≈ min(1, W/T)`.
This is exactly what MCP's charter means by "clients learn … by polling", and it is the
term that **A7 (`subscribe` ⇄ file-watch) would replace** — the one falsification
criterion with no measurement yet (register §4). Naming it is the honest alternative to
pretending push exists.

## 5. Our configuration, and why the parity is low *on purpose*

The spike holds one writer, two stores (tmpdir + sqlite) on **one machine**, in **one
process**. In §3's terms that is an `(n, m) = (2, 1)` mirror — RAID 1 — and it is
deliberate: **it was never worth spreading to another machine if the process failed
locally.**

That decision is not a limitation, and the mathematics says so precisely. Substituting
into §3's common-mode form, with `c` the process/host term and `p` the per-store term:

```
P(lost) ≈ c + p²         // our mirror, cheap and honest
P(lost) ≈ c              // the same, with replicas on other machines — unchanged
```

Adding machines reduces the `p²` term, which is not the term that dominates. The
`P(loss)` an extra machine would buy is `p² - p²_multi ≈ 0` while `c` stands untouched —
and `c` is the *dominant* mode: our own code, our own process, the writer. That is why
the parity stayed low: **redundancy is only worth its cost when it covers the dominant
failure mode, and ours is common-mode by construction.**

The RAID literature reaches the same place from the other direction: its own argument is
economic (the point of "inexpensive disks" is that the *cost* of redundancy is what has
to be justified), and its known limit is that correlated failure is not addressed by
parity at all.

**What follows is not "add parity".** It is: attack `c`. Which the lab already does —
and this is the part worth stating plainly, because it looks like more redundancy and is
not. Rung 3's canonicalization **re-derives** the spec's isotope rather than trusting
the stored record; the mirror holds **two invariants** (the traffic log, and the isotope
record of what should have happened) precisely because the third row of §1 — *wrong bit*
— is invisible to a copy. A copy cannot tell you that the original was wrong. **Parity
replicates; verification detects**, and the mode we have is the one detection answers.

## 6. Where the ladder sits in the mathematics

Each rung of the register's ladder (§1) attacks a **different factor**. This is the
quantification the spike opens, and it is why the rungs are additive rather than
alternatives:

| rung | what it adds | factor it moves | maths |
| --- | --- | --- | --- |
| 0 fast (self-report) | the writer's own claim | none — it *is* the claim | — |
| 1 mirror host | the same suite on sqlite | `p` (the per-store term) | §3, `(2,1)` |
| 2 live MCP surface | a separate process, on the wire | the **delivery** term | §2, §4 |
| 3 canonicalization | the record re-derived from the spec | **detection**: wrong-bit → detected | an error-*detecting* code |
| 3 consensus | `N ≥ 3F+1` independent instances | **agreement** under arbitrary failure | Lamport, Shostak & Pease 1982; `F = ⌊(N−1)/3⌋` (already grounded in `lab/MATH.md`) |
| 4 `ResultLedger` | history over time | `p` as a *function of time* — flakiness, regression | §3 applied per interval |

Read the middle rung's finding through this table and it stops being an instrument
quirk. When the surface answers but does not advertise a capability the target declared,
nothing has been **lost**: the event is present and readable. It is an **ambiguity** —
`§2`'s "the call returned, and the answer is not what was promised" — which is why no
amount of parity would have made it visible, and why what closed it was a **comparison**
against a declaration (smoke-alarm#34 → PR #36). Wrong-bit needs an invariant, not a
copy.

## 7. What is open, and what is not grounded

**Open, in the order #9 can measure them:**

1. **A7 — the pull term.** `P(caught in time)` is unmeasured because there is no
   subscription. Until it exists, §4's third factor is the largest unquantified term in
   the model. The surface's `resources/subscribe` is a **decoy**, not a subscription: it
   answers and records the knock, and pushes nothing — so it does not move this term at
   all, and a green `TestSubscribeDeliversNothing` is the absence of the mechanism, not
   evidence about it.
2. **The dedupe filter's parameters, its window, and its role.** The mechanism exists in
   design (§4); what is unmeasured is: **`n_burst` per tick** (which fixes `m`), whether
   the window `T` exceeds the retry horizon `τ_max` — the lab's defaults put them
   **equal**, so the escape case is normal rather than exceptional — and, the one that
   decides what a false positive *costs*, whether the filter is an **index** over the
   isotope records or the **authority**. If it is the authority, `p` is a lost-call rate
   and has to be sized as one.
3. **The common-mode term `c`.** Asserted dominant, not measured. Measuring it would
   mean counting how the spike's own failures actually occurred — which the traffic log
   already records.
4. **Correlation between the two stores.** `(2,1)` assumes the tmpdir and the sqlite file
   fail independently. They share a process and a host by design, so this is a *modelled*
   independence, and it is the independence §5 shows to be worth little.

**Not grounded — do not derive from it:** any specific value of `p`, `q`, `c`, `T` or
`W`. None is measured here, and fitting one would repeat the mistake `lab/MATH.md`
documents. The document is a structure — which factor each mechanism moves, and which
inequalities decide the trade — not a set of figures.

**Named cost, recorded rather than hidden.** Our store's parity is `(2,1)` across
*media* on one machine. That is defensible for the reasons in §5, and it is also the
reason the spike cannot answer a question about **host-level** failure at all: replica
count is one, and deliberately so. If a later step wants host failure in the model, it
adds a machine and a term — and by §3 it should expect the *rebuild* window, not the
steady state, to be where the loss lives.
