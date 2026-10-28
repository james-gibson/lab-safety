# Exercise: honeypot detection with an independent observer

**Status:** experiment report. **Reproduce:** `./tripwire.sh` (2026-10-28 run
below). Related: `FLOW.md`, `RECONCILE.md`.

The instruments are the KB vault's spin harnesses: `KB_SCRIPTS` (default
`~/projects/kb/scripts`). The graded contact needs james-gibson/adhd#61 applied in
the working tree; without it the join payload's `sensitivity` is ignored and the
first contact fires, as before.

## Objective

Verify (a) that a honeypot distinguishes a caller using the advertised surface from
a caller probing beyond it, (b) that detection is performed by a participant other
than the one hosting the honeypot, and (c) that the tripwire's **sensitivity** is a
dial rather than a switch — a distance below which a contact is seen but tolerated,
and at or above which it is an alert.

## Model

A honeypot is a target that appears legitimate but should not receive traffic.
Contact with it is a detection signal (see `smoke-alarm` `honeypot-nodes.md`). A
silent honeypot marks on any contact; a canary honeypot returns a pre-agreed
receipt whose appearance elsewhere proves a forgery.

**A contact is graded, and the grade accumulates.** The 42i model already prices a
boundary violation (`smoke-alarm/internal/mcp_faults/tool_proxy.go`: wrong prefix
+16, unknown tool +32); a contact on a decoy is priced by *how it was made*:

| contact | cost | why |
| --- | --- | --- |
| `tools/call` at the decoy | +16 | the advertised route, with no stamp for it (wrong-prefix case) |
| a direct method call at its name | +32 | a name invoked as a method the surface never offered (unknown-tool case) |

The cost is the **caller's**, not the decoy's: it accumulates per caller at the host
across contacts and decoys. The alert fires when a caller's distance reaches the
decoy's `sensitivity`. So the tripwire has a fuel gauge, not a switch — a caller
that knocks once and leaves is seen and tolerated; one that keeps probing crosses.
Only a crossing calls `fireSecurity`; the host logs every contact either way, so
"no alert" means "below the dial", never "blind".

Two properties are required of the arrangement:

- **the honeypotted node is not the detector.** A node is not trusted to report on
  itself, so the honeypot host only emits an alert; a second dashboard observes the
  alarm and is what indicates the detection.
- **the honeypotted node does not know it is honeypotted.** The decoy is installed
  by a participant acting outside the cycle (the pre-negotiation phase,
  `SPLASH.md`). The node operates inside the cycle and has no in-band knowledge of
  the decoy. A decoy visible to the marked node would not be a decoy. Detection
  therefore necessarily comes from outside the node. The threshold is part of that
  out-of-band wiring: it arrives in the `honeypot-tools` entry of
  `adhd.cluster.join`.

## Procedure

An `adhd` hosts a honeypot **tool** — a decoy registered through the `honeypot-tools`
field of `adhd.cluster.join`, returning a plausible `{"ok":true}` so that it does not
reveal itself (`internal/mcpserver/server.go`, `registerHoneypotTool`). A trusted
caller invokes the advertised tools; a probing caller invokes the decoy.

```sh
smoke-alarm/spin.sh up sentinel --empty
adhd/spin.sh        up hp  --smoke-alarm …sentinel    # honeypot host
adhd/spin.sh        up obs --smoke-alarm …sentinel    # observer
# then join a cluster that installs the decoy at a threshold:
#   honeypot-tools: [{name: adhd.project.canary.call, sensitivity: 48}]
```

`TRIPWIRE_SENSITIVITY` sets the threshold (default `48`, the run below): `0` is a
silent honeypot that fires on any contact, `32` fires on the direct method call
alone, `16` on the first `tools/call`. The host echoes the armed threshold back in
the join result, so the run does not have to assume it — it reads it.

## Observations

Run 2026-10-28, `TRIPWIRE_SENSITIVITY=48` (the default).

