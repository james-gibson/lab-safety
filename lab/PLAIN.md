# A plain reading

**For anyone who read the other documents and suspected the vocabulary was doing
the work. It isn't — or it doesn't have to. This page says the same thing in
ordinary engineering terms, names the two objections worth having, and states what
the design does not claim.**

Short version: there is a deterministic score over a set of things, and a loop
around three of them either closes or it doesn't. If it doesn't close, that is a
contradiction and we say so. Everything else is naming.

## Objection one: why not bare MCP?

Bare MCP is fine. It does transport: connect, list tools, call them, get JSON back.
Nothing here asks anyone to change a client, a transport or a protocol. The plane is
an **output format** — ordinary structured tool output. If it's wrong you can ignore
it and keep calling the tool exactly as before.

What bare MCP cannot do is tell you whether an answer is *right*. It has no place to
put the difference between:

- "these two answers match because they're both true", and
- "these two answers match because it's the same code path twice".

Both are byte-identical on the wire. So the only check a bare call supports is
liveness — did it respond, is it stable. That's worth having, and it's what traffic
mirroring buys. It is not correctness, and no amount of protocol work makes it so,
because the protocol never asked a question that could fail.

So the addition is the smallest thing that *can* fail: a quantity computed around a
closed loop. Two paths that agree on a loop integral have agreed about something
that could have disagreed. Two paths that agree on a scalar both computed the same
way have told you nothing new.

## Objection two: why waste compute duplicating calls?

Because duplication is not the expensive part, and because one witness cannot catch
a coherent forger.

Measured on one machine, 2026-10-19:

| what | cost |
| --- | --- |
| a proxied call vs a direct one | +2–3 ms (~2 %) |
| four identical calls concurrently vs one | same wall time (306 ms vs 306 ms) |
| eight concurrently | 344 ms |
| four scoped shards, serial → concurrent | 758 ms → 206 ms (3.7×) |

A duplicate read of a local service is a rounding error. Re-reading 3,000 files
costs 287 ms; the same query over 26 files costs 129 ms; a hundred results instead
of five costs 326 ms. The price of a lookup is mostly the price of *asking*.

The expensive duplication is the heterogeneous kind — two different systems
answering the same question — and that's the one this design is conservative about.
It belongs behind a synced copy, not in the hot path.

And the reason to pay anything at all: a single witness cannot distinguish a truthful
answer from a self-consistent lie. Agreement between *independent* observations can.
Agreement between an observation and itself cannot, which is why a mirror is a health
check and a cross-source check is a truth check.

## The words, and what they stand for

If the vocabulary is the objection, replace it with the right-hand column. The left
column is shorter and has tests.

| word | in the code | plain meaning |
| --- | --- | --- |
| gauge | `Tree.TransportTo`, rotation of ζ | a common rotation applied to every coordinate. Distances and loop failures do not move. |
| anchor | `--anchor`, `Space.Anchor` | the origin coordinates are measured from (`z[j] − anchor`). Changes every number; anchors are never mixed. |
| charge | `ParseCharges` | how fast a plane's angle moves when the whole system is rotated |
| transport | `Tree.Transport` | expressing one plane's angle in another plane's frame |
| distance | `Tree.Distance` | abs(transported − actual); the primary ordering, invariant under rotation |
| holonomy | `CycleHolonomy` | walk a loop, come back at a different angle |
| frustration | `TripleKey.Frustrated` | the loop didn't close — a contradiction |
| quarantine | `TripleKey.Quarantined` | don't use this triple |
| tumble | rotation by 2πk/5 | rotate every plane at once; washes the reference, preserves loops |

What a third party can actually check:

1. **Same anchor, same input, same bytes.** The output is byte-deterministic and
   there are tests that fail if it stops being so (`TestCLIByteDeterminism`,
   `TestCLIGenAndReload`). Reproduction needs no trust.
2. **The computation is closed-form from the reported coordinates.** Transport,
   distance and the loop product are arithmetic over the published ζ. Anyone can
   redo it.

