@lab-safety
@ingress
@wan
@border
Feature: WAN ingress — the resolver forwards only by agreement

  The origin of the binary requested in lab-safety#38. This file is the
  declaration; the implementation is its inversion. The gates recorded on the
  issue fix the meaning of two contested words: "cluster" is the lab's
  local-first mDNS cluster, reached only through a signed border, and
  "namespace" is the lab's session/workspace namespace. The resolver accepts
  essentially anything, but it derives no address and invents no id: it either
  finds an agreement or quarantines the input with a typed refusal.

  Provenance — requirements and decisions this converts:
    issue #38 body + feature requirements;
    issue comment 7052875000 (G1 WAN front -> lab border, G2 quarantine);
    issue comment 7052901595 (fuzzing plan: total function, no synthesis).

  Scenario: a known name resolves to an agreed border address and namespace
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "curl -sS '<addr>/resolve?name=42' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the body has status "200"
    And the JSON field "decision" is "forward"
    And the JSON field "namespace" is "session-42"
    And the JSON field "address" is the agreed border address

  Scenario: an unknown name is quarantined without a route
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "curl -sS -w '\n%{http_code}' '<addr>/resolve?name=nowhere' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the final line is "422"
    And the JSON field "reason" is "unknown"
    And the body has no "address" field
    And the body has no "namespace" field

  Scenario: an ambiguous name is quarantined rather than guessed
    Given a fresh process where two agreements both claim the name "42"
    When I run "curl -sS -w '\n%{http_code}' '<addr>/resolve?name=42' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the final line is "422"
    And the JSON field "reason" is "ambiguous"

  Scenario: an untrusted caller is refused before resolution
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "curl -sS -w '\n%{http_code}' '<addr>/resolve?name=42'"
    Then the command exits 0
    And the final line is "403"
    And the JSON field "reason" is "untrusted"

  Scenario: a malformed name is quarantined by the parser
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "curl -sS -w '\n%{http_code}' '<addr>/resolve?name=%00' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the final line is "422"
    And the JSON field "reason" is "malformed"

  # Fuzz regression 617c154ecf3f62e2: a one-pass trim left a trailing space,
  # so reading the same name twice produced two different keys.
  Scenario: a trailing-dot name canonicalizes to its agreed key
    Given a fresh process with an agreement mapping "0" to namespace "session-42" at an agreed border
    When I run "curl -sS '<addr>/resolve?name=0%20%2e' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the body has status "200"
    And the JSON field "decision" is "forward"
    And the JSON field "namespace" is "session-42"

  # Fuzz regression 5f8f3f6c02d5868c: "%25" decoded to "%" and then stopped
  # being a valid raw name on the next read. One decode, one interpretation.
  Scenario: an encoded percent is quarantined rather than reparsed
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "curl -sS -w '\n%{http_code}' '<addr>/resolve?name=%2525' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the final line is "422"
    And the JSON field "reason" is "malformed"

  Scenario: a known name reaches only the agreed border with a signature
    Given a fresh process with a capturing border and an agreement mapping "42" to namespace "session-42"
    When I run "curl -sS -X POST '<addr>/forward?name=42' -H 'Authorization: Bearer <token>' -H 'X-Lab-Address: http://attacker.invalid'"
    Then the command exits 0
    And the capturing border received exactly one request
    And the request reached the agreed border address
    And the request carried a non-empty signature header
    And no request reached the caller-supplied address

  Scenario: a known name is quarantined when no border key is configured
    Given a fresh process with an agreement but no border key
    When I run "curl -sS -w '\n%{http_code}' -X POST '<addr>/forward?name=42' -H 'Authorization: Bearer <token>'"
    Then the command exits 0
    And the final line is "422"
    And the JSON field "reason" is "border_unconfigured"
    And no request reached the agreed border

  Scenario: health answers without any agreements
    Given a fresh process with no agreements and no border key
    When I run "curl -sS '<addr>/healthz'"
    Then the command exits 0
    And the body is "ok"

  Scenario: a quarantine is visible on the status surface
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I run "sh -c 'curl -sS \"<addr>/resolve?name=nowhere\" >/dev/null; curl -sS \"<addr>/status\"'"
    Then the command exits 0
    And the JSON field "quarantined.unknown" is "1"

  # The ingress is a node in the lab, not an anonymous port. The monitor is the
  # lab's own smoke-alarm and the light is adhd's view of the alarm's target.
  # A defensive ingress answers but promises no forwarding: the declared
  # min_capability is absent, so the alarm reads degraded and adhd reads yellow.
  Scenario: a defensive ingress is degraded to the monitor
    Given a fresh process with no agreements and no border key
    When I run "curl -sS -X POST '<addr>/mcp' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}'"
    Then the command exits 0
    And the JSON field "result.serverInfo.name" is "wan-ingress"
    And the body has no "result.capabilities.ingress" field

  Scenario: a forwarding ingress is healthy to the monitor
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border and a completed forward
    When I run "curl -sS -X POST '<addr>/mcp' -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}'"
    Then the command exits 0
    And the JSON field "result.capabilities.ingress" is "forward"

  Scenario: a stopped ingress is unhealthy to the monitor
    Given a smoke-alarm target watching a fresh ingress
    When I stop the ingress process
    Then the alarm classifies the target as "unhealthy"

  # A registry name is a lease: renew it while serving, ask the registry to drop
  # it on a clean exit, and let a crash expire it. The beat retries; it is never
  # fatal. Until smoke-alarm#112 lands, the delete is best-effort and the TTL is
  # the authority — the scenario asserts the request, not a guarantee.
  Scenario: the registry name is renewed while the ingress serves
    Given a fresh process with a recording smoke-alarm registry and a heartbeat interval of one millisecond
    When I wait for two heartbeat intervals
    Then the registry received at least two registrations for "wan-ingress"

  Scenario: a clean shutdown asks the registry to drop the ingress name
    Given a fresh process with a recording smoke-alarm registry
    When I stop the process with SIGTERM
    Then the registry received a deletion for "wan-ingress"

  # Traffic is added later. SIGHUP swaps the config in place; a bad config is
  # refused without taking the listener down, so the last good policy survives.
  Scenario: a rejected reload keeps the last good agreements
    Given a fresh process with an agreement mapping "42" to namespace "session-42" at an agreed border
    When I send SIGHUP after replacing the agreements with invalid JSON
    Then I run "curl -sS '<addr>/resolve?name=42' -H 'Authorization: Bearer <token>'"
    And the JSON field "decision" is "forward"

  Scenario: a reload adds an agreed namespace without a restart
    Given a fresh process with no agreements
    When I send SIGHUP after adding an agreement mapping "42" to namespace "session-42" at an agreed border
    Then I run "curl -sS '<addr>/resolve?name=42' -H 'Authorization: Bearer <token>'"
    And the JSON field "decision" is "forward"

  Scenario: the service supervisor restarts a crashed ingress
    Given a launchd service definition for "wan-ingress"
    When I inspect the service definition
    Then the definition sets RunAtLoad to true
    And the definition sets KeepAlive to true
