# Safety Data Sheet — pennies harness surface

> **`pennies.*` / `barista.*`** · Streamable-HTTP JSON-RPC · `POST <addr>/mcp` ·
> protocol `2025-06-18` · owner `github.com/james-gibson/lab-safety` ·
> surface as served by `lab/pennies/server.go`
>
> Executable scenario: this sheet is the declaration the harness is checked
> against. `lab-safety check --probe=<addr> --surface=pennies --gate` is the test.

## §1 Identification

| field | value |
| --- | --- |
| surface id | `pennies` (the barista harness) |
| owner repo | `github.com/james-gibson/lab-safety` |
| endpoint | `http://<host>:<port>/mcp` (`--addr`, default loopback) |
| transport | `POST /mcp` = JSON-RPC 2.0 |
| protocol version | `2025-06-18` |
| server name | `pennies` (from `initialize` → `serverInfo`) |
| gating | opt-in: the harness only runs when started |

The surface exists to test lab-safety's invariants against a live peer. Its tools
are **installed at runtime** — the point of the surface — so they are declared in
the dynamic table even when a cold `tools/list` happens to show them.

## §2 Hazard identification

Classes present: **`R`, `W`.** `pennies.grant` and `barista.path` mutate harness
state (issue a token, mint a path), so they carry `W`; nothing egresses or
executes. The harness runs on loopback with no upstream.

## §3 Composition

Static inventory:

| # | tool | class | inputs | returns |
| --- | --- | --- | --- | --- |
| 1 | `pennies.status` | `R` | — | rotation id, price, gate state |

**Dynamic tools** (installed at runtime by `pennies.New`):

| tool | class | note |
| --- | --- | --- |
| `barista.manifest` | `R` | the handshake: policy manifest + current rotation id |
| `pennies.grant` | `W` | issues a penny bound to the current rotation |
| `barista.path` | `W` | the vanishing path; requires a penny from the current rotation |

Optional, only with `--register-undeclared`: `pennies.bonus` (`R`) — a tool no
sheet declares, so `check --probe` has something to catch.

## §4 First-aid measures

| symptom | code | meaning | recovery |
| --- | --- | --- | --- |
| no penny | `-32001` | `barista.path` called without a token | call `pennies.grant`, then pass `penny` |
| stale penny | `-32001` | the penny was issued in an earlier rotation | grant again; a grant rotates the surface |
| unknown tool | `-32602` | the tool is not installed | check `--register-undeclared` and `tools/list` |
| method wrong | `-32601` | not an MCP method | use `initialize`, `tools/list`, `tools/call` |

Status: **enforced** (the refusal arms are the harness's own code paths).

## §5 Fire-fighting measures

| control | value | status |
| --- | --- | --- |
| token gate | a penny from the current rotation is required for `barista.path` | enforced — `--enforce=false` opens it on purpose |
| rotation | every grant issues a new rotation and invalidates earlier pennies | enforced |
| rung gate | declared rungs are not read by the surface | declared, not enforced |
| bind | loopback by default | enforced |

## §6 Accidental-release measures

The surface holds no durable state: the rotation and the issued pennies live in
memory and disappear with the process. A stray caller leaves no trace, which is
a gap this harness does not claim to close.

## §7 Handling and storage

- **Inbound auth:** a penny, for `barista.path` only. Everything else is open —
  trust is post-connection, as it is for every surface in the lab.
- **Storage:** none.
- **Minimum rung by class (declared):** `R` → rung 0; `W` → rung 3. Status: not
  enforced (the surface does not read rungs).

## §8 Exposure controls

| rung | may read |
| --- | --- |
| 0+ | `pennies.status`, `barista.manifest`, `pennies.grant` |
| 3+ | `barista.path` (with a penny) |

## §9 Physical and chemical properties

One process, one listener, in-memory state; no federation.

## §10 Stability and reactivity

The rotation changes on every grant. Nothing else does. Restarting the surface
invalidates every penny.

## §11 Toxicological information

A priced tool that answers without a token is a **gate that does not exist the
moment the surface is not started with `--enforce`**. That is deliberate: the
harness's job is to be caught by `GATE-OPEN`.

## §11a Accounting

`barista.path` counts every redemption it accepts, including a second use of the same
penny in the same rotation. The ledger is `granted / accepted / recorded` on
`pennies.status`, and the invariant is **accepted == recorded**: a use that happened is a use
that is billed. `--unbilled-replay` accepts the second use and drops its record, which is the
failure the ledger exposes — *not* the replay itself.

## §12 Ecological information

None beyond the lab's LAN.

## §13 Disposal considerations

Stop the process; the state is in memory only.

## §14 Transport information

JSON-RPC over HTTP, no SSE responses. One request per POST.

## §15 Regulatory information

`policy/skills.yaml` declares the price (`grants.unit_value`) and the enforced
gate (`skills[scope=barista:path].gate.enforcement`). The manifest's
`status` stays `declared` while any other skill is unpriced-by-gate.

## §16 Other information

This sheet describes a **harness**, not a product surface: it exists so the
invariants in `internal/policy` have a peer that can pass them and a peer that
can fail them (`--enforce=false`, `--register-undeclared`). The prototype's
temporal cloak is not implemented here either — see the manifest's `timing`
note: a beat is not a server-side check.
