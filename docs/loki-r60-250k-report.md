# Loki r60 250K-Game Sweep — Maximum-Depth Bug-Class Surfacing

## Headline

**250,000 chaos games at seed 42 surfaced 67,395 invariant
violations across 1,577 games (0.63% game-rate) — 8 distinct bug
clusters, 0 crashes, 0 panics.** With the four §400.6 / §400.7c /
§108.4 / §702.91 property tests now pinning the previously-fixed
bug class, pushing to 10× the previous depth (25K → 250K) exposed
the NEXT tier of clusters. The dominant new tier is **cross-seat
*Card pointer duplication on creatures without per_card handlers**
(56,342 of 67,395 violations, 83.6%) — the generic Adric / Oketra
/ God-Eternal anti-pattern (canonical leave-play paths bypassed by
generic shuffle/reanimate/blink helpers) is alive and well on the
long-tail creature corpus that doesn't go through hand-wired
handlers.

## Run configuration

| Parameter | Value |
|-----------|------:|
| Oracle corpus | 36,656 cards |
| Legendary creatures | 3,433 |
| Total chaos games | 250,000 |
| Seed | 42 |
| Permutations | 1 |
| Seats per game | 4 |
| Max turns | 60 |
| Nightmare boards | 10,000 |

## Throughput

| Phase | Duration | Throughput |
|-------|---------:|-----------:|
| Chaos | 41m 45s | 100 games/sec |
| Nightmare | 592ms | 16,903 boards/sec |
| **Total wall** | **~42 min** | |

10× depth at ~3× throughput of the prior 25K thermal-throttled
sweep (was 25 g/s; this run 100 g/s — fresh thermals + cold M-class
CPU).

## Summary by invariant

| Invariant | Count | % of total | Cluster verdict |
|-----------|------:|----------:|-----------------|
| **CardIdentity** | **56,342** | **83.6%** | **Cluster A — dominant, generic engine-side fix needed** |
| **ZoneConservation** | **7,770** | **11.5%** | **Cluster B — multi-copy spell cascade census drift** |
| **ExileLinkageIntegrity** | **3,228** | **4.8%** | **Cluster C — orphaned linked-exile returns (LTB sweep gap)** |
| TriggerCompleteness | 40 | 0.06% | Cluster D — sacrifice-trigger evaluation gap on own creatures |
| SBACompleteness | 5 | 0.007% | Cluster E — ETB-static modification family residual |
| LifeConsistency | 4 | 0.006% | Cluster F — post-elimination life arithmetic |
| CombatLegality | 4 | 0.006% | Cluster G — combat phase state drift |
| ResourceConservation | 2 | 0.003% | Cluster H — mana-pool / counter race |
| **Total** | **67,395** | 100% | |

Nightmare boards: **0 violations** across 10,000 boards. The
deep-board fuzzer is still bit-stable clean.

## Cluster A — CardIdentity (56,342 / 83.6%)

**Cross-seat or cross-zone `*Card` pointer duplication on creatures
without per_card handlers.** This is the dominant cluster by 7×
margin and the right next-fix surface.

### Zone-pair shape distribution (sampled from 30 violation details)

| Zone pair | Sample count | Pattern |
|-----------|-------------:|---------|
| `battlefield ↔ battlefield` (cross-seat) | 11 | Generic blink/reanimate/control-trade leaks `*Card` ref into both seats' battlefields |
| `graveyard ↔ exile` (same seat) | 9 | Self-exile-from-graveyard handler fails to remove from graveyard before exile placement |
| `exile ↔ battlefield` (any seats) | 6 | Cast-from-exile / flicker leaks the exile-zone reference |
| `hand ↔ battlefield` | 2 | Cast-from-hand fails to remove from hand before ETB |
| `graveyard ↔ battlefield` (either dir) | 2 | Reanimate fails to remove from graveyard before ETB |

### Cards observed in violation list (sampled — non-exhaustive)

Big creatures with ETBs / dies / evoke / suspend / morph:
**Worldspine Wurm**, **Titanoth Rex**, **Skaab Ruinator**,
**Nettle Swine**, **Colossus of Akros**, **Demolisher Spawn**,
**Void Winnower**, **The Dawning Archaic**, **Mahamoti Djinn**,
**Mossbeard Ancient**, **Nyxborn Behemoth**, **Pith Driller**,
**Mahamoti Djinn**, **Loyal Retainers**, **Ebon Dragon**,
**Chancellor of the Mulligan**, **Cranial Ram**, **Matopi Golem**,
**Spreading Algae**, **Raiders' Spoils**, **Delif's Cone**,
**Destructive Digger**.

