# Topology: describing a system as nodes that report on themselves

**Status:** findings, merged from two historical design conversations (*DevOps System
Architecture YAML Generation*, *Multi-Tenant MCP Platform Skill Prompt*).

Both conversations attack the same problem from opposite ends: how do you write a
system down so that (a) a machine can read it, and (b) a *node* can describe itself in
the same vocabulary the description uses. The result is one format with two views.

## The reporting rule: do / know / see

Every node answers three questions, and only those three:

- **what they do** — intent plus platform mechanics. "Pushes real-time data when a
  payment succeeds; listens on the internal bus, formats a payload, signs it with HMAC,
  POSTs to a subscriber."
- **who they know** — configured state. Registries, subscriber lists, secret
  references, quotas. The things it holds *about* the world.
- **who they see** — network flow, both directions. Inbound triggers; outbound targets
  and the status codes they return.

The reason this is the right triple, and not "dependencies": *do* is a claim the node
makes about itself, *know* is state it owns, and *see* is evidence others could
independently have observed. Only the third is checkable by a third party — which is
the same split the trust model leans on ([`SPLASH.md`](SPLASH.md) §*Two invariants*).

**Output is YAML, and the consumer is another agent** that turns it into a diagram. So
the format optimises for being *expanded into context*, not for being compact.

## Worked examples the conversations developed

| node | what it does | the interesting part |
| --- | --- | --- |
| nginx static on `:8080`, no TLS | serves a local folder | the null case: one binding, one service, no zone crossing |
| docker swarm, 3 replicas, 3 hosts | runs a node.js server | one service name, three concrete bindings — the name/address split |
| proxy giving a stable address | fronts nginx + node | a service whose *identity* outlives its backends |
| port-checking script | scans known ports | ephemeral: `services: []`, but it still *sees* everything |
| multi-tenant MCP, blue/green | SSO-isolated tenants B and C, C pays for blue-only features | the trust boundary and the routing boundary are different axes |
| cron + UDP word-broadcaster | alerts when a file contains PII or a credential | egress-only node; the alert is the observable |
| webhook dispatcher | POSTs signed payloads on state change | signature = the "who they know" made checkable |
| `/.well-known/` endpoints | static metadata bootstrap | no state, no knowledge, deliberately public |
| sitemap / robots | discovery for crawlers | describes a system by *inviting* observation |

The behaviours at the end — webhooks, `/.well-known/`, sitemaps, health endpoints, CORS
preflight — are worth modelling because they *usually* ship together and are never
required to. They are the standard "seams" that make a deployed system legible.

## The type system (the modelling half)

The second conversation turned the same material into executable TypeScript, so that
every component, binding, connection and dependency is explicit and typed:

```
Application { name, zone, networkInterfaces, paths, services, bindings,
              neighbors, dependencies, connections, data }
Service     { name, endpoint{ direction, address, label }, contract? }
EndpointBinding { direction: 'ingress'|'egress', address, label }
AppConnection   { from, to, protocols, viaInterface? }
ApplicationDependency { application, requiredService, required, role }
Zone        { name, cidrs, managed, applications, connectedZones }
```

Four rules carry it:

1. **Zone = trust boundary.** Every application belongs to exactly one. External SaaS
   goes in a `managed: false` zone. Zones, not addresses, are what you reason about.
2. **Services are named capabilities**, not ports. One application may expose several on
   one port, differentiated by path or protocol. Other applications connect to
   *services*.
3. **Bindings are all I/O surfaces** — sockets, files, unix sockets — and each is
   labelled. This is where `0.0.0.0`-vs-`127.0.0.1` becomes a recorded decision rather
   than a default ([`ADDRESSING.md`](ADDRESSING.md)).
4. **Ephemeral processes have `services: []`.** A script is still an `Application` with
   bindings, neighbours and connections — it just exposes nothing. That is what makes
   the port-scanner and the cron job modellable in the same schema as the server.

## Why the two halves belong together

The YAML is what a human or model writes; the type system is what a compiler checks. A
node that can fill in `do / know / see` about itself is, in the type language, an
`Application` that can be asked for its `services`, `bindings` and `connections`. Same
fact, two representations — which is the rule this directory keeps arriving at: keep
both, and check them against each other rather than merging them into one that is wrong
for both purposes.

## Open threads from the source conversations

- The Heroku/Azure comparison (refining the template so a managed platform's implicit
  behaviours become explicit) was raised and not concluded.
- "Who they know" has no verification story. A node can claim a subscriber list it does
  not have; only "who they see" is externally observable.
- The blue/green multi-tenant case needs the tenant to appear in the *address*, not just
  the permission model, and no addressing scheme for that was settled here. See
  [`ADDRESSING.md`](ADDRESSING.md) for why an address is the wrong place to put it.
