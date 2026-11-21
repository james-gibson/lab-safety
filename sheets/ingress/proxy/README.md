# invite proxy — the visitor's side of the WAN front

This is a **harness-only** stand-in for another cluster's egress or alarms. It
is not part of the shipped `wan-ingress` binary and adds no capability to it.

A visitor to the ingress does not carry a URL. It carries an **invite** — an
isotope name that, in the full lab, resolves through a seed-42 pointer chain.
This proxy resolves that name from the smoke-alarm isotope registry
(`GET /isotope/list`), then forwards the call to the name's current binding.
The URL exists only inside the harness; no caller needs to know it.

```sh
go run ./sheets/ingress/proxy \
  -addr=127.0.0.1:18081 \
  -registry=http://127.0.0.1:18080 \
  -name=wan-ingress \
  -token=<the ingress bearer token>
```

- `X-Invite: <isotope-name>` overrides `-name` per request, so a test can act as
  a peer egress with its own invite.
- A caller-supplied `Authorization` header is preserved; the configured token
  is injected only when the caller has none. Red-team auth and rate-limit tests
  therefore still reach the ingress unchanged.
- An unresolvable invite returns `502`; nothing is guessed and no URL is
  inferred from the request.

`sheets/ingress/spike.sh` builds this proxy and points the smoke-alarm target at
it. The monitor probes `POST /mcp` through the proxy, and the data-plane forward
is sent through it without a caller token — the invite supplies the grant.
That is the deliberate detachment: **name first, binding second**.
