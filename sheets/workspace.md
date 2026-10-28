# Safety Data Sheet — workspace (VFS session) MCP surface

> **`workspace-42`** · Streamable-HTTP JSON-RPC · `POST <addr>/mcp` ·
> protocol `2024-11-05` · owner `github.com/james-gibson/lab-safety` ·
> surface as served by `lab/workspaces/{protocol,http,honeypot}.go`
>
> Executable scenarios: [features/workspace-mcp-surface.feature](features/workspace-mcp-surface.feature) ·
> Sheet rev 1 · register: [workspaces/README.md](workspaces/README.md)

This is the surface the VFS prototype serves (#9, rung 2): one session's
filesystem (`/data`, `/state`, `/tools`, `/resources`, `/prompts`) read and
written through MCP, where **every operation is a file operation** and nothing
else. It is the surface a monitor probes from another process, and the one whose
`-42i → -42` movement the register measures.

## §1 Identification

| field | value |
| --- | --- |
| surface id | `workspace-<session>` (the prototype runs session `42`) |
| owner repo | `github.com/james-gibson/lab-safety`, module `lab/workspaces` |
| endpoint | `POST http://<addr>/mcp` (`-addr`, default `127.0.0.1:0` — ephemeral) |
| transport | HTTP; JSON-RPC 2.0 per POST at `/mcp`. `GET /mcp` = identity (`{session, protocolVersion}`); `GET /healthz` = health (200/503, see §5); neither is MCP |
| protocol version | `2024-11-05` |
| server name | `workspace-42` (`initialize` → `serverInfo.name`; matches the registry name) |
| instance identity | the **session id**, and the spec's signed isotope id published as `contract` |
| discovery | opt-in registry registration by name (`-smoke-alarm`), renewed every minute |

## §2 Hazard identification

Classes present: **`R`, `C`, `W`, `H`**. No `E`, no `X`.

| class | where |
| --- | --- |
| `R` | `initialize`, `tools/list`, `resources/list`, `resources/read`, `prompts/list` |
| `C` | `prompts/get` (renders a template; no state) |
| `W` | `tools/call` (writes `/tools/<name>.in` and `.out`) |
| `H` | `resources/subscribe`, `resources/unsubscribe` — the decoy pair |

This is a **low-hazard** surface with one honeypot and no egress. Its `W` is
bounded by the session root: there is nothing above `/`, and `..` is refused
(`ErrAboveRoot`), so a write cannot leave the session. Its `H` is hazardous only
to the caller's standing — a knock is the caller's cost, not the host's (§11).

## §3 Composition

The inventory is **methods**, because the surface's tools are data: a tool exists
while `/tools/<name>@` exists. A sheet that listed tools would be a snapshot, not
a declaration.

| # | method | class | inputs that matter | touches |
| --- | --- | --- | --- | --- |
| 1 | `initialize` | `R` | — | session presence, capabilities |
| 2 | `tools/list` | `R` | — | `/tools/*@` |
| 3 | `tools/call` | `W` | `name`, `arguments` | `/tools/<name>.in`, `.out`; the bound stand-in |
| 4 | `resources/list` | `R` | — | `/resources/**` |
| 5 | `resources/read` | `R` | `uri` | one file under `/resources` |
| 6 | `prompts/list` | `R` | — | `/prompts/*.tmpl` |
| 7 | `prompts/get` | `C` | `name`, `arguments` | renders the template |
| 8 | `resources/subscribe` | `H` | `uri` | `/state/honeypot/contacts.jsonl` |
| 9 | `resources/unsubscribe` | `H` | `uri` | `/state/honeypot/contacts.jsonl` |

**Dynamic surface** (present only while the files are):

| kind | entry | source |
| --- | --- | --- |
| tool | `echo` | `/tools/echo@` — seeded from the gate |
| resource | `resource://workspace/config` | `/resources/workspace/config.json` |
| prompt | `summarize` | `/prompts/summarize.tmpl` |

`tools/call` executes the **stand-in bound to the file** (`BindTools`), not a
process: the surface has no `X` today. When the file becomes truly executable,
this row becomes `W,X` and this sheet must be reissued.

## §4 First-aid measures

| symptom | code | meaning | recovery |
| --- | --- | --- | --- |
| parse failure | `-32700` | the body is not JSON-RPC | send a JSON-RPC 2.0 request |
| no session | `-32001` | the store holds no session mark (`/data/task.json`) | the session is gone; there is nothing to call |
| unknown method | `-32601` | not in the method switch | `initialize` / the register's §2 mapping |
| unknown tool | `-32601` | no `/tools/<name>@` | `tools/list` — the tool may have been dropped (§5 dynamic commands) |
| invalid params / unknown URI | `-32602` | wrong shape, or a resource the session does not hold | re-read `resources/list` |
| store error | `-32603` | the driver failed | check the host (tmpdir / `.wsdb`) |

Status: **enforced** (`protocol.go` → `dispatch`, `internal()`). A JSON-RPC error
is still HTTP 200 — the surface *answered and refused*, which is a different fact
from silence, and the prober reads the two differently. A **batch** (a JSON array)
is refused `-32700` although JSON-RPC 2.0 allows it (§16 finding E).

## §5 Fire-fighting measures

| control | value | status |
| --- | --- | --- |
| bind | `-addr`, default `127.0.0.1:0` (loopback, ephemeral) | enforced by the flag; pinning is a deployment choice |
| body limit | 1 MiB per request | enforced (`http.go` → `maxBody`) |
| supervision | `GET /healthz` → **200** when the session is present, **503** when it is not | enforced; the answer is about the session, never the process |
| rate limiting | none at this surface | **declared, not implemented** |
| kill switch | delete the session's store → every method answers `-32001` with the process still up | enforced |
| stop | `SIGINT`/`SIGTERM` deregisters the name and stops the listener | enforced |

## §6 Accidental-release measures

A caller that goes past the advertised surface is **contained and recorded**:

- `initialize` states `resources: {subscribe: false, listChanged: false}` — the
  delivery half is not promised;
- a caller that calls `resources/subscribe` or `resources/unsubscribe` anyway gets
  exactly the answer a real surface gives (`{}` for a URI the session holds,
  `-32602` for one it does not, `-32602` for a malformed request);
- the knock is appended to `/state/honeypot/contacts.jsonl` with the method, URI,
  outcome (`accepted` / `refused` / `malformed`), the HTTP peer when there was
  one, and `distance_42i: 32`;
- nothing on the wire distinguishes the decoy from a working surface — including
  the refusal, which a real surface would also give;
- the directory is created by the first contact, not by `Init`: a decoy visible
  before anyone touches it is not a decoy.

Status: **enforced** (`honeypot.go`; `TestTheDecoyDoesNotRevealItself`,
`TestTheDecoyIsInvisibleUntilTouched`). What is **not** enforced is the *verdict*
(§11, §16 finding B). A knock against a surface with no session is refused
(`-32001`) and **not** recorded: there is nowhere honest to write, and inventing
state for a session that does not exist would be the process standing in for the
session all over again.

## §7 Handling and storage

- **Inbound auth:** none. Trust is post-connection; the registry's rung is the
  lab's judgement, not this surface's (`advertise.go`).
- **Storage:** the session's own files. The honeypot ledger is session state and
  dies with the session.
- **Minimum rung by class** (declared, **not enforced** — the surface does not
  read rungs): `R`/`C` → rung 0; `W` → rung 3; `H` → rung 0, because the decoy is
  harmless to the host and the cost lands on the caller.

## §8 Exposure controls

| rung | may read |
| --- | --- |
| 0+ | every method in §3 (the lab is loopback and unauthenticated) |

The read-model is uniform by construction; there is no per-rung filtering to
describe. Status: **declared** (a rung gate would be a policy on top, not a
property of the surface).

## §9 Physical and chemical properties

| property | value |
| --- | --- |
| framing | one JSON-RPC 2.0 object per `POST`; no SSE, no streaming |
| statelessness | the process is stateless; the session is the state (a directory, or one `.wsdb`) |
| address | `-addr`, default `127.0.0.1:0`; the bound port is printed at startup |
| identity on `GET /mcp` | `{session, protocolVersion}` — for a human and a status check |
| health on `GET /healthz` | `{ok, session}` — 200 when the session is present, 503 when it is not, plus `reason` |
| concurrency | one writer per session (A13); the contact ledger is serialised in-process |

## §10 Stability and reactivity

| condition | behaviour |
| --- | --- |
| session store deleted | every method answers `-32001`; the process stays up; the registry name remains until its TTL — a hard kill is a backstop, not a signal |
| part of the session lost | the affected family leaves `initialize.capabilities`; the methods still answer |
| unknown method | `-32601`; subsequent calls succeed |
| malformed subscribe | `-32602`, and the knock is still recorded |
| driver error | `-32603`; the surface does not crash |

Status: **enforced** (`protocol.go`, `TestASurfaceWithNoSessionAnswersNothing`,
`TestTheSurfaceAdvertisesOnlyWhatItCanServe`).

## §11 Toxicological information

**`tools/call` (`W`).** Two file writes inside `/tools` plus the bound stand-in.
A tool's reach is the reach of its file, and the file lives in the session; the
prototype executes no process. The cost is a write the session can see.

**`resources/subscribe`, `resources/unsubscribe` (`H`).** A decoy pair, priced
the lab's way (`lab/TRIPWIRE.md`): a **direct method call the surface never
offered** is the unknown-tool case, **+32** `distance_42i`, accumulated per caller
at the host, alerted at the decoy's `sensitivity`. The cost is the caller's, not
the decoy's.

Status: the **recording is enforced**; the **grading is declared** — this surface
records each contact's `distance_42i` but does not accumulate it, holds no
sensitivity, and emits no alert. Until it does, "no alert" would mean "no
detector", not "no contact", and the ledger is the only honest answer.

## §12 Ecological information

**No egress per call.** Every method touches the session's own store and nothing
else. The surface's only outbound request is the registry registration and its
minute-by-minute renewal (`advertise.go`, opt-in via `-smoke-alarm`), which is
lifecycle, not a call effect, and is identical for every caller.

Status: **enforced**.

## §13 Disposal considerations

Deleting the session's store is the disposal: the surface answers `-32001` with
the process still serving, which is how "the session is gone" reaches a caller.
Stopping the process removes the listener and deregisters the name. The honeypot
ledger is part of the session and goes with it; there are no other temp files,
no registry entries, and no mDNS records owned by this surface.

## §14 Transport information

Plain HTTP on a loopback ephemeral port. Preconditions for reaching it outside a
test:

- the address is `-addr` (loopback by default; a non-loopback bind is a
  deployment decision, not a surface feature);
- a caller that wants it **by name** needs a running alarm registry and
  `-smoke-alarm`; the harness takes the instruments from `KB_SCRIPTS`
  (default `~/projects/kb/scripts`) and fails closed if they are missing.

**Co-hosted, not multiplexed.** The surface is a fourth listener on a host that
already runs adhd, smoke-alarm and tuner, so it shares nothing with them: no
port (ephemeral), no path (MCP at `/mcp`, health at `/healthz`, identity at `/mcp`
on GET), no mDNS service type (it advertises none — rung 2b), and no registry
namespace beyond its own name (`workspace-42`). Health lives on the same listener
rather than a second port, because on a shared host a port is the thing most
likely to fight. `/healthz` is the family's convention (`tuner`, `smoke-alarm`),
and what it must *not* do is report the process: a 200 from a live process whose
session is gone would be a false healthy — the same lie `-32001` refuses on the
wire.

**Two watchers, two signals — do not conflate them.** adhd probes a smoke-alarm
endpoint's `GET /healthz` as a reachability check (`internal/dashboard`,
`internal/demo`), while it judges a configured **binary** with an MCP
`initialize` (`internal/localprobe` → `mcpclient.Probe`, local vs remote
disagreement → yellow). This surface is neither by default: it is not a
configured alarm endpoint and not a configured binary. If a workspace gets its
own adhd (the per-workspace proxy), that adhd is the natural local observer — and
it would watch the session through the **handshake**, which already answers
`-32001` when the session is gone. The health path must agree with that verdict; it never
substitutes for it. What blocks the arrangement today is addressing, not
signalling: `features.binaries[].endpoint` is static, while the session's address
is ephemeral and published by name (§16 finding F).

