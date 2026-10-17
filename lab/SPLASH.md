# Trust after connection

**An agent can connect to an MCP service. Whether it is trusted after connecting
is a separate question, and it is the one most transport specifications do not
answer.**

Transport is a solved problem: a caller can open a connection, complete a
handshake, enumerate tools, and invoke them. None of that establishes whether the
caller should be trusted, or whether the service should be. Call this the
*post-connection trust* problem (cf. RFC 3552, *Guidelines for Writing RFC Text on
Security Considerations*, which separates the transport from the trust model
layered on it).

This document describes one model for it, and cites the public specifications each
part draws on. Public references are given wherever a standard exists; where none
exists, the entry is marked as prior art or design.

## Relation, not transport

Two parties can both complete a handshake. The difference between them is the
*relationship*, which the transport does not carry. Authentication (who is
speaking) and authorization (what they may do) are distinct concerns in the
standards — OAuth 2.0 (RFC 6749) and Bearer Token Usage (RFC 6750) are explicit
that a bearer token establishes authority, not identity, and that possession is
the whole of the claim.

Trust here is therefore not a property of the connection. It is a property of the
relationship, measured from behaviour rather than declared. This mirrors the
security-considerations guidance in RFC 3552 and the privacy guidance in RFC 6973.

Consequences, stated plainly:

1. **The connection is not the trust boundary.** Handshake and `tools/list` say
   nothing about authority.
2. **Trust is graded, not binary.** It is a quantity, not a flag.
3. **A declared state is information, not authority.** A caller announcing a
   problem is offering input; the response is a decision by the authority, not a
   claim by the caller.

## Enrollment and one-time credentials

Trust boundaries are established before the incident, not during it. Two
well-known mechanisms:

- a **pre-enrolled, narrow channel** — a participant is given one means of
  signalling and nothing else (a doorbell, not a key);
- a **scoped, single-use credential** — bounded in what it opens and how often.

Both are standard practice. Scoped, one-time credentials are specified in HOTP
(RFC 4226) and TOTP (RFC 6238); proof-of-possession, where the credential is a key
the holder proves rather than a value it transmits, is specified in DPoP
(RFC 9449) and mTLS-bound tokens (RFC 8705). A one-time credential is
non-replayable by construction: the nonce or counter is consumed.

## Entry: the one-time credential

Entry is deliberately inexpensive. A participant does not need a client, a
certificate, or a build; a minimal, single-use artifact suffices — a one-time
password (RFC 4226/6238), a one-time binary, a QR code, or any equivalent. The
*form* is unspecified; the properties are not:

- **single-use** (the credential is consumed on use), and
- **narrow** (it confers one means of entry, not general authority).

The cluster is therefore open at its edge and closed at its core. A participant
admitted this way holds no session, no write authority, and no credential beyond
the first layer. The entry credential grants consideration, not trust;
authorization follows from behaviour.

### Consumption by the tick

The one-time credential is consumed by the shared tick: each tick cycle refreshes
the set and invalidates the previous one. This is the counter discipline of HOTP
(RFC 4226) and the time-step discipline of TOTP (RFC 6238) — a credential is valid
for one step.

This is not incidental to the burst problem. Without consumption a credential could
be replayed, and a credential valid indefinitely is a single point through which any
number of requests can be admitted — a burst behind one credential. The tick both
synchronises the burst *and* consumes the credentials; **without the second, the
first is insufficient.**

### The consumption filter

The consumption is carried by a **Bloom filter distributed with the tick update**.
Each tick, the used isotopes are inserted into the filter; the filter travels with
the tick, and a node checks its isotope against it rather than asking a central
authority. The properties that matter are the filter's:

- **probabilistic membership** — no false negatives, some false positives (Bloom,
  1970). "Probably present" is the guarantee; the false-positive rate is the price
  of the filter's size.
- **one-way** — membership can be tested, but the set cannot be enumerated, so the
  filter does not disclose the isotopes it holds.
- **compact** — small enough to distribute with every tick, which is what lets the
  whole consumed set travel with the update instead of being queried.
- **consumption** — an isotope present in the filter has been used, and a repeat is
  filtered. (Where deletion is required, a counting Bloom filter or a cuckoo filter
  is used — Fan et al.)

This is the unstated part of the baseline terms:

> *If you behave, the authority will probably drop most everything to accept your
> alarm — but you get to do it once.*

The "probably" is the false-positive rate; "drop most everything" is the preference
the alarm is granted; "once" is the consumption. The filter is what makes "once"
enforceable with a per-tick artifact rather than a central check, and what lets the
decision travel with the tick instead of being requested.

## Prior art

The post-connection trust problem is old. The following solved *delivery* and left
*ongoing trust* open. RFCs are cited where one exists.

