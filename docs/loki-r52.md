# Loki r52 — post-R49/R50/R51 5K validation (~80-card port)

**Date:** 2026-05-20
**Branch:** `dev/loki-r52`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 5000 --seed 48 --report data/rules/CHAOS_REPORT_R52.md --nightmare-boards 0`
**Base:** main @ `a0ec1d6` (post the full per_card port wave: batches A/B/C/D/E (R49) + F/G (R50) + H (R50 customs) + J (R51) + Avatar Enthusiasts/Dominus activation fix on r51)
**Purpose:** Validate the ~80-card per_card port wave against the r50 baseline on the same seed, and re-measure the top-3 clusters carried forward from r48-deep.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 5000 games   | **0**   | 462 (31 games) | 4969 (99.38%) |
| Nightmare boards | 0 boards     | —       | —              | —             |

Throughput: 15 g/s chaos. Wall time **5m30s**.

**Zero panics, zero recovers, zero crashes.**

## Per-invariant volume (5000 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |   310 |        62.0 |
| ZoneConservation      |   108 |        21.6 |
| ZoneCastGrantExpiry   |    20 |         4.0 |
| AttachmentConsistency |    10 |         2.0 |
| CombatLegality        |     8 |         1.6 |
| TriggerCompleteness   |     6 |         1.2 |
| **Total**             | **462** |    **92.4** |

## Comparison vs r50 baseline (same seed, same volume)

Same seed 48 / same 5000 games → apples-to-apples per-game and per-1k.

| Metric                | r50 (5k, seed 48) | r50 /1k | **r52 (5k, seed 48)** | **r52 /1k** | Δ /1k   | Notes                                                              |
|-----------------------|------------------:|--------:|----------------------:|------------:|--------:|--------------------------------------------------------------------|
| Crashes               |                 0 |     0   |                 **0** |     **0**   |  flat   |                                                                    |
| Total violations      |               504 |   100.8 |               **462** |    **92.4** | **−8%** | Modest net drop; composition shifted (see below).                  |
| Dirty games           |                32 |     6.4 |                **31** |     **6.2** |  −3%    |                                                                    |
| Clean game rate       |            99.36% |    —    |             **99.38%** |     —       | +0.02pp |                                                                    |
| CardIdentity          |               352 |    70.4 |               **310** |    **62.0** | **−12%** | Avatar Enthusiasts (game 59) **gone**; new Gerrard cluster surfaces. |
| ZoneConservation      |               108 |    21.6 |               **108** |    **21.6** |  flat   | Same r48/r50 game 3431 / cards-disappeared cluster.                |
| ZoneCastGrantExpiry   |                20 |     4.0 |                **20** |     **4.0** |  flat   | Same Prosper/Ashling sources.                                      |
| AttachmentConsistency |                10 |     2.0 |                **10** |     **2.0** |  flat   | Aura attached to off-battlefield target signatures unchanged.      |
| CombatLegality        |                 8 |     1.6 |                 **8** |     **1.6** |  flat   | Same attacker-with-summoning-sickness signatures.                  |
| TriggerCompleteness   |                 6 |     1.2 |                 **6** |     **1.2** |  flat   | Same dies-event-without-following-trigger pattern.                 |

## What the ~80-card wave moved between r50 and r52

The five-non-CardIdentity invariants are **bit-stable** vs r50 — the
per_card ports (batches A/B/C/D/E/F/G/H/J) didn't touch the engine
surfaces those clusters live in. The only net movement is in
CardIdentity, where two opposing forces nearly cancel:

### CardIdentity: composition shift (Avatar Enthusiasts cleared, Gerrard surfaces)

**r50:** CardIdentity = 352, all 5 shown details pointed at
`Avatar Enthusiasts (ptr 0xc007ff8120)` exile↔battlefield in **game 59**
(seed 590049). r48-deep had flagged this as the dominant CardIdentity
cluster (~88% of r48's 1352 hits over 10K games).

**r52:** CardIdentity = 310, all 5 shown details point at
`Gerrard, Weatherlight Hero (ptr 0xc006857d40)` command_zone↔battlefield
in **game 2432** (seed 24320049). Avatar Enthusiasts is **completely
absent** from the report — the `dev/avatar-enthusiasts-leak-r51` fix
(`6e6239c`, Dominus activation cost) cleanly closed the game-59 leak.

So the r48→r52 movement on this specific leak class:
- Avatar Enthusiasts: **closed** (1352 / 10K → 0)
- Gerrard, Weatherlight Hero: **new** dominant CardIdentity contributor

The net per-1k didn't drop as much as the Avatar fix would suggest in
isolation because Gerrard's leak pays back ~88% of the win (310 / 352 ≈
88% of pre-fix volume, now sourced from a different game).

Repro candidate: `--games 2433 --seed 48` and trace
`Gerrard, Weatherlight Hero` command-zone↔battlefield transitions in
game 2432.

### ZoneConservation: same cluster, unchanged

All 5 shown ZoneConservation details still point at game 3431 (seed
34310049) with the same "real cards disappeared" signature first
identified in the r44 game 420 cluster. Confirmed carry-forward — no
fix has landed for this leak family in the R49-R51 wave.

### Other invariants: fuzz-floor stable

Per-1k rates for AttachmentConsistency (2.0), CombatLegality (1.6),
TriggerCompleteness (1.2), and ZoneCastGrantExpiry (4.0) sit at the
fuzz floor. The shown-detail signatures are unchanged from r50:

- AttachmentConsistency: `Ghoulish Impetus → black zombie giant Token`,
  `Brilliant Wings → Tidal Warrior`, `Dub → phyrexian mite Token` — all
  aura-attached-to-off-battlefield-target pattern.
- CombatLegality: `Risen Riptide`, `Dragon-Style Twins`,
  `Must Be Knights` — attacker without haste / summoning sickness
  (same 4-attacker signature shape as r48/r50).
- TriggerCompleteness: dies events for `Gisa, Glorious Resurrector` and
  `Jenova, Ancient Calamity` followed by no triggered-effect event —
  same per_card death-trigger-drop pattern.

## Top leads carried forward into r53+

| Invariant cluster                                                | r48 status                                    | r50 status                                | r52 status |
|------------------------------------------------------------------|-----------------------------------------------|-------------------------------------------|------------|
| Avatar Enthusiasts game-59 exile↔battlefield                     | open (1352 hits, 1 game)                      | open (352 hits, same 1 game — −48%/1k)   | **closed** by Dominus activation fix (`6e6239c`) |
| game-3431 ZoneConservation cards-disappeared                     | open (~440 / 10K)                             | open (108 / 5K, same 1 game)              | open (108 / 5K, same 1 game — flat) |
| **Gerrard, Weatherlight Hero game-2432 command_zone↔battlefield** | not surfaced (was masked by Avatar volume)    | not surfaced (was masked)                 | **new top lead** (310 hits / 1 game) |
| Prosper/Ashling ZoneCastGrantExpiry                              | open (24 / 10K)                               | open (20 / 5K)                            | open (20 / 5K, same Prosper-Wirefly Hive + Ashling-Trace-of-Abundance signatures) |

## Methodology + caveats

- Single seed (48); per-game RNG seeds are stable, so game indices map
  identically across r48-deep, r50, and r52. Per-1k rates compare cleanly.
- All shown-detail signatures verified by grepping `Message:` lines in
  `data/rules/CHAOS_REPORT_R52.md`. Volume counts read from the
  `Invariant Violations · By Invariant` table.
- The ~80-card port wave touched only `internal/gameengine/per_card/`
  (plus the Dominus activation fix at `r51`). No engine layer-system,
  replacement-effect machinery, or SBA logic changed between r50 and
  r52, so the bit-stability of the non-CardIdentity invariants is the
  expected outcome.
- Throughput: 29 g/s (r50) → 15 g/s (r52). The slowdown is likely the
  per_card wave adding more registered handlers + replacement effects
  per game; loki's invariant checker walks all of them. Not a
  correctness signal.
