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
| isotope | `8R0ZXcvOD_Z0xdQ0qGKEuA9ZlfLTGfF6rCxm4W_tF9o` |
| tokens | 111 |
| parts | 19 |
| signature prefix (key `seed-42`) | `5205c3f5d2de34a8…` |

Parts 10 and 11 were added by fuzz crashers found in `FuzzCanonicalize`:
`"0 ."` was canonicalised with a trailing space, and `"%25"` decoded to `"%"` which
then failed to reparse. Both regression seeds live in
`internal/ingress/testdata/fuzz/FuzzCanonicalize/`.

Parts 12–19 make the front a member of the lab rather than an anonymous port:
the smoke-alarm handshake (deferred/healthy), the monitor's verdict when the
process dies, the registry lease (renew/deregister), SIGHUP reload (rejected
and accepted), and the host supervisor that restarts a crashed process. A
defensive front is **yellow**, not green: it answers, but advertises no
forwarding until agreed traffic has actually crossed the border.

Re-check with:

```sh
python3 ~/projects/personal/.opencode/skills/shell-to-gherkin/scripts/validate_spec.py \
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
- **Monitoring.** `configs/wan-ingress-alarm.yaml` declares the smoke-alarm
  target. `sheets/ingress/spike.sh` drives the real monitor and adhd light:
  `dark → yellow → green → red`, asserted from the monitor's own status and the
  light log, never from the front's `/healthz`.
- **Self-healing.** The registry entry is a renewed lease with a clean
  deregister; SIGHUP swaps bad config out without dropping the listener; a
  launchd/systemd unit (`scripts/`) gives crash recovery on the host.