And one thing a third party *cannot* do, stated plainly because it is the mistake
this document exists to prevent: **the anchor is not a gauge.** It is an origin,
subtracted from every coordinate (`zeta[j] = z[j] − anchor`), so changing it changes
the numbers. Same triple, same binary:

| anchor | distance | holonomy |
| --- | --- | --- |
| `42` | 243.7868 | −1.4357 |
| `42i` | 87.0879 | 0.3452 |

A client with a different anchor will not reproduce our answer, and should not
expect to. Anchors are never mixed inside a sort for that reason. What *is* invariant
is a common rotation of all coordinates at once — that is the gauge, and it is a
narrower thing than "pick a different anchor".

Whether the *decision* survives an anchor change is a separate question, and we do not
have the evidence to answer it. On a generated four-plane model, all four triples were
frustrated under both anchors — so the sample cannot distinguish "stable" from
"frustrated everywhere". That is the first experiment worth running on a model where
some triples close and some do not; until then, do not assume quarantine is
anchor-independent.

## Why "tumble" is not a metaphor

A route is a coordinate. Rotating every plane at once leaves every loop integral
unchanged and destroys everything that identified the route. That is the whole reason
an event can be recorded after the thing that observed it is gone: what was kept is a
curvature, not a coordinate.

It is also why the rotation is the right primitive. The tick period is five seconds
by policy; the plane's own symmetry is a common rotation of every coordinate, with a
per-plane charge `q` deciding how fast each one moves and how many distinct phases it
can take. The finite, enumerable state space comes from `q`, not from the number
five — but both make the same claim: the system is invariant under a rotation, and
the invariant is the thing worth keeping. "We exercised every tumble" is a plan over
a finite group, not an aspiration.

And it separates the two kinds of odd behaviour without a judgement call: a noisy
client is *inside* the gauge — it can spin freely and change no invariant. To change
an answer you must break a loop, which is a different act and is reported. That's
what "they have to tip the scale" means concretely.

## What this does not claim

- **Not** that the code index is accurate. It reports 124,978 of 179,626 edges
  unresolved while calling itself fully indexed — see [`SYNAPSE.md`](SYNAPSE.md).
- **Not** that routes or addresses are trustworthy. The route is deliberately
  perishable.
- **Not** Byzantine tolerance. If every witness colludes, the loops close and the
  score is consistent. Nothing here fixes that; independence of sources does.
- **Not** speed. The service is effectively one core, and saturates somewhere
  between 25 and 50 requests a second depending on what else is running.
- **Not** a replacement for tests. A closed loop means the observation is
  self-consistent, not that the code does what the feature said.

## The smallest falsifiable claim

> If every triple the plan covers has a closing loop **at the declared anchor**, and
> the plan covers every triple that matters, then the observed order is the declared
> order. If a loop fails to close, we report it and refuse to use that triple.

The arithmetic half of that is cheap and tested. The load-bearing half is **"the plan
covers every triple that matters"** — and that clause is a coverage claim, which is
exactly the kind of claim this document elsewhere refuses to take on faith. It should
be read as the weak point, because it is.

## How to check us

- Run `plane node`, `plane edge`, `plane triple` yourself. Same input, same bytes.
- Recompute the order from the reported coordinates. Use your own anchor if you
  prefer; the distances and the loop failures won't move.
- Try to make a frustrated triple look flat without changing the reported
  coordinates. If you can, the claim is false and the code is wrong.

That last one is the point. It's arithmetic, and arithmetic is refutable by anyone
willing to do it.

## Related

- [`PLANE.md`](PLANE.md) — the mechanism document this page paraphrases. Where the two
  differ, `PLANE.md` is the definition; this page is the argument.
- [`SPLASH.md`](SPLASH.md) — the trust and proxy model this sits inside.
- [`SYNAPSE.md`](SYNAPSE.md) — the measured cost of the service the plane is run over.
- [`CHIMERA.md`](CHIMERA.md) — composite application ids, the good-enough query
  threshold, and why a receipt outlives the access it proves.
