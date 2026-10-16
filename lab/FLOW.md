# Exercise: tuner, smoke-alarm, and adhd round-trip

**Status:** experiment report. **Basis:** one run, 2026-10-15, UTC 23:25–23:26.
**Reproduce:** `./exercise.sh`.

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
scripts/tuner/spin.sh      up t0                                   # channels + mDNS
scripts/smoke-alarm/spin.sh up flow --tuner --target http://127.0.0.1:9090/mcp
scripts/adhd/spin.sh        up dash --smoke-alarm http://127.0.0.1:51891
curl -XPOST :51892/tuner/audience -d '{"channel":"ntp","count":3,"signal":0.81}'
curl -XPOST :51892/tuner/caller/ntp -d '{"message":"clock drift rising","from":"tuner","priority":"warn"}'
curl -N ":51892/hosted/events?transport=sse"
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

## Surfaces observed

| component | port | surface |
| --- | --- | --- |
| tuner (broadcast) | 51887 | `/channels`, `/channels/{ntp,dns,ping}`, `/llms.txt` |
| tuner (health) | 51888 | `/healthz`, `/readyz`, `/status` |
| smoke-alarm (health) | 51891 | `/healthz`, `/status` |
| smoke-alarm (hosted) | 51892 | `/mcp`, `/tuner/audience`, `/tuner/caller/{channel}`, `/hosted/events` (SSE) |
| adhd | 9090 (idle) | `/mcp` |

## Observations of note

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
