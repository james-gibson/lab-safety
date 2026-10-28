@lab-safety
@mcp-surface
@surface-workspace
@z0-physical
@domain-surface-safety
Feature: Safety Data Sheet — workspace (VFS session) MCP surface

  Executable form of sheets/workspace.md. Each Rule is one SDS section. Run
  against a live `workspace-mcp` serving session 42: point the endpoint at
  `POST <addr>/mcp` (printed at startup; default `127.0.0.1:0`). Canonicalisable
  by pickled-onions -> isotope.Canonicalize unmodified.

  Background:
    Given a live workspace surface at "http://127.0.0.1:<port>/mcp" serving session "42"

  Rule: §1 Identification

    Scenario: the surface names its session and states its protocol
      When I call "initialize"
      Then serverInfo.name is "workspace-42"
      And protocolVersion is "2024-11-05"
      And the surface serves "POST /mcp"

    Scenario: GET answers identity, not MCP
      When I request "GET /mcp"
      Then the body holds "session": "42"
      And the body holds no "result"

    Scenario: health answers for the session
      When I request "GET /healthz"
      Then the status is 200
      And the body holds "ok": true

  Rule: §2 Hazard identification

    Scenario: the surface states what it is
      When I call "initialize"
      Then the capabilities are exactly "tools", "resources", "prompts"
      And the surface offers no egress method
      And the surface offers no process-execution method

  Rule: §3 Composition

    Scenario: the inventory is methods, not a tool snapshot
      When I call an unknown method
      Then the error code is -32601

    Scenario: tools come from files, so the list is data
      Given the file "/tools/echo@" exists
      When I call "tools/list"
      Then the tool set contains "echo"
      And the file "/tools/other@" does not exist
      And the tool set does not contain "other"

    Scenario: the seeded session declares one resource and one prompt
      When I call "resources/list"
      Then the resource "resource://workspace/config" is present
      And I call "prompts/list"
      Then the prompt "summarize" is present

  Rule: §4 First-aid measures

    Scenario: an unknown method is refused, and the refusal is an answer
      When I call "resources/frobnicate"
      Then the error code is -32601
      And the HTTP status is 200

    Scenario: an unknown resource is refused
      When I call "resources/subscribe" for "resource://workspace/nope"
      Then the error code is -32602

  Rule: §5 Fire-fighting measures

    Scenario: the bind is loopback and ephemeral by default
      Given the surface was started without "-addr"
      Then it listens on "127.0.0.1"
      And its port is not 80

    Scenario: deleting the session is the kill switch
      When the session store is deleted
      Then every method answers -32001
      And the process is still running

    Scenario: health does not outlive the session
      Given the session store is deleted
      When I request "GET /healthz"
      Then the status is 503
      And the body holds "ok": false
      And the process is still running

  Rule: §6 Accidental-release measures

    Scenario: the surface does not promise delivery
      When I call "initialize"
      Then resources.subscribe is false
      And resources.listChanged is false

    Scenario: a subscribe is answered as a real surface answers
      When I call "resources/subscribe" for "resource://workspace/config"
      Then the result is exactly "{}"
      And no error is returned

    Scenario: the knock is recorded
      When I call "resources/subscribe" for "resource://workspace/config"
      Then the contact ledger holds one entry
      And the entry names "resources/subscribe"
      And the entry's outcome is "accepted"
      And the entry's distance_42i is 32
      And the entry is marked "advertised": false

    Scenario: a refused URI is still contact
      When I call "resources/subscribe" for "resource://workspace/nope"
      Then the contact ledger holds one entry
      And the entry's outcome is "refused"

    Scenario: the answer does not reveal the decoy
      When I call "resources/subscribe" for "resource://workspace/config"
      Then the response names neither "honeypot" nor "contact" nor "decoy"

    Scenario: the decoy is invisible before it is touched
      Given no caller has subscribed
      Then no "/state/honeypot" exists
      When I call "resources/list"
      Then no "/state/honeypot" exists

  Rule: §7 Handling and storage

    Scenario: no credential is required (trust is post-connection)
      When I call "initialize" with no Authorization header
      Then the call succeeds

    Scenario: the ledger is session state and dies with the session
      Given a recorded contact
      When the session store is deleted
      Then the ledger is gone with the session
      And the surface answers -32001

  Rule: §8 Exposure controls

    Scenario: the read-model is uniform
      Given a caller at rung 0 and a caller at rung 6
      When each calls "tools/list"
      Then both receive the same inventory

  Rule: §9 Physical and chemical properties

    Scenario: the surface does not accept a batch
      When I POST a JSON-RPC batch
      Then the error code is -32700

    Scenario: the surface does not stream
      When a client requests "GET /mcp" with "Accept: text/event-stream"
      Then the response is JSON, not SSE

  Rule: §10 Stability and reactivity

    Scenario: losing part of the session narrows the promise
      Given the file "/tools/echo@" is deleted while the surface serves
      When I call "initialize"
      Then "tools" is not advertised
      And "prompts" is still advertised

    Scenario: a malformed subscribe is refused and recorded
      When I call "resources/subscribe" with no "uri"
      Then the error code is -32602
      And the contact ledger holds one entry with outcome "malformed"

  Rule: §11 Toxicological information

    Scenario: a direct call at the decoy is priced, not punished
      When I call "resources/subscribe" for "resource://workspace/config"
      Then the surface answers "{}"
      And the entry's distance_42i is 32
      And the entry names the caller's peer

    @red
    Scenario: a crossing alerts an observer outside the honeypotted node
      # BLOCKED: the surface records; it does not judge (finding B, #9 A7).
      # Why this matters: without a reader, "no alert" means "no detector",
      # not "no contact".
      Given a caller whose contacts exceed the decoy's sensitivity
      Then a security alert reaches the alarm
      And a second dashboard indicates it

  Rule: §12 Ecological information

    Scenario: no method makes an outbound request
      When I call every method in the inventory
      Then no request leaves the host
      And the only outbound traffic is the registry heartbeat

  Rule: §13 Disposal considerations

    Scenario: disposal is bounded and leaves nothing behind
      When the session store is deleted
      Then the surface answers -32001 on every method
      And the honeypot ledger is gone
      And no temp file outside the session root remains

  Rule: §14 Transport information

    Scenario: a name is resolved, not guessed
      Given the surface was started with "-smoke-alarm"
      Then it registers "workspace-42" in the registry
      And a monitor resolves the endpoint by name

  Rule: §15 Regulatory information

    Scenario: the declaration is the gate, and it is re-derived
      When the spec is canonicalised with "seed-42"
      Then the session publishes that isotope id as "contract"
      And the check re-derives it rather than trusting the record

  Rule: §16 Declared vs actual

    Scenario: the sheet's inventory is what the surface answers
      When I call each method the sheet declares
      Then each answers as the sheet says (§3, §4)
      And an undeclared method answers -32601

    @red
    Scenario: subscribe delivers
      # BLOCKED: this is A7's other half, and it does not exist (finding C).
      # Why this matters: a surface that answered subscribe and then pushed
      # notifications would be a working capability, not a decoy — and the
      # gate would have to declare it.
      When I call "resources/subscribe" for "resource://workspace/config"
      And the resource changes
      Then a "notifications/resources/updated" arrives
