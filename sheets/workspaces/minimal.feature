@lab-safety
@workspaces
@phase-0
@z0-physical
@domain-workspace-vfs
Feature: Workspace VFS — minimal contract (Phase 0 gate)

  The minimal VFS is defined here, in Gherkin, before any driver exists. If it
  cannot be expressed here, it is not in the minimal VFS.

  The tmpdir driver is the experiment; this suite is what it must satisfy, and
  the same suite must pass *unchanged* on the sqlite driver (register -> §4). An
  edit to make it pass is a leak, and a leak is evidence.

  Slowness: the fastest rung is the prototype's own green run. Each Rule names
  the slower validator that keeps it honest (register -> §1 and §3).

  One session. The prototype runs in exactly one session, id "42" — the same 42 the
  frame anchors on. The id is a *name*; the plane ("42" -> "-42i" -> "-42") is the
  *state* the store passes through. One session keeps the pair of stores the only
  pair, so the quarter-turn is not diluted across sessions.

  Background:
    Given a workspace rooted at a driver-managed store for session "42"

  Rule: §1 the on-disk conventions
    # slow: the same conventions re-read through the sqlite driver (rung 1)

    Scenario: the layout is the documented one
      Then the tree contains ".a2a/task.json"
      And the tree contains ".state/current"
      And the tree contains "messages/" with zero-padded entries
      And the tree contains "artifacts/{id}/parts/"
      And the tree contains "context/resources/**" and "context/tools/*@"
      And the tree contains "context/prompts/*.tmpl"

    Scenario: sequence numbers order as files
      When two messages are written in sequence
      Then "ls messages/" lists them in zero-padded order
      When two artifact parts are written in sequence
      Then "ls artifacts/{id}/parts/" lists them in zero-padded order

  Rule: §2 the MCP mapping
    # slow: smoke-alarm MCP spec probe on the live surface (rung 2)

    Scenario: resources are files
      Given the resource file "context/resources/workspace/config.json"
      Then "resources/list" includes "resource://workspace/config"
      And "resources/read" returns the bytes of that file
      And the set from "find context/resources" equals the set from "resources/list"

    Scenario: tools are files
      Given an executable endpoint "context/tools/echo@"
      Then "tools/list" includes "echo"
      And the set from "ls context/tools/*@" equals the set from "tools/list"

    Scenario: tools/call is a file write plus a result read
      When "tools/call" is invoked for "echo" with an input
      Then the result is read from the tool's result path
      And no transport other than the file is used

    Scenario: prompts are templates
      Given the template "context/prompts/summarize.tmpl"
      When "prompts/get" is called for "summarize"
      Then the rendered template is returned

  Rule: §3 the state machine is driven by file writes
    # slow: ResultLedger history over many runs (rung 4)

    Scenario: a message write moves SUBMITTED to WORKING
      Given ".state/current" is "SUBMITTED"
      When "messages/00000001.user.json" is written
      Then ".state/current" becomes "WORKING"
      And ".state/transitions.log" records the transition

    Scenario: a final artifact moves WORKING to COMPLETED
      Given ".state/current" is "WORKING"
      When the final artifact is written under "artifacts/"
      Then ".state/current" becomes "COMPLETED"

  Rule: §4 access is scoped per session, downward only
    # slow: consensus across independent alarm instances (rung 3)

    Scenario: the prototype runs a single session, id "42"
      Given the prototype opens exactly one session
      Then the session id is "42"
      And the session's store is "42.wsdb"

    Scenario: a descendant gains the session's access
      Given the session store is opened for "42"
      When an agent is added beneath "42"
      Then that agent may read "42"'s store

    Scenario: access never propagates upward
      Given the session store is opened for "42"
      When an agent above "42" attempts to read "42"'s store
      Then the read is refused
      And "42"'s store is not listed to the ancestor

  Rule: §5 commands are dynamic (registered at runtime)
    # slow: smoke-alarm re-probe sees the changed surface (rung 2)

    Scenario: a tool dropped in at runtime becomes listable and callable
      Given "context/tools/" contains no tool named "late"
      When the file "context/tools/late@" is written
      Then "tools/list" includes "late"
      And "tools/call" for "late" returns without a restart

    Scenario: a tool removed at runtime stops being listed
      Given "context/tools/echo@" exists
      When "context/tools/echo@" is removed
      Then "tools/list" no longer includes "echo"

  Rule: §6 commands replicate across hosts, and the mirror records the comparison
    # slow: consensus across hosts and ResultLedger over time (rungs 3-4)

    Scenario: the same command runs on two hosts and agrees
      Given a command "echo" declared in "context/tools/echo@"
      When it is invoked on the tmpdir host and on the sqlite host
      Then both return the same result
      And the agreement is recorded, not assumed

    Scenario: the mirror holds the isotope record of the run
      Given a run completed on the tmpdir host
      When the sqlite mirror is read
      Then it contains the isotope record for that run
      And a log trace without an isotope record is a discrepancy

    Scenario: the mirror is independent of the traffic log
      Given the traffic log of the run
      And the isotope records in the mirror
      Then the two are independent measurements
      And their disagreement is the detection signal

  Rule: §7 the resulting surface is verifiable by the lab
    # slow: the smoke-alarm verdict is authoritative (rung 2/3)

    Scenario: the surface passes MCP spec compliance
      Given the VFS surface is registered as a smoke-alarm target
      Then smoke-alarm classifies it as "mcp"
      And the spec-compliance check passes

    Scenario: every fast claim names a slower validator
      Given the Phase-0 suite passes on the tmpdir driver
      Then each Rule names the slower validator that audits it
      And the register records which slower rungs have actually run

  Rule: §8 rotation tracing is the feature, surfaced on the lab's instruments
    # slow: the smoke-alarm test is the authority; the light is the record
    # finding (2026-10-16): headless adhd renders a target only green/red, so the
    # -42i light reads green today — yellow has no producer without a dissent
    # signal (register §2). The Rule stands; the instrument owes the rung.

    Scenario: an operation is traced with its plane and rotation
      When "tools/call" is invoked for "echo"
      Then the operation is traced with its plane ("42", "-42i" or "-42")
      And the operation is traced with the quarter-turn it performs

    Scenario: the rotation is visible as an adhd status light
      Given a store that does not exist
      Then its light is dark (plane 42)
      When the store exists
      Then its light is yellow (plane -42i)
      When the store works
      Then its light is green (plane -42)

    Scenario: a smoke-alarm test asserts the rotation
      Given the surface is a smoke-alarm target
      When the quarter-turn completes
      Then the smoke-alarm test passes on the transition
      And the resulting light matches the plane

    Scenario: OTel debugs but does not decide
      Given an operation with a span but no light and no alarm test
      Then it does not count as evidence

    Scenario: a scratch script may prime the loop but is not a deliverable
      Given a stand-in scratch script driving the first transitions
      Then it may prime the loop
      And it is removed once the lights and alarm tests carry the evidence

    Scenario: a feature that needs numbers is a finding, not a build item
      Given a prototype feature that needs numbers to fit the frame
      Then it is recorded as a finding
      And it is not built until it fits qualitatively