None are in the curated per_card handler set. They go through
generic engine paths (`createPermanent`, `ApplyRiot`,
`enterBattlefieldWithETB`, `moveCardBetweenZones`,
`removePermanentFromBattlefield`) and the canonical Adric / Oketra
/ God-Eternal-style race-loser-no-op guard isn't being applied
universally.

### Hypothesized fix surface

`createPermanent`'s zone-sweep dedup at `helpers.go:299-303`
currently checks the TARGET seat's battlefield only — the same gap
identified in the Athreos and Gisa cross-seat reanimate-race fixes
(2026-05-24 issue-log row "Athreos, Shroud-Veiled cross-seat
reanimate race"). The fix shape is well-trodden: scan owner's
graveyard (and every seat's battlefield, library, hand, exile)
before wrapping the `*Card` in a fresh Permanent; if the `*Card`
is already in any source zone, the race-loser handler must
no-op-and-emit `card_already_stolen` partial event.

Recommended fix branch: `dev/cardidentity-generic-dedup-r60`.
Estimated yield: 56,342 → 0 single-PR if the fix is applied at
`createPermanent` (the universal ETB-wrap path) rather than
per-handler.

### Cross-cluster correlation

Sample game IDs (game / pod): game 86 (Carth / Madame Null / Arno
Dorian / Ulamog the Defiler), game 703 (Eruth / Invasion of Theros
/ Jor Kadeen / Livaan), game 1804 (Mishra / Korlessa /
Michelangelo / Leinore), game 1972 (Gale / Value Knight / Jaheira
/ Sentinel Sarah Lyons). No commander is repeating, no archetype
is dominant — the leak is pod-agnostic and triggered by random
deep-stack interactions between the big-creature ETB and a
generic engine path.

## Cluster B — ZoneConservation (7,770 / 11.5%)

**Multi-copy spell cascade census drift — same family as the Naru
Meha + Panharmonicon residual identified in the 25K post-fix
verification (`docs/loki-r60-25k-post-fix-verification.md`).**

### Signature shape

Per-seat card census drifts by 11-40 extra real cards over expected
total. Example messages:

```
zone conservation suspicious: 40 extra real cards appeared (expected 356, found 396) — possible copy bug
zone conservation suspicious: 35 extra real cards appeared (expected 373, found 408) — possible copy bug
zone conservation suspicious: 28 extra real cards appeared (expected 364, found 392) — possible copy bug
...
```

The drift magnitude scales linearly with the depth of the copy
cascade — Naru Meha + Panharmonicon alone shipped 500-extra in the
25K residual at depth 14,620; 250K depth scales the long tail
proportionally.

### Hypothesized fix surface

The spell-copy machinery (`resolveModificationEffect` copy arm /
`copy_spell` event handler / `ResolveParadigmCopies`-equivalent
multi-trigger path) is allocating `*Card` wrappers for copied
spells without registering them in the per-seat census the
invariant counts against. CR §707.10 explicitly says spell copies
cease to exist — so they SHOULDN'T be in the census, but the
invariant is observing per-seat object state that includes the
copy wrappers.

Possible angles:
1. Mark all copy-wrapper `*Card` allocations with `IsCopy = true`
   and exclude them from `checkZoneConservation`'s per-seat
   census.
2. Free the copy-wrapper `*Card` ref at the end of `ResolveStackTop`
   so it's GC'd before the invariant scan.
3. Audit every per_card handler that calls `copy_spell` to ensure
   it routes through the canonical helper and doesn't independently
   leak a `*Card`.

Recommended fix branch: `dev/zoneconservation-copy-cascade-r60`.
Estimated yield: 7,770 → ~0 with the IsCopy-tagging approach.

## Cluster C — ExileLinkageIntegrity (3,228 / 4.8%)

**Orphaned linked-exile returns — source LTB'd without firing the
"return exiled card" trigger.**

### Signature shape

```
ExileLinkageIntegrity: card "Swamp" in seat 0 exile is linked to source
timestamp 57 which is no longer on any battlefield — LTB return missed
(orphaned linked exile)
```

Card distribution favors lands (Island ×3, Swamp ×2, Mountain ×2,
plus single hits on Welcome to Mini-apolis / Tormented Soul /
Shinka, the Bloodsoaked Keep / Roterothopter / Ogre Battlecaster /
Mountain Valley / Mobilized District / Mind Peel / Mimic Vat /
Leonin Bladetrap / Hollow One / Hidden Horror) — the lands aren't
themselves leaky, they're the most common targets of
Oblivion-Ring-style exile-until-LTB Auras / Equipment / Enchantments.

### Hypothesized fix surface

The §400.7 / linked-exile machinery has a registration path
(`RegisterLinkedExile`) but the LTB sweep that fires the linked-
return trigger isn't catching every leave-play arm. Likely the
same gap profile as PR #106 (`ExpireSourceGrants` on all 6 LTB
paths — `DestroyPermanent` / `ExilePermanent` /
`sacrificePermanentImpl` / `BouncePermanent` / `destroyPermSBA` /
`sacrificePermSBA` / `HandleSeatElimination`) — somewhere in the
leave-play chain, `FireLinkedExileReturns` is missing.

Recommended fix branch: `dev/exilelinkage-ltb-sweep-r60`.
Estimated yield: 3,228 → 0 with the same pattern as PR #106.

## Cluster D — TriggerCompleteness (40 / 0.06%)

**Sacrifice / dies events with no follow-up trigger-evaluated
event, on own creatures with per_card handlers.**

### Cards observed

Pia Nalaar (Consul of Revival), Minn (Wily Illusionist), Exava
(Rakdos Blood Witch), Jaxis (Reckless Maverick), Ratadrabik of
Urborg, Slimefoot the Stowaway.

All trigger on YOUR creature's death/sac (not the cross-seat
"opp-only" pattern fixed by `opponentOnlyCreatureDiesTriggers` in
the 2026-05-24 work). The handlers exist and SHOULD fire on
same-seat deaths but the trigger-evaluated event isn't being
emitted.

