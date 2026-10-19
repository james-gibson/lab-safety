# Project Chimera: a decentralised knowledge and accountability framework

**Captured from a design conversation that was never written down.** The summary
existed only in a chat export; this file is that summary, plus the scalability
material the conversation spent most of its time on, plus the notes a reviewer needs
to understand what will and will not hold.

## Abstract

Chimera is a local-first framework for collaborative knowledge creation under
conditions of uncertainty and risk, between parties who may not fully trust each
other. It is not a database and not a file-sharing network. It is an **evidence
engine**: its goal is not to prevent unauthorised sharing — impossible once a party
has read something — but to create an immutable, verifiable record of who knew what,
when, and what they did with it.

## Core principles

1. **History is immutable.** An edit is a new event that links to the past; it never
   overwrites. The store is an append-only causal log, in the manner of Git.
2. **Access is a revocable privilege.** It is granted, not inherent. Revoking it does
   not alter the historical fact that access once existed.
3. **The client is the source of truth.** The network is untrusted. Canonicalisation,
   conflict resolution and decryption all happen on the client.
4. **Context is king.** Data without its web of relationships, disagreements and
   perspectives is meaningless.
5. **The network is a witness, not a judge.** It stores and serves signed, encrypted
   blobs with fidelity. It does not interpret, arbitrate or decide legality.

## Architecture

- **Client** — local-first; holds the private keys and the local database (SQLite is
  sufficient); performs every cryptographic operation and all conflict resolution.
- **Network** — untrusted pinning nodes plus a DHT for discovery. Zero knowledge of
  content.
- **Co-operative** — a federated trusted entity managing PKI, anti-freeloading policy
  and a baseline web of trust that blunts Sybil attacks.

## The atomic unit: the signed record

### Identifier grammar

A **universe id** is the atomic unit. It is a single string, self-describing and
recursive:

```
🦄☁️.<sourceString>:<traceString>:<ksuid>

sourceString = reverse(base64(universeId))     -- a universe id, encoded
traceString  = a sourceString, or a uuidv7
```

- **prefix** `🦄☁️.` marks the value as a universe id, so a parser knows before it
  reads further. Nothing else in the system carries it.
- **sourceString** — the author. It is literally another universe id, base64-encoded
  and then reversed. Reversal is what pushes the base64 padding to the front, which
  is the marker a reader keys on (`==…` at the start means "decode me"). Recursion
  terminates at a **genesis** id, the application's root, which every service in one
  application shares.
- **traceString** — the span root. It is either a source string (a trace nested in
  another universe) or a bare `uuidv7`.
- **ksuid** — 27 chars of base62: 32-bit seconds since 2014-05-13T16:53:30Z plus a
  128-bit random payload. This field is what makes the whole string sortable.

Two derived forms matter:

- **Merge.** Referencing two upstream ids chains source strings with `:` —
  `<srcA>:<srcB>:<trace>:<ksuid>`. That is the commit-chain shape: the current source
  followed by the sources it incorporates, in one addressable string.
- **Re-stamp.** Replacing the ksuid yields a new universe id with the same provenance
  and a later position in sort order. Nothing else changes, which is what makes the
  id cheap to re-emit.

**Type safety is a requirement, not a nicety.** `Source`, `Trace` and `Uuid` are
distinct types: a function taking a `Source` must not accept a raw `Uuid`, because
they are not interchangeable in any position of the string. External validation on
input is acceptable; internal ambiguity is not.

The genesis id from a working run, for reference:

```
🦄☁️.ZG9udC1wYW5pYy1hbmQtYWx3YXlzLWtub3ctd2hlcmUteW91ci10b3dlbA==:
     0198b05b-3777-763a-b44b-c9d6da1c8c4f:31LZgUnARElDWIV7nAflfILswPl
```

Three structural rules carry the design:

- **Composite ids are identified by construction.** The prefix announces the type, the
  reversed padding announces that a source needs decoding, and the trailing ksuid is
  always the last field. A reader never has to guess.
- **A genesis source id exists.** Every application registers a root; any id whose
  parent is genesis is a root-level application id. Multiple services comprising one
  application share that genesis and carry distinct source ids. This is the recursive
  escape clause.
- **Sorting is a property of the tail, not the head.** Universe ids sort by ksuid, so
  events produced concurrently across nodes interleave correctly once sorted — which
  is why the debug path issues timestamps through `Promise.all` and leaves the overlap
  invisible until the sort reveals it.

### Payload

