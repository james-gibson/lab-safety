# Safety Data Sheet — ocd-smoke-alarm hosted service MCP surface

> **`smoke.*`** · Streamable-HTTP JSON-RPC + SSE · `POST <host>:<port>/mcp` ·
> protocol `2024-11-05` · owner `github.com/james-gibson/smoke-alarm`
>
> Executable scenarios: [features/ocd-smoke-alarm-mcp-surface.feature](features/ocd-smoke-alarm-mcp-surface.feature) ·
> Sheet rev 1 · surface as served by `internal/hosted/server.go`

## §1 Identification

| field | value |
| --- | --- |
| surface id | the alarm's **hosted service** (a distinct surface from its health endpoints) |
| owner repo | `github.com/james-gibson/smoke-alarm` |
| endpoint | `/mcp`, `/acp`, `/a2a` on `Hosted.ListenAddr` (default `:0`) |
| transport | `HTTP` + `SSE` (`Hosted.Transports`) |
| protocol version | `2024-11-05` |
| server name | `<ServiceName>-hosted-mcp` (from `initialize` → `serverInfo`) |
| gating | opt-in: `Hosted.Enabled`; default protocols `mcp` + `acp` |

Note: the alarm has **two unrelated faces** — its health/isotope REST API (which
monitors others) and this embedded hosted surface (which serves MCP). This sheet
covers only the latter.

## §2 Hazard identification

Classes present: **`R` only.** Every declared tool reads; none writes, egresses or
executes. This is the lab's **lowest-hazard MCP surface**. See §16 for the
invocation caveat — the class is `R`, but a `tools/tools/call` may not be wired
at all.

## §3 Composition

Inventory as served (`tools/list`):

| # | tool | class | inputs | returns |
| --- | --- | --- | --- | --- |
| 1 | `smoke.health` | `R` | — | hosted service health summary |
| 2 | `smoke.tuner_list_channels` | `R` | — | channels available for Tuner observation |
| 3 | `smoke.tuner_audience` | `R` | — | audience metrics per channel |
| 4 | `smoke.tuner_caller_messages` | `R` | `channel` | caller messages from Tuner viewers |

Non-tool surface:

| kind | entry |
| --- | --- |
| resource | `hosted://status` — "Embedded hosted service state and capabilities" |
| ACP/A2A method | `prompt/turn` → `{output:{text}, done:true}` (stub) |
| method | `ping`, `initialize`, `resources/list`, `session/setup` (ACP only) |

**Cold inventory:** 4 tools, 1 resource. No dynamic tools.

## §4 First-aid measures

| symptom | code | meaning | recovery |
| --- | --- | --- | --- |
| method wrong for protocol | `-32601` | e.g. `tools/list` on `acp`, `session/setup` on `mcp` | call the protocol the endpoint names |
| unknown method | `-32601` | method not in the switch | check the tool/protocol mapping |
| `tools/call` | `-32601` | **not implemented** (see §16) | the 4 tools cannot be invoked today |

Status: **enforced** (the `default:` arm returns `-32601`).

## §5 Fire-fighting measures

| control | value | status |
| --- | --- | --- |
| opt-in | `Hosted.Enabled` must be set | enforced |
| bind | `Hosted.ListenAddr` (default `:0`) | enforced — *note `:0` is all-interfaces, random port; not loopback-pinned* |
| protocols/transports | explicit allow-lists (`Hosted.Protocols`, `Hosted.Transports`) | enforced |
| rate limiting | none at this surface | declared (the alarm's rate policy applies to *monitored targets*, not to this surface) |

## §6 Accidental-release measures

Every protocol event (connect/disconnect/error) is appended to a bounded
in-memory event ring and counted per protocol (`HostedCounters`), so a stray
caller leaves a trace. SSE connections emit a heartbeat every 2s and are dropped
on context cancellation.

Status: **enforced**.

## §7 Handling and storage

- **Inbound auth:** none. Like the adhd surface, trust is post-connection.
- **Storage:** no durable state; events/counters are in-memory and lost on
  restart.
- **Rung:** declared minimum rung 0 (`R` only). Status: not enforced (surface does
  not read rungs).

## §8 Exposure controls

| rung | may read |
| --- | --- |
| 0+ | all four `smoke.*` tools and `hosted://status` |

Because the surface is `R`-only, no rung gate is required for safety; the
read-model is uniform. Status: **declared** (uniform by construction).

## §9 Physical and chemical properties

| property | value |
| --- | --- |
| framing | JSON-RPC 2.0; SSE stream on `Accept: text/event-stream` |
| statelessness | stateless; only counters + a bounded event ring |
| address | `Hosted.ListenAddr`, default `:0` |
| protocols | `mcp`, `acp`, `a2a` (default `mcp`+`acp`) |
| transports | `http`, `sse` |

## §10 Stability and reactivity

| condition | behaviour |
| --- | --- |
| protocol/method mismatch | `-32601`, no crash |
| SSE client disconnect | stream ends; `disconnected` event recorded |
| streaming unsupported by writer | HTTP 500 + `error` detail |
| shutdown | bounded by `Hosted.ShutdownTimeout` |

Status: **enforced**.

## §11 Toxicological information

No hazardous tools. The two `tuner_*` tools read *from* Tuner (channel list,
audience, viewer messages); they do not contact Tuner's network from this surface
— the data is presented by the hosted layer. `smoke.tuner_caller_messages`
exposes viewer-supplied content, so treat its output as untrusted input, not as a
safety hazard of the surface.

## §12 Ecological information

No outward reach. The surface makes no egress calls. Status: **enforced**.

## §13 Disposal considerations

`Server.Shutdown` stops the HTTP server within `Hosted.ShutdownTimeout`; no temp
files, no registry entries, no mDNS records owned by this surface.

## §14 Transport information

Plain HTTP/SSE on the configured address. Because the default `:0` is
all-interfaces, an operator must pin `Hosted.ListenAddr` to restrict reachability
— this is a *deployment* precondition, not enforced by the surface.

## §15 Regulatory information

The alarm is the lab's isotope **monitor** and consensus reporter; the hosted
surface is separate from that role. Rung information for the alarm itself is
exposed on its health/isotope REST face (`/isotope/list`), not here.

## §16 Other information

**Findings (not edited here — they belong to the owning tool):**

- **A — present, not invocable:** `tools/list` advertises 4 tools, but the method
  switch has **no `tools/call` arm**, so every invocation returns `-32601`. The
  tool set is *declared* (listed) but not *served* (callable). Either implement
  `tools/call` or drop the tools from `tools/list`.
- **B — bind default:** `Hosted.ListenAddr` defaults to `:0`, which binds all
  interfaces; a safety-positive default would be loopback.

**Revision.** Rev 1 — created with the MCP-surface SDS pattern. Derived from
`tools/list` + `internal/hosted/server.go` and `internal/config/config.go`
(hosted defaults). Re-enumerate against a live `Hosted.Enabled` instance on every
reissue.
