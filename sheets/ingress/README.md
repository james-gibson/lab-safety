# WAN ingress — spec register

> Ticket: [lab-safety#38](https://github.com/james-gibson/lab-safety/issues/38) ·
> Implementation: [`internal/ingress/`](../../internal/ingress/) ·
> Origin: [`wan-ingress.feature`](wan-ingress.feature)

This directory is a **spec register**, not a set of MCP safety data sheets. It is
deliberately a subdirectory: `policy.LoadSheets` reads only `sheets/*.md`, so a
register here does not change the published manifest. The sheet remains the
declaration; the feature is the executable origin of the binary.

## Origin and identity

`wan-ingress.feature` is the origin: no code was written until it existed. It
was validated twice with the real isotope canon (via `brine.py`) and both runs
gave the same identity:

| field | value |
| --- | --- |
| isotope | `8R0ZXcvOD_Z0xdQ0qGKEuA9ZlfLTGfF6rCxm4W_tF9o` |
| tokens | 111 |
| parts | 19 |
| signature prefix (key `seed-42`) | `5205c3f5d2de34a8…` |

Parts 10 and 11 were added by fuzz crashers found in `FuzzCanonicalize`:
`"0 ."` was canonicalised with a trailing space, and `"%25"` decoded to `"%"` which
then failed to reparse. Both regression seeds live in
`internal/ingress/testdata/fuzz/FuzzCanonicalize/`.

Parts 12–19 make the front a member of the lab rather than an anonymous port:
the smoke-alarm handshake (deferred/healthy), the monitor's verdict when the
process dies, the registry lease (renew/deregister), SIGHUP reload (rejected
and accepted), and the host supervisor that restarts a crashed process. A
defensive front is **yellow**, not green: it answers, but advertises no
forwarding until agreed traffic has actually crossed the border.

Re-check with:

```sh
python3 ~/projects/personal/.opencode/skills/shell-to-gherkin/scripts/validate_spec.py \
  sheets/ingress/wan-ingress.feature \
  --brine ~/projects/kb/projects/scripts/brine
```

The Go gate (`spec_test.go`) re-reads the same file with
`isotope.ParseGherkin` + `CanonicalizeGherkinToIsotope`, so the declaration and
the binary's claim cannot drift apart.

## Gates fixed by the ticket

- **G1 — scope.** "Cluster" is the lab's local-first mDNS cluster; the WAN
  front forwards only through a signed, trusted border. It does not become an
  authority inside the lab.
- **G2 — resolver.** Accept essentially anything, but forward only by
  agreement. Unknown or ambiguous names are quarantined with a typed reason;
  addresses and namespaces are read from the agreement, never synthesised from
  the input.
- **Fuzzing.** The resolver is a total function: every input maps to a route or
  a typed refusal. See the five `testing.F` targets in `internal/ingress`.
- **Monitoring.** `configs/wan-ingress-alarm.yaml` declares the smoke-alarm
  target. `sheets/ingress/spike.sh` drives the real monitor and adhd light:
  `dark → yellow → green → red`, asserted from the monitor's own status and the
  light log, never from the front's `/healthz`.
- **No URL for visitors.** The front's listener URL is only a binding; its
  address is the isotope name `wan-ingress`. The harness's
  [`proxy/`](proxy/) stands in for another cluster's egress/alarms: it resolves
  that name from the registry (what a seed-42 invite ultimately points to) and
  forwards both the monitor probe and the data-plane call. No caller carries
  the ingress port.
- **Blue-team baseline.** [`blue_adhd_proxy_test.py`](blue_adhd_proxy_test.py)
  stands up two caller adhd proxies, a target adhd and a harness border. One
  caller reaches the target directly; the other reaches it by invite through
  wan-ingress and its signed border. Both call `adhd.proxy` with the same method,
  so only the boundary varies. The test asserts that an unsigned border copy and
  a caller without a token are refused, then reports latency with blue-latency's
  own rank test. `--connection-mode hold|new` additionally keeps or closes the
  ingress↔border and border↔target connections, exposing the connection-churn
  cost in the border's own `/stats`. Per the lab's playbook, latency is the
  **cost** of the boundary; the finding is whether trust is enforced.
  Run both connection modes to get the churn comparison:

  ```sh
  python3 sheets/ingress/blue_adhd_proxy_test.py --calls 60 --connection-mode hold --out /tmp/blue-proxy-hold.json
  python3 sheets/ingress/blue_adhd_proxy_test.py --calls 60 --connection-mode new  --out /tmp/blue-proxy-new.json
  ```
- **Self-healing.** The registry entry is a renewed lease with a best-effort
  delete on clean exit (the TTL remains authoritative until smoke-alarm#112);
  SIGHUP swaps bad config out without dropping the listener; a launchd/systemd
  unit (`scripts/`) gives crash recovery on the host.

## Operating it — network access is not enough

Installing the binary and opening a port buys you a listener that refuses
everything. The defaults are fail-closed, so a deployment needs operator-owned
inputs before it forwards a single byte:

| input | default | without it |
| --- | --- | --- |
| agreements | none | every name quarantines `unknown`; no address is ever guessed from the request |
| admission token | empty | every `/resolve` and `/forward` caller is `untrusted` (403) |
| border key and agreed `border` | empty | `border_unconfigured` (422); no payload crosses the boundary |
| smoke-alarm registry | empty | no `wan-ingress` lease, so nobody can address the front by name |
| a real border service | not shipped | nothing understands the signed envelope; `border_adapter.py` is only a test double |

`scripts/install-wan-ingress.sh` therefore has no usable default config: it
requires `CONFIG=...`. Copy
[`configs/wan-ingress.example.json`](../../configs/wan-ingress.example.json) and
[`configs/wan-ingress-agreements.example.json`](../../configs/wan-ingress-agreements.example.json)
as shapes, fill in the real token, HMAC key, border URL and registry, then install
that file. Only then does `/readyz` go green and `/mcp` advertise
`"ingress":"forward"` — and only after agreed traffic has actually crossed the
border.

## Rotational expectations — the address plane, not the border

The invite proxy in `proxy/` proves that a visitor never carries a URL. The
production-capable twin is [`cmd/seed-invite`](../../cmd/seed-invite/): it runs on
the edge just as it does on localhost, and adds one thing the harness proxy did
not need — a signed, tick-scoped expectation before it forwards.

The loop is:

1. A seed-addressed client signs a rotation record and publishes it as a DNS
   `TXT` record (WAN/prod) or an mDNS `TXT` announcement (localhost/LAN). The
   payload carries `seed`, `name`, `slot`, `rotation`, `tick`, `prev`, a one-shot
   `nonce` and an HMAC. DNS/mDNS only move the bytes; the seed-scoped HMAC is the
   authority.
2. `seed-invite` verifies the record, pins it to the configured seed, rejects
   old ticks and replays, and arms a one-shot expectation that lives for two
   ticks. A node configured with `tick_offset: 1` announces `N+1` while its peers
   are still on `N` — that phase skew is the evidence, not a timestamp.
3. The client calls with `X-Invite: wan-ingress` and
   `X-Invite-Ticket: <nonce>`. The expectation is consumed, the invite is
   resolved from the isotope registry, and the call is forwarded with the normal
   bearer grant. The shipped `wan-ingress` binary sees an ordinary agreed call;
   it knows nothing about DNS, mDNS, seed or slots.
4. The upstream monitor issues an inversion challenge:
   `GET /challenge?name=wan-ingress&rotation=…&tick=…`. The address plane checks
   its own view *and* each peer's `/status`. A claim from the ahead node is
   confirmed only when the peers' cluster tick catches up to the same rotation.

This is deliberately fail-closed:

| condition | result |
| --- | --- |
| no announcement for the invite | `403` before resolution |
| expired or replayed ticket | `403`, entry removed |
| invitation name does not match the announced name | `403` |
| no DNS domain and no mDNS source configured | service refuses to start |
| announcement key or seed missing | service refuses to start |
| peer missing from an inversion challenge | `confirmed: false` |

Try it locally with two shells (the announcer and the address plane share a key):

```sh
KEY="$(printf '0123456789abcdef0123456789abcdef' | base64 | tr -d '\n')"

# Shell 1 — edge address plane; no call reaches the ingress without a ticket.
go run ./cmd/seed-invite \
  -registry http://127.0.0.1:18080 -addr 127.0.0.1:18090 \
  -seed seed-42 -slot wan-ingress -announce-key "$KEY" \
  -mdns -tick-seconds 5 -tick-offset 0

# Shell 2 — the node one tick ahead publishes a signed rotation.
go run ./cmd/seed-announce -key "$KEY" -seed seed-42 \
  -slot wan-ingress -rotation rot-1 -mdns -tick-offset 1

# The visitor carries an invite and the nonce the announcer printed.
curl -H 'X-Invite: wan-ingress' -H 'X-Invite-Ticket: <nonce>' \
  -X POST http://127.0.0.1:18090/forward
```

For the prod WAN, swap `-mdns` for `-dns-domain _rot.seed-42.lab.example.` in
`configs/seed-invite.example.json` and publish the printed `TXT` body in that
zone. `scripts/install-seed-invite.sh` supervises the binary with launchd or
systemd; `scripts/build-wan-ingress.sh` emits static `seed-invite` and
`seed-announce` artifacts alongside `wan-ingress`. The feature file and the
registered isotope identity are unchanged — this is an address-plane addition,
not a new claim by the border.
