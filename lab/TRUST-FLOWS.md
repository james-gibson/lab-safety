# Flows

**Status:** informational. Diagrams of the flows described across this directory.
Definitions are in `SPLASH.md`; figures in `MATH.md`; the measure in `PLANE.md`.

## 1. Overview

Two flows operate together: **admission** (what may enter) and **measurement** (how
far a value departs from nominal).

```
  OUT OF BAND  (decided before the cycle; not visible to a node inside it)
  ┌──────────────────────────────────────────────────────────────────────┐
  │  pre-negotiate the isotope · install the decoy · build the doors ·     │
  │  stack the credentials · plan the failover                             │
  └──────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
  the burst ──▶ ┌────────┐  mirror  ┌────────┐  authority ┌────────┐  credential
                │   A    │ ───────▶ │   B    │ ─────────▶ │   C    │ ────────▶ enters
                │session │          │surface │            │authority│           memory
                └────┬───┘          └────┬───┘            └────┬───┘
                     │ B verifies A      │ C verifies B         │ loop closes
                     └─────────◀─────────┴─────────────◀────────┘
                                   │
                                   ▼
  MEASUREMENT  (a position, not a count)
  real ─┐
        ├─ departure (42i distance, +8/+16/+32 …)  ─────────▶  rung  ──▶ fork / evict
  nominal┘        measured on the internal tick
```

## 2. Admission

A caller passes all three layers before a credential is authorized to enter memory.
A harness's automatic `tools/list` reaches **B** and stops.

```
   caller
       │
       ▼
  ┌─────────┐   who is here?     ┌─────────┐  what is exposed?  ┌─────────┐  what is
  │    A    │ ─────────────────▶ │    B    │ ─────────────────▶ │    C    │ ───────▶ CRED
  │ session │   session, one-to-one        │  the tool surface  │authority│  enters memory
  └─────────┘                    └────▲────┘                    └─────────┘
                                      │
                      a harness ──────┘  tools/list is answered here
                      auto-lists          → names, not authority
```

## 3. Verification

Each node is verified by the next; the loop closes on C, so no node is its own
auditor.

```
                    ┌───────────────────────────────┐
                    │   B ──── verifies ────▶ A     │   B holds A's real work
                    │  (idle)               (ingress)│   via the mirror
                    │     ▲                     │    │
                    │     │                     ▼    │
                    │   C ── verifies ──▶ B         │
                    │  (monitor)          (verifier) │
                    └───────────────────────────────┘
```

## 4. Measurement

```
   value
     ▲
     │                    ● the sample
     │                   ╱
     │   nominal ●──────╱────────  the level promised
     │            ╲   ╱
     │             ╲ ╱  departure = the gap  →  +8/+16/+32 …
     │              ╲
     └───────────────●────────────▶  linear time  (the internal tick)
                     the gap is not constant → a plane, not a line

   departure  →  rung  →  read gate · proxy · fork · evict
   the quorum (F = ⌊(N−1)/3⌋) is the discrete readout, taken after the measure
```

## 5. Out-of-band and in-band

```
  OUT OF BAND  (before the tick)              IN BAND  (the tick)
  ──────────────────────────────             ──────────────────────────────
  pre-negotiate the isotope                  the burst, in step
  install the decoy                          a verifier checks the ingress
  build the doors (enrol early)              failover swaps seats
  stack the credentials                      fork on disagreement
  plan the failover
               └──────────────┬───────────────┘
                              ▼
     a node inside the cycle has no in-band view of the out-of-band decisions
     → A does not know it is honeypotted; detection comes from outside A
```

## 6. Failover

```
        A compromised
             │
             ▼
   ┌───┐   B already holds:   ┌───┐   B takes A's seat   ┌───┐
   │ A │   • mirrored traffic  │ B │ ──────────────────▶  │ B*│  (was B, now ingress)
   └───┘   • pre-negotiated    └───┘                      └───┘
             isotope
   ┌───┐
   │ C │ ──────────────────────────────────────────────────────▶ fills B's seat,
   └───┘                                                          or provisions a new B
   pre-negotiation provides immediate failover · mirroring immediate stand-in
```

## 7. Partitions

```
   ┌─ merged ─┐        ┌─ partition ─┐        ┌─ reconnection ─┐
   │ accuracy │ ─────▶ │ internal    │ ─────▶ │    merge       │
   │          │        │ tick runs   │        │ (branches meet)│
   └──────────┘        └─────────────┘        └────────────────┘
        one branch        diverge (fork)          reconcile, not re-derive
```

## 8. The proxy surface

```
  ONE TASK, TWO PROXIES  →  consensus
     client ──task──▶ proxy α (tool "x")  ──┐
     client ──task──▶ proxy β (tool "y")  ──┼──▶ compare ──▶ agree  = corroboration
                                            │                disagree = one node wrong
     (α and β resolve to the same underlying tool, renamed)

  PLANNED PATH  →  routing update
     participant with the plan
        └─ follows a planned path ──▶ finds a cheaper route ──▶ updates routing tables
        └─ (a pathless probe instead ──▶ triggers detection)
```

## Summary

**Out-of-band decisions let A/B/C admit without deciding; admission is three layers
(session, surface, authority); verification traverses the triangle; measurement is a
position on a plane; disagreement forks; and the internal tick means the arrangement
holds across partitions.**
