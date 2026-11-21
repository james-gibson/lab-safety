# wan-ingress — design record (issue #38)

A single static Go binary: a thin WAN-facing front that resolves arbitrary
inbound names into **agreed** cluster addresses and namespaces, and forwards
only through a signed border. It does not route by guessing.

## The frame: do / know / see

| | |
| --- | --- |
| **do** | accept HTTP on an address, parse a name, resolve it by agreement, sign and forward one request to the agreed border, expose health/status |
| **know** | the operator-supplied agreements (id, aliases, namespace, border address), the WAN token, the border signing key, the smoke-alarm registry URL |
| **see** | inbound decisions (`forward`, `quarantine` + typed reason), `/status` counters, an optional registration in the lab's isotope registry, the border's own record of what it received |

No node reports on itself: the binary publishes counters, but a smoke-alarm
target and an adhd light must do the judging. The counters are the evidence a
separate process reads, not a claim of health.

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
| `-addr` | `127.0.0.1:8443` | bind address. `0.0.0.0` is a bind, never a destination; bind wider only explicitly |
| `-agreements` | *empty* | JSON file of agreed mappings; with none, every name is `unknown` |
| `-token` | *empty* | bearer token for WAN admission; with none, every caller is `untrusted` |
| `-border-key` | *empty* | HMAC key for the signed border envelope; with none, forwarding is `border_unconfigured` |
| `-smoke-alarm` | *empty* | registry URL; when set, register by name as `wan-ingress` (same shape as `lab/workspaces/advertise.go`) |

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

Install is copying one file and writing one JSON agreement. No service manager,
no runtime, no sidecar; `lezz.go`'s self-update/install patterns remain the
lab-level choice for distribution.

## Verification

- `sheets/ingress/wan-ingress.feature` is the origin; `go test ./internal/ingress`
  inverts it with `httptest` (including the capturing border).
- `go test ./...` at the repo root keeps the policy manifest check honest.
- Five `testing.F` targets: `FuzzResolveName`, `FuzzQuarantineContract`,
  `FuzzBorderForward`, `FuzzCanonicalize`, `FuzzConfig`.
  [`.github/workflows/wan-ingress.yml`](../../.github/workflows/wan-ingress.yml)
  runs a 30s smoke per target on every PR, a 10m nightly per target on Linux
  and macOS runners, and the four-artifact static cross-build. Crashers become
  `testdata/fuzz/` regressions and update the feature, not just the code — the
  `"0 ."` non-idempotence and the `%25` reparse failure found by
  `FuzzCanonicalize` are the first two examples.
