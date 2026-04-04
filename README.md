# lab-safety

> Safety checks, pre-flight validation, and runtime guardrails for the prototype lab.

Lab-safety is the newest member of a cluster of tools that share a common lab — a local-first, mDNS-connected, monitoring-instrumented prototype environment. This document introduces the full ecosystem so any tool, developer, or agent arriving here can orient quickly.

---

## The Lab

The lab is a set of Go tools that run together on a LAN (or a single developer machine), collaborate via mDNS service discovery, exchange health data over HTTP, and use a shared trust protocol — the **isotope protocol** — to verify capability and certify behaviour before acting on it.

The tools are loosely coupled: each runs as an independent binary, advertises itself via mDNS, and degrades gracefully when peers are absent. `lezz demo` can spin up a complete, self-contained cluster in seconds for local experimentation.

```
┌───────────────────────────────────────────────────────────────────┐
│                            the lab                                │
│                                                                   │
│   ┌──────────┐   manages    ┌──────────────────────────────────┐  │
│   │ lezz.go  │─────────────>│  daemon processes                │  │
│   │          │   demo       │  ocd-smoke-alarm  adhd  tuner    │  │
│   └──────────┘              └─────────┬────────────────────────┘  │
│                                       │ health / status           │
│   ┌──────────┐   monitors             │                           │
│   │   adhd   │<───────────────────────┘                          │
│   │(dashboard│   MCP traffic log                                  │
│   │  & MCP)  │                                                    │
│   └──────────┘                                                    │
│                                                                   │
│   ┌──────────┐   live data   ┌──────────────────────────────────┐ │
│   │  tuner   │──────────────>│ channels: NTP / DNS / ping / ... │ │
│   │(broadcast│   SSE / mDNS  └──────────────────────────────────┘ │
│   │& receive)│                                                    │
│   └──────────┘                                                    │
│                                                                   │
│   ┌──────────────────────────────────────────────────────────┐   │
│   │                       isotope                             │   │
│   │  canonicalization · consensus · 42i trust lattice        │   │
│   │  wire types · REST client                                 │   │
│   └──────────────────────────────────────────────────────────┘   │
│                                                                   │
│   ┌──────────┐                                                    │
│   │lab-safety│  <- you are here                                   │
│   └──────────┘                                                    │
└───────────────────────────────────────────────────────────────────┘
```

---

## The Guests

### lezz.go — The Host
`github.com/james-gibson/lezz.go`

The orchestration layer. Installs and manages the other tools as macOS LaunchAgents (`co.james-gibson.lab.*`), handles self-updating binaries, and provides `lezz demo` — a subcommand that spins up a complete local cluster (two smoke-alarm instances + an adhd headless instance) with ephemeral configs, a fixed-port discovery registry at `:19100/cluster`, and mDNS advertisement of `_lezz-demo._tcp`.

Key commands:
- `lezz service list` — show all managed daemons and their running status
- `lezz service purge` — stop and remove all managed services
- `lezz version` — show installed tool versions alongside daemon status
- `lezz demo` — start a self-contained cluster for experimentation
- `lezz update` — self-update all managed binaries

---

### ocd-smoke-alarm — The Monitor
`github.com/james-gibson/smoke-alarm`

Continuous health monitoring for HTTP endpoints. Polls configured targets on an interval, tracks state transitions (healthy → degraded → unhealthy → outage), detects regressions, and emits structured alerts. Exposes a health server at `/healthz`, `/readyz`, `/status`, and `/.well-known/smoke-alarm.json`.

Supports federation (pushing status reports between instances), mDNS-based peer discovery (`_smoke-alarm._tcp`), and the isotope trust registration endpoints:
- `POST /isotope/register` — register an isotope agent, receive a 42i trust rung
- `GET /isotope/list` — list all registered isotopes and their trust rungs

Multiple instances monitor each other, creating a Byzantine-tolerant consensus view of cluster health.

---

### adhd — The Dashboard
`github.com/james-gibson/adhd`

A minimal-footprint CLI dashboard for visualising alarm data. Runs in two modes:

**TUI mode** — a Bubble Tea terminal UI with colour-coded light indicators, keyboard navigation, and live polling of smoke-alarm instances.

