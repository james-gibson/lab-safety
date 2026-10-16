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

  Background:
    Given a workspace rooted at a driver-managed store for session "s1"

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

    Scenario: a descendant gains the session's access
      Given the session store is opened for "s1"
      When an agent is added beneath "s1"
      Then that agent may read "s1"'s store

    Scenario: access never propagates upward
      Given the session store is opened for "s1"
      When an agent above "s1" attempts to read "s1"'s store
      Then the read is refused
      And "s1"'s store is not listed to the ancestor

  Rule: §5 the resulting surface is verifiable by the lab
    # slow: the smoke-alarm verdict is authoritative (rung 2/3)

    Scenario: the surface passes MCP spec compliance
      Given the VFS surface is registered as a smoke-alarm target
      Then smoke-alarm classifies it as "mcp"
      And the spec-compliance check passes

    Scenario: every fast claim names a slower validator
      Given the Phase-0 suite passes on the tmpdir driver
      Then each Rule names the slower validator that audits it
      And the register records which slower rungs have actually run
