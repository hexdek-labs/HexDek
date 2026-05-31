# Loki r60 — Token-mint chokepoint sweep closure (PR #871 verification)

**Date:** 2026-05-30
**Branch:** `dev/mint-coverage-5k-verify-r60` (cut from `origin/main` @ `6f3e912e`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42`
**Verification target:** PR #871 — Phase 5 mint-coverage sweep (11 token-copy per_card handlers routed through `MintTokenAsCopyOf`).

## Headline

**Token-mint chokepoint sweep holds at 5k scale.** The Loki canonical-seed-42 baseline shows **34 ZoneConservation hits in 1 game, 0 disappearance, 0 CardIdentity, 0 crashes, 0 nightmare violations**. The 34 remaining hits are a single non-token-copy fabrication signature (`h1OGVR200056 / Lash Out`) — outside the scope of the mint chokepoint family this sweep targeted.

## Trajectory across PRs

| PR | Date | ZoneConservation | CardIdentity | Notes |
|---|---|---:|---:|---|
| `docs/loki-r60-report.md` baseline | 2026-05-30 | 192 (120 fab / 72 disap) | 4 | Pre-#834, fresh-main strict-census enabled |
| #834 (seat-elim sideband cease) | 2026-05-30 | 80 (80 fab / 0 disap) | 4 | Disappearance arm closed |
| #853 (Drafna `MintTokenAsCopyOf`) | 2026-05-30 | 80 | 0 | CardIdentity Spikeshell Harrier cluster closed |
| #871 (Phase 5 sweep — 11 handlers) | 2026-05-30 | 34 | 0 | 46-hit `h1OGVR200096` cluster closed end-to-end |
| **THIS RUN (5k post-merge)** | 2026-05-30 | **34** | **0** | Sweep closure holds at scale |

**Cumulative ZoneConservation reduction across the four PRs: 192 → 34 (-82%).** Disappearance arm fully closed; CardIdentity fully closed; fabrication arm down 60% from the original baseline.

## Pre-sweep vs post-sweep token-disappearance count

The original `docs/loki-r60-report.md` baseline (2026-05-30) reported **72 ZoneConservation disappearance hits across 37 unique games**, token-heavy: food / treasure / soldier / zombie / phyrexian-mite / clue tokens, plus non-token leaks for Saprazzan Legate, Rushwood Grove, Queen's Bay Soldier, Mountain, Harbin Vanguard Aviator, Tolarian Sentinel.

PR #834 closed the disappearance arm via two-pronged fix:
1. `HandleSeatElimination` sideband cleanups (`ZoneCastGrants`, `MadnessExile`, `PlotExile`, `MayhemDiscards`) now `MarkInstanceIDCeased` before deleting (mirroring `ParadigmExile`'s pre-existing pattern).
2. `SweepOrphanedInstanceIDs(gs)` defensive backstop appended to `HandleSeatElimination` after all explicit cleanup.

This run confirms **0 disappearance hits across 5000 chaos games + 10000 nightmare boards** at canonical seed 42 — the closure is stable at scale.

## Post-sweep token-copy chokepoint validation

PR #871 closed the `h1OGVR200096` cluster (46 hits across 2 distinct games on the pre-sweep baseline) by routing 11 per_card handlers through `gameengine.MintTokenAsCopyOf`:

| Family | Handlers | Closure |
|---|---|---|
| Vanilla copy | `hazel_of_the_rootbloom`, `orvar_all_form`, `phoenix_fleet_airship`, `calix_guided_by_fate`, `satya` | confirmed clean |
| Copy-with-different-name | `paradigm_echocasting_symposium`, `era3_batch` (Urza-Construct) | confirmed clean |
| Copy-with-template-changes | `hashaton`, `altair_ibn_la_ahad`, `terra`, `shiko` | confirmed clean |
| In-place perm rewrite | `brudiclad_telchor_engineer` (fallback DeepCopy removed) | confirmed clean |

Combined with PR #853's Drafna fix (the original `*.Card.DeepCopy()` audit anchor), the full set of 12 known-bypass handlers is now wired through the Phase 5 mint chokepoint.

## Residual — Lash Out (`h1OGVR200056`)

The 34 remaining hits trace to a single OG (oracle-card) InstanceID — Lash Out, a Red CMC-2 instant — appearing fabricated in 1 chaos game across 34 invariant ticks. This is a NON-token-copy signature: Lash Out is an instant that never spawns tokens, and its OG ID was minted at deck construction. The fabrication arm fires because the ID is either ceased prematurely (stale ceased) OR the *Card pointer is in a zone without a matching MintedInstanceIDs entry. Either way it's a different shape from the token-mint chokepoint family this PR series targeted.

This residual is left as the next follow-up — likely tied to one of:
- An instant-spell-copy path (Smirking Spelljacker / Riku-style copy of a noncreature spell) that's stamping `ExiledByTimestamp` or registering a sideband entry without proper InstanceID propagation.
- A non-canonical zone splice in a per_card handler that fires only on this specific game's commander/card pile combination.

## Run output

```
=== CHAOS GAMES COMPLETE ===
  games:           5000
  duration:        ~2m
  crashes:         0 (in 0 games)
  violations:      34 (in 1 games)
  clean games:     4999

=== NIGHTMARE BOARDS COMPLETE ===
  boards:          10000
  crashes:         0
  violations:      0
  clean boards:    10000
```

## Engine surface summary

After PRs #834, #853, #871, the canonical-seed-42 invariant surface is:

| Invariant | Hits |
|---|---:|
| ZoneConservation (fabrication, single Lash Out signature) | 34 |
| ZoneConservation (disappearance) | 0 |
| CardIdentity | 0 |
| ExileLinkageIntegrity | 0 |
| TriggerCompleteness | 0 |
| SBACompleteness | 0 |
| LifeConsistency | 0 |
| AttachmentConsistency | 0 |
| CombatLegality | 0 |
| ResourceConservation | 0 |
| ReplacementCompleteness | 0 |
| StackIntegrity | 0 |
| ZoneCastGrantExpiry | 0 |
| Nightmare phase (all invariants) | 0 |

Functionally clean except for the single Lash Out fabrication signature.

## Reproducer

```bash
git fetch origin main && git checkout -B repro origin/main
go run ./cmd/hexdek-loki --games 5000 --seed 42
# Lash Out signature isolation:
go run ./cmd/hexdek-loki --games 411 --seed 42 --invariant zone-conservation
```
