# Historical design sources: what they were, and where they went

**Status:** register. Eighteen historical design artifacts — **ten chat exports and eight
printed conversations** — processed on 2026-10-19. Ten of the eighteen were read and
absorbed; eight are recorded here by title only, with the reason.

The originals are preserved unmodified in the knowledge base (see *Where the originals
live*, below), under their original filenames, so a later agent can re-read a claim in
context rather than trusting this summary.

This page exists so nobody re-derives or re-imports the same material. Each row says
what the source contained and, where it survived, which document now carries it.

## Read and absorbed

| source | held | now lives in |
| --- | --- | --- |
| *Scalability Concerns for Application Level IDs* (104 msgs) | single composite string vs structured columns; the leading-wildcard scan; `traceId`/`sourceId` conflation; the good-enough threshold; `KSUID_timestamp % 5` buckets and skew | [`CHIMERA.md`](CHIMERA.md) |
| *Algebraic Notation for String IDs* (6 msgs) | the identifier algebra: `🦄☁️.<source>:<trace>:<ksuid>`, sources as reversed base64 of other ids, merge chains, type safety, and the decoding debug path | [`CHIMERA.md`](CHIMERA.md) §*Identifier grammar* |
| *Distributed Event Handling and Traceability* (16 msgs) | node-first addressing; universes as first-class temporal nodes; per-universe epochs and precision scales; reference types and reality gradients; the four worked chains | [`TEMPORAL.md`](TEMPORAL.md) |
| *Addressing Microservice Network Paths* (2 msgs) | bind vs destination addresses; a path as a `(source, destination, interface, protocol)` vector; SPIFFE, service mesh, intentions | [`ADDRESSING.md`](ADDRESSING.md) |
| *Domain Name Conventions For Localhost, Loopback, Or LAN* (2 msgs) | the reserved vs informal TLD table, with RFCs; `.local` is mDNS; `home.arpa` needs a resolver | [`ADDRESSING.md`](ADDRESSING.md) |
| *DevOps System Architecture YAML Generation* (16 msgs) | the `do / know / see` reporting rule; nine worked nodes from nginx-static to a cron word-broadcaster | [`TOPOLOGY.md`](TOPOLOGY.md) |
| *Multi-Tenant MCP Platform Skill Prompt* (2 msgs) | a multi-tenant MCP SaaS architecture (SSO-isolated tenants, blue/green feature routing) plus a typed `Application / Service / Binding / Zone` model | [`TOPOLOGY.md`](TOPOLOGY.md) |
| *Bed and Breakfast Testing Scenario* (10 msgs) | a CLI world model with typed verbs, planes, transparent spaces, fixed-vs-movable items, and the "two truths and a lie" test method | [`SCENARIOS.md`](SCENARIOS.md) |
| *Rewrite Server Logs As Village Story* (6 msgs) | a real captured docker log corpus, and the idea of piping it through a local model to produce a narrative | the log corpus, below |
| *Naming Ideas for LLM Subdomain* (14 msgs) | puns for an LLM/MCP/ACP subdomain | nothing; recorded below so it is not re-imported |

### The log corpus

The village-story conversation carried an actual capture from a five-service stack, and
it is worth keeping as a fixture regardless of the narration idea:

- **SeaweedFS** master and volume servers — `volume.grow`, `volume.fix.replication`,
  `volume_layout.go: volume 3 does not have enough copies`, topology
  `DefaultDataCenter:0:203.0.113.17:8082`.
- **master_quorum** — `topo leader: leader not selected yet`, and
  `raft.Server: Not current leader`. A Raft cluster during leader election.
- **HAProxy** — terminate-mode log lines against an `ollama_front` /
  `ollama_backend/ollama-primary` pair, including a `cD` termination (client abort)
  with a 600-second timer.
- **step-ca** — structured JSON logs, `/health` returning 200 in 14.8 µs.
- **Ollama** — the inference endpoint the narration was aimed at.

The proposed mechanism: a script that collects docker logs and pushes them through the
Ollama API with a fixed prompt, so the log becomes prose a human will actually read. The
corpus is the valuable half; the prompt is one line to rewrite.

## Where the originals live