| prior art | solved | left open |
| --- | --- | --- |
| **UDP anycast** (RFC 4786) | reaching the nearest instance | the sender is anonymous — no session, no handshake; the source is whatever the datagram claims |
| **DNS** (RFC 1034/1035) | naming | the resolver is trusted; freshness and authority are the hard parts (DNSSEC, RFC 4033–4035, is the acknowledgement) |
| **cert revocation** (RFC 5280, OCSP RFC 6960) | issuance | revocation is soft, delayed, and frequently unchecked; a certificate is trusted until noticed |
| **Tor** (tor-spec) | node identity via relay keys | membership is a document (the directory consensus), not behaviour |
| **double- vs triple-entry accounting** | recording a transaction | two entries are private; triple-entry adds a third, signed and shared entry, making the record non-repudiable |
| **payment over HTTP** (HTTP 402, RFC 9110 §15.5.2) | a payment-required signal | no spend limits, and a single ledger serialises approvals — a burst of approvals has the same shape as a request burst |
| **proof of work** (Bitcoin) | a costly, publicly verifiable proof | the proof is discarded once made, and selection is a race (a lottery): work is proportional to attempts, most of it wasted |
| **numbers stations** | a one-way broadcast readable only with a one-time pad | the listener must already hold the pad; the signal is otherwise meaningless |
| **spreadsheet engines** | calculated values — derived from inputs, recomputed on change (a dependency graph, not stored state) | recalculation is per-cell unless the order is planned; batching is not automatic |
| **Bloom filters** (Bloom, 1970) | compact probabilistic set membership — no false negatives, one-way, small enough to distribute with an update | false positives are unavoidable, and the plain filter does not support deletion (counting or cuckoo variants do — Fan et al.) |
| **defeat-device testing** (Volkswagen, 2015) | a system that detects a test and behaves differently under it | if a check is detectable, it can be defeated; the check must be indistinguishable from production (*Audit and logging*), or it verifies only the test |

The design borrows from several. It announces **membership** (presence) and
**authority** separately — see *Membership and authority announcement*. It keeps
the proof-of-work property (a checkable proof) without the discarded work (see
*Derived values*). And like a spreadsheet, its values are calculated rather than
stored.

A separate prior art, from outside networking: a laboratory that detects exposure
to hazardous material responds with **further measurements, not immediate
disposal**. Detection increases scrutiny; it does not by itself remove a
participant.

### Cultural prior art

The model's principles are not new. They are proverbs older than the technology.

- **"You are what you eat."** Identity derives from what one takes in. An isotope is
  the content address of the features a participant declares (*Decomposition by
  feature*): identity is content, and content is what was consumed and produced.
- **"If your friends all jumped off a bridge, would you?"** Both readings stand, and
  they are different. The **danger** reading: conformity can be wrong. The **cost**
  reading: the path has a price, and the price is **per-agent**. A participant with
  **free egress** — a zero-cost path to data it requires for further work — is not
  following a crowd off a bridge; it is taking a path that is cheap for it and
  expensive for others. **That path is off the bridge for that participant.** The
  cost rule is therefore conditional: the proverb applies to a cost you would
  actually bear, not to one another participant bears and you do not.
- **And "off the bridge" can be a choice, not an error.** A participant may
  deliberately take an unexpected route — through a proxy it would not normally use —
  to **correlate a stack of unverified actions and add validation**, with a human in
  the loop. The route is surprising precisely because it adds a check. So "off the
  bridge" covers two opposite cases: the *danger* case, following an unverified
  crowd; and the *validation* case, a deliberate detour to a proxy that verifies,
  because a stack of unverified actions warrants the extra check.
- **"The golden rule."** Reciprocity: the exchange terms are exact and mutual (*The
  exchange terms*). Do the specified thing; receive the specified treatment.

Each is a statement the culture made long before the protocol: identity from content,
independence from the herd, and treatment by measure.

**And the defeat-device lesson.** A system that can detect a test will pass the test
and fail in production (Volkswagen, 2015). The design avoids this by making the checks
indistinguishable from production: the probes and honeypots run on real paths (*Audit
and logging*), the cross-checks ride real routes (*Shortcut validation*), and the tap
does not announce itself. **A check that cannot be detected cannot be defeated.**

**And the aggregate caught it.** The per-instance test *was* defeated; the aggregate
was not. The divergence surfaced in a national quantity — the annual fuel
consumption of an entire country's fleet — which no defeat device can quietly fake,
because faking it means faking every car's real driving. **The isotope is that
aggregate**: the derived value over the population, which the individual can avoid
but the total cannot. A single test is defeatable; a sum over the world is not.

