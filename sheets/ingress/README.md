# WAN ingress — spec register

> Ticket: [lab-safety#38](https://github.com/james-gibson/lab-safety/issues/38) ·
> Implementation: [`internal/ingress/`](../../internal/ingress/) ·
> Origin: [`wan-ingress.feature`](wan-ingress.feature)

This directory is a **spec register**, not a set of MCP safety data sheets. It is
deliberately a subdirectory: `policy.LoadSheets` reads only `sheets/*.md`, so a
register here does not change the published manifest. The sheet remains the
declaration; the feature is the executable origin of the binary.

## Origin and identity

`wan-ingress.feature` is the origin: no code was written until it existed. It
was validated twice with the real isotope canon (via `brine.py`) and both runs
gave the same identity:

| field | value |
| --- | --- |
| isotope | `6Ond5kxsQgMt43RBzU2kRcsrdAvTbLFPzCWYqUHNrj4` |
| tokens | 73 |
| parts | 11 |
| signature prefix (key `seed-42`) | `36adf48859846abc…` |

Two parts were added by fuzz crashers found in `FuzzCanonicalize`: `"0 ."`
was canonicalised with a trailing space (tenth part), and `"%25"` decoded to
`"%"` which then failed to reparse (eleventh part). Both regression seeds live
in `internal/ingress/testdata/fuzz/FuzzCanonicalize/`.

Re-check with:

```sh
python3 ~/.opencode/skills/shell-to-gherkin/scripts/validate_spec.py \
  sheets/ingress/wan-ingress.feature \
  --brine ~/projects/kb/projects/scripts/brine
```

The Go gate (`spec_test.go`) re-reads the same file with
`isotope.ParseGherkin` + `CanonicalizeGherkinToIsotope`, so the declaration and
the binary's claim cannot drift apart.

## Gates fixed by the ticket

- **G1 — scope.** "Cluster" is the lab's local-first mDNS cluster; the WAN
  front forwards only through a signed, trusted border. It does not become an
  authority inside the lab.
- **G2 — resolver.** Accept essentially anything, but forward only by
  agreement. Unknown or ambiguous names are quarantined with a typed reason;
  addresses and namespaces are read from the agreement, never synthesised from
  the input.
- **Fuzzing.** The resolver is a total function: every input maps to a route or
  a typed refusal. See the five `testing.F` targets in `internal/ingress`.
