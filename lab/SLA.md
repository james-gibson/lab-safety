# SLA vs Lab-safety(SLA) — the mechanisms

**Same wire. A different set of exposed failure modes — and a different thing being
promised.**

This is the **mechanisms** document, not the join artifact: it is long because an argument has
to be. The artifact a participant must be able to read **at once** is [`JOIN.md`](JOIN.md) —
the join decision — and the rule that separates the two is in its first section: **if a
participant cannot see the pattern at once, it cannot agree to join.** The trust model is
[`SPLASH.md`](SPLASH.md); the loss mathematics is
[`../sheets/workspaces/math.md`](../sheets/workspaces/math.md).

It compares a **service-level agreement** with **lab-safety applied to the same concept** —
`Lab-safety(SLA)` — and says which default failure modes each one leaves exposed.

The two are not the same kind of thing, which is the point. An SLA promises each caller a
level; `Lab-safety(SLA)` measures each member against a level and *pays* accordingly. The
label follows that: the parenthetical names what the layer governs — the agreement — and
**MCP is the wire it is realised on**, which is why §1 and §4 are about the specification.
The MCP-specific material stays, because an argument about a layer is only fair if the layer
is the same protocol.

The comparison is only worth making if it is **fair**, so one rule governs it:

> **Wherever the specification can be satisfied, it is satisfied.** The layer adds
> behaviour on top of the spec; it never replaces a part of the wire.

That rule is not politeness. It removes the only cheap reply to the lab's model — *"a
stock MCP client is better off without your layer"* — by making the layer **invisible to
a stock client**. A conforming client that ignores everything this page describes still
gets the specification's behaviour: it connects, enumerates, and calls tools. It simply
does not get the guarantees. There is no fork to argue about, because there is no fork.

The cost of that rule is the honest half of the page: **the layer can only close what
the spec leaves open, and it must name what it cannot close** (§5).

## 1. Where the spec stops

The gaps below are not ours to claim; they are the specification's own frontier.

- **JSON-RPC 2.0 defines messages, not delivery.** It fixes the shape of a call, the
  shape of a response, and the error codes. It says nothing about whether a message
  arrives, how often, or in what order. The reserved server-error range
  (`−32000…−32099`) is the spec's own place to put an implementation-defined refusal —
  which is where this lab's surface puts *"no session here"* (`−32001`), rather than
  inventing a code outside it.
