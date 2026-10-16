# Exercise: honeypot detection with an independent observer

**Status:** experiment report. **Reproduce:** `./tripwire.sh`. Related:
`FLOW.md`, `RECONCILE.md`.

## Objective

Verify (a) that a honeypot distinguishes a caller using the advertised surface from
a caller probing beyond it, and (b) that detection is performed by a participant
other than the one hosting the honeypot.

## Model

A honeypot is a target that appears legitimate but should not receive traffic.
Contact with it is a detection signal (see `smoke-alarm` `honeypot-nodes.md`). A
silent honeypot marks on any contact; a canary honeypot returns a pre-agreed
receipt whose appearance elsewhere proves a forgery.

Two properties are required of the arrangement:

- **the honeypotted node is not the detector.** A node is not trusted to report on
  itself, so the honeypot host only emits an alert; a second dashboard observes the
  alarm and is what indicates the detection.
- **the honeypotted node does not know it is honeypotted.** The decoy is installed
  by a participant acting outside the cycle (the pre-negotiation phase,
  `SPLASH.md`). The node operates inside the cycle and has no in-band knowledge of
  the decoy. A decoy visible to the marked node would not be a decoy. Detection
  therefore necessarily comes from outside the node.

## Procedure

An `adhd` hosts a honeypot **tool** — a decoy registered through the `honeypot-tools`
field of `adhd.cluster.join`, returning a plausible `{"ok":true}` so that it does not
reveal itself (`internal/mcpserver/server.go`, `registerHoneypotTool`). A trusted
caller invokes the advertised tools; a probing caller invokes the decoy.

```sh
smoke-alarm/spin.sh up sentinel --empty
adhd/spin.sh        up hp  --smoke-alarm …sentinel    # honeypot host
adhd/spin.sh        up obs --smoke-alarm …sentinel    # observer
```

## Observations

| # | actor | event | evidence |
| --- | --- | --- | --- |
| 1 | cluster | honeypot registered | `adhd.cluster.join` → `{"accepted":true,"honeypots_set":1}` |
| 2 | trusted caller | invokes the advertised surface | `adhd.status`, `adhd.lights.list` → no trigger |
| 3 | probing caller | invokes the decoy (direct) | → `{"ok":true}`; log `honeypot tool triggered tool=adhd.project.canary.call caller_ip=…` |
| 4 | probing caller | invokes the decoy (`tools/call`) | → `{"content":[{"text":"{\"ok\":true}"}]}`; log `honeypot tool triggered via tools/call …` |
| 5 | honeypot host | emits a security alert | `fireSecurity("warn","honeypot_triggered", …)` → POST to `/federation/report` |
| 6 | alarm | records the alert | `/status` → `security/honeypot_triggered healthy=false — level=warn …` |
| 7 | observer | indicates the detection | indicator `smoke:primary/component:security/honeypot_triggered` → `red` |

Events 1–4 establish that the honeypot distinguishes the two callers and does not
reveal itself. Events 5–7 require two code changes, applied in the working tree:
`smoke-alarm` `handleFederationReport` now merges `incoming.Components` (previously
only `incoming.Targets`; filed as james-gibson/smoke-alarm#23), and `adhd`
`smokelink` now decodes the alarm's `components` and emits an indicator per
component (previously only `targets`).

## Results

- The honeypot distinguishes a caller using the advertised surface (event 2) from
  one probing beyond it (events 3–4). The decoy is indistinguishable from a real
  tool: it returns `{"ok":true}`.
- Detection is observable by a participant other than the honeypot host (events
  6–7), consistent with the requirement that the honeypotted node not be the
  detector.

## Known limitations

- **Alert propagation depends on the component fix.** Without smoke-alarm#23, the
  alert is discarded in transit; see the issue.
- **No threshold was exercised.** The distance model that grades a contact (unknown
  tool `+32`, wrong prefix `+16`; `smoke-alarm/internal/mcp_faults/tool_proxy.go`)
  and its eviction threshold live in the engine and are not reachable from the HTTP
  surface. They are unit-tested only (`internal/engine`). A tool-honeypot contact
  at the dashboard does not currently feed that path.

## References

- `smoke-alarm/docs/features/catalog/honeypot-nodes.md`, `chaos-skill-detection.md`.
- james-gibson/smoke-alarm#23 — `/federation/report` dropped `Components`.
- `SPLASH.md` — *Probes as capability checks*, *Two invariants*.