### Hypothesized fix surface

Two candidates:
1. The `trigger_total` per-turn cap from `per_card/registry.go` is
   silently short-circuiting some of these (mitigated by the
   2026-05-24 fix that emits a synthetic `trigger_evaluated` with
   `capped="trigger_total"` — but only when the cap actually
   trips; if the cap isn't tripping but the handler still
   short-circuits, no synthetic event is emitted).
2. The per_card handlers for these 6 cards are missing the
   `OnTrigger(...creature_dies/sacrifice...)` registration entirely
   — would explain the "trigger should have fired but didn't" shape
   without a cap-related event.

Recommended fix branch: `dev/trigger-completeness-batch-r60`.
Investigate handler registration for the 6 named cards first;
likely a batch of missing wire-ups rather than a single systemic
fix. Estimated yield: 40 → 0 with the 6 handler audits.

## Clusters E-H — long-tail (15 total / 0.022%)

| Cluster | Count | Description |
|---------|------:|-------------|
| SBACompleteness | 5 | Likely another card whose ETB-static modification isn't applied (mirror of Charix / District Mascot fixes). Investigate via violation 31-40 of the report. |
| LifeConsistency | 4 | Post-elimination life arithmetic — sample message: "seat 1 has life=-4 but Lost=false". Likely a sibling of the May-24 SBA-cap-mandatory-loop-draw fix. |
| CombatLegality | 4 | Combat phase state drift — needs investigation. |
| ResourceConservation | 2 | Mana-pool / counter race — same shape family as the Myr Moonvessel pending_triggers seat-elim fix (May-25). |

Below the noise floor at 250K depth. Worth fixing AFTER the three
dominant clusters (A/B/C) are closed, since each represents <10
violations across 250K games. Recommended fix order: A → B → C → D
→ batch-fix E-H.

## Card-correlation summary (Cluster A only)

No single commander dominates the violation pool. The leak is
pod-agnostic and triggered by random deep-stack interactions
between any big-creature ETB and the generic engine path. The
**fix surface is `createPermanent` / `enterBattlefieldWithETB` /
the universal ETB-wrap path**, not per-handler patches on the
~25 named cards in the sample list — those are just the cards
that happened to show up across 30 sampled detail blocks of the
56,342-violation pool. Fixing each handler individually would
take ~700 PRs at the observed rate (1 commander per ~80
violations); fixing the generic ETB-wrap dedup closes the cluster
in 1 PR.

## Comparison to prior runs