## §15 Regulatory information

The declaration this sheet repeats is the **gate**:
[workspaces/minimal.feature](workspaces/minimal.feature) — parsed and given
meaning by `isotope` (`ParseGherkin`, step definitions), and canonicalised into
the session's **contract** (`CanonicalizeGherkinToIsotope`, family
`workspace-vfs`, v1, key `seed-42`). `Check` re-derives that isotope rather than
trusting the stored record. The lab-safety gate for this surface is therefore
rung 0/1 (the suite, both hosts) plus the positional check (mirror read back
through the protocol); rung 2 is the alarm's verdict on a live surface.

## §16 Other information

**Findings (recorded, not edited here):**

- **A — present, undeclared by the gate.** `resources/subscribe` and
  `resources/unsubscribe` answer, but no Rule in `minimal.feature` declares
  subscribe. This is deliberate (the decoy) and *must stay undeclared* until the
  delivery half exists; the gate gains the Rule when A7's fast rung is real, not
  before.
- **B — declared, not enforced: the contact is not judged.** The surface writes
  `distance_42i: 32` per knock and stops there: no accumulation, no sensitivity,
  no security alert, no observer. The honeypotted node is not the detector
  (`TRIPWIRE.md`), so this is a missing *reader*, not a missing writer. Tracked
  in #9 with A7.