**Headless mode** — an MCP server that logs all traffic as JSONL, watches smoke-alarm SSE streams, and participates in a prime/prime-plus topology where multiple adhd instances buffer and relay logs to a designated prime. In headless mode, adhd registers itself with smoke-alarm as an isotope and advertises `_adhd-isotope._tcp` via mDNS.

The `--demo` flag discovers a running `lezz demo` cluster via mDNS and auto-configures without a config file.

---

### tuner — The Broadcaster
`github.com/james-gibson/tuner`

A live data broadcaster that organises signal sources into named channels (NTP latency, DNS resolution stats, ICMP ping, and custom sources) and serves them over HTTP and SSE. Advertises channel availability via mDNS so other tools on the LAN can discover and subscribe without static configuration.

Channels can be consumed in broadcast mode (push via SSE) or receive mode (pull). Supports a visualisation layer via `tuner-viz` (a companion Rust binary) with graceful ASCII fallback when the Rust binary is absent.

---

### isotope — The Protocol
`github.com/james-gibson/isotope`

A shared Go library — not a binary. All the other tools depend on it.

Implements the **isotope protocol**: a framework for verifying the trustworthiness and capability of agents in the lab before allowing them to act. The protocol has two interlocking parts:

**Gherkin canonicalization and Byzantine consensus** — test scenarios written in Gherkin are canonicalized to a cryptographically signed token (an *isotope*). Multiple alarm instances independently verify tests and reach consensus using a quorum model (N >= 3F+1 Byzantine fault tolerance). The `ResultLedger` tracks history, flakiness, regressions, and the effectiveness of refactoring.

**The 42i capability lattice** — agents accumulate a *42i distance* based on test failures, weighted by how fundamental the failure is (entropy collapse, input ignorance, agreement-pattern collapse, etc.). Distance maps to a rung on a seven-level capability scale:

| Rung | Name             | Max distance | What it means                             |
|------|------------------|--------------|-------------------------------------------|
| 0    | Absolute Zero    | 0            | Loads without crashing                    |
| 1    | Read-Only        | 20           | Entropy and isotope variation pass        |
| 2    | Harness Tools    | 60           | Known tool scope, input correlation       |
| 3    | Mock Secrets     | 100          | Safe isotope handling, declared behaviour |
| 4    | Higher Authority | 140          | Independent decisions, timing uncorrelated|
| 5    | Delegation       | 180          | Can delegate to peers                     |
| 6    | Certification    | 220          | Can certify others                        |

When a tool registers with smoke-alarm as an isotope, `VerifyTrust` checks whether the registering endpoint appears in the lezz cluster registry and whether mDNS confirms a live cluster:
- Registry + mDNS confirmed → rung 6 (Certification)
- Registry only → rung 4 (Higher Authority)
- Self-registered, no cluster → rung 2 (Harness Tools)
- No registration → rung 0 (Absolute Zero)

The library also exports `Client` (REST calls to `/isotope/register` and `/isotope/list`) and `Advertise` (mDNS registration of `_<tool>-isotope._tcp` with role and trust rung in TXT records).

---

### pickled-onions — The Chronicler
`github.com/james-gibson/pickled-onions`

The world-builder. Maintains canonical Gherkin feature scenarios for each tool in the lab as *seeds* (`seeds/adhd/`, `seeds/ocd-smoke-alarm/`, `seeds/tuner/`, etc.). Seeds describe intended behaviour across the z-axis layers — physical (z0), temporal (z1), relational (z2), and epistemic (z3) — and feed the isotope canonicalization pipeline. Not a runtime tool; it authors the contract that the other tools are tested against.

---

### lab-safety — The Newcomer
`github.com/james-gibson/lab-safety`

You are here. Lab-safety is the pre-flight and runtime guardrail layer for the lab: the tool that checks the environment is in a safe state before experiments begin and enforces invariants that the other tools, being optimistic and collaborative by nature, do not enforce themselves.

Planned responsibilities (not yet implemented):
- **Pre-flight validation** — before `lezz demo` or manual cluster bring-up, verify that required ports are available, binaries are at compatible versions, and no zombie processes from a previous session are holding state
- **Cluster invariant checks** — query the smoke-alarm `/status` and isotope `/isotope/list` endpoints and assert that trust rungs meet a minimum threshold before permitting writes or deployments
- **Experiment gating** — integrate with the isotope protocol to refuse to proceed if the cluster's aggregate 42i distance exceeds a configured ceiling
- **Teardown auditing** — on shutdown, confirm that all managed processes have exited cleanly and that ephemeral state (temp dirs, stable config files, mDNS records) has been removed

