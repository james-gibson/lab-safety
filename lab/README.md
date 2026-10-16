# The lab exercises — index and spine

This directory is a set of **exercises** (runnable) and **findings** (written
down) about the lab ecosystem: `tuner`, `smoke-alarm`, `adhd`, `isotope`, and the
vault that records them. Read this page first; it says what each file is for and,
more importantly, what the whole thing is *about*, so a later agent does not
re-derive it from scratch.

## Trust after connection

**A caller can connect to an MCP service; whether it is trusted after connecting is
a separate question, and one most transport specifications do not answer.** This lab
measures it. A caller carries a **42i distance** — the extent to which it has
departed from **nominal** (the service level a call promises) against **linear time**
(the shared tick) — and the distance maps to a **rung** that limits what the caller
may read. A caller that keeps departing is removed from the routeable set.

```
 nominal ●────────────  the level a call promises
          ╲
           ╲ distance     +8 / +16 / +32 …
            ●───────────  where it landed (linear time →)
```

One measure, regardless of source — a stale credential, a clock skew, a tampered
receipt, a honeypot contact. Model: `lab/PLANE.md`. Figures:
`lab/MATH.md`. Full page with references: `lab/SPLASH.md`.

## Read in this order

| file | kind | what it holds |
| --- | --- | --- |
| `README.md` | map | this page — the index and the one-paragraph spine |
| `SPLASH.md` | reusable | the post-connection trust summary, to place in a library README |
| `TRUST-FLOWS.md` | diagrams | the trust flows as pictures — admission, the triangle, the plane, failover, partitions |
| `MATH.md` | findings | the grounded figures, and which of the others are not grounded |
| `PLANE.md` | findings | the measure as a plane in local-time: delivered vs promised vs linear time |
| `THREADS.md` | findings | the whole arc — every thread, its state, its issue |
| `FLOW.md` + `exercise.sh` | exercise | tuner ⇄ smoke-alarm ⇄ adhd, end to end |
| `TRIPWIRE.md` + `tripwire.sh` | exercise | trusted caller vs probing caller; the honeypot |
| `RECONCILE.md` + `accounts.sh` | exercise | the verification chain and who enacts a disagreement |

`MATH.md`, `PLANE.md` and `THREADS.md` are the ones that save time. The exercises
are reproducible; the three findings documents are the *conclusions*, so an agent
can start from the answer rather than the search.

## Summary

The lab is a cluster of tools that monitor and verify each other. A monitor
(`smoke-alarm`) checks that an MCP endpoint behaves to spec; a dashboard (`adhd`)
renders the result; a broadcaster (`tuner`) supplies live channels; a shared library
(`isotope`) supplies the trust protocol — receipts, the 42i lattice, Byzantine
consensus. The design is a verification chain in which no node is trusted to report
on itself: a busy ingress node is verified by an idle sibling, that sibling is
verified in turn, and a disagreement is settled without a single node deciding. The
value that decides "is this node still trusted" is not a vote count; it is a
distance from nominal, measured against linear time. The same `42i` measure applies
whether the departure came from a stale credential, a clock skew, a tampered
receipt, or a honeypot contact. The other files here are instances of that.

## Use with a fresh agent

Provide a new agent this directory; have it read `MATH.md` and `PLANE.md` before
working with any figures. The failure this prevents: an agent starts from the code
(which contains placeholder, non-physical constants), derives from them, and does
not converge — because those figures are not grounded. See `MATH.md` → *Why earlier
agents recursed*.

## Exercises at a glance

```sh
./exercise.sh    # tuner + smoke-alarm + adhd round-trip
./tripwire.sh    # honeypot: trusted vs probing caller, observed by a second adhd
./accounts.sh    # ingress -> verifier -> monitor chain; who settles a disagreement
```

Each builds the working-tree binaries, uses only ephemeral `/tmp` state, and tears
itself down. They assume the local checkouts under `~/projects/personal/`.
