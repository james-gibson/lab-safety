# wan-ingress — design record (issue #38)

A single static Go binary: a thin WAN-facing front that resolves arbitrary
inbound names into **agreed** cluster addresses and namespaces, and forwards
only through a signed border. It does not route by guessing.

## The frame: do / know / see

| | |
| --- | --- |
| **do** | accept HTTP on an address, parse a name, resolve it by agreement, sign and forward one request to the agreed border, expose health/status, renew its registry name, reload policy on SIGHUP |
| **know** | the operator-supplied agreements (id, aliases, namespace, border address), the WAN token, the border signing key, the smoke-alarm registry URL |
| **see** | inbound decisions (`forward`, `quarantine` + typed reason), `/status` counters, a registration lease in the lab's isotope registry, the border's own record of what it received, and the smoke-alarm/adhd light derived from `/mcp` |

No node reports on itself: the binary publishes counters and a probe surface,
but a smoke-alarm target and an adhd light must do the judging. The counters are
the evidence a separate process reads, not a claim of health.

## Why these decisions

The ticket asked for a resolver that "accepts pretty much everything". Two
refuted presumptions made that unsafe as written; the user resolved both on
issue #38 before code was written:

1. **WAN front → lab border.** The public side has a thin admission model; the
   lab's isotope rungs and BAML border stay the authority inside. A caller
   cannot name an arbitrary destination.
2. **Accept all, quarantine unknowns.** Parsing is total, but forwarding needs
   an agreement. Unknown/ambiguous/malformed inputs get a typed refusal; ids
   and addresses come from the agreement file only.

The lab's own vocabulary forced this: ids are agreed, never synthesised
(`lab/baml/SKILL.md`); an address is a claim (`lab/ADDRESSING.md`); and
lab-safety is the policy gate — no proxy path is live without its sign-off
(`README.md`).

## Configuration (minimal, fail-closed)

| flag | default | meaning |
| --- | --- | --- |
| `-config` | *empty* | JSON config file; every policy field below can live here. Unknown keys are refused |
| `-addr` | `127.0.0.1:8443` | bind address. `0.0.0.0` is a bind, never a destination; bind wider only explicitly |
| `-agreements` | *empty* | JSON file of agreed mappings; with none, every name is `unknown` |
| `-token` | *empty* | bearer token for WAN admission; with none, every caller is `untrusted` |
| `-border-key` | *empty* | HMAC key for the signed border envelope; with none, forwarding is `border_unconfigured` |
| `-smoke-alarm` | *empty* | registry URL; when set, register by name as `wan-ingress` (same shape as `lab/workspaces/advertise.go`) |
| `-heartbeat` | `1m` | renewal interval for the registration lease; deliberately inside smoke-alarm's 5m TTL |

A config file is the normal install; flags override it only when explicitly set:

```json
{
  "addr": "127.0.0.1:8443",
  "agreements": "/etc/lab/wan-ingress/agreements.json",
  "token": "…",
  "border_key": "…",
  "smoke_alarm": "http://127.0.0.1:18080"
}
```

The agreement file is unchanged:

```json
{
  "agreements": [
    {
      "id": "workspace-42",
      "namespace": "session-42",
      "border": "http://127.0.0.1:19100/cluster",
      "aliases": ["42", "workspace-42", "ws42"],
      "rung": 2
    }
  ]
}
```

Empty defaults are intentional: out of the box the process serves health and
status and refuses everything else. It becomes useful one file and two secrets
later, not one dangerous default earlier.

**SIGHUP reloads policy in place.** The new config and agreements are read and
built off to the side, then swapped atomically; a bad file logs and keeps the
last good policy. Counters survive the swap, so the monitor does not forget what
it saw. The listener and registry name never move.

## Surface

- `GET /healthz` → `200 ok` (needs no configuration)
- `GET /readyz` → `200 ready` when agreements, token and border key are set;
  `503 not ready` otherwise
- `GET /resolve?name=<raw>` → `200` with the agreed route, or `422` with a typed
  refusal (`malformed`, `unknown`, `ambiguous`)
- `POST /forward?name=<raw>` → signed border forwarding, or `422` with a typed
  refusal (`border_unconfigured` included); a caller-supplied address header is
  ignored
