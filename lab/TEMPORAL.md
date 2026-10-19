# Temporal reference system — specification prompt

**A language-agnostic build prompt, recovered from a chat export.** It was written to
be handed to an implementing model with no further conversation, so it deliberately
carries its own tests. Three worked chains are included because a spec without tests
drifts; they use *Back to the Future* because it contains four distinct temporal layers
at once — narrative, production, paradox and anachronism.

Companion documents: [`CHIMERA.md`](CHIMERA.md) for the record and identifier format
this addressing sits inside, and [`SPLASH.md`](SPLASH.md) for the trust model.

---

## System overview

Design and implement a distributed temporal reference system that treats any
identifiable work — film, book, patent, historical event, personal experience,
simulation — as a **universe**: a first-class temporal node with its own epochs,
precision and windowing scheme. The system must handle cross-universe references,
reality gradients, attribution requirements and temporal influence chains across
vastly different time scales.

## Core concepts

### 1. Distributed architecture

- Any node may hold temporal data, addressable by id.
- Ids follow `<node-id>.<domain:temporal-event-id>`.
- **Node first, then window.** Bucket-first ordering (`<window>.<node>.<event>`) looks
  natural but puts the volatile part first, which breaks replication of static records
  and destroys the "something happened in this window" metadata the id is meant to
  carry. Node-first lets one node's records be replicated as a unit while the search
  service answers the two remaining access paths: a specific node in a window, or all
  records in a window.
- Clock drift is absorbed by configurable window width (default 5 s for real-time
  systems; arbitrary for narrative systems).

### 2. Temporal universes

A universe is any identifiable work or event that defines its own temporal coordinate
system.

- **Types** — film, series, book, patent, historical_event, personal_experience,
  simulation.
- **Identifiers** — ISBN, DOI, patent number, film id, historical consensus id.
- **Multiple epochs per universe** — a film has a *runtime* epoch (0–116 min), a
  *narrative* epoch (in-story dates) and a *production* epoch (when it was made). These
  are different coordinate systems over the same work and must not be collapsed.
- **Reality relation** — documentary, historical_fiction, inspired_by, pure_fiction,
  alternate_history, metafiction, parody, mythology.
- **Fictionalisation degree** — 0.0 documents, 1.0 invents.

**Custom epochs are load-bearing.** Millisecond precision is meaningless on geological
time and essential inside a biological simulation, and real time is not simulated time.
The epoch is chosen per universe, and an event id is tied to its window's epoch.

### 3. Temporal windows

- Fixed-duration or narrative-based (scene, chapter, act).
- Shareable across universes at different precisions: a year window may contain
  day-precision events from another universe.
- Aliases are expected: `cal:1916`, `wwi-peak`, `modernist-period` may all name the
  same or overlapping windows.

### 4. Reference types

- **Direct** — depicts, documents, recreates, quotes.
- **Influenced** — inspired_by, sublimated, allegorises.
- **Meta** — contradicts, reinterprets, parodies, homage.
- **Reality-blending** — metafiction, alternate_history, mythologises.

### 5. Attribution

Copyright status must be tracked. Historical events are generally public domain.
Personal experiences carry privacy levels. References to copyrighted material need
attribution; some reference types (recreates, depicts) may need permission.

### 6. Precision scales

```
NANOSECOND      1
MICROSECOND     1e3
MILLISECOND     1e6
SECOND          1e9
YEAR            31,536,000,000,000,000
MILLION_YEARS   31,536,000,000,000,000,000,000
```

## Data shape

```
universe:
  universeId:       unique identifier
  identifiers:      primary and alternate ids
  realityRelation:  relationship to reality
  attribution:      copyright and citation metadata
  layers:           production | narrative | runtime
  epochs:           temporal boundaries per layer
```

Temporal address:

```
<node-id>.<domain:temporal-event-id>
node-42.film:bttf:1985:runtime:enchantment_dance:a7b3c9d2
```

## Required capabilities

1. Find all events in a temporal window across nodes.
2. Trace influence chains between universes.
3. Query by reality gradient.
4. Correlate temporally across domains.
5. Validate attribution for references.

---

## Test cases

### Test 1 — Recursive reference: the Johnny B. Goode paradox

Universes:

