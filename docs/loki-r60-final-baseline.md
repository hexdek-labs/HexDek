# Loki r60 — FINAL CLEAN BASELINE (5000 games / seed 42)

**Date:** 2026-05-30
**Branch:** `dev/loki-5k-final-baseline-r60` (cut from `origin/main` @ `b27f72d8`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42`
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`
**Raw log:** `/tmp/loki-final/run.log`

## Headline

**0 crashes / 0 invariant violations across 5000 chaos games + 10000 nightmare boards.**

The engine is **bit-stable clean at canonical seed 42 / 5k depth** after the InstanceID Phase C → G closure series. This is the new clean baseline going forward — any future regression against canonical seed 42 should be measured against this 0/0 result.

## Run output

```
=== CHAOS GAMES COMPLETE ===
  games:           5000
  duration:        ~3m38s
  throughput:      23 games/sec
  crashes:         0 (in 0 games)
  violations:      0 (in 0 games)
  clean games:     5000

=== NIGHTMARE BOARDS COMPLETE ===
  boards:          10000
  duration:        1.654s
  throughput:      6046 boards/sec
  crashes:         0
  violations:      0
  clean boards:    10000

Verdict: CLEAN
```

## Per-invariant surface (all clean)

| Invariant | Hits |
|---|---:|
| `ZoneConservation` (fabrication + disappearance) | 0 |
| `CardIdentity` | 0 |
| `ExileLinkageIntegrity` | 0 |
| `TriggerCompleteness` | 0 |
| `SBACompleteness` | 0 |
| `LifeConsistency` | 0 |
| `AttachmentConsistency` | 0 |
| `CombatLegality` | 0 |
| `ResourceConservation` | 0 |
| `ReplacementCompleteness` | 0 |
| `StackIntegrity` | 0 |
| `ZoneCastGrantExpiry` | 0 |
| `WinCondition` | 0 |
| `LayerOrdering` | 0 |
| **Nightmare phase (all invariants × 10k boards)** | **0** |

Every tracked invariant reports 0 violations on canonical seed 42 at 5k depth.

## Trajectory across the InstanceID Phase closure series

| Milestone | Date | ZoneConservation | CardIdentity | ELI | Notes |
|---|---|---:|---:|---:|---|
| **Phase C — strict-census default ON** | 2026-05-29 | 2,904,066 (25k sweep, ~116 per game) | — | — | Disappearance arm flipped on by default (`SetStrictCensusDefault(true)`); surfaced 2.9M real mint-coverage gaps |
| **Phase D — private-zone mint-coverage closure** (PR #773) | 2026-05-30 | ~98,230 (25k) / -96.6% | — | — | Universal `markPermanentCeaseIfToken` chokepoint at `gs.removePermanent`; closed dominant token-cease class |
| **Phase E — residual tail-edge orphan sweep** (PR #776) | 2026-05-30 | further reduction | — | — | `SweepOrphanedInstanceIDs` at §514.2 EOT cleanup; closed control-change transients + basic-land *Card drops |
| **r60 fresh-main baseline** (PR #781 / `docs/loki-r60-report.md`) | 2026-05-30 | **268** (120 fab / 72 disap / `ExileLinkageIntegrity` 72 / CardIdentity 4) | 4 | 72 | First post-Phase-C-E canonical 5k seed-42 measurement |
| **PR #800 (fireTrigger ctx-fallback for LTB)** | 2026-05-30 | unchanged | unchanged | 72 → 2 | Banisher Priest family LTB cleanup |
| **PR #817 (Knowledge Pool LTB + seat-elim cleanup)** | 2026-05-30 | unchanged | unchanged | 2 → 0 | KP ExiledByTimestamp tag clear + `ClearLinkedExileTagsForSource` |
| **PR #834 (seat-elim sideband cease + orphan sweep)** | 2026-05-30 | 192 → 80 (-58%, disap 72 → 0) | unchanged | 0 | Disappearance arm fully closed |
| **PR #853 (Drafna `MintTokenAsCopyOf`)** | 2026-05-30 | 80 | 4 → 0 | 0 | First per_card mint-chokepoint fix; closed CardIdentity Spikeshell Harrier |
| **Phase F — `MintSpellCopy` chokepoint + 10 sibling sites** (PR #851) | 2026-05-30 | ~1,000 → 80 / -92% on phase scope | 0 | 0 | §707.10 spell-copy mint-coverage closure |
| **PR #871 (Phase 5 mint-coverage sweep — 11 token-copy handlers)** | 2026-05-30 | 80 → 34 (-58%) | 0 | 0 | Closed `h1OGVR200096` cluster end-to-end |
| **Phase G — Aziza spell-copy `MintSpellCopy`** (PR #873) | 2026-05-30 | 34 → 0 (-100%) | 0 | 0 | Closed lone Lash Out `h1OGVR200056` residual |
| **THIS RUN — final clean baseline** | 2026-05-30 | **0** | **0** | **0** | All invariants clean across 5k chaos + 10k nightmare |

**Net trajectory at the post-strict-census era:** every measurable invariant signal drained to 0 across 6 major closure phases (C → D → E → F → G + the bridge PRs #800/#817/#834/#853/#871). At canonical seed 42 depth-5000 the engine is now functionally indistinguishable from a clean reference implementation under the full Phase-4-strict-census invariant suite.

## What this baseline means for future work

1. **Regression detection** — any future PR that lands a ZoneConservation / CardIdentity / ELI violation against canonical seed 42 at 5k depth is now an immediate regression signal, not a residual.
2. **Wider seed coverage** — the next forensic surface is extended-seed sweeps (seed 43 / 99 / 1337 / 271828 / 31415) to find shapes seed 42 didn't surface. The historical r60-extended-seeds runs (`docs/loki-r60-extended-seeds.md`, `docs/loki-r60-canonical-final.md`) reported clean across multi-seed gauntlets pre-Phase-C-strict-census; those need re-running on this main.
3. **Stress-depth coverage** — 25k / 100k / 250k Loki runs become the next coverage frontier. The Phase E 25k run (post-PR #776) was the last published large-depth baseline; this clean canonical 5k should hold proportionally at higher depths but the verification is open work.
4. **Nightmare phase** — already bit-stable clean across every recent run; 0/0 at 10k confirms post-merge.

## How to reproduce

```bash
git fetch origin main && git checkout -B repro origin/main
go run ./cmd/hexdek-loki --games 5000 --seed 42
```

Expected output: `crashes: 0`, `violations: 0`, `clean games: 5000`, `nightmare violations: 0`. Any deviation is a real regression.