```json
{
  "recordId": "historian_jones:acme_corp_fraud:2a7m4o1Z6B3V5n7q9r1s2t3u4v5w6x7y8",
  "eventType": "NOTE_CREATED",
  "parent": "historian_jones:acme_corp_fraud:2a7l...",
  "citations": ["sec_filings:public:acme_corp_fraud:2a7k..."],
  "proofOfDecryption": {
    "citedRecordId": "theorist_x:acme_corp_fraud:2a7j...",
    "hashOfContent": "sha256-a1b2c3..."
  },
  "content": {
    "encryptedBlob": "...",
    "encryptionMetadata": {
      "policyId": "theorist_x:policy:research_group_v1:2a7i...",
      "wrappedCEK": "..."
    }
  }
}
```

## Key mechanisms

### Confidentiality: encrypt for a policy, not a person

Data is bound to a **policy record**, not to individuals. The policy owns a keypair;
content is encrypted with the policy's public key; the policy's private key is
"wrapped" (encrypted) once per member with that member's public key.

A theorist shares a note with Alice and Bob: create `research_group_v1` with its own
keypair, encrypt the note to the policy's public key, wrap the policy private key for
Alice and for Bob. Adding Bob later means adding one wrapped blob to a new policy
version — the original ciphertext is never touched. Removing Alice means a new policy
version without her, optionally with a fresh policy keypair for forward secrecy.

The consequence worth protecting: membership churn costs one small record, never a
re-encryption of the corpus, and the policy itself is a versioned, signed document
trail an auditor can walk.

### Auditability: proof of decryption

When Alice cites a private document, her client embeds a `proofOfDecryption`: a hash
of the plaintext she decrypted, inside her own signed record. That is non-repudiable
evidence that she could read the source at that moment.

Tuesday: Alice cites the theorist, embedding the hash. Wednesday: access is revoked.
Thursday: an auditor can verify that producing that hash required access, and can see
when the revocation happened. *Live access is gone; the fact of past access is not.*

This mechanism is why access can be revocable at all. Without a receipt, revocation
would also erase the record of legitimate use, and every historical citation would
become unprovable.

### Conflict resolution: the client decides

Different sources will hold incompatible versions of one event. The system does not
choose. It serves every valid signed version, and the client resolves them against a
local, user-configurable trust graph. The interface then shows a **delta view**: the
public canon as a baseline, with a preferred source's differences overlaid. The point
is to display *where* a user's preferred reality diverges from consensus, without
censoring either.

## Scalability: where this breaks, and what "good enough" means

This was the substance of the design review, and it is the part most likely to be
lost.

### The ID is a routing packet, not a string

The application parses the composite id into its parts *before* handing anything to
the database, materialising them as ordinary indexed columns. The ID remains portable,
self-describing and stateless; the columns are the physical form of the same
information. Both views, one fact — the same discipline as a boundary view and an
index view.

That choice is what makes the cost manageable, and it is also where the design can
fail.

### The failure boundary: an unknown leading component

A composite key is prefix-addressable. Everything works when you can start at
`sourceId`. The cost collapses when you cannot.

Concretely, at 100,000 events per second a five-second bucket holds ~500,000 ids. A
trace spans a handful of them. Query by `LIKE '%:<uuid>:%'` and the planner sees a
leading wildcard, cannot use the index, and scans the bucket — half a million
strings compared to return three rows. Query `WHERE trace_id = ?` against an indexed
column and it reads only those three.

The gap is orders of magnitude in I/O, not a percentage, and it appears exactly at the
scale where diagnosis matters most.

### The conflation to avoid

`traceId` and `sourceId` are different things and must be stored as such. The trace id
is minted once at the origin and never changes. The source id changes at every hop:
for service B, the source is the id of A's event, not the trace.

If the trace id is buried mid-string, reconstructing a trace is not one query but a
recursive traversal — find the root, take its KSUID, query that as a source, repeat.
That is slow, fragile, and unusable in an incident. With a real `trace_id` column it
is one ordered select.

### The good-enough threshold

"Good enough" here is a measurable statement, not a mood:

> The design is good enough when the **dominant** query patterns are prefix-addressable
> and the residue is bounded in cost at the target scale.

Cheap by construction: by source, by bucket, by KSUID range and order.
Expensive by construction: anything starting from a component you do not know —
cross-source trace reconstruction, substring search, alias resolution without a
starting source.

So the design is deliberately pushed *right up to that limit*: the id remains a single
portable string, parsing happens in the application, and the database is given the
structure it needs. What must not happen is discovering the limit during an incident.
The honest form of the claim is a cost curve with the scale written on it, not the
sentence "it works."

### Time, and why the bucket is deterministic

Buckets are `KSUID_timestamp % 5 seconds`, not creation time and not processing time.
That determinism is the point: an event that queues for fifteen seconds still hashes
to the same bucket a consumer computes independently, so discoverability does not
depend on when the work happened.

Consequences to keep:

- **Skew tolerance is under one bucket.** A service whose clock is off by more than the
  bucket width will compute a different bucket and its events will be missed.
