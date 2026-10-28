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
| `JOIN.md` | **join artifact** | the pattern at a glance — the one page that must be readable **at once**, because a participant that cannot see the pattern cannot agree to join |
| `SPLASH.md` | reusable | the post-connection trust model (a mechanism doc; long by design) |
| `SLA.md` | reusable | **SLA vs Lab-safety(SLA)** — the mechanisms: the exposure table, idempotence, herd immunity, the reward path, the instrument with no licence, BGP routing |
| `TRUST-FLOWS.md` | diagrams | the trust flows as pictures — admission, the triangle, the plane, failover, partitions |
| `CHALLENGE.md` | design | re-attestation: why a fresh run, not a stable token, is what re-admits — isotope, challenge, receipt, and the order they force |
| `MATH.md` | findings | the grounded figures, and which of the others are not grounded |
| `PLANE.md` | findings | the measure as a plane in local-time: delivered vs promised vs linear time |
| `THREADS.md` | findings | the whole arc — every thread, its state, its issue |
| `adr/` | decisions | the choices the exercises rest on, one file per decision (canon facts in `isotope/docs/adr/`) |
| `FLOW.md` | exercise | tuner ⇄ smoke-alarm ⇄ adhd, end to end — reproduced by `workspaces/spike.sh`, phase 2b |
| `TRIPWIRE.md` + `tripwire.sh` | exercise | trusted caller vs probing caller; the honeypot |
| `RECONCILE.md` + `accounts.sh` | exercise | the verification chain and who enacts a disagreement |
| `survey/README.md` | dataset | the **model survey** — what agents want from an integration endpoint. `survey/profiles/*.json` are the observations; `models.csv`, the summary table and `survey.feature` are checked projections of them |
| `survey/tool/README.md` | dataset | the **tool survey** — what tools need from the lab before it may trust them. `survey/tool/profiles/*.json` are the observations; `tools.csv`, the summary table and `tools.feature` are checked projections of them |

`MATH.md`, `PLANE.md` and `THREADS.md` are the ones that save time. The exercises
are reproducible; the three findings documents are the *conclusions*, so an agent
can start from the answer rather than the search.

`survey/` is the first entry that is neither an exercise nor a finding: it is
**data**, and its value is that the data cannot drift from what it claims.
`survey/README.md` says which way its Gherkin arrow points (the opposite way to
`sheets/`), and `go run ./cmd/survey --check` is the proof. `survey/tool/` is the
same pattern for the tool survey: one shared engine, a second vocabulary, and
`go run ./cmd/survey --dataset tool --check` as its proof.

**Two kinds of document, and the rule that separates them.** A **join artifact** must
be readable **at once** — if a participant cannot see the pattern in one view, it
cannot agree to join, and an agent decides per turn under partial visibility anyway.
A **mechanism doc** may be as long as the argument requires, because it is read to
*argue with*, not to join. `JOIN.md` is the join artifact; `SPLASH.md`, `SLA.md`,
`MATH.md` and `PLANE.md` are mechanisms. When something here grows, it grows into a
mechanism doc — never into the join artifact.

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
./tripwire.sh           # honeypot: trusted vs probing caller, observed by a second adhd
./accounts.sh           # ingress -> verifier -> monitor chain; who settles a disagreement
./workspaces/spike.sh   # the lab's real instruments, end to end: the plane
                        #   dark -> green -> yellow -> red, and the
                        #   tuner ⇄ smoke-alarm ⇄ adhd round-trip (phase 2b)
```

`tripwire.sh` and `accounts.sh` build the working-tree binaries from the local
checkouts under `~/projects/personal/`. `spike.sh` runs the installed instrument
binaries and finds the spin harnesses through `KB_SCRIPTS` (default
`~/projects/kb/scripts`), stopping with the path it wanted when one is missing — it
does not infer them from its own location. Each script uses only ephemeral spin
instances and tears itself down.
