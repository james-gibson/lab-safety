# Safety Data Sheet — adhd MCP server

> **`adhd.*`** · Streamable-HTTP JSON-RPC · `POST <addr>/mcp` (default `:9090`) ·
> protocol `2024-11-05` · owner `github.com/james-gibson/adhd`
>
> Executable scenarios: [features/adhd-mcp-surface.feature](features/adhd-mcp-surface.feature) ·
> Sheet rev 1 · surface as served by `internal/mcpserver/server.go`

## §1 Identification

| field | value |
| --- | --- |
| surface id | `adhd` (dashboard & MCP server) |
| owner repo | `github.com/james-gibson/adhd` |
| endpoint | `http://<host>:9090/mcp` (`--mcp-addr` override; `mcp_server.enabled`) |
| transport | `POST /mcp` = JSON-RPC 2.0; `GET /mcp` = SSE |
| protocol version | `2024-11-05` |
| server name | `adhd` (from `initialize` → `serverInfo`) |
| instance identity | `adhd.isotope.instance`; advertised `_adhd-isotope._tcp` |

## §2 Hazard identification

Classes present: **`R`, `C`, `W`, `E`, `X`, `H`** — i.e. every class. This is a
**hazardous surface**: it can run local processes (`X`) and reach arbitrary
third-party endpoints (`E`), so it should not be exposed below a rung requiring
independent verification. Safety-relevant exposure is concentrated in a minority
of tools (§11); the surface as a whole is read-oriented (§3).

Status — *declared* that such a surface is gated; **not enforced** by the surface
itself today (see §15, §16).

## §3 Composition

Inventory as served (`tools/list`). Class per the sheet legend.

| # | tool | class | inputs that matter | touches |
| --- | --- | --- | --- | --- |
| 1 | `adhd.status` | `R` | — | light summary, peers, paths, dynamic tools, rung |
| 2 | `adhd.lights.list` | `R` | — | all lights |
| 3 | `adhd.lights.get` | `R` | `name` | one light |
| 4 | `adhd.isotope.status` | `R` | — | role, topology |
| 5 | `adhd.isotope.peers` | `R` | — | peer isotopes + rungs |
| 6 | `adhd.isotope.instance` | `R` | — | own isotope id |
| 7 | `adhd.rung.respond` | `C` | `feature_id`, `nonce` | computes a receipt |
| 8 | `adhd.rung.verify` | `C` | `receipt`, `feature_id`, `nonce` | verifies a receipt |
| 9 | `adhd.rung.challenge` | `E` | `target`, `feature_id`, `nonce` | peer MCP server |
| 10 | `adhd.cluster.join` | `W` | `name`, alarms, `projects`, `honeypot-tools` | registry + **installs dynamic + `H` tools** |
| 11 | `adhd.cluster.leave` | `W` | `name` | removes peer + its tools |
| 12 | `adhd.hurl.run` | `X` | script body | runs `hurl` on the host |
| 13 | `adhd.gh.run` | `X` | `args`, `timeout` | runs `gh` on the host |
| 14 | `adhd.dependabot.alerts` | `R`,`E` | `repo`, `state` | GitHub via `gh` |
| 15 | `adhd.path.negotiate` | `E`,`W` | `target.isotope`, `feature_id` | multi-hop peers |
| 16 | `adhd.path.verify` | `E` | `target.isotope`, `feature_id` | peers |
| 17 | `adhd.path.list` | `R` | `target.isotope` | negotiated paths |
| 18 | `adhd.proxy` | `E` | `target_endpoint`, `auth`, `call` | **any endpoint, optional creds** |

**Dynamic tools** (registered at runtime by `adhd.cluster.join`, not present in a
cold `tools/list`):

| tool | class | note |
| --- | --- | --- |
| `adhd.project.<name>.call` | `E` | proxies JSON-RPC to the project's `mcp_url` |
| `adhd.dependabot.<owner>.<repo>` | `R`,`E` | per-repo alerts tool |
| decoys from `honeypot-tools` | `H` | plausible fake result; caller logged |

## §4 First-aid measures

