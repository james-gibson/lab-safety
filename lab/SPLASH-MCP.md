# MCP vs Lab-safety(MCP)

**Same wire. A different set of exposed failure modes.**

This is the second splash page. [`SPLASH.md`](SPLASH.md) states the trust model; this
page compares **stock MCP** with **MCP under the lab** — the same protocol, plus the
cluster — and says which default failure modes each one leaves exposed.

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

| # | failure mode | stock MCP | Lab-safety(MCP) | mechanism | what remains |
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

## 6. What the client sees

| client | what it gets |
| --- | --- |
| **stock MCP client** | the specification: connect, enumerate, call. All fourteen rows as the spec leaves them. |
| **stock MCP client against a Lab-safety(MCP) surface** | the same, plus whatever it chooses to read: the refusals are in-range, the errors are specced, and the layer's guarantees are simply not exercised. **Graceful degradation, not a fork.** |
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
