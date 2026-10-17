@lab-safety
@workspaces
@phase-0
@z0-physical
@domain-workspace-vfs
Feature: Workspace VFS — the session filesystem

  The metaphor: **a session is the root filesystem**, and all unix assumptions
  apply. "/" *is* the session. "~/" is a home directory **under** "/" — it is not
  "/" itself. Paths are absolute, and "/" is the top. And, as on unix, two trees
  can be joined at a mount point, so **sessions may share** paths.

  This file defines the roots. The conventions *inside* the root are
  minimal.feature §1.

  Background:
    Given a session "42"

  Rule: a session is the root, and unix assumptions apply
    # slow: a second, independent session cannot see the first's paths (rung 3)

    Scenario: the session is "/"
      Then the session is "/"
      And "/" is the session's own root
      And there is no parent above "/"
      And paths are absolute, and "/" is the top

    Scenario: a path above the root does not resolve
      When the session requests "/.."
      Then the request is refused
      And it does not reach any path outside the session's own root

  Rule: a session's home is under the root
    # slow: the same home re-read through the sqlite driver (rung 1)

    Scenario: the home is a directory under "/", not "/" itself
      Then the session's home is "~/"
      And "~/" is under "/"
      And "~/" is not "/"
      And the session starts in "~/"

    Scenario: the home is a normal unix home
      Given the session's home "/root"
      Then its parent is "/"
      And "~" is a home directory, not a mount of the root

  Rule: the session's data, state and MCP tools are defined in "/"
    # slow: the same definitions re-read through the sqlite driver (rung 1)

    Scenario: the session defines its data, state and tools in the root
      Then the session's data is defined in "/"
      And the session's state is defined in "/"
      And the session's MCP tools are defined in "/"
      And the session's MCP resources and prompts are defined in "/"

    Scenario: the definitions sit in the root, and are the session's own
      Then "/data", "/state" and "/tools" are the session's definitions
      And no definition lives outside "/"

    Scenario: the root is the container, not a definition
      Then "/" holds the session's definitions
      And "/" is not itself a data, state or tool path

  Rule: a child session does not see its parent
    # slow: consensus across independent sessions (rung 3)

    Scenario: a child session originates at its own "/"
      Given the parent session "42"
      When a child session originates beneath "42"
      Then the child is "/"
      And the child's "~/" is the child's own home
      And the child cannot see any path of the parent

  Rule: sessions may share
    # slow: consensus across independent sessions (rung 3)

    Scenario: two sessions may share a path
      Given the sessions "42" and "43"
      When a path of "42" is joined into "43"
      Then "43" reads the same path
      And the sharing is a join at a mount point, not a parent-relative path

    Scenario: sharing is explicit, not inherited
      Given a child session of "42"
      Then it does not see the parent's paths by default
      And a shared path must be joined, not assumed

  Rule: the two hosts present the same filesystem
    # slow: the Phase-0 suite passes unchanged on the sqlite host (rung 1)

    Scenario Outline: the roots are host-independent
      Given the session on the "<host>" host
      Then the session is "/"
      And the session's home is under "/"
      And "/data", "/state" and "/tools" are defined

      Examples:
        | host   |
        | tmpdir |
        | sqlite |
