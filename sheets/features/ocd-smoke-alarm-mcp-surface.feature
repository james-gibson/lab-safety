@lab-safety
@mcp-surface
@surface-ocd-smoke-alarm
@z0-physical
@domain-surface-safety
Feature: Safety Data Sheet — ocd-smoke-alarm hosted service MCP surface

  Executable form of sheets/ocd-smoke-alarm.md. Each Rule is one SDS section.
  Run against a live alarm started with Hosted.Enabled: point the endpoint at
  `POST <Hosted.ListenAddr>/mcp`. Canonicalisable by pickled-onions ->
  isotope.Canonicalize unmodified.

  Background:
    Given a live ocd-smoke-alarm hosted surface at "http://localhost:<port>/mcp"

  Rule: §1 Identification

    Scenario: the surface names itself as hosted and states its protocol
      When I call "initialize"
      Then serverInfo.name is "<service>-hosted-mcp"
      And protocolVersion is "2024-11-05"
      And the surface serves "POST /mcp"

  Rule: §2 Hazard identification

    Scenario: the surface is read-only
      When I call "tools/list"
      Then every declared tool is classed R
      And no declared tool is classed W, E, X or H

  Rule: §3 Composition

    Scenario: the cold inventory is exactly four read tools
      When I call "tools/list"
      Then the tool set is exactly:
        | smoke.health                |
        | smoke.tuner_list_channels   |
        | smoke.tuner_audience        |
        | smoke.tuner_caller_messages |

    Scenario: the surface also exposes one resource
      When I call "resources/list"
      Then the resource "hosted://status" is present

  Rule: §4 First-aid measures

    Scenario: a method wrong for the protocol is refused
      Given the endpoint speaks MCP
      When I call "session/setup"
      Then the error code is -32601

    @red
    Scenario: declared tools are invocable
      # BLOCKED: the hosted method switch has no tools/call arm (finding A)
      # Why this matters: tools/list advertises 4 tools that tools/call returns
      # -32601 for — declared but not served.
      When I call tools/call for "smoke.health"
      Then the response is a result, not -32601

  Rule: §5 Fire-fighting measures

    Scenario: the surface is opt-in
      Given "Hosted.Enabled" is not set
      Then no hosted surface listens

  Rule: §6 Accidental-release measures

    Scenario: a stray caller leaves a trace
      When a caller connects and disconnects
      Then a "disconnected" event is recorded for that protocol
      And the per-protocol counter is incremented

  Rule: §7 Handling and storage

    Scenario: no credential is required (trust is post-connection)
      When I call "smoke.health" with no Authorization header
      Then the call succeeds

    Scenario: no durable state is written
      When the surface restarts
      Then its event ring and counters start empty

  Rule: §8 Exposure controls

    Scenario: the read-model is uniform because the surface is R-only
      Given callers at rung 0 and rung 6
      When each calls "tools/list"
      Then both receive the same four tools

  Rule: §9 Physical and chemical properties

    Scenario: SSE is offered alongside POST
      When a client requests "GET /mcp" with "Accept: text/event-stream"
      Then the surface streams SSE events

  Rule: §10 Stability and reactivity

    Scenario: an unsupported method does not destabilise the surface
      When I call an unknown method
      Then the error code is -32601
      And subsequent calls still succeed

  Rule: §12 Ecological information

    Scenario: the surface makes no egress calls
      When I call "smoke.tuner_list_channels"
      Then no outbound request is made to Tuner by this surface

  Rule: §13 Disposal considerations

    Scenario: shutdown is bounded and leaves nothing behind
      When the alarm process stops
      Then the hosted server shuts down within "Hosted.ShutdownTimeout"
      And no hosted temp files remain

  Rule: §14 Transport information

    Scenario: the bind address is operator-controlled
      Given "Hosted.ListenAddr" is set to a loopback address
      Then the surface is reachable only on that address

  Rule: §16 Declared vs actual

    @red
    Scenario: listed tools are served, not merely advertised
      # BLOCKED: no tools/call arm (finding A)
      # Why this matters: a surface that lists callable tools it cannot call is
      # a false declaration to any client that trusts tools/list.
      When I call "tools/list"
      And I call tools/call for each declared tool
      Then each invocation returns a result rather than -32601
