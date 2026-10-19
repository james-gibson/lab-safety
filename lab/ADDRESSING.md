# Addressing: bind, destination, identity

**Status:** findings, merged from two historical design conversations (*Addressing
Microservice Network Paths*, *Domain Name Conventions For Localhost, Loopback, Or LAN*)
and reconciled against what this lab actually hit.

The problem: documenting a set of services that expose RPC on discrete networks, where
NAT may or may not exist and tunnels may or may not exist, and you need to name every
*path between* services rather than every service.

## Three concepts that must never be used interchangeably

| value | role | meaning |
| --- | --- | --- |
| `0.0.0.0` | **bind** address | listen on all interfaces. Never a valid destination. |
| `127.0.0.1` | loopback IP | leaves the OS stack nowhere; predictable. |
| `localhost` | DNS name | resolves to `127.0.0.1` **or** `::1` depending on config. Overridable. |
| `::1` | IPv6 loopback | `localhost` may resolve here instead. |

The load-bearing rule: **`localhost` is never a canonical address in documentation.**
It is a name requiring resolution, so it is subject to host-file and resolver
configuration and can differ between environments. `0.0.0.0` is server-side (bind);
the others are client-side (destination).

## A path is a vector, not an address

The same service is reachable by several distinct paths — loopback, LAN, NAT-translated,
proxy tunnel — and each is a *different* path to the same logical thing. So the unit of
documentation is the tuple `(source, destination, interface, protocol)`, not the address.

The community conclusion, and the one this lab adopted, is that **network addresses are
the wrong level of abstraction for service identity**. Identity should travel with the
workload, not with the interface.

## The conventional answers, in order of rigour

1. **SPIFFE / SPIRE.** A workload identity is an RFC 3986 URI —
   `spiffe://trust-domain/path/to/workload`. The trust domain scopes the network
   segment; the path is yours to define; the identity is cryptographically verifiable
   (X.509 SVID or JWT) and survives NAT and proxies. This is the principled answer: the
   path component *is* the vector label.
2. **Service mesh.** Logical service names plus explicit `source → destination`
   intentions (Consul), which model the vector directly. Consul network segments exist
   specifically for the no-full-mesh case.
3. **Hostname conventions.** The weakest option, and the only one that works with no
   infrastructure. It is what you fall back to, not what you design for.

## Special-use domains, and which are actually reserved

| domain | standard | behaviour |
| --- | --- | --- |
| `localhost`, `*.localhost` | RFC 2606, RFC 6761 | always loopback; resolvers MUST NOT forward upstream |
| `*.test` | RFC 2606, RFC 6761 | immediate negative; safe for dev |
| `*.invalid` | RFC 2606, RFC 6761 | always NXDOMAIN |
| `example.com` et al. | RFC 2606 | permanently IANA-registered, documentation only |
| `*.local` | RFC 6762 | **mDNS**, not unicast DNS (224.0.0.251 / FF02::FB : 5353) |
| `*.home.arpa` | RFC 8375 | residential; needs a local resolver; replaced informal `.home` |
| `*.internal` | IANA reservation, 2024 | corporate private-use; not delegated in the root |

Informal and **not** reserved: `.lan`, `.home`, `.corp`, `.localdomain`, `.intranet`.
These are in widespread use (most home routers default to `.lan`) and all carry
collision risk. `.corp` is flagged by ICANN as high-risk.

Two practical traps from the table: `.local` is mDNS, so using it as a unicast domain
breaks Apple clients; and `home.arpa` requires a resolver, so it is not a zero-config
choice.

## This lab's own instances of the bug

Recorded because each was found the hard way:

- **A LAN address on the wrong subnet.** `~/.config/zed/settings.json` pointed at
  `192.0.2.224` while the host is on `198.51.100.224` — a dead endpoint *and* an
  unauthenticated control surface reachable from the LAN (the offence tracked as
  adhd#27).
- **A port that moved and a config that did not.** The `adhd` MCP entry pointed at
  `127.0.0.1:59147` while the service listened on `9090`. Nothing detected it; the MCP
  server was simply unavailable, and the failure looked like a missing tool.
- **Scope mistaken for address.** Registering a nested repository changed which paths
  the code index answered for, with no error. The caller's belief about *where* it was
  searching and the thing that answered were different — see [`SYNAPSE.md`](SYNAPSE.md).

The pattern in all three: an address is a claim, and nothing in the format says whether
it is still true.

## What the lab should do with this

- Never write `localhost` in a config or a doc. Pick `127.0.0.1` or the LAN address
  deliberately, and say why.
- Name a path by `(source, destination, interface, protocol)`, and record the *name*
  (`adhd`, `synapse`) as the identity, with the address as an attribute that may change.
- Anything bound to a non-loopback interface is a trust decision, not a convenience
  one — which is what [`SPLASH.md`](SPLASH.md) §*Three admission layers* is about.