| symptom | code | meaning | recovery |
| --- | --- | --- | --- |
| unknown tool | `-32601` | not in static or dynamic inventory | re-`tools/list` (dynamic tools appear only after a join) |
| malformed params | `-32602` | required field missing/typed | read the tool's `inputSchema` |
| target unreachable | `-32603` | proxy could not reach target | check DNS/egress (§14) |
| auth rejected | `-32002` | bearer / api-key / oauth2 token refused | re-issue credential |
| forbidden | `-32003` | target denied | scope/permission |
| not found | `-32004` | target URL 404 | fix `target_endpoint` |
| bad request | `-32005` | target rejected call shape | fix the forwarded `call` |
| target 5xx | `-32006` | target error | retry/leave |
| unavailable | `-32007` | target 503 | retry/leave |
| rate limited | `-32008` | target 429 | back off (rate policy is the alarm's, §5) |

`-32002`–`-32008` come from the proxy path (`internal/proxy/proxy.go`). Status:
**enforced**.

## §5 Fire-fighting measures

| control | who owns it | status |
| --- | --- | --- |
| rate limiting on proxy paths | `ocd-smoke-alarm` enforces; `lab-safety` audits | declared (alarm-side) |
| proxy-path registration gate | `lab-safety` must acknowledge each registration before the path is live | **declared, not implemented** |
| kill switch | `adhd.cluster.leave` removes a peer and its dynamic tools; process stop removes the surface | enforced (leave); declared (gate) |

## §6 Accidental-release measures

An untrusted caller is contained, not crashed:

- invoking a registered `H` decoy returns a plausible fake result and **does not
  error**, so the caller learns nothing;
- the event fires a `warn` security alert carrying the caller IP, and is logged.

Status: **enforced** (`honeypot` dispatch in `server.go` → `fireSecurity`).

## §7 Handling and storage

- **Inbound auth:** none. Any reachable process may call the surface; trust is
  *post-connection* (the isotope protocol's whole premise).
- **Outbound auth:** optional, per call, on `adhd.proxy` only:
  `type: bearer | api-key | oauth2`. `oauth2` is currently a bearer token — no
  refresh, scope or exchange (see #7). Status: enforced-with-caveat.
- **Acknowledgements:** every proxy registration an alarm makes is expected to be
  acknowledged by `lab-safety` before it goes live. Status: **declared, not
  implemented**.

Minimum rung by class (declared): `R`/`C` → rung 0; `W` → rung 3; `E` → rung 4;
`X` → rung 6. Status: **not enforced** (§16, finding B).

## §8 Exposure controls

What a caller at each rung *should* be able to read. Read-model is declared; only
§7's status applies.

| rung | may read | may not |
| --- | --- | --- |
| 0 Absolute Zero | `adhd.status`, own `adhd.isotope.instance` | peer identities, paths, dynamic tools |
| 1 Read-Only | `adhd.lights.*`, `adhd.isotope.status` | writes, egress |
| 2 Harness Tools | + `adhd.rung.*` | egress |
| 3 Mock Secrets | + cluster **leave** | `join`, `X`, `E` |
| 4 Higher Authority | + `E` tools (`path.*`, `proxy`, `dependabot`) | `X` |
| 5 Delegation | + `adhd.hurl.run`, `adhd.gh.run` (`X`) | — |
| 6 Certification | everything, including certifying peers | — |

Status: **declared, not enforced** — today the surface returns all tools to any
caller (finding B).

## §9 Physical and chemical properties

| property | value |
| --- | --- |
| framing | JSON-RPC 2.0; SSE on `GET /mcp` |
| statelessness | per-call stateless; the *only* durable surface state is the dynamic tool set + peer/path registry |
| address | `:9090` default; `--mcp-addr :0` for ephemeral |
| enabled by | `mcp_server.enabled`, or auto-enabled when `--mcp-addr` is set |
| cold inventory | 18 static tools; grows only via `adhd.cluster.join` |

## §10 Stability and reactivity

| condition | behaviour |
| --- | --- |
| peer unreachable | resolution skipped; other peers unaffected |
| remote feature batch | skipped in headless (TUI-only) |
| corrupt JSONL line | skipped by the log scanner |
| replayed receipt | `adhd.rung.verify` returns `valid:false` (nonce mismatch) |
| duplicate tool registration | silently ignored (idempotent) |

Status: **enforced**.

## §11 Toxicological information

The hazardous minority, in detail:

- **`adhd.hurl.run` (`X`)** — executes `hurl` on the host with a caller-supplied
  script. Arbitrary HTTP from the host's network position.
- **`adhd.gh.run` (`X`)** — executes `gh` with caller-supplied args. Reaches
  whatever the ambient `gh` credentials may reach.
- **`adhd.proxy` (`E`)** — forwards an arbitrary JSON-RPC call to an arbitrary
  `target_endpoint`, with optional credentials. **One HTTPS hop**, no alarm
  monitoring (`τ` probe, eviction, cycle cap), so the far side is bare-MCP
  availability behind one egress node. See #7.
- **`adhd.rung.challenge` / `adhd.path.*` (`E`)** — contact peer MCP servers.
- **`adhd.cluster.join` (`W`,`H`)** — writes registry state and can **install
  honeypot tools**; a caller that can join can plant decoys.

## §12 Ecological information

Outward reach of the surface:

| tool | reaches |
| --- | --- |
| `adhd.proxy`, `adhd.project.*` | any caller-specified MCP endpoint |
| `adhd.gh.run`, `adhd.dependabot.alerts` | github.com (via ambient `gh`) |
| `adhd.hurl.run` | any endpoint the script names |
| `adhd.rung.challenge`, `adhd.path.*` | peer MCP servers |

Nothing in the surface writes to disk beyond logs and the runtime registry.

## §13 Disposal considerations

- `adhd.cluster.leave` removes the peer, its project/repo tools and any decoys it
  planted — status enforced.
- Process stop withdraws the surface and its mDNS record.
- `adhd.path.*` state is in-memory; a restart clears negotiated paths.
- No surface-owned temp files to sweep.

## §14 Transport information

- **Egress precondition:** the surface host must resolve DNS and reach
  `target_endpoint`. A caller with blocked DNS still works **iff** it can reach
  this surface (the surface does the resolving).
- **Ingress:** plain HTTP; no TLS at the surface (TLS is the caller's/leaf's).
- One hop only: the surface does not chain proxies.

## §15 Regulatory information

- Every lab tool is an isotope; rungs come from `VerifyTrust`
  (registry+mDNS → 6; registry → 4; self → 2; none → 0).
- The `adhd.rung.*` trio lets peers challenge and verify receipts; receipts are
  nonce-bound and replay-rejected.
- The **lab-safety policy gate** — "no proxy path is live without its sign-off" —
  is this repo's charter and is **declared, not implemented**.

## §16 Other information

**Declared vs actual.** The surface's *own* feature suite names tools the server
does not serve:

| declared in | tool | in `tools/list`? |
| --- | --- | --- |
| `adhd/features/adhd/headless-mode.feature:72,78` | `adhd.features.list` | **absent** |
| `adhd/features/adhd/certified-features.feature` | `adhd.features.certified` | **absent** |
| `adhd/features/adhd/certified-features.feature` | `adhd.features.verify` | **absent** |
| `adhd/features/adhd/certified-features.feature` | `adhd.features.cluster` | **absent** |

Tracked as [#24](https://github.com/james-gibson/adhd/issues/24). This is the
defect an SDS exists to surface: the composition in §3 is derived from
`tools/list`, so every entry above is a **false declaration** in the source suite.

**Findings (not edited here — they belong to the owning tool):**

- **A — declared, absent:** `adhd.features.*` (§16 table).
- **B — undeclared enforcement gap:** classes `W`/`E`/`X` are returned to any
  caller regardless of rung; §7/§8 read-models are declared only.
- **C — auth caveat:** `oauth2` is a bearer token; no refresh/scope (#7).

**Revision.** Rev 1 — created with the MCP-surface SDS pattern. Derived from
`tools/list` and `internal/mcpserver/server.go` (`internal/proxy/proxy.go` for
§4/§11). Re-enumerate against a live instance on every reissue.