```
film:bttf:1985
  realityRelation: pure_fiction, fictionalizationDegree 1.0
  layers:
    runtime:         0 → 116 min, ms precision
    narrative_1985:  Oct 26 – Nov 5 1985, minute precision
    narrative_1955:  Nov 5 – Nov 12 1955, minute precision

history:chuck_berry:career
  realityRelation: documentary, fictionalizationDegree 0.0
  keyEvents: johnny_b_goode_composed → 1958, year precision
```

Chain `bttf_johnny_b_goode_paradox`:

1. `film:bttf:1985:narrative_1955:enchantment_dance` — Marty performs the song in 1955.
2. `history:chuck_berry:career:johnny_b_goode_composed` — Berry writes it in 1958.
3. `film:bttf:1985:narrative_1955:marvin_calls_chuck` — Marvin phones his cousin Chuck.

Connections: 1→2 `temporal_paradox`, direction `backward_influence`; 3→2
`causal_loop`. The chain must be representable as a **valid cycle**, not rejected.

### Test 2 — Reality layers: the Sports Almanac

Chain `bttf2_sports_almanac`:

1. `film:bttf2:1989:prop:grays_sports_almanac` — a physical prop; exists in
   production, narrative_2015 and narrative_alt1985 simultaneously.
2. `film:bttf2:1989:narrative:almanac_contents` — fictional results 1950–2000;
   realityRelation `fictional_documentation` of an alternate timeline.
3. `history:sports:actual_results:1950-2000` — documentary.

Connections: 1→2 `prop_represents_document` (production→narrative); 2→3
`alternate_history` with `deviation_point: Nov 12 1955`. The same object must be able
to occupy two layers at once without merging them.

### Test 3 — Meta-reference: "Hey McFly!"

Chain `bttf_your_kids_gonna_love_it`:

1. `film:bttf:1985:narrative_1985:marty_audition` — judge says "too loud".
2. `film:bttf:1985:narrative_1955:marty_performs` — "your kids are gonna love it".
3. `film:bttf:1985:production:huey_lewis_cameo` — Huey Lewis plays the judge,
   judging his own song.
4. `history:music:rock_evolution:1955` — rock and roll emerging.

Connections: 1→3 `meta_casting`; 2→4 `predictive_reference`; 2→1 `temporal_irony`.

### Test 4 — Cross-universe window match

`personal:tolkien:war_experience:1916` and `history:wwi:battle_of_somme:1916` are
different nodes, different reality relations, same window. A user looking for similar
references must be able to find the second from the first without knowing the second
exists — the window is the join key, not the node.

## Validation criteria

1. **Paradoxes are representable.** Test 1 yields a cycle the store accepts.
2. **Layers stay separate.** Test 2 keeps production, narrative_1985 and narrative_1955
   distinct while allowing one object to appear in several.
3. **Attribution is tracked.** Chuck Berry, Calvin Klein and Huey Lewis all carry
   attribution requirements; historical events carry none.
4. **Windows align.** Everything in a "1955" window is discoverable together across
   universes of different precision.
5. **Reality gradients are distinguishable.** Historical fact, fictional narrative,
   meta-reference and prop-vs-content must not collapse into one another.
6. **Queries answer** at minimum:
   - all anachronisms in a universe
   - all temporal paradoxes created by time-travel narratives
   - all references between two windows in any universe
   - all meta-casting references where an actor passes judgement on themselves

## Success metrics

Represent all four chains without data loss; keep temporal consistency inside each
universe; traverse references in both directions; handle precise timestamps and fuzzy
windows with equal competence; distinguish reality relations; attribute copyright
while marking public-domain content.

<details>
<summary>Provenance and open questions</summary>

Recovered 2026-10-19 from the chat export *"Distributed Event Handling and
Traceability"* (16 messages). The spec prompt is message 15; it had never been saved
to disk. Test 4 was reconstructed from messages 12–14 rather than the prompt body.

The conversation's unresolved threads, worth keeping:

- **Window width vs. cross-node interleaving.** A 5 s window bounds drift but forces
  every consumer to agree on bucket boundaries before the data is useful.
- **Epoch per universe vs. one global epoch.** The design wants per-universe epochs;
  every cross-universe query then needs an epoch conversion, and no conversion rule was
  settled.
- **Year-precision windows containing day-precision events.** Containment is stated,
  but the arithmetic over mixed precision was never written down.
- **Copyright and `recreates`.** The spec says permission "may" be required. That
  ambiguity will surface as a legal question the first time it matters.

</details>