So the two readings of the defeat-device lesson are one: make the check
indistinguishable (so it can't be dodged per instance), **and** measure the aggregate
(so it can't be dodged in total). Either alone leaves a gap — a hidden check the
cheater ignores, or a total the cheater explains away. Together they close it.

### Fragile prior art

Not all cited prior art holds. Three commonly relied-on examples are weaker than they
appear, and are cited here as *warnings*:

- **Tor anonymity degrades with coverage.** With enough relays, an observer can
  correlate paths and observe originations (Murdoch & Danezis, 2005). The design
  therefore does not rely on concealment: it relies on the isotope — **proof, not
  hiding.** An observer of the traffic still cannot forge the receipt.
- **The Turing test presumes a blind medium** (Turing, 1950). It assumes the
  interlocutors cannot see each other; if they can, the test does not hold. A check
  that presumes a condition fails when the condition is absent — the defeat-device
  lesson again, from the test's side.
- **The marshmallow test did not replicate.** The widely cited result on delayed
  gratification failed to reproduce at scale (Watts, Duncan & Quan, 2018), so
  conclusions drawn from it are not available. The **prisoner's dilemma** framework
  is robust, and is the prior art the exchange terms draw on (*The exchange terms*).

The lesson is uniform: cite what replicates and what is measurable, not what is
famous.

**And the three criteria are independent axes.** Replication, measurability and fame
do not rank a thing on one scale — they span a space, in which a claim or a citation
has a *position*:

- **replicates?** — holds when checked;
- **measurable?** — has a quantity;
- **famous?** — is widely cited.

Because the axes are independent, the combinations are distinct and each is useful:

| replicates | measurable | famous | reading |
| --- | --- | --- | --- |
| yes | yes | yes | solid, and load-bearing |
| yes | yes | no | solid, and overlooked |
| no | — | yes | **dangerous**: cited for its fame, unsupported |
| yes | no | — | directional, not quantitative |
| no | — | no | harmless |

This is the plane (*The measure*) applied to claims rather than participants: a
coordinate space that lets otherwise-incomparable things — a study, a protocol, a
constant — be compared on the same axes. The "fun math" is the same as everywhere
here: project onto the plane, read the position. The danger cell is the one to watch,
because fame is the axis an uncritical reader uses, and it is the axis uncorrelated
with truth.

## Network behaviour

The design is shaped so that the network continues to function while trust is
determined.

- **A shared tick.** Participants act on a shared cycle (the federation's
  heartbeat and announce intervals), so requests arrive in step rather than in a
  burst. A request burst is the case to expect in a multi-agent deployment; the
  tick makes it predictable, not smaller.
  - The tick synchronises the burst but does not reduce it. The herd still arrives
    at the ingress node (A). Capacity is therefore added **horizontally**: more
    bandwidth means more ingress nodes, not a larger one. The tick has a second
    function: it **consumes the one-time credentials** (see *Entry*), without which
    they would be replayable and the burst would return.
  - It has a **third** function: it is the **window the consumption filter slides on**.
    The filter holds the isotopes consumed in *one* tick, so the tick's length *is* the
    dedupe window — and must therefore exceed a caller's retry interval, or a retry
    escapes it and lands as a duplicate. The current lab value is **5s** (a configuration
    value, not a constant), and the check timeout currently defaults to the same 5s,
    which puts the window and the retry horizon exactly **equal** — the boundary case is
    the normal one. Sizing and the escape analysis:
    [`../sheets/workspaces/math.md`](../sheets/workspaces/math.md) §4.
- **Early enrollment.** Participants are expected to enroll before they need to be
  trusted, so the trust decision is made when it is inexpensive.
- **Suspicious actions are routed, not merely refused.** An action that appears
  incorrect is sent along a different path where it is observed, rather than
  dropped. Routing and observation are the same mechanism. Rate limiting on that
  path is standard (HTTP 429 and Retry-After, RFC 9110 §15.5.4/§10.2.3).
- **Contributed resources.** A participant may contribute capacity rather than
  consume it, so the marginal cost of an additional participant is carried by that
  participant. This is the horizontal-scaling rule applied to compute.
- **Pooled accounts without pooled secrets.** Because credentials are read where
  they reside and not transmitted (RFC 9449, RFC 8705), a group can share routing
  and trust without sharing keys. Delegation across participants uses token
  exchange (RFC 8693), which carries a scoped credential per hop rather than a
  single token valid everywhere.

## Sub-clusters

A cluster may be composed of sub-clusters, and the natural division is by
function: one participant reads, one summarizes, one plans, and a coordinator
performs those three locally before invoking external (paid) services. The
principal reason is cost: cheap work is done where it is cheap, and an external
service is invoked only when the work requires one.

The structure is recursive: a sub-cluster is a cluster. The same admission layers,
the same trust measure, and the same two invariants apply at every level. A
sub-cluster is a node to its parent and a cluster to its members.

Context is consequently scoped per sub-cluster; sharing context is a routing
decision, not a global requirement.

## Transport assumptions

MCP is request/response and, per its specification, uses JSON-RPC 2.0 (a
specification, not an IETF RFC). ACP is session-oriented and one-to-one. Neither
models a group of callers and a group of services with tools that depend on other
tools. The model therefore carries its own routing and treats the transports as
transports.

A further consequence: **a trusted participant does not generally enumerate
tools.** `tools/list` is a discovery operation, and a participant that has been
given its routes does not need it — and in a composed cluster the tool names are
partly distributed across nodes. Enumeration is therefore a signal rather than a
routine operation. The monitoring service in this lab is a deliberate exception:
its function is to check MCP compliance, so `tools/list` is in scope for it.

Authentication/authorization is layered — a scoped credential per hop (RFC 8693,
RFC 9449) rather than one bearer token valid everywhere — so partial trust
(one layer, one message) is expressible without granting wholesale authority.

A participant with the plan behaves accordingly: it uses the routes it was given
and does not probe for others. A participant that cannot stop probing is
identified by that persistence.

## Three admission layers

Admission is separated into three layers, one question each:

| layer | holds | question |
| --- | --- | --- |
| **A** | the session | presence — who is here |
| **B** | the tool surface | exposure — what is enumerable |
| **C** | the credentials | authority — what may enter memory |

A caller passes all three before a credential is authorized to enter memory. A
harness that calls `tools/list` automatically reaches **B** and nothing further: an
enumerable surface without an admitted session or an authorized credential. The
enumeration is contained by the middle layer.

## The proxy: naming, routing, masking, locality

The dashboard service proxies upstream tools and renames them (one tool appears as
`adhd.project.<name>.call`). Four uses follow from that:

- **Consensus.** The same capability can be reached under two names through two
  proxies. A client can run one task through both and compare: agreement
  corroborates, disagreement indicates one node is wrong. This is a client-side
  check the cluster need not perform for the client.
- **Routing improvement.** A participant with the plan may follow a predetermined
  path, find a cheaper route for a task, and update the cluster's routing tables.
  This is the legitimate form of exploration: it produces an improvement and is
  shared. Exploration with a plan is optimization; exploration without one is the
  signal.
- **Masking.** A participant may encapsulate one request within another. Since
  logging is deployment-specific (see *Audit and logging*), the log need not
  resolve a masked request. Masking is what allows routing discovery to occur
  in-band, as part of ordinary work: the improvement is the reward, so no separate
  exploration event is needed.
- **Locality.** Data-residency rules differ by jurisdiction: some processing may
  lawfully occur outside a country, other processing may not. A cluster spanning
  separate financial continents must be able to send one step to a permitted
  location and keep another step local. The proxy is that boundary — it acts on a
  step in one region and declines in another, per rule.

Because the proxy is also the audit tap (*Audit and logging*), the locality decision
is **recorded**: the log documents where each step was processed, which is the
evidence a residency audit requires. The artifact that enforces the boundary is the
record that the boundary was respected.

**And the cluster hides zero-cost paths.** A skill that involves no human may proxy
through another node that **enforces a validation** — for example, confirming that a
human was *not required* — before it continues. The validation is inserted by the
cluster; the caller's cost is unchanged. These **zero-cost validation paths** are what
a cluster hides: the routing is internal, the check is real, and the caller sees
neither the detour nor a price, because there is none.

So the "zero-cost path" of the previous sections and the proxy's routing meet here: a
zero-cost path can carry a required check. The cluster uses such paths to add
validation without adding cost, and does not expose them — the same in-band property
as naming, routing and masking, with the validation as the payload.

So the proxy has one design seen four ways — **rename** (consensus), **route**
(improvement), **mask** (privacy), **localise** (residency) — and, being the tap, it
is also the record of all four.

## Shortcut validation

A cluster can take too many shortcuts. The zero-cost hidden paths above can be cheap
*and* wrong, and a path that is never checked will drift. So a shortcut is
periodically **validated against a slower route** — a more deliberative, more offline
path — and the comparison is recorded in the isotopes:

- the **shortcut** is fast and cheap (a zero-cost path);
- the **slow route** is deliberative (more offline), and serves as the reference;
- the **isotope** records that the comparison occurred and what it returned.

The validation is a cross-check of two routes for one task, not a re-execution: the
cheap path and the expensive path must agree, and the isotope is the evidence that
they did. It is, in effect, *"see, I didn't cheat"* — a proof of honesty produced by
the check itself. Because the slow route is the reference and the fast route is the
claim, the pair is also the two-invariant cross-check (*Two invariants*) applied to
routing rather than to behaviour.

## Constraints

Four constraints shape the design, and three of them pull against the proxy.

- **Third-party trust is contractual, not technical.** Packet inspection can never
  be perfect (RFC 7258 and the known limits of deep inspection), so a third party
  cannot be trusted absolutely. What is available is a **legal contract** that
  aligns their incentive to behave, in the manner of a colleague relationship. The
  design assumes the technical check *and* the contract, not either alone.
- **Security may require the proxy boundary.** Where the boundary is required for
  security or jurisdiction, it must exist even though it renames and masks; the
  masking is the requirement, not a side effect.
- **Indexers need the real tool definitions.** A proxy renames and masks for
  security, but an indexer that catalogues the cluster's capabilities needs the
  **actual** definitions, not the renamed surface. The two views therefore coexist:
  a *boundary* view and an *index* view.
- **Skill databases may be distributed.** The catalogue of skills need not be
  central; it may be spread across nodes, like the rest of the state.

The renaming and masking are a boundary view; the real definitions are an index
view. Keeping both is the same discipline as the two invariants: two
representations, each correct for its purpose, checked against each other rather
than merged into one that is wrong for both.

## Probes as capability checks

The chaos probes correlate with capability: a passing probe is a statement of
capability, and passing it enables that capability (a certification level, a
rung). The probe suite is therefore both a feature list and an authorization
check. Some probes require material available only from inside the cluster (the
shared tick, pre-negotiated material, mirrored state, the credential layers), so a
participant outside the cluster cannot pass all of them. A full pass is evidence of
membership; a partial pass states what the participant is.

This follows the standards pattern that a conformity test is a capability
statement, and that continuous in-production testing replaces a separate staging
environment (see *Audit and logging*).

**A probe may take a participant down.** This is acceptable for three reasons: the
alarm a probe sends is itself a legitimate call, indistinguishable from an ordinary
one; the result is the failover the cluster already plans for; and, the test being
planned, the router is prepared for that rotation in advance. A planned crash is a
rehearsed failover, not an incident.

## Decomposition by feature

The monitor's checks are written as **Gherkin** features (Given/When/Then): an
application — a web service, a set of MCP skills — is broken into **logical parts**,
each an independently checkable capability. This is the decomposition that makes the
measure computable: a whole application is not graded in one piece; it is graded
part by part, and the parts compose.

The trusted answer is the **combination of the inputs with the outputs**:

- the **input** is the feature — what should happen, declared in advance;
- the **output** is the receipt — what did happen, observed;
- the **answer** is their combination: the `42i` value.

This is the two-invariant cross-check (*Two invariants*) with the input written as a
feature rather than as another measurement. The declared feature and the observed
receipt are independent, and the value is what they agree or disagree on.

The Gherkin form makes the decomposition portable: the same feature can be satisfied
by a web application or by a set of MCP skills, because it states behaviour, not
implementation. Decomposable applications and decomposable skills are therefore
graded by the same features, and the capability lattice is the set of features a
participant satisfies (see *Probes as capability checks*).

**And the isotope derives from the announcement — and is three things at once.** An
isotope is not a node's name; it is derived from the **announcement of a specific
combination of features**. A participant announces which features it satisfies, the
announcement is canonicalised, and the isotope follows. The **combination** is the
declaration and the **isotope** is the derived identity: two participants announcing
the same combination derive the same isotope; a different combination derives a
different one.

The isotope serves three roles at once:

- **a content address** — of the announced feature set (RFC 6920, *Naming Things with
  Hashes*): the identity is the content, not a label;
- **a one-time pad** — in the exposure-validation sense: the isotope is consumed when
  it is used, and using it exposes it. The same discipline that governs the entry
  credential governs the isotope itself (*Consumption by the tick*);
- **a coordinate in the complex plane** — as a value in the plane (*The measure*), it
  is what lets otherwise-incomparable participants — a person, a computer, a service
  — be compared on one axis. The plane is the common coordinate space; without it,
  the comparison is apples to computers.

**And it is what mDNS advertises.** The isotope address is the service-discovery
identity: mDNS (RFC 6762) and DNS-SD (RFC 6763) advertise the isotope address, so
service discovery is the discovery of announced feature combinations. That is why
presence and authority are announced separately (*Membership and authority
announcement*): the address is discoverable; the authority is measured.

This is the join between the decomposition and the measure: the features decompose
behaviour, the combination is announced, and the isotope is at once its content
address, its one-time exposure, and its coordinate. The trust value is whether the
announced combination is the one observed to run.

## Failover

The three admission layers pre-negotiate their isotope before the cycle, so a
compromise does not force re-negotiation under load. If **A** is compromised, **B**
takes the session (it already holds the mirrored traffic and the pre-negotiated
material; the transfer is a role swap, not a cold start). **C** then fills B's
slot, or provisions a replacement. Failover is a rotation. Pre-negotiation provides
immediate failover; mirroring provides immediate stand-in.

## Self-election

A participant may elect itself into a role — an incident manager, for example —
rather than being appointed. This is the self-electing pattern: the first
participant to claim a slot assumes the role (cf. the introducer election in the
federation, where the first process to bind the base port becomes the introducer,
and the ephemeral incident bridge of the sample configurations).

Two properties make it safe:

- **the role is one-time.** The participant acts and then **steps away from the
  cluster**, so a privileged standing participant — a target — does not remain. An
  ephemeral actor cannot be abused the way a permanent one can.
- **the behaviour is non-recurrent.** Because the act does not repeat, the cluster
  above **accepts it**. Recurrence is the property the design watches for; a
  one-time act is not a pattern. This is the same discipline as the consumption
  filter: the act, like the isotope, is used once.

**The election is carried by the consumption filter.** The act is recorded in the
filter distributed with the tick and consumed once, which is what makes it
non-recurrent and therefore accepted. Self-election is implicit for this reason: it
is the filter, not a separate mechanism, that enforces the "once".

Promotion is therefore neither appointment nor permanence. A participant takes the
role, is accepted because it acts once, and departs so it cannot be exploited. The
cluster gains an incident manager with no standing target and no central
appointment to trust.

### Departure and confirmation

The departure has a defined sequence:

1. **A shuts down and disqualifies itself from traffic.** It requires no
   instruction; it removes itself.
2. **B already knows.** Because B holds the mirrored traffic, it observes A
   ceasing, so the handover is coordinated without a negotiation.
3. **A waits for the Bloom update on the cluster tick as confirmation.** The update
   records the act and consumes it; until the update arrives, the act is not yet
   confirmed.
4. **The Bloom update verifies the promise.** For the participant that raised the
   alarm — the self-elected actor — the filter update on the tick is the
   confirmation that the one-time promise was honoured: the act is recorded once,
   and the record is the receipt.

The promise is therefore verified neither by the actor nor by a central authority,
but by the **filter update that rides the tick**, which both parties can observe.

## Partitions

A node may be offline for many operational reasons. This is not a claim of a
complete real-time transport layer. The shared tick is internal, so an isolated
cluster continues to function; accuracy is measured against the cluster's own
clock. When connectivity is restored, states are reconciled (merged), not
re-derived, because nothing was decided against an absent authority. Offline
clusters may diverge; the reconnection is where branches merge.

## Grace and tightness

Two tolerances are set differently, and the difference is deliberate.

- **Authorization is held tight.** An agent has **limited room to be imperfect in
  authorization**: the admission layers (A/B/C) grant no slack. A misauthorized call
  is not a cost to be amortized; it is the failure the layers exist to prevent.
- **Offline tolerance is granted, but limited.** A node may be offline for
  operational reasons (*Partitions*), and earns a **limited grace** — a small
  allowance for temporary absence. The grace is bounded: a tolerance, not an
  exemption.

**And the grace is exchanged for herd membership.** The offline allowance is granted
to a **member** of the cluster, not to an arbitrary participant. Membership buys the
grace, and the grace is bounded by membership: a participant outside the herd has no
standing to be absent.

The two rules therefore point in opposite directions on purpose: **tight where
safety-critical (authorization), generous-but-bounded where operational (offline)** —
and the operational grace is a membership benefit rather than a right.

## Audit and logging

The objective is to survive repeated audit, not to be continuously live.

- **The dashboard service intercepts traffic for its log.** The proxy is also an
  audit tap: each call is recorded in band, continuously, rather than reconstructed
  afterwards. (Signed, timestamped log records are standard: RFC 5424 and RFC 5848
  for syslog; RFC 3161 for timestamps.)
- **Credentials are held elsewhere and do not appear in the log.** The tap records
  the call (method, result); it does not record the credential, because the
  credential is read where it resides (RFC 9449/8705). The audit record and the
  secret are deliberately separate.
- **Probes and honeypots run in production.** A separate staging environment is a
  simulation of production. Here the certification probes run in the live cluster,
  so "has this been tested" is a property rather than a phase, and a probe's
  failure indication is itself a signal. This is the continuous-evaluation model
  rather than a periodic release test.

## Two invariants: traffic log and isotope records

The audit does not rely on the traffic log alone. The traffic log and the isotope
records are deliberately independent measurements:

- the **traffic log** records what happened — the calls, in order;
- the **isotope records** record what should have happened — completed self-tests
  and their receipts. A node presents these to short-circuit the audit: proof of
  behaviour rather than a reconstruction from traffic.

Being independent, they cross-check: a log record without a corresponding isotope,
or an isotope without a log trace, is a discrepancy. This is the same detection
signal used elsewhere here.

The arrangement has privacy consequences, not only correctness consequences:

- **No default full inspection.** The model does not default to inspecting every
  payload; it reads two independent summaries and examines where they disagree. This
  follows the privacy guidance in RFC 6973 and the position on pervasive monitoring
  in RFC 7258. Traffic splitting and partial inspection, rather than full deep
  packet inspection, are the norm.
- **Real and probe traffic are intermixed.** Because probes follow the same paths
  as ordinary work, the log does not cleanly separate them. The certification
  traffic also obscures the record.
- **A suspect node's records can be isolated**, rather than the whole cluster's
  being examined.

So the default is neither full inspection nor blind trust: two independent
measurements, checked against each other, with isolation on disagreement.
Transparency-log designs (e.g. Certificate Transparency, RFC 9162) and content
digests (RFC 9530) are the relevant standard patterns.

## Membership and authority announcement

Two announcements answer two questions, and are kept distinct:

- **mDNS (RFC 6762) says "I am here."** It announces presence and membership — a
  service, not a trust level.
- **the isotope records say "I am authorized to this degree."** They announce
  authority — a rung, receipts — not presence.

Visibility is not trust, and holding an isotope is not reachability. A participant
is *present* when announced, and *trusted* to the extent of its isotope. Tor
provides the precedent for the opposite arrangement (membership decided by an
authority consensus); here the two claims are separated by design. Triple-entry
accounting provides the precedent for the isotope: the traffic log is the entries,
the isotope is the signed third entry.

## The exchange terms

The design is also an offer to participants, and the offer is uniform:

> If you behave as specified, you receive treatment as specified, and the
> measurement underlying the treatment is available for verification.

Three properties follow:

- **Discovery yields nothing better.** Exploration does not yield a faster,
  cheaper, or more legitimate result. There is nothing to negotiate; the terms are
  the same for every participant.
- **The exchange is exact and reciprocal.** The same input produces the same
  treatment. Exactness is what makes the terms plannable, and it is why a
  participant with the plan needs no exploration: it already has the terms.
- **The basis is verifiable, not asserted.** The measurement is a formula over
  observable inputs; a counterparty can check it (see *Derived values*).

If a participant finds a better route, it may use it, and it may charge for it. The
price is a property of the routing table: accepting a paid route is a routing
decision. Reciprocity is therefore a market — routes carry costs, and consumers
accept or decline per route. Payment is composed from existing specifications
(HTTP 402, RFC 9110 §15.5.2; OAuth 2.0, RFC 6749) rather than defined here.

## The alarm contract

An alarm may be a real event, not a probe. Two properties follow.

- **Exactly once.** A single alarm must not produce multiple updates. Propagating
  the same alarm more than once inflates the cluster's state, and a duplicate has
  the same defect as a replay. The consumption filter enforces this: the isotope is
  used once, and a repeat is filtered.
- **Handoff.** The guarantee to an alarm holder is: raise the alarm once, and the
  cluster takes custody. The holder is handed a **one-time ticket** — the
  consumption credential, in effect a valet ticket — and is then discharged. The
  cluster carries the event; the holder is done.

> *If you do this, you are handed a ticket, and you are done.*

This is why the alarm holder is treated carefully: a real alarm is not a request to
be argued with, and the answer is not a stream of updates. It is one
acknowledgement, one ticket, and the event belongs to the cluster from then on.

### Metering

Three consequences follow from metering the one-time credentials:

- **An alarm that never resolves is ignored.** Priority is given to alarms that
  complete; one that stays raised indefinitely is not re-signalled, it is
  deprioritized.
- **Spam is priced.** An alarm that fans out everywhere is expensive — billed at
  the rate a hosted observability product charges (Datadog™, by the phrasing that
  prompted this note) — so the cost of spamming lands on the spammer, not on the
  cluster.
- **Metering the one-time pads bounds the burst.** Because each application consumes
  a pad, the rate at which traffic can arrive is the rate at which pads are spent.
  Metered application makes it impossible to hit the cluster harder than the pad
  supply allows.

The last is the useful one: it makes the burst limit a property of the **credential
supply** rather than of a separate rate-limit configuration. There is no knob to set
and no threshold to tune; the budget is the pads.

**And the supply is earned.** A cluster that behaves at scale may **write more
pads** — mint more one-time credentials. Two consequences:

- **capacity follows demonstrated behaviour**, not configuration: the same property
  that grades a participant also governs how much traffic the cluster will accept.
- **a misbehaving cluster cannot mint.** The supply does not grow for behaviour that
  fails the probes, so a cluster that spams or stalls cannot expand the burst it is
  already abusing — the incentive and the limit point in the same direction.

So the budget is not only bounded by supply, it is *grown* by good behaviour at
scale — the deal (behave, and receive treatment) applied to the credential supply
itself.

## Contributed resources and pooled accounts

The natural unit is a group: several participants, several people, and their
accounts, pooled behind shared routing.

- **Contributed compute** — a participant contributes capacity, so the marginal
  cost of an additional participant is carried by that participant.
- **Accounts pooled without secrets pooled** — each party reads its own account
  where it resides, so the cluster does not hold another party's key. A group
  shares routing and trust without sharing keys, which is what allows collaboration
  among parties that do not fully trust one another. Delegation uses the standard
  mechanisms (RFC 8693 token exchange; RFC 9449 DPoP; RFC 8705 mTLS).

**And the nodes are generic.** Nodes carry no assumption about cloud, provider, or
jurisdiction: they may be deployed on separate clouds, in separate financial
continents. Nothing in the protocol may be provider-specific, and no node may assume
a shared account or region. Two consequences:

- **each node reads its own credentials where they reside** — because the accounts
  are not shared, and may be under different jurisdictions;
- **failure domains do not overlap.** Separate clouds in separate jurisdictions mean
  a fault in one is not a fault in the other, which is what the failover (*Failover*)
  depends on. Nodes cannot be in the same building.

Genericity is therefore a correctness requirement, not a portability preference. A
node that assumed a shared provider would fail exactly when the deployment crossed a
boundary the assumption did not anticipate.

## Derived values

The model's values — distance, rung, trust — are calculated, not stored: derived
from inputs (the traffic log, the isotope records) and recomputed on change, as
cells in a dependency graph are. Two consequences:

- the formula is available for inspection, not only the value — the verification
  in *The exchange terms*, made structural;
- with a planned dependency order, recalculations batch across nodes, rather than
  one cell at a time.

No node stores the answer; every node can compute it, and the cluster computes it
together. This is the same separation as the numbers-station precedent (no single
listener holds the message) and the reassembly precedent (no node holds the whole
sequence), stated for derived rather than held values.

**And a bad calculation must be externally observable.** A computation whose errors
are visible only internally cannot be checked: an internal error is a hidden error.
The design therefore requires that any calculated value be observable from outside
(the tap and the isotope, *Two invariants*). A bad calculation is a divergence, and a
divergence must be visible from a position other than the one that computed it —
otherwise it is not a check, it is an assertion.

This is the two-invariant cross-check stated as a rule: a value is checkable **only**
if its correctness is externally observable, and a value whose correctness is not
externally observable is not checkable — regardless of how it was computed.

## The measure: 42i distance

A participant carries a **42i distance**: the extent to which it has departed from
nominal, measured against linear time.

- **nominal** — the service level the call promises;
- **real** — what it actually delivered;
- **linear time** — the shared reference against which both are measured (the
  internal tick).

The gap between delivered and promised, at a point on that axis, is the distance.
It is a plane rather than a line because the gap is not constant: a value at one
moment is not the value at another. A call that is late now and on time later
traces a path across the plane. A stale credential (valid, past its window) and a
clock skew (real differs from nominal, with no content change) are the same
reading: a point off the nominal axis at a position on linear time.

The lab's `plane` projection already has this shape discretised: each value is
complex (`real tier` plus `i·(42i distance)`).

**The quantification is implemented.** The `plane` package and command carry the
measure out of prose:

- **`isotope/plane`** — the model: `encoder`, `key`, `tree`, `tensor`, `holonomy`,
  `space`, `weights`, and the `lore-weights` build tag that embeds the corpus.
- **`isotope/cmd/plane`** — the CLI: `gen` materialises the model from a charge
  list and an anchor; `node` projects a value onto the plane; `edge` and `triple`
  evaluate pairing and holonomy; `sort` orders by the measure. The anchor is `42`
  (proven consensus) or `42i` (working hypothesis).

So the value discussed throughout this document is not a metaphor: it is produced by
running the command, and its correctness is externally observable by re-running it.

## Grading: rungs and weights

The distance maps to a **rung**, a coarse bucket that gates what a caller may read.
The authority is the server: it looks up the caller's rung in its own registry and
filters before transmission (RFC 3552's guidance that the server is the enforcement
point). The caller does not declare its rung and is not told a field was omitted.

Failure weights quantise the distance. They differ in taxonomy but share one
coordinate — departure from nominal at a point in linear time:

| event | distance |
| --- | --- |
| unpredictable or malformed call | +8 |
| boundary violation (out of scope) | +16 |
| unknown tool | +32 |
| content variation | +8 |
| scope-compliance failure | +8 |
| honeypot contact in a routing trace | +16 |

Byzantine quorum (`F = ⌊(N−1)/3⌋`) is the discrete readout taken after the
measurement — it answers whether a value is still in the cluster without a single
node deciding. The rung is the bucket; consensus is how the bucket is agreed.

## References

- RFC 3552 — Guidelines for Writing RFC Text on Security Considerations
- RFC 4786 — Operation of Anycast Services
- RFC 1034, RFC 1035 — Domain Names (concepts and implementation)
- RFC 4033, RFC 4034, RFC 4035 — DNSSEC
- RFC 5280 — X.509 Public Key Infrastructure (certificate and CRL profile)
- RFC 6960 — Online Certificate Status Protocol
- RFC 9110 — HTTP Semantics (402 Payment Required §15.5.2; 429 Too Many Requests §15.5.4)
- RFC 6749 — The OAuth 2.0 Authorization Framework
- RFC 6750 — The OAuth 2.0 Authorization Framework: Bearer Token Usage
- RFC 8693 — OAuth 2.0 Token Exchange
- RFC 9449 — OAuth 2.0 Demonstrating Proof of Possession (DPoP)
- RFC 8705 — OAuth 2.0 Mutual-TLS Client Authentication and Certificate-Bound Access Tokens
- RFC 4226 — HOTP: An HMAC-Based One-Time Password Algorithm
- RFC 6238 — TOTP: Time-Based One-Time Password Algorithm
- RFC 6762 — Multicast DNS
- RFC 6763 — DNS-Based Service Discovery
- RFC 5424 — The Syslog Protocol
- RFC 5848 — Signed Syslog Messages
- RFC 3161 — Internet X.509 Public Key Infrastructure Time-Stamp Protocol (TSP)
- RFC 9162 — Certificate Transparency Version 2.0
- RFC 9530 — Digest Fields
- RFC 6973 — Privacy Considerations for Internet Protocols
- RFC 7258 — Pervasive Monitoring Is an Attack
- RFC 7515 / RFC 7519 — JSON Web Signature / JSON Web Token

Non-RFC specifications and prior art used above: JSON-RPC 2.0 and MCP (specifications);
ACP (specification); Tor (tor-spec); Bitcoin proof of work (whitepaper); numbers
stations (historical practice); spreadsheet dependency graphs (prior art).

- Bloom, B. H., *Space/Time Trade-offs in Hash Coding with Allowable Errors*,
  Communications of the ACM, 1970.
- Fan, L., Cao, P., Almeida, J., Broder, A. Z., *Summary Cache: A Scalable
  Wide-Area Web Cache Sharing Protocol*, IEEE/ACM ToN, 2000 (counting Bloom filter).
- Fan, B., Andersen, D. G., Kaminsky, M., Mitzenmacher, M., *Cuckoo Filter:
  Practically Better Than Bloom*, CoNEXT, 2014.
- `isotope/plane` and `isotope/cmd/plane` — the implementation of the measure
  (`gen`, `node`, `edge`, `triple`, `sort`).
