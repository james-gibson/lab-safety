# Worked derivation — `minimal.feature`

The Phase-0 gate, derived by hand. Use it as the pinned vector: if you re-derive
`minimal.feature` and get anything but this, find the disagreement before you
report.

## Pin

| | |
| --- | --- |
| spec | `sheets/workspaces/minimal.feature` |
| sha256 | `4feb5c704d2b8b83d013f5558f8255cc0523f77d09aa01446db695923bfa9e7a` |
| bytes | 8250 |
| family | `workspace-vfs` |
| version | `1` |
| key (seed) | `seed-42` (session "42") |
| canon version (authority) | `isotope v0.4.2-0.20261017004323-9cae21124db1`, commit `9cae211` |

If the spec's sha256 changes, this vector is void — re-derive. Do not "update" the
vector without re-running the derivation.

If the **isotope version** changes, this vector may be stale even though the spec
is unchanged. Re-derive from the new canon (see `SKILL.md` → *Authority and
version*), then replace this vector and the recorded version together.

## Walk

Parse → the matching steps, in source order, all live in Rule §2:

| step | normalized | definitions that fire |
| --- | --- | --- |
| `the resource file "/resources/workspace/config.json"` | `resource-file-resourcesworkspaceconfigjson` | `resource_file` = `/resources/workspace/config.json`; `resource` = `resource://workspace/config` |
| `an executable endpoint "/tools/echo@"` | `executable-endpoint-toolsecho` | `tool_file` = `/tools/echo@`; `tool` = `echo` |
| `the template "/prompts/summarize.tmpl"` | `template-promptssummarizetmpl` | `prompt_file` = `/prompts/summarize.tmpl`; `prompt` = `summarize` |

No other step matches the table, so the declaration set is exactly those six
entries. `Background:` and `Rule:` lines, and non-matching steps, contribute
nothing.

ByKind (dedupe, sort) then gives `tools: [echo]`,
`resources: [resource://workspace/config]`, `prompts: [summarize]`; the capability
families are therefore `prompts/list`, `resources/list`, `tools/list`. Note the
hand-written harness declaration (`min_capabilities: ["tools/list"]`) is a
**proper subset** of this.

## Isotope

```
family:    workspace-vfs
version:   1
signature: 0657925bfde6f9a8dc0590b703b14fd8d7b03e165a99cf1215c5a1db14e7654d
raw:       112 lines, below
```