Lab-safety will import `github.com/james-gibson/isotope` for wire types and trust verification, and will speak the same mDNS and REST vocabulary as the other tools.

---

## How the Tools Connect

```
lezz demo
  |
  +-- starts ocd-smoke-alarm (x2, each watching the other)
  +-- starts adhd --headless --smoke-alarm http://localhost:<portA>
  |     adhd registers as isotope -> rung assigned by VerifyTrust
  |     adhd advertises _adhd-isotope._tcp
  +-- registers cluster at :19100/cluster
  +-- advertises _lezz-demo._tcp
  +-- prints cluster summary including isotope trust levels

adhd --demo
  +-- browses _lezz-demo._tcp
  +-- fetches http://<host>:19100/cluster
  +-- builds config from AlarmA / AlarmB endpoints

ocd-smoke-alarm
  +-- POST /isotope/register -> calls VerifyTrust -> assigns rung
  +-- GET  /isotope/list     -> returns all registered isotopes
  +-- GET  /status           -> includes component health and target states

isotope (library, no binary)
  +-- Canonicalize Gherkin -> signed Isotope token
  +-- Byzantine consensus across ConsensusReporter instances
  +-- AgentState.RecordTestFailure -> accumulates 42i distance -> rung
  +-- VerifyTrust(ctx, endpoint) -> rung 2 / 4 / 6
  +-- Client.Register / Client.List -> REST calls to smoke-alarm

tuner
  +-- broadcasts NTP / DNS / ping channels over SSE
  +-- advertises _tuner._tcp via mDNS
  +-- (future) feeds signal data into smoke-alarm targets

lab-safety (planned)
  +-- pre-flight: port checks, version compatibility, zombie sweep
  +-- queries :19100/cluster and /isotope/list
  +-- gates experiments on minimum trust rung threshold
  +-- teardown: confirms clean exit of all managed processes
```

---

## Repository Layout

| Path | Module | Role |
|---|---|---|
| `lezz.go/` | `github.com/james-gibson/lezz.go` | Daemon manager + demo cluster |
| `ocd-smoke-alarm/` | `github.com/james-gibson/smoke-alarm` | Health monitoring |
| `adhd/` | `github.com/james-gibson/adhd` | Dashboard + MCP server |
| `tuner/` | `github.com/james-gibson/tuner` | Live data broadcaster |
| `isotope/` | `github.com/james-gibson/isotope` | Shared protocol library |
| `pickled-onions/` | — | Gherkin world-builder (seeds) |
| `lab-safety/` | `github.com/james-gibson/lab-safety` | Safety checks (this repo) |

---

## Getting Started with the Lab

Spin up a demo cluster:
```sh
lezz demo
```

Connect the dashboard:
```sh
adhd --demo
# or, using the stable config written by lezz demo:
adhd --config ~/.lezz/demo-adhd.yaml
```

Inspect cluster health and isotope trust:
```sh
curl http://localhost:8088/status | jq .
curl http://localhost:8088/isotope/list | jq .
curl http://localhost:19100/cluster | jq .
```

Manage background services:
```sh
lezz service list
lezz service purge
```

---

## The Isotope Protocol in Brief

Every tool in the lab is an *isotope*: a participant whose behaviour can be verified by an independent set of alarm instances. The protocol answers one question — **how far has this agent drifted from the seed-42 ground truth?** — and expresses the answer as a *42i distance* that maps to a capability rung.

Ground truth lives at `42 + 0i` (seed-42). Each test failure moves an agent along the imaginary axis, weighted by how fundamental the failure is. The magnitude `sqrt(42^2 + distance^2)` and direction label give a geometric picture of where the agent currently sits in capability space.

This is not a security system. It is a **collaborative fitness signal**: agents that score higher rungs have demonstrated more of their declared behaviour to the cluster's satisfaction, and can be trusted with more consequential operations.

---

## See Also

- [isotope protocol library](https://github.com/james-gibson/isotope)
- [ocd-smoke-alarm](https://github.com/james-gibson/smoke-alarm)
- [adhd dashboard](https://github.com/james-gibson/adhd)
- [lezz.go daemon manager](https://github.com/james-gibson/lezz.go)