The originals are **not in this repository.** They live in the knowledge base, at
`~/projects/kb/personal/sources/`, split by whether they can be read:

```
kb/personal/sources/
  json/  × 10   912 KB   chat exports, machine-readable, re-processable
  ps/    ×  8    24 MB   Safari PostScript printouts, not text-extractable here
```

They were placed under `personal/` deliberately. The vault separates `public/` (the
published tree) from `personal/` (private), and this material should not be published:
it includes an authentication design, an overview of the lab's own monitor, and a
captured log corpus carrying internal addresses (`172.21.0.x`, `192.168.x`) and a Raft
cluster mid-leader-election. The same reason ruled out keeping them here —
`james-gibson/lab-safety` is a **public** repository, so anything committed to it is
published permanently, history included.

Two consequences of that placement:

- **The vault is not a git checkout.** There is no commit to make and no history to
  protect; tracking is whatever syncs `~/projects/kb`. That is a weaker guarantee than
  version control, and it is the reason the register on this page exists at all.
- **`ps/` is excluded from the code index** (`synapse_manage_repos` →
  `update_excludes: ["personal/sources/ps"]`). The printouts are hex-heavy subset-font
  data, so indexing them would cost a great deal and retrieve nothing. `json/` is *not*
  excluded: those exports contain the full design discussions and are worth searching.

## Not read

Eight conversations exist only as PostScript, printed from Safari
(`~/Documents/*.ps`, `cgpdftops` filter). **They are not text-extractable with the tools
on this machine.** The filter emits subset fonts with per-page encodings and draws text
through a custom `xS` (xshow) operator; glyph codes are page-local permutations of the
original glyph names. A partial decoder recovered page headers
(`10/18/26, 10 27 PM Page 1 of 6 about:blank`) and garbled the body, which is worse than
nothing, so it was abandoned rather than guessed at.

| source | pages | title suggests |
| --- | --- | --- |
| Smoke-Alarm Project Overview | 10 | an overview of the lab's own monitor — **highest value, most likely already covered by `SPLASH.md` / `FLOW.md`** |
| Authentication Security Matrix Explained | 22 | authn/authz matrix; likely belongs with [`SPLASH.md`](SPLASH.md) §*Three admission layers* |
| Short Code Address Space Calculation | 10 | short-code/link address space sizing; probably belongs with [`CHIMERA.md`](CHIMERA.md) §*good-enough threshold* |
| Documenting Moments: Social History Concepts | 10 | temporal/documentation concepts; probably [`TEMPORAL.md`](TEMPORAL.md) territory |
| Pathfinding Optimization and Multi-Agent Coordination | 17 | agent coordination — possibly relevant to the fan-out work |
| Space Colony Simulation MVP Pitch | 14 | unrelated product idea |
| OpenTTD Cargo Profit Calculation Class | 5 | a game mod calculation |
| Bridging Medieval and Sci-Fi UI | 6 | interface design |

**To recover them**, in order of cost: print or export the originals again as text or
PDF; or `brew install ghostscript` and run `ps2ascii`; or re-run the decoder with a
correct font-resource emulation, which is a PostScript interpreter subset and not worth
writing. The four marked as probably-covered should be checked against the existing docs
before any effort is spent.

## What processing changed

Three things were only discoverable by reading the sources against the existing docs:

1. **`ADDRESSING.md` names three failures this lab already hit** — a LAN address on the
   wrong subnet, a port that moved while the config did not, and scope mistaken for
   address. All three are the same defect: an address is a claim, and the format carries
   no truth.
2. **"Who they know" has no verification story.** Of the three reporting questions, only
   *who they see* is observable by a third party. The other two are self-reports — the
   same asymmetry `SYNAPSE.md` documents for `fully indexed`.
3. **The fixed-versus-movable rule is the same rule, three times.** A merge queue cannot
   be picked up because it is a process; a `Service` is not its `bindings`; a route is
   not the record that it carried. Each document had arrived at it separately.

## Open

- Six sources unread, listed above with a recommended reading order.
- `Naming Ideas` produced nothing worth keeping; it is recorded only so it is not
  re-imported.
- The village-story prompt was never applied to a fixture the lab can reproduce, because
  the stack it was captured from is not among this lab's exercises.
