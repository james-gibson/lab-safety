# Exercise: tuner, smoke-alarm, and adhd round-trip

**Status:** experiment report. **Basis:** one run, 2026-10-15, UTC 23:25–23:26;
re-run 2026-10-28 (below).
**Reproduce:** `lab/workspaces/spike.sh` — the round-trip is its phase 2b. It used
to have a script of its own (`lab/exercise.sh`), which set its root one level too
high and looked for the spin harnesses inside this repo, where they are not
(lab-safety#11). The harness is now the one that was already known to run, and it
takes the instruments' location from `KB_SCRIPTS` (default
`~/projects/kb/scripts`) rather than inferring it from its own path.

## Objective

The components exist so that a dynamic manifest can route a participant through the
cluster without static configuration (`SPLASH.md`). This exercise verifies the
movement of events between the three components: a broadcaster (`tuner`), a monitor
(`smoke-alarm`), and a dashboard (`adhd`).

## Observation paths

```
        tuner ──────── broadcast (HTTP/SSE) ──────────────────┐
   (broadcaster)                                               │
        ▲  discovers _smoke-alarm._tcp (mDNS, RFC 6762)       │
        │  posts audience ──► /tuner/audience                  │
        │  posts caller   ──► /tuner/caller/{channel}          ▼
   smoke-alarm ─── checks MCP endpoints (targets) ───►  adhd MCP endpoint
    (monitor)   ─── advertises _smoke-alarm._tcp ───►  (mDNS)

        adhd ────── polls/subscribes smoke-alarm /status ──►  indicators
   (dashboard)
```

The tuner broadcasts; a smoke-alarm with the `tuner` block exposes the audience and
caller endpoints the tuner feeds. The smoke-alarm checks MCP endpoints and
advertises itself. The dashboard watches the smoke-alarm and reflects each of its
targets as an indicator.

## Procedure

```sh
lab/workspaces/spike.sh    # phase 2b is this exercise; the ports are ephemeral
```

`spike.sh` starts the three components and drives the requests below; it reads each
port from the instance it started, so nothing is hard-coded. The same sequence by
hand, with the alarm holding a tuner block (`--tuner`) and therefore a hosted port:

```sh
scripts/tuner/spin.sh       up <name>                     # channels + mDNS
scripts/smoke-alarm/spin.sh up <name> --tuner --target-session <isotope> --registry <url>
scripts/adhd/spin.sh        up <name> --smoke-alarm-instance <alarm>
curl -XPOST :<hosted>/tuner/audience    -d '{"channel":"ntp","count":3,"signal":0.81}'
curl -XPOST :<hosted>/tuner/caller/ntp  -d '{"message":"clock drift rising","from":"tuner","priority":"warn"}'
curl -N    ":<hosted>/hosted/events?transport=sse"
```

## Observations

| # | UTC | actor | event | evidence |
| --- | --- | --- | --- | --- |
| 1 | 23:26:10 | smoke-alarm | advertises on mDNS | `mdns: advertising _smoke-alarm._tcp.local. on port 51891 (host: …, ips: […], txt: [])` |
| 2 | 23:26:10 | smoke-alarm | federation introducer | `INFO federation introducer listening addr=127.0.0.1:6100` |
| 3 | — | smoke-alarm | target check passes | `/status` → `target-1 healthy — http(s) handshake passed: initialize, tools/list` |
| 4 | 23:26:34 | tuner → alarm | audience metric recorded | POST → `{"status":"ok"}`; GET → `{"audience":[{"channel":"ntp","count":3,"signal":0.81,…}]}` |
| 5 | 23:26:34 | tuner → alarm | caller message recorded | POST → `{"channel":"ntp","status":"received","subscribers":0}` |
| 6 | 23:26:34 | smoke-alarm | hosted event, `protocol: tuner` | `{"protocol":"tuner","method":"audience","path":"/tuner/audience","outcome":"ok",…}` |
| 7 | 23:26:34 | smoke-alarm | hosted event, `protocol: tuner` | `{"protocol":"tuner","method":"caller","path":"/tuner/caller/ntp","outcome":"ok",…}` |
| 8 | 23:26:41 | adhd | indicator transition | `{"type":"light-update","target_id":"target-1","endpoint":"primary","old_status":"dark","new_status":"green","via":"poll"}` |
| 9 | 23:26:45 | adhd | dashboard summary | `adhd.status` → `lights:{green:1,total:1}` |

Events 4–7 are the integration: the alarm accepts what the tuner reports and logs
it under its own `tuner` protocol. Events 8–9 close the loop: the alarm's own
verdict about the MCP endpoint returns as an indicator on the dashboard.

Re-run 2026-10-28, through `spike.sh`'s phase 2b: both reports are accepted and
logged under `protocol: tuner` with `outcome: ok`, the audience reads back
`ntp count=3 signal=0.81`, and the dashboard moves `dark -> green`. One difference
in the frame: there the alarm's target is the workspace session, not the idle
dashboard on :9090, so events 3 and 8–9 are the same observation about a different
endpoint. Events 1–2 are the alarm's own startup lines and `spike.sh` does not print
them, so they still rest on the original run.

## Surfaces observed

Ports from the 2026-10-15 run; every port here is ephemeral and `spin.sh` picks a
free one.

| component | port | surface |
| --- | --- | --- |
| tuner (broadcast) | 51887 | `/channels`, `/channels/{ntp,dns,ping}`, `/llms.txt` |
| tuner (health) | 51888 | `/healthz`, `/readyz`, `/status` |
| smoke-alarm (health) | 51891 | `/healthz`, `/status` |
| smoke-alarm (hosted) | 51892 | `/mcp`, `/tuner/audience`, `/tuner/caller/{channel}`, `/hosted/events` (SSE) |
| adhd | 9090 (idle) | `/mcp` |

## Observations of note

- **The report is driven by the harness, not by the broadcaster.** `tuner`
  advertises `_tuner._tcp` and browses `_smoke-alarm._tcp`; nothing in it posts
  `/tuner/audience` unattended. The two POSTs stand in for a broadcaster reporting
  what it sees, and the alarm accepts them from a loopback caller.
- **Service discovery did not complete this run.** The tuner's browser did not
  enumerate `_smoke-alarm._tcp`, though the system resolver did
  (`dns-sd -B _smoke-alarm._tcp local.` listed the instance) and a second tuner
  found the first's `_tuner._tcp`. Filed as james-gibson/tuner#3. The browser
  should key on registered isotope service types and dedupe by endpoint, rather
  than on bare tool names (see `SPLASH.md`, *Membership and authority
  announcement*).
- **Bare `/` returns 404** on both the health and hosted ports of the alarm, so a
  discovered instance can appear unresponsive to a caller that probes the root.
  Filed as james-gibson/smoke-alarm#22.
- The alarm advertises `_smoke-alarm._tcp` with an **empty TXT set** (`txt: []`),
  carrying no version or protocol hints; cf. the TXT record convention in RFC 6763.

## References

- RFC 6762 — Multicast DNS; RFC 6763 — DNS-Based Service Discovery.
- RFC 9110 — HTTP Semantics (the observability of `/status`).
- `SPLASH.md` — the model; `TRIPWIRE.md`, `RECONCILE.md` — related exercises.
