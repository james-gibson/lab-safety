@lab-safety
@workspaces
@phase-0
@z0-physical
@domain-workspace-vfs
Feature: Workspace VFS — the session filesystem

  Every session is given its own filesystem, and that filesystem is its origin.
  This file defines the *roots*: where a session starts, where it originates, and
  what it defines at its root. The conventions *inside* the root are
  minimal.feature §1; this file says what "/" and "~/" are.

  The isolation is **structural**, not a permission check: a session cannot see a
  parent because there is no path above its own root — not because a rule refuses
  it. A rule can be misconfigured; a root cannot be walked out of.

  Background:
    Given a session "42"

  Rule: a session originates as "/"
    # slow: a second, independent session cannot see the first's paths (rung 3)

    Scenario: the session's root is its own
      Then the session's root is "/"
      And there is no parent above "/"
      And "/" is the session's own root, not a shared or host root

    Scenario: a path above the root does not resolve
      When the session requests "/.."
      Then the request is refused
      And it does not reach any path outside the session's own root

  Rule: a session starts in its home, "~/"

    Scenario: the session's home is its origin
      Then the session's home is "~/"
      And the session starts in "~/"
      And "~/" and "/" name the same place

    Scenario: a path under the home is a path under the root
      When the session reads "~/state"
      Then it reads "/state"
      And the two name one path

  Rule: the session's data, state and MCP tools are defined in "/"
    # slow: the same definitions re-read through the sqlite driver (rung 1)

    Scenario: the session defines its data, state and tools in the root
      Then the session's data is defined in "/"
      And the session's state is defined in "/"
      And the session's MCP tools are defined in "/"
      And the session's MCP resources and prompts are defined in "/"

    Scenario: the definitions sit in the root, and are the session's own
      Then "/data", "/state" and "/tools" are the session's definitions
      And no definition lives above "/"
      And no definition is shared with another session

    Scenario: the root is the container, not a definition
      Then "/" holds the session's definitions
      And "/" is not itself a data, state or tool path

  Rule: a child session does not see its parent
    # slow: consensus across independent sessions (rung 3)

    Scenario: a child session originates at its own "/"
      Given the parent session "42"
      When a child session originates beneath "42"
      Then the child's root is "/"
      And the child's "~/" is the child's own home
      And the child cannot see any path of the parent

    Scenario: the parent is not reachable from the child
      Given a child session of "42"
      When the child requests the parent's "/data"
      Then the request is refused
      And the parent is not listed to the child

  Rule: the two hosts present the same filesystem
    # slow: the Phase-0 suite passes unchanged on the sqlite host (rung 1)

    Scenario Outline: the roots are host-independent
      Given the session on the "<host>" host
      Then the session's root is "/"
      And the session's home is "~/"
      And "/data", "/state" and "/tools" are defined

      Examples:
        | host   |
        | tmpdir |
        | sqlite |
