@lab-safety
@register
@workspaces
@z1-temporal
@domain-workspace-vfs
Feature: Workspace VFS — Standards & Validation Register

  Executable form of this register. Each Rule is a register section, so a failing
  Rule reads as the section it invalidates. These scenarios pin the *choices*
  (which existing standard, which slower validator), not the prototype's code.

  Background:
    Given the register at sheets/workspaces/README.md

  Rule: §2 standards are chosen from what already exists

    Scenario Outline: each layer lands on an existing standard with tooling
      Given the layer "<layer>"
      Then the adopted standard is "<standard>"
      And existing tooling is available: "<tooling>"
      And no bespoke standard is introduced

      Examples:
        | layer              | standard                              | tooling |
        | filesystem         | POSIX                                 | io/fs, os.DirFS, testing/fstest |
        | MCP                | MCP spec 2024-11-05 + JSON-RPC 2.0     | an existing Go MCP SDK |
        | fs -> HTTP         | WebDAV                                | golang.org/x/net/webdav |
        | subscribe          | file-watch                            | fsnotify |
        | second backend     | SQLite                                | modernc.org/sqlite |
        | telemetry          | OTLP                                  | go.opentelemetry.io/otel |
        | formats            | JSON / JSONL / JSONC                  | encoding/json |

    Scenario: the register is written before the first commit
      When the prototype adopts a new layer
      Then the layer appears in the register with a status
      And status is one of chosen, available, adopted, verified
      And a layer with no register entry fails the check

  Rule: §1 every fast claim has a slower validator

    Scenario: the ladder is additive, not a replacement
      Given a "mostly works" claim from the fastest rung
      Then at least one slower rung is named that audits it
      And the slowest rung actually run is recorded with the claim

    Scenario: a refuted assumption is remembered, not lost
      Given assumption "A4" was refuted on a run
      When the result is recorded in the ResultLedger
      Then "DetectFlakiness" and "DetectRegression" can query it later
      And the register records the refutation against the assumption

  Rule: §1 the slowest validator is Byzantine consensus over canonicalized Gherkin

    Scenario: the Phase-0 scenarios canonicalize to a signed isotope
      Given the Phase-0 feature files
      When they are canonicalized with "CanonicalizeGherkinToIsotope"
      Then the token is signed and content-addressed by family and version

    Scenario: the report is accepted only at quorum
      Given N alarm instances independently verify an isotope
      Then the report is accepted only with a quorum of N >= 3F+1
      And no single instance decides the outcome

    Scenario: ground truth is the seed
      When consensus is reached
      Then the distance measured is the departure from "42 + 0i" (seed-42)
      And the distance maps to a rung on the 42i lattice

  Rule: §4 a bespoke standard is evidence against the hypothesis

    Scenario: inventing a standard falsifies the layer
      Given a layer whose only implementation is a bespoke standard
      When the register is updated
      Then the entry is recorded as evidence-against
      And the hypothesis is marked "will not work" for that layer
      And the failing scenario is linked, not worked around

    Scenario: a passing existing-filesystem run supports the hypothesis
      Given the tmpdir driver runs the Phase-0 suite on a real filesystem
      When the suite mostly passes using standard tooling
      Then the hypothesis reads "not a bad idea"

    Scenario: a tearing existing-filesystem run falsifies the hypothesis
      Given the tmpdir driver runs the Phase-0 suite on a real filesystem
      When the suite tears on filesystem semantics
      Then the hypothesis reads "bad idea"
      And the tearing scenario is recorded with its location

  Rule: the phases are quarter-turns of one shared plane

    Scenario: a single session keeps the pair the only pair
      Given the prototype assumes a single session, id "42"
      Then there is exactly one pair of stores to correlate
      And the session id is a name, while the plane is the store's state

    Scenario: the pair is identical at the ground state
      Given neither the tmpfs nor the sqlite store exists
      Then the pair shares exactly one property: nonexistence
      And at the shared plane (42) the pair is identical

    Scenario: a quarter-turn moves the pair into existence
      When the dual-write spike runs
      Then both stores exist
      And they are still conjugate (one writer, two stores)
      And that stage is the -42i plane

    Scenario: a second quarter-turn separates the pair
      When the stores are split into separate writers
      Then both work independently
      And the residual asymmetry between them is the evidence

    Scenario: the frame is falsified if it needs numbers
      Given the frame expressed without numbers
      When relating the phases requires the embedder
      Then 42i is recorded as wrong for this situation
      And the measure is deferred to the embedder project