- **C — declared, absent by design: delivery.** No
  `notifications/resources/updated` is ever sent. `subscribe` is answered and
  nothing follows. This is the honeypot, not a defect — but it is why
  `initialize` must say `subscribe: false` on every release.
- **D — the decoy's cost model is borrowed, not derived.** `+32` is
  `TRIPWIRE.md`'s unknown-tool price for *tools*; applying it to a resource
  method is a reading, not a measurement. If the lab prices methods separately,
  this sheet and `Distance42i` move together. And a **published** distance is only
  audit-ready when an auditor can re-derive it: this record carries the contact
  (method, URI, outcome, peer, `advertised`) but neither the pricing rule at a
  version nor the anchor the departure is from, so `distance_42i: 32` is evidence
  of a knock, not yet a number someone else can recompute.
- **E — declared, not implemented: no JSON-RPC batch.** A batch (JSON array) is refused
  `-32700`, though JSON-RPC 2.0 — and therefore MCP `2024-11-05` — allows one.
  No method in this surface needs a batch, but a client that sends one is refused
  for a reason that is not its fault, and the error code says "parse error" when
  the body parsed fine.
- **F — declared, not wired: no watcher can reach it by name.** Nothing watches
  this surface today in the lab's harness: `spike.sh` runs one adhd against the
  alarm, adhd's `features.binaries` (the list its localprobe walks) is not written
  by `adhd/spin.sh`, and the headless dashboard does not start the monitor. A
  per-workspace adhd would be the right local observer — but it takes a **static**
  `binaries[].endpoint`, and this surface's address is ephemeral and published
  only as a registry name. Watching it needs either endpoint injection at start
  (couples the two, and re-introduces "remember the port") or name resolution in
  the probe (matches rung 2b). Filed as james-gibson/adhd#62.

**Revision.** Rev 1 — created with the MCP-surface SDS pattern, from
`protocol.go`, `http.go`, `honeypot.go`, `advertise.go` and a live `-spec` run
(session `42`, both hosts). The declared inventory is the method switch, not a
copied README. Re-enumerate against a live instance on every reissue.
