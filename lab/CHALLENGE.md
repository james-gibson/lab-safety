# The challenge: a re-attestation is a run, not a token

**Status:** design sketch. **Audience:** whoever implements re-attestation, the prober,
or the consensus path. Related: [`SLA.md`](SLA.md) §4 and §12,
[`RECONCILE.md`](RECONCILE.md), [`TRIPWIRE.md`](TRIPWIRE.md), [`MATH.md`](MATH.md).

The lab's zero-trust rule is that **no node reports on itself**: a claim is checked by a
neighbour, and a disagreement is settled without a single node deciding. This page applies
that rule to the one place it is easiest to fake — a node claiming it has been fixed.

## The problem

Isotopes are **canonical by design**: the same Gherkin yields the same token
(`CanonicalizeGherkinToIsotope` → `iso-login-v1:a3f2b1c4d5e6f7a8`). That stability is what
makes a spec comparable and re-derivable — and it is exactly what makes a *passing* spec
replayable. An old token, presented again, is indistinguishable from a fresh certificate
unless something binds the run to the moment.

A waiting period does not fix this. Time is not an inspection; a decayed penalty says the
flap stopped, not that the fault is gone.

## Three objects, three jobs

| object | lifetime | job | forged or replayed means |
| --- | --- | --- | --- |
| **isotope** — canonical, signed Gherkin | stable | *what* is promised: the feature set and the refusals | a promise nobody can compare or re-derive |
| **challenge** — nonce, verifier, window, scope | one run | *when* it was proved, and to whom | an old pass certifies forever |
| **receipt** — signed `(isotope, challenge, result, slot, outcome)` | one attestation | *who* accepted it, and at which position on the tick | self-certification |

No one of the three is sufficient. The isotope without a challenge is replayable; a
challenge without an isotope tests nothing in particular; a receipt without a quorum is
the node's own word.

## The order is forced

```
declare  →  canonicalise  →  sign  →  challenge  →  run  →  result  →  quorum  →  attest
```

Each step constrains the next, and every reordering has a name:

- **challenge before the spec is fixed** — the prober can be steered to a subset that
  passes, so the certificate describes a promise nobody made;
- **run before the challenge** — the transcript cannot contain the nonce, so the result is
  reusable;
- **result before the run** — an assertion wearing a digest;
- **attest before the quorum** — the node reports on itself, which is the one thing the
  triad exists to prevent.

## What each binding defeats

| attack | defeated by |
| --- | --- |
| replay an old passing token | the challenge is fresh and single-use; the result is bound to it |
| pre-compute a pass | the challenge is issued after the spec is signed, and is unpredictable |
| swap the spec after the challenge | the result binds the isotope's canonical hash, not the text |
| substitute the moment (stale receipt) | absolute expiry plus the slot window; age is counted, not ignored |
| one verifier accepts a bribe | the quorum decides; see *Sizing* below |
| present a different run than the one verified | the transcript digest is signed by the executor and re-derived by the verifier |
| quiet partial pass | no receipt is issued for a partial run; an unissued receipt is silence, not success |

## Rules

1. **The challenge is never reused.** Not across a retry, not across a verifier's second
   opinion. A retry gets a new challenge; that is the cost of a retry being meaningful.
2. **It expires.** A challenge carries a window in slots. An expired run produces no
   receipt, even if it would have passed.
3. **It changes fixtures, not expectations.** The challenge may vary inputs, seeds, and
   order; it must not vary the expected outcomes. Otherwise the run becomes an oracle that
   reveals which behaviours are currently failing.
4. **The result is `H(isotope ‖ challenge ‖ transcript_digest ‖ outcome)`** — one value
   that is meaningless under any other challenge, and cheap to compare across verifiers.
5. **Divergence localises.** Same isotope, same challenge, different result means a bug or
   a lie. Different isotope means the claim changed; different challenge means the runs are
   not comparable at all. Do not collapse those into "disagreement".
6. **Silence is failure.** No receipt means no attestation. A node that cannot complete the
   run stays revoked; it is not "pending trust".

## Sizing

`F = ⌊(N−1)/3⌋`, so `N = 3` gives `F = 0` ([`MATH.md`](MATH.md)): the three-node
verification chain can certify only under an honest-majority assumption. Re-attestation is
a consensus event, not a single inspection — if the certificate must survive a dishonest
verifier, the quorum is larger than the triad. The electrician is plural.

## What this does not solve

- **What the transcript digest must cover.** Fixtures and observations, certainly. Whether
  it must also cover the interpreter build and the environment is undecided, and it changes
  how much of a run is reproducible elsewhere.
- **How the challenge reaches the runtime without becoming an input oracle.** A "challenge
  mode" that touches fixtures only is the intent; the boundary needs writing down per
  harness.
- **Where the commitments live.** The receipt is compact; the log that makes it auditable
  is not. The split between what a verifier signs and what a stranger may later re-derive
  is designed on the placement side (`lezz.go` `docs/attestation.md`, proposed) and not yet
  here.
- **Key custody for the verifiers.** A quorum of signers is only as good as the signing
  keys' separation; that constraint is not in this page.

## Summary

An isotope says *what*; a challenge says *when*; a quorum says *who*. Re-attestation is the
three together, run fresh, with the result bound to the moment. Anything less is a token
being mistaken for a certificate — which is the same class of error as a name being
mistaken for a heartbeat.