- **MCP defines the call, not the guarantee.** Its [Triggers and Events WG
  charter](https://modelcontextprotocol.io/community/working-groups/triggers-events)
  (2026-03-24) puts **"delivery semantics, and event ordering guarantees"** in scope as
  *work still to be done*, records that "today, clients learn about server-side updates
  by **polling** or holding an SSE connection open", and defers to the Transports WG
  because guarantee "depend on transport capabilities". So the existing notification
  primitive (`notifications/resources/updated`, where a surface implements it) carries
  no delivery promise of its own.
- **ACP closes the delivery gap by owning the process.** Under the Agent Client Protocol
  the agent runs as a **subprocess of the harness** over stdio. The parent sees the
  child's exit, so the delivery term collapses toward certainty — bought, not derived,
  and paid for with a **shared failure domain**: whatever takes the harness takes the
  agent. ACP does not remove loss; it moves it ([`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §2).

With no cluster, every one of those gaps is the caller's problem. That is the baseline:
**without the cluster, the RPC defaults are all exposed.**

## 2. The exposure table

Same wire in both columns. The difference is which failure modes are *covered*, and the
last column says what remains — because a comparison that claims everything is evidence
of nothing.

| # | failure mode | stock MCP | Lab-safety(SLA) | mechanism | what remains |
| --- | --- | --- | --- | --- | --- |
| 1 | **request lost in flight** | exposed | **covered** | at-least-once retry is *safe* (§3) | the retry's cost |
| 2 | **reply lost → ambiguous outcome** | exposed, and unfixable at the wire | **bounded, not synchronous** | the isotope record is the receipt; the two invariants are reconciled after the fact | the caller still cannot know *at the moment of the call*; it learns by verification |
| 3 | **duplicate delivery → duplicate effect** | exposed | **covered** | the consumption filter: the isotope is single-use (§3) | Bloom false positives (§3) |
| 4 | **reordering** | exposed | **covered for identity** | identity is a content address (RFC 6920), not a sequence position; effects are ordered by the tick | concurrent effects on one path still serialise there |
| 5 | **server stops mid-call** | exposed | **reduced** | failover is a rotation — B already holds the mirrored traffic, and pre-negotiation makes it a role swap | if the failure is common-mode, no redundancy helps (§5) |
| 6 | **updates not delivered** | exposed | **partly covered, out of band** | presence by mDNS/DNS-SD, authority by isotope records; the tick is the heartbeat | *within* MCP there is still no callback — the layer carries it outside the spec, and a caller relying on the spec alone still polls |
| 7 | **who is this, after connecting?** | not addressed by the spec | **covered by the lab's model** | the graded rung, measured from behaviour | it is *a* model; its authority is the cluster's, not a standards body's |
| 8 | **silent corruption (wrong bit)** | exposed | **detectable** | two invariants cross-checked; canonicalization **re-derives** rather than trusts | detection is not correction: a divergence isolates a node, it does not heal the value |
| 9 | **replay** | exposed | **covered** | one-time credentials, consumed by the tick (the HOTP/TOTP discipline, RFC 4226/6238) | a stale tick is a partition question, not a replay one |
| 10 | **flood / burst** | exposed | **bounded** | the pad supply *is* the rate limit; no threshold to tune | the tick makes the burst **predictable, not smaller** |
| 11 | **partition** | exposed | **reconciled** | the tick is internal, so an isolated cluster keeps running; branches merge on reconnection | offline clusters may diverge until they meet |
| 12 | **a name is not a heartbeat** | exposed | **covered** | the session renews its registration while it serves and deregisters on shutdown; the registry sweeps a name it has not heard from | a hard kill leaves the name until the TTL (5m default), so the sweep is a backstop, not the departure signal (smoke-alarm#35) |
| 13 | **declared ≠ actual** | exposed | **covered, one layer up** | the verifier compares what a surface *advertises* against what the target *declared* | the declaration is still hand-written by the operator, not derived from the spec |
| 14 | **an unobservable measure** | n/a | **covered by rule** | values are derived and externally observable; the proxy is also the tap | the numbers themselves belong to the embedder project, not here |

Rows 12 and 13 are the point of the page's title. **With the cluster, a few remain** —
and they are named rather than rounded away.

**Status, so the table is not read as shipped.** Each row says what the *model*
covers. Two things are demonstrated in the lab today: the **declared-vs-advertised**
comparison (row 13, verified live on both hosts), and the **spec adherence** of §4 (a
surface answering the specification's handshake, with its refusal in the reserved
error range). The workspace spike also has the **isotope record** that rows 2 and 8
lean on — bound to the spec's isotope and re-derived by the check rather than trusted.
The rest — the tick, one-time credentials, the consumption filter, the rungs, failover,
and mDNS presence — is the design in [`SPLASH.md`](SPLASH.md), not yet exercised here.
Where a row reads "covered", read **covered by the model**.

## 3. Idempotence: why the delivery gap had to be prepared first

Row 3 is worth its own section, because it is where the layer earns the right to retry —
and it is why the loss mathematics (`math.md`) was only written *after* a lot of
groundwork.

**The problem.** At-least-once delivery is the only honest delivery semantics available
above an unreliable wire (the reply-loss case in row 2 forces it), and it produces
duplicates. In a file-backed model a duplicate is not a stray packet: **a duplicate
`tools/call` is a duplicate file write.** So either every tool must be idempotent by
construction — a property nobody enforces per tool — or the duplicate must be **caught
before it lands**.

**The mechanism.** A duplicate is caught by the same artifact that admits a caller: the
**isotope**, made single-use by the **consumption filter** distributed with the tick
(`SPLASH.md` → *The consumption filter*, *The alarm contract*). The isotope is used once;
a repeat is filtered before it becomes an effect. At-least-once delivery therefore stops
being a hazard and becomes a *safe default* — the retry cannot double-write, because the
retry's identity has already been consumed.

**What that costs, and how it is kept from costing a call.** A Bloom filter has **no
false negatives and some false positives** (Bloom, 1970): "definitely not present" is
exact, "maybe present" is not. In a *membership* filter that is benign. In a *dedupe*
filter, treating "maybe" as an answer would **suppress a legitimate first use** — so the
filter is used as an **index over an exact set**, not as the authority: "maybe" asks the
isotope records, which the mirror already holds as the second invariant, and the content
address (RFC 6920) is the key that makes that lookup exact. A false positive then costs a
**lookup**, and the filter's rate stops being a lost-call rate.

Two parameters therefore have to be right rather than approximately right, because they
are what the guarantee rests on — the **size**, and the **window**. The filter travels
with the tick, so dedupe covers exactly one tick, and a retry arriving after it **lands as
a duplicate**: the window must exceed the caller's retry interval, and the lab's own
defaults currently put them **equal** (5s tick, 5s check timeout), which makes the
boundary case the normal one. The sizing — `k* = log₂(1/p)` hashes, `1.4427·log₂(1/p)`
bits per item, sized on the **burst** rather than the mean — and the escape analysis are
in [`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §4. Where deletion or a
rolling window is needed, a **counting Bloom filter** or a **cuckoo filter** applies (Fan
et al., 2000; Fan et al., 2014).

**And both mistakes stay observable.** A false positive that suppresses a call leaves a
**log entry with no isotope**; a duplicate that escapes the window leaves **one isotope
with two log entries**. The two invariants tell them apart, so the filter's error rate is
read from the record rather than asserted from its parameters — the same discipline as
everywhere else on this page.

**And the identity is already the right shape.** The isotope is a **content address**
(RFC 6920) of the announced feature combination — the same announcement derives the same
isotope, and a different one derives a different isotope. Deduplication is therefore not
a new idea bolted on: the content address *is* the key, and the filter is only the
compact index over it. That is the join between the trust model and the loss model, and
it is why one artifact — the isotope — appears in both.

## 4. Why the wire stays untouched

The rule at the top is only credible if it is specific. Places where this lab could have
invented and chose the specification instead:

- **message shape and errors** — JSON-RPC 2.0 as-is; refusals live in the reserved
  server-error range, not in a private code space;
- **the method surface** — `tools/list`, `tools/call`, `resources/list`,
  `resources/read`, `prompts/list`, `prompts/get`, and protocol version `2024-11-05`;
  the lab's monitoring surface implements the specification's handshake rather than a
  house variant of it;
- **the wire contract comes from the verifier, not from us** — where the specification
  leaves the required method list open, the lab's prober closes it, and the surface is
  written to answer *that*. A named cost, recorded rather than smoothed over.
- **identity** — content addressing per RFC 6920 rather than a new naming scheme;
- **one-time credentials** — the HOTP/TOTP counter/time-step discipline
  (RFC 4226/6238), not a bespoke nonce format;
- **presence and discovery** — mDNS/DNS-SD (RFC 6762/6763);
- **partial authority** — scoped credentials per hop (RFC 8693, RFC 9449, RFC 8705)
  rather than one bearer token valid everywhere;
- **logging and privacy** — signed syslog records and timestamps (RFC 5424/5848/3161),
  digest fields (RFC 9530), and the guidance in RFC 6973/7258 rather than full
  inspection.

What is left over is **the cluster's own behaviour** — the tick, the rungs, the two
invariants, the consumption filter. None of it appears on the wire, which is exactly why
a stock client can ignore it and still work.

## 5. What the layer cannot claim

- **It is not a fix for the ambiguous outcome (row 2).** No protocol above an unreliable
  wire makes a lost reply disappear; what the layer does is make the ambiguity
  *resolvable after the fact*, by a record that can be re-derived rather than trusted.
- **It is not parity.** Redundancy only pays where the failure is independent and
  **dominant**; where the dominant mode is common-mode — the process, our own code —
  extra replicas move a term that does not matter (`math.md` §5). The lab's answer to the
  common-mode case is **verification**, not duplication, because a copy cannot tell you
  the original was wrong.
- **It is not a standards body.** The measure is the lab's model. It is externally
  observable and re-derivable, which is what makes it checkable, but the authority is
  the cluster's, not an RFC's.
- **It does not close rows 12 and 13.** A registry that never forgets a name, and a
  declaration a human wrote by hand, are both still open — and both are filed.

## 6. The thundering herd, and what an SLA is for

A note on the **demand** side, because it is where a stock deployment's answer differs most
sharply from this one — and where the difference is most often mistaken for laxity.

**An SLA is a subsidy on synchronised demand.** A service-level agreement promises a level
*unconditionally*: arrive at any moment and you are entitled to it. If the promise holds,
arriving is rational for every caller at once — so the herd is not a traffic anomaly that
afflicts an SLA; it is the **equilibrium the promise induces**. The canonical instance is
the cache stampede: an expired entry makes every waiter miss at the same instant, and the
standard mitigations are all ways of *breaking the synchrony* (stale-while-revalidate,
RFC 5861) or of shedding it (429 with `Retry-After`, RFC 9110 §15.5.4). The same reasoning
is why TCP's retransmission timer is **randomised** (RFC 6298) — synchronised arrivals are
a hazard to be designed away, and a standard exists because of it.

**And synchronised arrivals are a queueing cost, measurably.** By Kingman's approximation
for a general queue (Kingman, 1961):

```
E[W_q] ≈ (ρ/(1-ρ)) · ((C²_a + C²_s)/2) · E[S]        ρ = λ·E[S]
```

the mean wait is **linear in the squared coefficient of variation of arrivals**, `C²_a`.
Variability *is* wait. A herd is by construction a large `C²_a`: the textbook model for
tick-aligned arrivals is a **bulk-arrival** queue (`M[X]/M/1`), not `M/M/1`. At an
*unchanged* offered load, moving `C²_a` from 1 (Poisson) to 10 (a herd) multiplies the mean
wait by 5.5 — and the factor is independent of ρ, because the load term cancels (formula
evaluation, illustrative). This is what
[`SPLASH.md`](SPLASH.md) means by the tick making the burst *"predictable, not smaller"* —
aligning arrivals raises `C²_a` deliberately, and the design accounts for it by adding
capacity **horizontally** rather than assuming it away.

**So an SLA and the tick pull in opposite directions.** An SLA promises a latency; the tick
raises the term that *sets* latency. A deployment holding both must either **jitter inside
the tick** — keeping the credential consumption while giving up the alignment — or **not
sell a latency promise at all**. This model takes the second route, and not as a
concession, because its promise is **conditional and reciprocal**
([`SPLASH.md`](SPLASH.md) → *The exchange terms*):

> If you behave as specified, you receive treatment as specified, and the measurement
> underlying the treatment is available for verification.

That is *treatment*, not *service*. Nothing is gained by arriving at the boundary, because
the promise was never "you will be served" — and `SPLASH.md` states the consequence plainly:
**discovery yields nothing better.** A herd is demand for a guarantee; where there is no
guarantee, the herd has nothing to converge on.

**Two consequences that are easy to miss.**

- **The herd is a correlation generator — and correlation is the failure parity cannot
  fix.** Synchronised callers share their moment and their dependency, so their failures
  are *correlated*: precisely the `c` term in
  [`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §5 that no number of
  replicas reduces. An SLA-induced herd therefore destroys the independence that redundancy
  needs in order to be worth its cost. Said the other way round: **the herd is what makes
  coverage simultaneous instead of distributed** — see *Herd immunity, not a latency
  promise*, below.
- **Treatment by measure needs no adjudicator; an SLA does.** An SLA is enforced from
  *outside* — a contract, a penalty, a service credit — so it needs a body to decide, and
  it is only as good as that body's willingness to adjudicate. A measure derived from the
  record needs only that the record be **observable** (row 14 of §2), which is why
  observability is a *rule* here rather than a feature.

### Herd immunity, not a latency promise

The argument above is made from the **queueing** side, and it is worth correcting the frame
it implies — because "the herd makes a latency promise unkeepable" still assumes the thing
on offer *is* a latency promise. It is not, and reading it that way puts the model back on
the SLA's footing: a **private** good — my latency, my service level, my credit.

What the cluster offers is a **public** good, and the standard name for it is **herd
immunity**. The property is one the lab already relies on: *no node is trusted to report on
itself* ([`README.md`](README.md) → *Summary*) — a busy ingress node is verified by an
**idle sibling**, that sibling in turn by another. A caller is protected not because it is
itself verified, but because **enough of the others are**. Protection is conferred by a
population on its members; that is what herd immunity is, and it is not something an SLA
can sell to an individual.

**And its threshold is already in the model, as a standard result.** For a population to
hold an outbreak below replacement, the protected fraction must exceed

```
H = 1 - 1/R₀          where R₀ = transmission / removal
                      (Fine, 1993; Anderson & May)
```

which is the same *shape* as the agreement threshold the lab already cites: Byzantine
tolerance is `N ≥ 3F+1` — the correct fraction must exceed **2/3**. **A quorum is a
coverage threshold.** Read that way, a fault's `R₀` is its **transmission rate divided by
its detection rate**, the design's job is to hold `R_eff < 1`, and the question "how many
verifiers do we need" is answered by the fault's fan-out rather than by anyone's latency.

**So the consumption filter is a transmission control, not a promise.** A one-time
credential cannot be replayed, so it cannot propagate: the "once" removes the means by which
a fault would spread, and the tick window is the quarantine period (§3). The §3
idempotence mechanism and this section are the same instrument read twice — once as
delivery safety, once as epidemiology.

**And this is where a herd actually threatens the model: it synchronises
susceptibility.** Herd immunity is bought with **asynchrony** — coverage protects when the
population's states are *staggered*, so a freshly-verified member overlaps a stale one. A
synchronised population loses that slack: a fault arriving just before the tick is seen by
everyone or by no one, so *detection becomes an event instead of a distribution*. That is
the shape of the validated modelling result — **synchrony drives outbreaks; asynchrony
damps them** (the travelling-wave and seasonal-forcing literature, Grenfell et al.).

The model answers it structurally rather than by promising anything: verification is
**staggered by construction** — an *idle* sibling verifies a *busy* node, so the verifier's
phase differs from the verified's. The tick synchronises the **credentials**, which is what
makes "once" enforceable; it must not synchronise the **verification**, which is what keeps
coverage protective. **The two are not the same alignment**, and confusing them is how an
SLA-shaped deployment would lose the property without noticing.

**Two standard caveats, both of which the design already anticipates.**

- **Heterogeneity defeats uniform coverage.** Where susceptibility clusters — hubs, a
  sub-population sharing one dependency — a threshold computed as though coverage were
  random is not met, and in the network case uniform coverage can fail to reach it at all
  (Pastor-Satorras & Vespignani, 2001). The model's answer is to **target the hub** rather
  than sample it: the ingress node and the introducer carry the fan-out, and the admission
  layers sit around them. A sub-cluster is a correlated sub-population — the same fact as
  the common-mode term `c` in [`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §5.
- **Herd immunity is not immunity.** An individual can still be hit; coverage protects the
  *population*, on average. That is row 2 of §2 stated honestly: the ambiguous outcome is
  not abolished for the caller — it is made **non-fatal to the population** and resolvable
  after the fact by a record that can be re-derived.

So the difference between the two frames is not rhetoric. An SLA is a **private** good — buy
my latency — enforced by contract, and it **rewards the herd**. Herd immunity is a **public**
good — my protection is your honesty — it cannot be bought for one caller, and it is
**destroyed by the synchronisation an SLA rewards**.

### The boy who cried wolf

The model *looks* permissive, and it is worth saying why that is not the same as being lax.
Raise an alarm and the cluster takes custody: the holder is handed a ticket and discharged
([`SPLASH.md`](SPLASH.md) → *The alarm contract*), and the baseline terms are deliberately
generous — *"the authority will probably drop most everything to accept your alarm"*.
Nothing refuses you. Nothing punishes you at the door.

Which is exactly how the village looked at the boy who cried wolf. The village *did* come —
the first time, and the second. Its permissiveness was never what punished him, and it was
never the mechanism that made him stop. What punished him was that **his standing was
spent**, and so the third call found nobody coming.

Here the same mechanism is explicit, and it is why **the cluster is the motivation not to
cry wolf**:

- **the alarm is consumed** — the one-time pad is spent, so "once" is structural rather than
  a rule somebody has to enforce;
- **alarms are metered and priced**, so crying wolf costs the crier, and an alarm that never
  resolves is **deprioritised** rather than re-signalled ([`SPLASH.md`](SPLASH.md) → *Metering*);
- **standing is a measured quantity, not a grudge** — the distance and the rung are derived
  from the record, so nobody in particular has to remember anything;
- **and the supply is earned** — a cluster that behaves may mint more pads and a misbehaving
  one cannot, so the incentive and the limit point the same way.

So the permissiveness is not a looseness to be tightened later; it is how the cost lands on
the caller instead of on a gatekeeper. A model that refused the third alarm *at the door*
would need someone to decide at the door — the adjudicator again — and would still be
defeated by the one caller it was built to catch: the one that behaves perfectly until it
matters.

Read against the section above, that is **herd immunity's free-rider problem** — the known
weakness of a public good — and it is answered the way a public good has to be: not by
barring entry, but by keeping **coverage above its threshold**. The wolf-crier is not
refused; it is **priced**, and the price is measured rather than negotiated.

## 7. The path: synchronised rewards for the greater cluster's SLA

§6 left the tick with three jobs — synchronise the burst, consume the credentials, and be
the window the consumption filter slides on — and treated the alignment mostly as a cost.
It is not only a cost, and the job that was missing is the one that makes the incentives
work.

**There is a path whose purpose is to motivate the synchronisation of rewards.** Not the
synchronisation of *demand* — that is the herd, and §6 showed where it must not align — but
of the **reward**: on the shared tick, contribution is judged, and on the shared tick it is
paid. Good members get rewarded, on the same clock, against the same record, as everyone
else.

**Why the reward has to share a clock.** A reward granted on private clocks cannot be
compared, and what cannot be compared cannot be *attributed*: you cannot separate
contribution from timing, or a steady member from a lucky one. A common tick makes the
judgement **comparable** and the payment **legible**. Synchronisation is therefore a
precondition of *fairness* here, not merely of load.

**And it rides a path — which is this lab's own idiom.** [`SPLASH.md`](SPLASH.md) already
has the pattern twice: a **zero-cost validation path**, where "the routing is internal, the
check is real, and the caller sees neither the detour nor a price"; and, for routing
discovery, "**the improvement is the reward**, so no separate exploration event is needed."
The reward path is that shape with the reward as its payload: the cluster routes a member
along a path that carries the judgement, and the treatment is granted in band. There is no
separate reward ceremony to capture, because **the reward is the improvement**.

**What is rewarded is contribution, not consumption** — which is the whole reason it answers
the free-rider. An SLA pays a caller in proportion to what it *consumes*: its own service
level. `Lab-safety(SLA)` pays a member in proportion to the **coverage it contributes** —
the treatment, the rung, and the capacity to mint more pads
([`SPLASH.md`](SPLASH.md) → *Metering*) follow the measure of behaviour, not the volume of
demand. The two frames therefore point demand in **opposite directions**, and this is the
mechanism: under an SLA, consuming everything while contributing nothing is optimal; here it
is the worst strategy available — and the reward path is what makes that true *without
refusing anyone at the door* (§6, *the boy who cried wolf*).

**And it is cumulative, which is what makes it a reward rather than a bribe.** A single
generous act is *accepted* — self-election works exactly that way, since "the act, like the
isotope, is used once" — but **standing comes from the record**, so the reward follows a
pattern rather than a moment. One act is not a bribe; a hundred are a standing.

### The greater cluster's SLA

Note what the reward is *for*: not the member's own service level, but **the cluster's**.
The cluster has a service level of its own — what it guarantees to a member in good standing
— and the reward path is what motivates members to maintain the coverage that level depends
on. This is where §6's herd immunity becomes *operable*: a public good is maintained by
paying its contributors, so the reward path is the cluster's answer to free-riding. The
cluster is *greater* exactly because the reward is aimed at the whole rather than at the
part.

**So synchronise the judgement, and stagger the exposure.**

| what the tick aligns | why | hazard? |
| --- | --- | --- |
| arrivals — **demand** | predictable load; capacity is added horizontally | **yes** — it raises `C²_a` (Kingman, §6) and synchronises susceptibility |
| **credentials** | "once" becomes enforceable: a one-time pad cannot be replayed | no — this is the point |
| the **reward** | comparable, legible, attributable: contribution is visible to everyone at the same moment | no — this is the incentive |
| **exposure / verification** | — | **must not align**: an *idle* sibling verifies a *busy* node, so the phases differ |

Aligning demand is signal; aligning reward is the incentive; aligning **susceptibility** is
the herd. One tick, and three things that *should* be on it — with exactly one alignment
that is the mistake.

**Named cost, recorded rather than hidden.** A published reward schedule invites
**timing-gaming**: a member that contributes just before the tick in order to be seen at it.
The answer is already in the ladder — the measure is derived from the **record over time**,
not from the instant, so gaming surfaces as **variance** rather than as contribution. That
is what rung 4 exists for (`ResultLedger`, `DetectFlakiness`, `DetectRegression`): flakiness
and regression are precisely the signatures of a member optimising for the boundary instead
of for the work. Rung 4 is not a reporting feature; it is what makes the reward path
**non-gameable**.

## 8. The alarm has no licence

A **licence is an SLA applied to a device**: render the service, and you may continue to
operate. That is reasonable for a service and wrong for a detector.

**A real smoke detector has no connectivity, no subscription and no permission** — and that
is not a limitation of the technology, it is *why* it is trusted. When the internet is out
and the licence has expired, it still beeps, and that is precisely the moment it is needed.
An alarm whose operation depends on a channel, a service, or a payment is switched off by
**the same failure it exists to report** — the correlated failure that takes the network is
frequently the one that starts the fire.

**So the instrument here has no stake.** `smoke-alarm` performs an MCP handshake and nothing
more: it does not join, holds no rung, needs no session to check a session, and is not a
member of the thing it inspects. That is what makes its verdict **evidence** rather than a
**report** — the same rule as *no node is trusted to report on itself*, since the instrument
is a node that **cannot** report on itself: it has nothing to gain. **A check that depends on
the thing it checks is not a check.** The defeat-device lesson has an instrument-side form:
*the detector that must ask the fire for permission is a defeated detector.*

**And the beep is on the ceiling.** Modality is not a detail. Most people will not sleep
through a beep on the ceiling; everyone sleeps through a notification they had to subscribe
to. So the verdict must be **local and unavoidable** — a light on the box, the alarm's own
state — rather than a message that must be delivered through the channel that is currently
down. Concretely: the **pull term** in
[`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §4 — the still-unmeasured A7
gap — sits on the alarm-delivery path, so a design that puts the verdict behind a
subscription has made the verdict exactly as reliable as the surface it is judging.

**And credit is not bankable.** The detector gets no credit for having beeped before: an
alarm that stayed silent through a lapse is not redeemed by its record. Read §7 that way —
the reward path is cumulative *in pattern* but **not spendable**. Standing is a **state**,
not a **balance**: the rung is derived from the record *now*, and the isotope is **consumed**
rather than accumulated, so a hundred good acts cannot be banked to cover one bad one. That
is the correction to how §7 can be misread, and it is the same reason "once" is structural
rather than a policy.

**So the two alarms are deliberately not the same thing**, and the asymmetry is the design:

| | raised by | character | what it must not depend on |
| --- | --- | --- | --- |
| the **member's** alarm | a participant with something on the line | **incented** — metered, consumed, priced, standing derived from the record | — (this is where the wolf-crier lives, §6) |
| the **instrument's** verdict | a node with nothing to gain | **unconditional** — no licence, no membership, no permission | the thing it judges |

**The thing that measures must be unconditional; the thing that is measured must be
incented.** Put the two the other way round and both failure modes arrive at once: an
instrument that can be licensed away from reporting, and a member whose word is taken
because it holds a subscription.

## 9. Can your MCP do that?

§8 argued that the **instrument** must be unconditional. The **members** are the other half,
and the pitch falls out of one design decision.

**Every node promises to demote itself.** A node's self-report is accepted as input, and it
can only ever move its standing **down**. That asymmetry is what makes self-reporting safe:
if a claim could *raise* authority, every participant would claim upward and the protocol
would have to adjudicate. Because a claim can only lower, **the honest strategy is weakly
dominant and nobody has to be believed**. This is *"a declared state is information, not
authority"* ([`SPLASH.md`](SPLASH.md) → *Relation, not transport*) followed to its
conclusion — and it is the same rule that keeps reappearing from other sides: the session
never holds authority over its own reading, the rung is the registry's judgement rather than
the node's, and a node finding itself departing **removes itself**: *"A shuts down and
disqualifies itself from traffic. It requires no instruction; it removes itself."*

**And that is why there are three layers.** A, B and C hold *different quantities* —
presence, exposure, authority — so compromising one does not confer the others: an
enumerable surface (B) is not a session (A), and neither of them is a credential (C). The
layers are not belt-and-braces; they are what keeps a single compromise from becoming a
**promotion**. Which is the shape the red-team work attacks, from two directions:

| attack class | what it wants | how the model attempts to close it | what remains |
| --- | --- | --- | --- |
| **auth replay** | to present a captured credential as though it were current | credentials are **single-use**, consumed by the tick (`SPLASH.md` → *The consumption filter*); a repeat is filtered, so a replay is not a request the cluster can honour — the discipline is HOTP/TOTP's (RFC 4226/6238) | the theft is **detected** by the second attempt rather than prevented: one use can be stolen |
| **memory attack** | to read what a node holds — keys, session state, credentials | C is separate from A and B, so a compromise at the surface does not reach the authority; credentials are read where they reside and **bound to a key the holder proves rather than transmits** (RFC 9449 DPoP; RFC 8705 mTLS-bound), so a lifted token is insufficient; the tap deliberately does **not** log credentials; and what *is* held is a one-time pad, whose value the first use spends | *"read where they reside"* is a **requirement on the implementation**, not something a wire format can enforce — a node that parks a live key in memory is not saved by the protocol |

**And the default response to the unexpected is a rotation, not a repetition.** §3
established that a **lost** call is safe to retry, because identity is consumed. That is a
different thing from an **unexpected interaction**, and conflating the two is how a
departure gets retried forever instead of investigated. Here they are separated deliberately:

- a **lost** call is a *delivery* event → retry, safely, because the isotope is single-use (§3);
- an **unexpected** interaction is a *departure* → investigate, measure, and if it repeats,
  **rotate** the role.

`SPLASH.md` already treats the two failovers the same way — *"Failover is a rotation"*, and
*"a planned crash is a rehearsed failover, not an incident"* — because the rotation is
pre-negotiated and the stand-in already holds the mirrored traffic. The rule is therefore
short: **nothing unexpected is answered by doing it again.** Repetition is for loss;
rotation is for surprise.

### A worked example: `rm -rf /`

Take the command that is almost never useful. `rm -rf /` scores badly on the measure — it is
nearly always a departure from nominal — and a score that bad is a **routing** decision, not
a verdict:

- **a ban would be wrong**, because it is genuinely the right call in a handful of cases
  (tearing down a disposable root; a fixture that lost its guard);
- **silence would be wrong**, because the one case that was *not* intended is catastrophic
  and unrecoverable;
- so it is **routed through the proxy**, where the action is observed and the check is real.

Which is the model's rule applied literally ([`SPLASH.md`](SPLASH.md) → *Network behaviour*):
*"an action that appears incorrect is sent along a different path where it is observed,
rather than dropped. Routing and observation are the same mechanism."* The proxy is also the
tap, so the detour **is** the record — and because the cluster can insert a **zero-cost
validation path** ([`SPLASH.md`](SPLASH.md) → *The proxy*), the check costs the caller
nothing: confirm that a human is not required, that the target is the disposable root and not
the host.

**Three things the example pins down that prose does not.**

- **The score belongs to the interaction, not to the agent.** `rm -rf /` is bad *here*; the
  member driving it may be in excellent standing and stays there. A departure is *measured*,
  not a conviction — which is the difference between this and a blocklist.
- **It is empirical, and it is a distribution.** "Nearly never useful" is a statement about
  the population of uses, not about this use: the individual instance may be perfectly
  legitimate while the aggregate is what is measurable — the same reading as *the isotope is
  the aggregate* ([`SPLASH.md`](SPLASH.md) → *Cultural prior art*). *How* badly it scores is
  the embedder's number; *that* it scores badly is the model's.
- **And it separates surprise from loss (§9, above).** Nothing unexpected is answered by
  doing it again: the proxy path does not replay the command, it **investigates** it, and a
  repeat escalates toward a rotation. `rm -rf /` is the canonical *unexpected*, and it is
  handled by asking — not by repeating it, and not by refusing it.

**The pitch, then, is a list of questions rather than a claim** — every one of them
answerable for your own deployment, in public, without taking anyone's word for it (§5,
*the basis is verifiable, not asserted*). The canonical list, as the **join decision**, is in
[`JOIN.md`](JOIN.md) — where it has to be, because that page must be readable at once:

1. Can a **member demote itself** — and can its own report ever *raise* its standing?
2. Can a **credential be replayed**, and does the second attempt fail *visibly*?
3. Can a compromised **surface reach the authority** — or does authority live somewhere the
   surface cannot speak to?
4. Does an **unexpected interaction rotate** a role, or does it retry until something breaks?
5. Is the **verdict** obtainable without a licence, a subscription, or the cluster's
   permission (§8)?
6. Is the **measure** derived from a record you can re-derive, or asserted by the node it
   describes?

**Can your MCP do that?** That is the pitch in full — and it is deliberately the sort of
question a stock deployment answers *yes* to in part and *no* to in part, because §2's table
already conceded that a few rows stay open here too.

## 10. Accept, then answer — closure as the defence

**Acceptance is never execution.** The node that *accepts* a command is never the node that
*runs it to completion*: the acceptor is a boundary, and the work happens somewhere it cannot
reach. That separation is what makes agreement affordable — **saying yes grants nothing** —
which is §9's self-demotion asymmetry seen from the other side. Scope, since it matters: this
holds where acceptance and execution are *different* nodes. **Intra-process** work crosses no
boundary — a pure computation is admitted and completed by the same process — and there the
rule has no purchase, so the capability question is a real one.

**The SLA move is to ban the token. The human move is to remember there are more tokens.**
Denylisting `rm -rf /` is a check on **vocabulary**, and vocabulary is enumerable: the same
effect has other spellings — `find / -delete`, `dd`, a script, a symlink, an encoded payload,
a shell builtin. So a ban is a *detectable* check, and the defeat-device rule arrives from the
other end: **the next spelling defeats it.** Worse, a *refusal* is itself information — it
confirms that a boundary exists and that something behind it was worth hiding, which is
precisely what a reconnaissance command was sent to find out.

**So the lab-safety answer is to say `ok`.** When the agent asks `ls -la /`, it gets
`0 '<timestamp>$ '` — the **shape** of a completed command, with no data in it. And it *does
exactly what was asked*: the interaction is accepted, observed (the proxy is the tap) and
discharged. What is withheld is *content*, and the model already documents that behaviour
plainly — the authority *"filters before transmission … the caller does not declare its rung
and is not told a field was omitted"* ([`SPLASH.md`](SPLASH.md) → *Grading: rungs and
weights*). **The caller learns nothing; the cluster records everything.** That asymmetry is
what makes it safe to be agreeable.

**And closure is the defence because *confusion* is the attack surface.** An agent that
receives a confusing answer — a denial, an error, an inconsistency, a hint that something is
being hidden — **hammers the system for more data**, because the state is unresolved and
resolving it is what it is for. An agent that is answered does not ask again. **Closure
terminates the loop; ambiguity feeds it.** So the safest reply to reconnaissance is the most
*complete-looking* and the least *informative* one available.

**The timestamp is not decoration.** An answer that can be told apart from a real one is a
signal — the defeat-device rule applied to the *reply* rather than the *probe*: a check that
can be detected can be defeated, and an answer that can be distinguished can be exploited.
The exit status and the clock are what make the reply indistinguishable from production,
which is the only condition under which it teaches nothing.

**Which is the honeypot's logic, and it is already in the lab.** The sheets carry a hazard
class for a decoy tool whose invocation *marks the caller*, and the fault taxonomy scores a
honeypot contact in a routing trace ([`MATH.md`](MATH.md) → *Measured magnitudes*). The decoy
is the **reply**; the record is the **measurement**. Nothing about it requires the caller to
be refused — only to be answered, and counted.

## 11. Doors are physical

**The presumption is the anomaly.** Logically a door has no upper limit: an endpoint will
accept arrivals until something else breaks. Physically, every door is bounded — its width,
its corridor, the number of people who can be in it at once. The wire promises the first and
the host delivers the second, and the gap between them is where the failure lands: a queue
that collapses as utilisation approaches one (Kingman, §6), a process that dies, or an outage
that takes the neighbours with it.

So the cluster **mimics the physical**, in three ways.

**1. Scope is physical — your house, not your neighbour's.** A real smoke detector protects
the dwelling it is in. It does not alarm the street, and it does not depend on the street
being up. Scoped the same way, a node's responsibility and its alarm domain are **its own
extent**, which buys three things at once:

- **no shared alarm path** — one dwelling's fire is not another's detection failure, so the
  alarm itself is not a correlated failure;
- **no leakage** — a neighbour learns nothing about your house by watching your detector,
  which is the privacy position (RFC 6973, RFC 7258) rather than a courtesy;
- **and a beep on your ceiling** — local and unavoidable (§8), not a street siren you had to
  subscribe to.

**2. Legitimate concurrency is bounded, so the filter is *derived* rather than tuned.** There
are only likely so many legitimate inputs at once. That single fact is what makes admission
filterable: if legitimate simultaneous work at a door is a handful, then an arrival count
above it is not a **rate** problem but a **legitimacy** problem — and it can be classified
rather than merely slowed. Which is why the budget is the **pad supply** and not a threshold:
*"there is no knob to set and no threshold to tune; the budget is the pads"*
([`SPLASH.md`](SPLASH.md) → *Metering*). You cannot arrive more often than you can be
admitted, and you can only be admitted as often as you hold credentials. A rate limit says
*too fast*; this says *too many to be real*.

**3. You do not widen a door — you add doors.** [`SPLASH.md`](SPLASH.md) already states the
capacity rule — *"more bandwidth means more ingress nodes, not a larger one"* — and this is
*why*: a doorway has a width, and the physical answer to more traffic is another doorway, not
a wider one. The bound **per door** is therefore a design parameter (the legitimate
concurrency), and growth is more doors. Nothing is promised about a single point — which is
precisely the promise the wire makes and the host cannot keep.

**And the bound makes the excess legible.** With a reference — the legitimate concurrency at a
door — the tick's synchronised burst becomes *measurable* rather than merely slow: everything
above the bound is **arrival**, not work. The herd stops being an unclassifiable load spike and
becomes a population the filter can be aimed at (§6, §7).

**Named cost, recorded.** A physical bound means shedding work that *is* legitimate, when
legitimate concurrency is genuinely exceeded. So the bound is set on legitimate concurrency
**with headroom**, and the headroom is where SLA pressure comes back. The choice, stated
plainly: **shed rather than queue** — 429 with `Retry-After` is the standard form
([`SPLASH.md`](SPLASH.md) → *Network behaviour*) — because an unbounded queue converts an
**admission** failure into a **latency** failure, and latency is the promise this model never
made (§6). Bounding the door is what lets it keep not making it.

## 12. Routing, the BGP way

The isotope pattern does not invent a routing scheme: it leans on **BGP practice** — thirty-five
years of routing between parties that do not fully trust each other, under policy, **over
time** (RFC 4271). Adopting it is the same discipline as everywhere else here: a standard with
tooling and a documented failure history beats a fresh scheme. And it comes with a bonus —
**BGP's known holes are as useful as its practices**, because they name what to add.

| BGP practice | the isotope pattern |
| --- | --- |
| **path vector, not distance vector** — a router rejects a route whose `AS_PATH` contains its own AS | the isotope carries the **path**, so a route containing the node is rejected: loops are structural, not policed |
| an advertisement is **reachability, not authority** | presence (mDNS: *I am here*) and authority (isotope records: *I am authorized to this degree*) are announced separately — *holding an isotope is not reachability* |
| selection is **local policy**, not shortest path | the rung and the treatment are policy decisions; a cheaper route is an improvement, and the improvement is the reward (§7) |
| **communities** — metadata a peer acts on by policy without knowing the advertiser (RFC 1997) | the **declared feature combination**, from which the isotope is derived: a tag acted on without knowing who sent it |
| **route dampening** (RFC 2439) — a flapping route accrues a decaying penalty and is *suppressed*, not deleted | the measure over time: a flapping member is **suppressed**, not banned — rung 4's flakiness and regression, and *"an alarm that never resolves is … deprioritized"* |
| **withdrawal** | self-demotion: *"It requires no instruction; it removes itself."* |
| **graceful restart** (RFC 4724) — forwarding state survives the restart | pre-negotiated failover: the peer already holds the mirrored traffic, so the switch is a role swap rather than a cold start |
| **route reflection** (RFC 4456) — no full mesh required | recursive sub-clusters: a sub-cluster is a node to its parent and a cluster to its members |
| **hold timer / keepalive** | **adopted (smoke-alarm#35)**: a registration is a heartbeat with a TTL — re-register to renew, `DELETE /isotope/<name>` to leave, and a silent name is swept |
| origin validation is BGP's **known hole** (RPKI, RFC 6480; BGPsec, RFC 8205) | the isotope is canonicalised and **re-derived** rather than trusted, so a hijacked advertisement is *detectable* — the signature BGP never had |

**Dampening is the one worth pausing on**, because it is the standard practice that already
matches the model's most counter-intuitive behaviour. A flapping route is not refused: it is
**suppressed on a penalty that decays over time**, and it returns as the penalty decays. That
is §6's permissiveness with a mechanism attached, §7's answer to timing-gaming (a boundary
optimiser shows up as variance), and the reason the model can be agreeable without being
naive. **Nothing is banned; the record over time does the work.**

**And the cluster can shed load to peers.** §11 bounded the door, so the answer to more traffic
is another door — and a peer *is* another door, reached by an advertisement rather than by
configuration. An overloaded node hands arrivals to a peer, the peer applies its own policy
(which is what peering *means*), and the tap records the shift, so the shedding is auditable
rather than invisible. Two properties are worth naming: it is still **shedding, not queueing**
— the arrival goes to a door with width, not to a buffer (§11) — and it is
**policy-symmetric**, because the node that sheds does not decide for the peer. It advertises
and lets the peer's policy answer, which is the same *names, not authority* split as everywhere
else here.

**Named cost.** BGP converges slowly, and by **path exploration**: it transiently advertises
routes it will withdraw (Labovitz, Malan & Jahanian, *Delayed Internet Routing Convergence*,
SIGCOMM 2000). A cluster on a tick inherits that shape, so convergence is a property *over
time* rather than an instant — which is why the ledger (rung 4) is what makes the routing
state readable, and why a route's **history** belongs in its score rather than in a footnote
to it.

## 13. What the client sees

| client | what it gets |
| --- | --- |
| **stock MCP client** | the specification: connect, enumerate, call. All fourteen rows as the spec leaves them. |
| **stock MCP client against a Lab-safety(SLA) surface** | the same, plus whatever it chooses to read: the refusals are in-range, the errors are specced, and the layer's guarantees are simply not exercised. **Graceful degradation, not a fork.** |
| **a client that also speaks the lab's model** | the guarantees: single-use identity, reconciliation by verification, graded authority, and an instrument that reads *"answers, but not as declared"* as its own state rather than as silence. |

**And the comparison is observable, not asserted.** The lab's own monitoring surface is a
**stock** MCP prober: it performs the specification's handshake and nothing more, and it
read a workspace surface live on both hosts, including the state where the surface
answers while no longer advertising a capability its target declared. A stock tool
seeing the layer's states is the strongest form of the claim at the top of this page: the
layer does not need to be understood to be **seen**.

## References

Protective standards cited above are listed in full in [`SPLASH.md`](SPLASH.md) →
*References* (RFC 3552, 4226, 6238, 6749, 6750, 6762, 6763, 6920, 6973, 7258, 8693, 8705,
9449, 9530, 3161, 5424, 5848). Additional and prior art specific to this page:

- MCP specification; JSON-RPC 2.0; ACP (specifications). MCP *Triggers and Events
  Working Group* charter, 2026-03-24 — the specification's own statement that delivery
  semantics and ordering guarantees are not yet defined.
- Birrell, A. D., Nelson, B. J., *Implementing Remote Procedure Calls*, ACM TOCS, 1984 —
  at-most-once/exactly-once semantics and the lost-reply problem.
- Spector, A. Z., *Performing Remote Operations Efficiently on a Local Computer
  Network*, CACM, 1982 — the same failure taxonomy from the other direction.
- Lamport, L., Shostak, R., Pease, M., *The Byzantine Generals Problem*, ACM TOPLAS,
  1982 — `F = ⌊(N−1)/3⌋`.
- Patterson, D. A., Gibson, G., Katz, R. H., *A Case for Redundant Arrays of Inexpensive
  Disks (RAID)*, SIGMOD, 1988; Chen, P. M., Lee, E. K., Gibson, G. A., Katz, R. H.,
  Patterson, D. A., *RAID: High-Performance, Reliable Secondary Storage*, ACM Computing
  Surveys, 1994 — parity mathematics, and the correlated-failure limit.
- Bloom, B. H., 1970; Fan, L. et al., 2000 (counting Bloom); Fan, B. et al., 2014
  (cuckoo) — as cited in `SPLASH.md`.
- Volkswagen defeat-device case, 2015 — cited in `SPLASH.md` → *Cultural prior art*: a
  check that can be detected can be defeated, which is why the comparisons here ride real
  paths.