```text
FEATURE:name:workspace-vfs-minimal-contract-phase-0-gate
SCENARIO:name:layout-is-documented-one
THEN:hash:b35e1710dc9945f7
AND:hash:90add65a8ff08af9
AND:hash:c1c711c10f129e4f
AND:hash:61658c1e1cc7d4f1
AND:hash:8121a17443d6738c
AND:hash:8473903b79a60d63
AND:hash:dbdc9d58d9479c4a
SCENARIO:name:sequence-numbers-order-as-files
WHEN:hash:b626a6b2345b2220
THEN:hash:56274ef796e2ea95
WHEN:hash:a2a7d75ebb271fca
THEN:hash:325609e15829ce4d
SCENARIO:name:resources-are-files
GIVEN:hash:a05ae99dae3b41c5
THEN:hash:50a4562c49bf365d
AND:hash:ebcba607dc78cd20
AND:hash:fa59e4fbd8af8fef
SCENARIO:name:tools-are-files
GIVEN:hash:f9315b8edd9f3f79
THEN:hash:328d6899ae5b747b
AND:hash:a19e425d2f3e9d4c
SCENARIO:name:toolscall-is-file-write-plus-result-read
WHEN:hash:ed41adc478fc8460
THEN:hash:b64d0e1cf38ed3e8
AND:hash:51fb996afbc1050f
SCENARIO:name:prompts-are-templates
GIVEN:hash:2ac7704877961ba3
WHEN:hash:0ff903804396d7e0
THEN:hash:f0990837afee9dc1
SCENARIO:name:message-write-moves-submitted-to-working
GIVEN:hash:9f84956121b9a729
WHEN:hash:ad8d42a66f91969c
THEN:hash:c95f27ede5131349
AND:hash:8e6b53ae8455b559
SCENARIO:name:final-artifact-moves-working-to-completed
GIVEN:hash:46b1edfff23276e4
WHEN:hash:e2ba8dac5a21bcbe
THEN:hash:20f0f926868a0b5d
SCENARIO:name:prototype-runs-single-session-id-42
GIVEN:hash:0b4ff19a5fd5e5bb
THEN:hash:295a9100e84b83f2
AND:hash:4860b36dfe1cba13
SCENARIO:name:descendant-gains-sessions-access
GIVEN:hash:cf8e214b924e339b
WHEN:hash:51294b0473fe8147
THEN:hash:a2e1b820303f56f0
SCENARIO:name:access-never-propagates-upward
GIVEN:hash:cf8e214b924e339b
WHEN:hash:087744296e8f937a
THEN:hash:84030702c7ed52a1
AND:hash:e9251268ff5836be
SCENARIO:name:tool-dropped-in-at-runtime-becomes-listable-and-callable
GIVEN:hash:14fff6b98eb85b23
WHEN:hash:3ba969b702da93c9
THEN:hash:b258980c13f04201
AND:hash:6406e20a81e7508a
SCENARIO:name:tool-removed-at-runtime-stops-being-listed
GIVEN:hash:d263ae4e71c3df3b
WHEN:hash:7134a23bd1e7d4cc
THEN:hash:9bbfd98104c3025d
SCENARIO:name:same-command-runs-on-two-hosts-and-agrees
GIVEN:hash:f5554cc476e31c4a
WHEN:hash:bdf3fb4e52ed5777
THEN:hash:64e714811aafd361
AND:hash:10ba24314b6a7821
SCENARIO:name:mirror-holds-isotope-record-of-run
GIVEN:hash:a754dfac16e001bb
WHEN:hash:7feb2eec6a72ce4a
THEN:hash:94e28f607992da7d
AND:hash:e529d437e49be692
SCENARIO:name:mirror-is-independent-of-traffic-log
GIVEN:hash:03786d5faa383caa
AND:hash:a69d2017147cb29e
THEN:hash:4c7843a08c0ffb70
AND:hash:0e977a14248f7a5f
SCENARIO:name:surface-passes-mcp-spec-compliance
GIVEN:hash:9780a5160e9fb9b4
THEN:hash:67a9bb8fc6e5b8c2
AND:hash:55c105712cbe54ee
SCENARIO:name:every-fast-claim-names-slower-validator
GIVEN:hash:4e3355e89e91562c
THEN:hash:39971d861163056b
AND:hash:65b3f2be9bacbd3a
SCENARIO:name:operation-is-traced-with-its-plane-and-rotation
WHEN:hash:419ebacfa0045e48
THEN:hash:446f590157ce7d1f
AND:hash:73ddf32eb8803526
SCENARIO:name:rotation-is-visible-as-adhd-status-light
GIVEN:hash:b7789d5947e3a4d4
THEN:hash:c0407e3670130881
WHEN:hash:b5bcf8dab6f2bccf
THEN:hash:feec0c6584679a7c
WHEN:hash:1b7558474cc70ceb
THEN:hash:1b28425f72176a35
SCENARIO:name:smoke-alarm-test-asserts-rotation
GIVEN:hash:ac7bf7ec463197e3
WHEN:hash:a08ca6124a7c1c9e
THEN:hash:2c36033af3fdbc8a
AND:hash:dc68c28bf14437bc
SCENARIO:name:otel-debugs-but-does-not-decide
GIVEN:hash:6c6b142e8ee6eba6
THEN:hash:4af83513829d6f4c
SCENARIO:name:scratch-script-may-prime-loop-but-is-not-deliverable
GIVEN:hash:f3738cd817cb8bea
THEN:hash:bd159fff8ed0732f
AND:hash:243c8925a64a1bc7
SCENARIO:name:feature-that-needs-numbers-is-finding-not-build-item
GIVEN:hash:9caad26e1578c6cf
THEN:hash:ca4b10ea7e26a60c
AND:hash:e5ddc4fcca4883a0
```

## How it was checked (not loaded to derive)

1. Hand-applied §1–§9 of [`canon-logic.md`](canon-logic.md).
2. Independently re-derived the three checkpoint hashes and the signature in a
   throwaway python script (SHA-256 + word-boundary article strip + `[^\w\s-]`
   strip + `hmac`), **without** importing or reading the canon. Same hashes; same
   signature.
3. Only then compared against the canon as a black-box oracle — at
   `isotope v0.4.2-0.20261017004323-9cae21124db1` (commit `9cae211`): 6
   declarations in source order, the same expectation, the same 112-token `raw`,
   the same signature. Agreement — the hand derivation stands.

The point of ordering (1) → (2) → (3) is that the answer existed *before* the
canon was consulted. That is what makes it a check rather than a lookup.