- **Lookups cost a tick.** The broadcast/lookup protocol acknowledges on the next
  artificial tick, which can add up to one bucket of latency to a dependent step.
  CQRS or Raft were noted as possible mitigations.
- **The bucket is an alias, not a truth.** Buckets let unknown events be *found*;
  they do not establish order. Order comes from the causal chain.

## Security considerations and the analog loophole

The security boundary ends at the user's eyes. Once a party has legitimately
decrypted something, no technical control stops a screenshot or a retyping.

The purpose is therefore not to make leaks impossible but to make them **provably
attributable**: the original grant, the decryption receipt and the later publication
form an immutable chain that would be presented in a civil proceeding. Recourse is
social and legal; the system's job is to be the best possible witness.

## Notes for the next reviewer

- **The local cache is a firewall, not a bug.** Working locally on private data
  without republishing or re-keying it is what prevents a confused-deputy attack. The
  public record holds pointers and proofs, never another source's plaintext.
- **The co-op is a deliberate trade.** Pure P2P invites Sybil attacks and unusable key
  management. Federating trust sacrifices absolute decentralisation for practical
  security.
- **Key management is the critical failure point.** A co-op-managed backup and recovery
  path is foundational, not optional, if non-technical people are to use this at all.
- **The receipt is load-bearing.** If `proofOfDecryption` is dropped to save space, the
  system can no longer distinguish "never had access" from "had access, then lost it",
  and every historical citation becomes unprovable.
- **Do not let the id become the storage format.** The moment queries are written
  against the string rather than the parsed columns, the cost curve stops holding.

## Design unification

This is the same design family as the other documents in this directory, and the
shared principles are worth stating once:

| principle | here | elsewhere |
| --- | --- | --- |
| a declared value is information, not authority | the KSUID timestamp orders but does not establish sequence; the causal chain is ground truth | `fully indexed` is a self-report, not reachability ([`SYNAPSE.md`](SYNAPSE.md)) |
| the receipt outlives the access | `proofOfDecryption` survives revocation | the isotope survives the route ([`SPLASH.md`](SPLASH.md), [`PLAIN.md`](PLAIN.md)) |
| self-describing, self-contained records | composite ids parse in the app, index in the store | the plane reports phases a client can recompute |
| cost is bounded by pattern, not by wish | prefix-addressable queries are cheap; the rest is a scan | the index has a fixed per-query floor and one core of throughput |
| explicit non-claims | the network is a witness, not a judge | closed loops mean self-consistency, not correctness |

The temporal addressing built on these ids — a universe as a first-class node, an epoch
per universe, windows shared across precision scales, and the reality gradient — is
specified in [`TEMPORAL.md`](TEMPORAL.md), which carries four worked reference chains
as its own tests.

## Appendix: pseudo-types and formats

- **sourceId** — human-readable string representing a cryptographic identity,
  managed by co-op PKI (`user_alice`, `publisher_corp`).
- **traceId** — grouping identifier; CUSIP, ISBN, UUID or another application id.
- **ksuid** — 27-char base62: 32-bit seconds since 2014-05-13T16:53:30Z + 128-bit
  random. Lexicographically sortable.
- **Encryption transform** — content is encrypted with a random symmetric CEK
  (AES-256-GCM); the CEK is wrapped with the policy's public key (RSA-4096 or ECIES);
  the policy's private key is wrapped per member with each member's public key.
- **Timestamping** — authoritative order comes from `parent` and `citations`, never
  from the client-reported KSUID time. The KSUID time is evidence and a sort key.

<details>
<summary>Scratchpad — for a future model picking this up</summary>

Captured 2026-10-19 from a chat export titled *"Scalability Concerns for Application
Level IDs"* (104 messages). The conversation's own README draft is the backbone of this
file; the sections **Scalability**, **Time and why the bucket is deterministic**, and
**Design unification** were reconstructed from the discussion around it, because the
draft omitted exactly the material the review was about.

What was actually contested, in order: whether a single composite string plus a
time-bucket partition is sufficient; whether substring search inside a bucket is
acceptable; whether `traceId` and `sourceId` may be conflated; whether structured
columns are a different philosophy or the physical form of the same design; and how
clock skew interacts with a deterministic bucket derived from `KSUID_timestamp % 5`.

The one-line summary a reviewer should carry: **the ID is a routing packet, the columns
are the storage format, the causal chain is the clock, and every receipt exists because
the thing that produced it will be gone.**

Open questions worth keeping: the tick's effect on dependent latency (bounded by one
bucket); whether CQRS or Raft is needed for the lookup broadcast; skew detection and
what to do with a service that drifts past one bucket; and the alias graph — aliases
may link ids at *different* recursive levels, which was proposed but never stress
tested.

</details>
