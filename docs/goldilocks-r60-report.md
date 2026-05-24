# Thor Goldilocks — R60 Sweep Report

**Date:** 2026-05-24
**Branch:** `dev/goldilocks-r60-sweep` (built from `origin/main`)
**Invocation:** `hexdek-thor -goldilocks --failures-csv /tmp/goldilocks-r60.csv`
**Runtime:** 430 ms over 31,963 card-tests (74 k tests/s)

## Headline

| metric                  |       count |
| ----------------------- | ----------: |
| cards tested            |      35,708 |
| effect tests            |      31,963 |
| effect passes           |      31,944 |
| effect dead-effect      |           0 |
| effect panics           |           0 |
| effect invariant fails  |      **19** |
| keyword tests           |       2,013 |
| keyword fails           |           0 |

**Result: 19 invariant failures across 19 unique cards. No panics, no dead
effects, no keyword regressions.**

Compared to the last logged goldilocks baseline implied by the issue log
(Loki r41 found 8 `ZoneCastGrantExpiry` hits via fuzz), the goldilocks
deterministic sweep surfaces the same family at **17 cards** plus a new
**`TurnStructure`** cluster (2 cards) that the issue log does not yet
mention.

## Failures by invariant

| invariant              | hits | in CLAUDE.md issue log?            |
| ---------------------- | ---: | ---------------------------------- |
| `ZoneCastGrantExpiry`  |   17 | yes — Loki r41 (8 fuzz hits)       |
| `TurnStructure`        |    2 | **no — new category**              |

## Failures by card

### `ZoneCastGrantExpiry` (17)

All 17 hits share the same shape: a "cast / play from exile" or
"play from graveyard" grant with `duration=until_end_of_turn` and
`grantTurn=1` is still present in `gs.ZoneCastGrants` after end-of-turn
cleanup should have removed it. The grants point at exile cards
(`LibCard 0-N` / `LibCard 1-N`) in 16 of 17 cases and a graveyard card
(`GraveCard 0-6`) in one (Magus of the Will).

Surfaced from these resolve paths (effect column in the CSV):

| resolve path             | cards |
| ------------------------ | ----: |
| `modification_effect`    |    11 |
| `counter_mod`            |     2 |
| `create_token`           |     1 |
| `mill`                   |     1 |
| `parsed_effect_residual` |     1 |
| `[other]`                |     1 |

Affected cards:

- Illusionary Mask
- Fight Rigging
- Wildfire Eternal
- Rabble Rousing
- Mosswood Dreadknight // Dread Whispers
- Thieving Aven
- Powerbalance
- Collector's Cage
- Twinning Glass
- Cemetery Tampering
- Spectral Arcanist
- Maelstrom Archangel
- Omnispell Adept
- Wiretapping
- Polterheist
- Magus of the Will (graveyard zone variant)
- Mindleech Mass

**Sample message** (the rest are identical modulo cardname / card slot
/ effect path):

```
ZoneCastGrantExpiry: grant for "LibCard 0-0"
  (zone=exile duration=until_end_of_turn grantTurn=1
   sourceTimestamp=0 source=Illusionary Mask)
  has expired but is still in ZoneCastGrants — cleanup missed
```

**Status vs the issue log.** CLAUDE.md (Open table) records this as a
Loki r41 finding at 8 hits and suspects the impulse-play residual path
in `resolve_helpers.go:4691`. The deterministic goldilocks pass shows
the cluster is **at least twice as large** as Loki estimated and
exercises **five distinct resolve paths**, not just `impulse_play`:
`modification_effect`, `counter_mod`, `create_token`, `mill`, and
`parsed_effect_residual` all leak grants. The fix needs to be at the
expiry / cleanup site (the `until_end_of_turn` reaper that walks
`gs.ZoneCastGrants` on end-of-turn cleanup), not at each individual
resolve path — otherwise the same shape will keep appearing in any new
zone-grant primitive.

`sourceTimestamp=0` across every failing entry is also worth flagging:
either the grants are being created before the source's `Timestamp` is
assigned, or the expiry walker keys on `sourceTimestamp` but every
caller stamps zero. Worth checking when the fix lands.

### `TurnStructure` (2) — **new category**

```
TurnStructure: step "begin_combat" is invalid for phase "combat"
```

| card                          | effect site |
| ----------------------------- | ----------- |
| Karlach, Fury of Avernus      | `[untap]`   |
| Finest Hour                   | `[untap]`   |

**Not** in the issue log.