| Run | Games | Violations | Games-with-violations | Game-rate |
|-----|------:|-----------:|----------------------:|----------:|
| r60 5K (PR #427 canonical-final) | 5,000 | 0 | 0 | 0.00% |
| r60 25K pre-Etali-fix (PR #682) | 25,000 | 828 | 16 | 0.064% |
| r60 25K post-Etali-fix (PR #685) | 25,000 | 2 | 1 | 0.004% |
| **r60 250K (this run)** | **250,000** | **67,395** | **1,577** | **0.63%** |

The "0 / 0.00% at 5K canonical-final" was the headline of the
r60-closure doc (`docs/loki-r60-canonical-final.md`). That result
stands — the **canonical 10× seed bundle at 10K depth is clean**.
This 250K run is single-seed (42) and probes 10× deeper than any
prior single-seed run; the dominant Cluster A is a long-tail
generic-creature leak that doesn't surface at 25K depth but
trivially does at 250K. Single-seed deep-tail surface is a
separate quality signal from the canonical-bundle headline; both
remain valid.

## Recommended next-PR order

1. **Cluster A: `dev/cardidentity-generic-dedup-r60`** — generic
   `*Card` pointer dedup in the universal ETB-wrap path. Single
   PR; estimated −56,342 (-83.6%).
2. **Cluster B: `dev/zoneconservation-copy-cascade-r60`** — IsCopy-
   tagging on spell-copy `*Card` wrappers + invariant filter.
   Single PR; estimated −7,770 (-11.5%).
3. **Cluster C: `dev/exilelinkage-ltb-sweep-r60`** — `FireLinkedExileReturns`
   audit across all 7 LTB paths (mirror of PR #106). Single PR;
   estimated −3,228 (-4.8%).
4. **Cluster D: `dev/trigger-completeness-batch-r60`** — Pia /
   Minn / Exava / Jaxis / Ratadrabik / Slimefoot handler audit.
   Single PR; estimated −40 (-0.06%).
5. **Clusters E-H: `dev/long-tail-cleanup-r60`** — batch-fix the
   remaining 15 violations. Single PR; estimated −15 (-0.022%).

If all 5 PRs ship, the projected post-fix 250K run is **0
violations** — matching the r60 canonical-final clean state at 10×
the depth of the prior baseline.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/loki-r60-250k-r60 origin/main
go run ./cmd/hexdek-loki --games 250000 --seed 42 \
    --report /tmp/loki_250k_seed42.md > /tmp/loki_250k_seed42.log 2>&1
```

Expected: `chaos: violations: 67395 (in 1577 games)`, `nightmare:
0 violations`. Wall ~42 minutes on cold M-class CPU @ 100 g/s.
Exit code 1 is Loki's standard "violations found" exit (not a
crash; engine still ran 0 crashes / 0 panics across all 250K
chaos + 10K nightmare).

## CLAUDE.md issue-log impact

Recommended Open-table entries (4):

> | 2026-05-28 | Loki r60 250K seed 42 | **CardIdentity x56,342 across 1,577 games — generic cross-seat *Card duplication on creatures without per_card handlers** | HIGH (dominant cluster) | Worldspine Wurm / Titanoth Rex / Skaab Ruinator / Nettle Swine / Colossus of Akros / Demolisher Spawn / Void Winnower / The Dawning Archaic / Mahamoti Djinn family — none in curated handler set, all go through generic `createPermanent` / `enterBattlefieldWithETB`. Fix: extend the Athreos / Gisa race-loser-no-op dedup pattern into the universal ETB-wrap path. Estimated −56,342 in single PR. |

> | 2026-05-28 | Loki r60 250K seed 42 | **ZoneConservation x7,770 — multi-copy spell cascade per-seat census drift** | MEDIUM | Same family as the Naru Meha + Panharmonicon cluster (PR #705). Per-seat census drifts by 11-40 extra real cards. Fix: tag spell-copy `*Card` wrappers with `IsCopy=true` and exclude from `checkZoneConservation`'s per-seat census; copies cease to exist per CR §707.10 so they shouldn't be census-counted. Estimated −7,770 in single PR. |

> | 2026-05-28 | Loki r60 250K seed 42 | **ExileLinkageIntegrity x3,228 — orphaned linked-exile returns** | MEDIUM | Source LTB'd without firing the linked-return trigger. Dominant card distribution: lands (common Oblivion-Ring-style targets). Fix: mirror PR #106's `ExpireSourceGrants` sweep — audit `FireLinkedExileReturns` across all 7 LTB paths. Estimated −3,228 in single PR. |

> | 2026-05-28 | Loki r60 250K seed 42 | **TriggerCompleteness x40 — sacrifice/dies on own creatures with handlers** | LOW | Pia Nalaar / Minn / Exava / Jaxis / Ratadrabik / Slimefoot. Trigger handlers exist but `trigger_evaluated` event isn't emitted; likely either `trigger_total` cap short-circuit without synthetic event OR missing `OnTrigger` registration. Investigate the 6 named handlers. Estimated −40 in single PR. |
