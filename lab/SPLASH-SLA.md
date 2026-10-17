# SLA vs Lab-safety(SLA)

**Same wire. A different set of exposed failure modes — and a different thing being
promised.**

This is the second splash page. [`SPLASH.md`](SPLASH.md) states the trust model; this page
compares a **service-level agreement** with **lab-safety applied to the same concept** —
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
| 12 | **a name is not a heartbeat** | exposed | **still exposed** | the probe notices; the registry does not deregister | **open** — filed as smoke-alarm#35 |
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

## 8. What the client sees

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