- `GET /status` → counters, including `quarantined` by reason
- `POST /mcp` → the smoke-alarm probe surface (JSON-RPC): `initialize`,
  `ingress/status`, and `notifications/initialized`

### What the monitor sees

The lab's monitor is a smoke-alarm target, declared in
[`configs/wan-ingress-alarm.yaml`](../../configs/wan-ingress-alarm.yaml). It
probes `POST /mcp` with a JSON-RPC handshake and requires `ingress/forward` in
`min_capabilities`. The front advertises that capability **only after at least
one agreed request has actually crossed the signed border**. So:

| state | alarm | adhd light | meaning |
| --- | --- | --- | --- |
| answers but has never forwarded | degraded | **yellow** | defensive: alive, no agreed traffic yet |
| has forwarded | healthy | **green** | agreed route is live |
| process is down | unhealthy/regression | **red** | probe fails after a previous healthy state |

A stopped process is judged by the monitor, not by itself. `sheets/ingress/spike.sh`
drives the real smoke-alarm and adhd spin harnesses through that whole
`dark → yellow → green → red` sequence and asserts every hop.

Admission runs before resolution: a caller without the token gets `403
untrusted` and learns no route. A name is parsed once (a single percent-decode,
bounded length, UTF-8 and control-character checks), lower-cased and used only
as a lookup key. The route's address is copied from the agreement.

The signed envelope is HMAC-SHA256 over method, path, namespace, a SHA-256 of
the body and a timestamp; the border can verify intent and freshness without
the WAN front being able to choose a destination.

## Build and install (lightweight, multi-OS)

Pure standard library in the binary; the isotope dependency is test-only (the
spec gate). Cross-compile with `CGO_ENABLED=0`, so each artifact is one static
file:

```sh
./scripts/build-wan-ingress.sh          # darwin/linux × amd64/arm64 + SHA-256SUMS
```

Install is copying one file and writing one JSON config plus its agreement file.
No runtime and no sidecar. For crash recovery, install it under the host's own
supervisor — the binary does not restart itself:

```sh
BINARY=/path/to/wan-ingress CONFIG=/path/to/config.json \
  ./scripts/install-wan-ingress.sh
```

That writes a launchd `LaunchAgents` plist (`RunAtLoad` + `KeepAlive`) on macOS
or a systemd user unit (`Restart=always`) on Linux. `SIGTERM`/`SIGINT` drain the
listener and send the registry a delete; a hard crash leaves the lease to expire
and lets the supervisor bring the process back.

The registry name is a lease, not a label: it is renewed every `-heartbeat`
while serving, a delete is attempted on clean shutdown, and the TTL expires it
on a crash. A failed beat is logged and retried, never fatal — a registry outage
cannot take the front down. smoke-alarm has no delete route yet
([smoke-alarm#112](https://github.com/james-gibson/smoke-alarm/issues/112)), so
the delete is best-effort and the 5m TTL is the authority until that lands.

## Verification

- `sheets/ingress/wan-ingress.feature` is the origin (isotope
  `8R0ZXcvOD_Z0xdQ0qGKEuA9ZlfLTGfF6rCxm4W_tF9o`, 19 parts); `go test
  ./internal/ingress` inverts it with `httptest`, including the capturing
  border, the probe handshake, the registry lease, and the reload switch.
- `sheets/ingress/spike.sh` is the rung-2 check: the lab's own smoke-alarm and
  adhd judge the transitions over the wire.
- `go test ./...` at the repo root keeps the policy manifest check honest.
- Five `testing.F` targets: `FuzzResolveName`, `FuzzQuarantineContract`,
  `FuzzBorderForward`, `FuzzCanonicalize`, `FuzzConfig`.
  [`.github/workflows/wan-ingress.yml`](../../.github/workflows/wan-ingress.yml)
  runs a 30s smoke per target on every PR, a 10m nightly per target on Linux
  and macOS runners, and the four-artifact static cross-build. Crashers become
  `testdata/fuzz/` regressions and update the feature, not just the code — the
  `"0 ."` non-idempotence and the `%25` reparse failure found by
  `FuzzCanonicalize` are the first two examples.
