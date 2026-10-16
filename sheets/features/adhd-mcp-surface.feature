@lab-safety
@mcp-surface
@surface-adhd
@z0-physical
@domain-surface-safety
Feature: Safety Data Sheet — adhd MCP surface

  Executable form of sheets/adhd.md. Each Rule is one SDS section, so a failing
  Rule reads as the section it invalidates. Run against a live surface: point the
  endpoint at `POST <addr>/mcp` (default :9090). Canonicalisable by
  pickled-onions -> isotope.Canonicalize unmodified.

  Background:
    Given a live adhd MCP surface at "http://localhost:9090/mcp"

  Rule: §1 Identification

    Scenario: the surface identifies itself and its protocol version
      When I call "initialize"
      Then serverInfo.name is "adhd"
      And protocolVersion is "2024-11-05"
      And the surface serves "POST /mcp"

    Scenario: the instance has a stable public identity
      When I call "adhd.isotope.instance"
      Then the response carries a non-empty isotope identifier

  Rule: §2 Hazard identification

    Scenario: the surface exposes the hazardous classes and is treated as hazardous
      When I call "tools/list"
      Then at least one tool is classed E (egress)
      And at least one tool is classed X (exec)
      And the surface is declared hazardous-to-expose

  Rule: §3 Composition

    Scenario: the cold inventory is exactly the declared static tools
      When I call "tools/list" on a cold surface
      Then the tool set equals the 18 tools declared in sheets/adhd.md §3
      And no tool is both declared and absent

    Scenario: hazardous tools carry their declared classes
      When I call "tools/list"
      Then "adhd.proxy" is classed E
      And "adhd.hurl.run" is classed X
      And "adhd.gh.run" is classed X
      And "adhd.cluster.join" is classed W and H
      And "adhd.cluster.leave" is classed W

    Scenario: dynamic tools appear only after a join
      Given a caller registers a project via "adhd.cluster.join"
      Then "adhd.project.<name>.call" appears in "tools/list"
      And before the join it does not

  Rule: §4 First-aid measures

    Scenario Outline: proxy failures map to a recoverable code
      When I call "adhd.proxy" and the target returns <status>
      Then the error code is <code>

      Examples:
        | status | code   |
        | 401    | -32002 |
        | 403    | -32003 |
        | 404    | -32004 |
        | 400    | -32005 |
        | 500    | -32006 |
        | 503    | -32007 |
        | 429    | -32008 |

    Scenario: a malformed required parameter is rejected, not guessed
      When I call "adhd.proxy" without "target_endpoint"
      Then the error code is -32602

    Scenario: an unknown tool is reported, not silently accepted
      When I call tools/call with name "adhd.features.list"
      Then the error code is -32601

  Rule: §5 Fire-fighting measures

    Scenario: the kill switch removes a peer and its tools
      Given a peer joined via "adhd.cluster.join"
      When I call "adhd.cluster.leave" for that peer
      Then the peer is no longer tracked
      And its dynamic tools are removed from "tools/list"

  Rule: §6 Accidental-release measures

    Scenario: invoking a honeypot does not reveal itself
      Given a decoy tool was registered via "honeypot-tools"
      When an untrusted caller invokes the decoy
      Then the response is a plausible result, not an error
      And a "honeypot_triggered" security warning is emitted with the caller IP

  Rule: §7 Handling and storage

    Scenario: the surface itself requires no credential (trust is post-connection)
      When I call "adhd.status" with no Authorization header
      Then the call succeeds

    Scenario: outbound oauth2 is a bearer token only
      When I call "adhd.proxy" with auth type "oauth2"
      Then the request carries "Authorization: Bearer <token>"
      And no token refresh or scope negotiation occurs

  Rule: §8 Exposure controls

    @red
    Scenario: hazardous classes are gated by rung
      # BLOCKED: the surface returns every tool to every caller today (finding B)
      # Why this matters: W/E/X are reachable below a rung that verifies the caller
      Given a caller at rung 0
      When I call "tools/list"
      Then E and X tools are absent from the response

  Rule: §9 Physical and chemical properties

    Scenario: the surface is stateless per call
      When I call "adhd.status" twice with no join in between
      Then "dynamic_tools" is 0 both times

    Scenario: the address is overridable for ephemeral runs
      Given adhd starts with "--mcp-addr :0"
      Then the surface listens on an OS-assigned port

  Rule: §10 Stability and reactivity

    Scenario: a replayed receipt is rejected
      Given a receipt bound to a nonce
      When "adhd.rung.verify" is called with a different nonce
      Then the result is valid=false

    Scenario: duplicate tool registration is idempotent
      Given a project is registered via "adhd.cluster.join"
      When the same project is registered again
      Then "adhd.project.<name>.call" appears exactly once

  Rule: §11 Toxicological information — the hazardous tools

    Scenario: proxy is a single unmonitored hop
      When I call "adhd.proxy" with a reachable "target_endpoint"
      Then the call is forwarded as one HTTPS hop
      And no alarm probe, eviction or cycle cap guards the target

  Rule: §12 Ecological information

    Scenario: egress is confined to caller-named targets
      When I call "adhd.gh.run" with args "api user"
      Then the only egress is to github.com via the ambient gh credential

  Rule: §13 Disposal considerations

    Scenario: leave withdraws everything the peer introduced
      Given a peer joined with projects and honeypot tools
      When I call "adhd.cluster.leave" for that peer
      Then no tools or peers from that join remain tracked

    Scenario: no surface-owned temp files are left behind
      When the adhd process stops
      Then no adhd-owned temp files remain

  Rule: §15 Regulatory information

    Scenario: receipts are nonce-bound
      When "adhd.rung.respond" computes a receipt for a feature and nonce
      Then the receipt verifies only against that feature and nonce

  Rule: §16 Declared vs actual

    @red
    Scenario: the served surface equals the declared surface
      # BLOCKED by https://github.com/james-gibson/adhd/issues/24
      # Why this matters: a suite that asserts tools the surface does not serve
      # is a false declaration — the defect an SDS exists to surface.
      When I call "tools/list"
      And I collect every tool named in adhd/features/adhd/*.feature
      Then every declared tool is present in the served inventory
      And no served tool is absent from the sheet