| # | actor | event | evidence |
| --- | --- | --- | --- |
| 1 | cluster | honeypot registered at a stated threshold | `adhd.cluster.join` → `{"accepted":true,"honeypots_set":1,"honeypots":[{"name":"adhd.project.canary.call","sensitivity":48}]}` |
| 2 | trusted caller | invokes the advertised surface | `adhd.status`, `adhd.lights.list` → no contact logged, no distance |
| 3 | probing caller | invokes the decoy (direct, **+32**) | → `{"ok":true}`; log `via=method distance_42i=32 threshold_42i=48 fired=false` — *seen, tolerated* |
| 4 | probing caller | invokes the decoy (`tools/call`, **+16**) | → `{"content":[{"text":"{\"ok\":true}"}]}`; log `via=tools/call distance_42i=48 threshold_42i=48 fired=true` — the crossing |
| 5 | honeypot host | emits a security alert *at the crossing only* | `fireSecurity("warn","honeypot_triggered", {via, distance_42i:48, threshold_42i:48, …})` → POST to `/federation/report` |
| 6 | alarm | records the alert | `/status` → `security/honeypot_triggered healthy=false — level=warn map[via:tools/call distance_42i:48 threshold_42i:48 …]` |
| 7 | observer | indicates the detection | indicator `smoke:primary/component:security/honeypot_triggered` → `RED` |

Events 1–4 establish that the honeypot distinguishes the two callers, does not
reveal itself, and grades the contact: after event 3 the alarm has **no** security
component — the knock happened and nothing fired, which is the tolerance the
threshold buys, not a miss. Event 4 is what crosses, and events 5–7 are the same
detection path as before.

At the other end of the dial, `TRIPWIRE_SENSITIVITY=0` (2026-10-28): event 3 fires
the alert on its own (`threshold_42i=0 distance_42i=32 fired=true`), reproducing the
pre-threshold behaviour — a silent honeypot marks on any contact. `32` and `16` move
the crossing to the direct method call and to the first `tools/call`, respectively.

Events 5–7 require two code changes, applied in the working tree:
`smoke-alarm` `handleFederationReport` now merges `incoming.Components` (previously
only `incoming.Targets`; filed as james-gibson/smoke-alarm#23), and `adhd`
`smokelink` now decodes the alarm's `components` and emits an indicator per
component (previously only `targets`). The grading in events 3–5 is
james-gibson/adhd#61.

## Results

- The honeypot distinguishes a caller using the advertised surface (event 2) from
  one probing beyond it (events 3–4). The decoy is indistinguishable from a real
  tool: it returns `{"ok":true}`.
- Detection is observable by a participant other than the honeypot host (events
  6–7), consistent with the requirement that the honeypotted node not be the
  detector.
- The threshold is a policy the exercise sets and the host reports back (event 1):
  the same two contacts are tolerated at 48 and an alert at 0, and the distance
  carried on the alert says which side the caller crossed.

## Known limitations

- **Alert propagation depends on the component fix.** Without smoke-alarm#23, the
  alert is discarded in transit; see the issue.
- **The weights are restated, not imported.** `+16`/`+32` live in
  `smoke-alarm/internal/mcp_faults` and are repeated as two constants in `adhd`'s
  honeypot (`server.go`, `contactViaToolsCall` / `contactViaMethod`), because adhd
  does not depend on smoke-alarm. If the model moves, two places move.
- **The caller is an address.** The ledger is keyed on the caller's host, so
  everything behind one NAT is one caller — a threshold can be crossed by two
  different probes arriving from the same address, and cannot distinguish them.
- **The distance is spent, not decayed.** It accumulates for the life of the
  process; a caller that stops probing is not forgiven, and a restart forgets.
  Decay belongs to the tick, and the tick is not wired to this path.
- **Eviction is still in the engine.** The distance grades a *contact* here; the
  eviction threshold (`distance > 60`) still lives in `smoke-alarm`'s engine and is
  not reachable from the HTTP surface.

## References

- `smoke-alarm/docs/features/catalog/honeypot-nodes.md`, `chaos-skill-detection.md`.
- `smoke-alarm/internal/mcp_faults/tool_proxy.go` — the +16/+32 scale.
- james-gibson/smoke-alarm#23 — `/federation/report` dropped `Components`.
- james-gibson/adhd#61 — a decoy fires at a configured 42i distance.
- `SPLASH.md` — *Probes as capability checks*, *Two invariants*.