**Root cause — scaffold-side, not engine.** The Thor scaffold
`condScaffoldMainPhaseOrFirstCombat` writes `gs.Step = "begin_combat"`
when satisfying first-combat-phase conditions (Karlach and Finest Hour
both gate on "the first combat phase"). The engine's
`checkTurnStructure` invariant only accepts the canonical step names
`"begin_of_combat"` and `"beginning_of_combat"` for phase `"combat"` —
`"begin_combat"` is not in the allow-list — so the post-snapshot
invariant pass trips immediately:

```go
// cmd/hexdek-thor/conditional_setup.go:5023
case "first_combat_phase":
    gs.Phase = "combat"
    gs.Step  = "begin_combat"   // <- not in checkTurnStructure's allow-list
```

```go
// internal/gameengine/invariants.go:878-886 — combat phase allow-list
"combat": {
    "begin_of_combat": true, "beginning_of_combat": true,
    "combat_start": true,
    "declare_attackers": true, "attackers": true,
    "declare_blockers": true, "blockers": true,
    "first_strike_damage": true, "combat_damage": true,
    "end_of_combat": true, "combat_end": true,
    "": true, // transitional
},
```

The interaction site shows `[untap]` because the scaffold runs *before*
the untap-step effect being tested, leaving the state already invalid
when the snapshot is taken.

**Recommended fix** (scaffold-only, one line):

```diff
-        gs.Step = "begin_combat"
+        gs.Step = "beginning_of_combat"
```

`classifyTrigger`'s `"begin_combat"` slug (`conditional_setup.go:142`)
is the registry-key abbreviation and stays as-is — it's never written
into `gs.Step`. Only the scaffold's direct phase/step mutation needs
updating.

## New failure categories to add to the issue log

Recommended addition under "Open" in CLAUDE.md:

| Date | Source | Issue | Severity | Notes |
|------|--------|-------|----------|-------|
| 2026-05-24 | Goldilocks R60 | **TurnStructure invariant — `step "begin_combat" invalid for phase "combat"`** (2 hits: Karlach Fury of Avernus, Finest Hour) | Low | Scaffold-side: `condScaffoldMainPhaseOrFirstCombat` writes `gs.Step="begin_combat"` (not in `checkTurnStructure` allow-list which only accepts `"begin_of_combat"` / `"beginning_of_combat"`). One-line fix in `cmd/hexdek-thor/conditional_setup.go:5023`. |
| 2026-05-24 | Goldilocks R60 | **ZoneCastGrantExpiry cluster has 17 deterministic hits** (vs the 8 fuzz hits Loki r41 logged) and exercises 5 resolve paths — not just `impulse_play`. Cards: Illusionary Mask, Fight Rigging, Wildfire Eternal, Rabble Rousing, Mosswood Dreadknight // Dread Whispers, Thieving Aven, Powerbalance, Collector's Cage, Twinning Glass, Cemetery Tampering, Spectral Arcanist, Maelstrom Archangel, Omnispell Adept, Wiretapping, Polterheist, Magus of the Will (graveyard variant), Mindleech Mass | Med | All 17 share `sourceTimestamp=0` and `grantTurn=1`. Fix needs to live in the EOT cleanup reaper, not per-resolve-path. Supersedes/updates the existing Loki r41 ZoneCastGrantExpiry row. |

## Run details

- AST corpus: `data/rules/ast_dataset.jsonl` (35,708 cards loaded, 31,963 had testable abilities)
- Oracle corpus: `data/rules/oracle-cards.json` (35,708 cards)
- Workers: default (`runtime.NumCPU()`)
- Phases: default off
- Scaffold flag: **off** (the TurnStructure cluster surfaces even without `--scaffold`; the scaffold is invoked through the goldilocks setup path regardless)
- CSV: `/tmp/goldilocks-r60.csv` (19 rows)

## Conclusion

The goldilocks surface is small and well-bounded — 0.06% failure rate
across the corpus — with both clusters tractable:

1. **`TurnStructure` (2 hits)** is a one-line scaffold typo. Fix is
   trivial and unblocks future first-combat-phase scaffolding without
   risk.

2. **`ZoneCastGrantExpiry` (17 hits)** is the same cluster Loki r41
   logged, but bigger and broader than the issue log records. The fix
   should live in the EOT reaper that walks `gs.ZoneCastGrants` — a
   per-resolve-path fix will leave the next zone-grant primitive
   leaking the same way.

No new failure modes beyond these two. No panics, no dead effects, no
keyword regressions, no zone conservation hits, no card-identity hits.
