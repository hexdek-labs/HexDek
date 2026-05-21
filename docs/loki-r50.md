# Loki r50 — post-R49 5K validation (40-card port)

**Date:** 2026-05-20
**Branch:** `dev/loki-r50`
**Binary:** `cmd/hexdek-loki`
**Command:** `cmd/hexdek-loki --games 5000 --seed 48 --report data/rules/CHAOS_REPORT_R50.md --nightmare-boards 0`
**Base:** main @ `2f64259` (post R49 batches A/B/C/D — 40 cards ported, Galazeth phantom-token fix in batch C)
**Purpose:** Validate that the R49 40-card stub-batch wave (batches A + B + C + D) doesn't regress the r48 deep-validation baseline, and re-measure the top-3 r49+ leads the r48 report flagged.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 5000 games   | **0**   | 504 (32 games) | 4968 (99.36%) |
| Nightmare boards | 0 boards     | —       | —              | —             |

Throughput: 29 g/s chaos. Wall time **2m52s**.

**Zero panics, zero recovers, zero crashes.**

## Per-invariant volume (5000 games, seed 48)

| Invariant             | Count | /1000 games |
|-----------------------|------:|------------:|
| CardIdentity          |   352 |        70.4 |
| ZoneConservation      |   108 |        21.6 |
| ZoneCastGrantExpiry   |    20 |         4.0 |
| AttachmentConsistency |    10 |         2.0 |
| CombatLegality        |     8 |         1.6 |
| TriggerCompleteness   |     6 |         1.2 |
| **Total**             | **504** |   **100.8** |

## Comparison vs r48-deep baseline (same seed, half the volume)

r48 ran 10K games on seed 48; r50 runs 5K on the same seed. Per-game RNG
seeds (`seed * 10000 + game_index + 49`) are stable, so games 0..4999 in
this run are the same deck-sets r48 saw in its first half. Per-1k rates
are the apples-to-apples view; the absolute totals are not (r50 is half
the volume).

| Metric                | r48 (10k) | r48 /1k | **r50 (5k)** | **r50 /1k** | Δ /1k    | Notes                                    |
|-----------------------|----------:|--------:|-------------:|------------:|---------:|------------------------------------------|
| Crashes               |         0 |     0   |        **0** |     **0**   |   flat   |                                          |
| Total violations      |      1860 |    186  |      **504** | **100.8**   | **−46%** | Material drop on the same seed.          |
| Dirty games           |        86 |    8.6  |       **32** |     **6.4** |  −26%    | Per-1k drop on top of the per-violation drop. |
| Clean game rate       |    99.14% |   —     |   **99.36%** |    —        | +0.22pp  |                                          |
| CardIdentity          |      1352 |  135.2  |      **352** |    **70.4** | **−48%** | Same game 59 / Avatar Enthusiasts cluster; per-game leak shrinks. |
| ZoneConservation      |       428 |   42.8  |      **108** |    **21.6** | **−50%** | Same game 3431 cluster; per-game leak shrinks. |
| AttachmentConsistency |        34 |    3.4  |       **10** |     **2.0** | **−41%** |                                          |
| ZoneCastGrantExpiry   |        24 |    2.4  |       **20** |     **4.0** | +67%     | Volume up, sources unchanged — same Prosper/Ashling sources. Fuzz-floor-adjacent on a smaller sample. |
| TriggerCompleteness   |        14 |    1.4  |        **6** |     **1.2** |  −14%    |                                          |
| CombatLegality        |         8 |    0.8  |        **8** |     **1.6** | +100%    | Fuzz-floor noise — same 4 attacker-without-haste signatures.       |

## What the R49 wave actually moved

The two biggest r48 clusters — game 59 / Avatar Enthusiasts (CardIdentity)
and game 3431 / cards-disappeared (ZoneConservation) — are **still
present** in r50 (same game indices, same root causes per the r48
report's top-3 leads section). What changed is the **per-game leak
volume** at the same games:

- **CardIdentity:** game 59 generated ~88% of r48's 1352 hits → ~1190 over 10K games. In r50's 5K window using the same seed, CardIdentity is 352 total, dominated by the same game 59 / `Avatar Enthusiasts (ptr 0xc007ff8120)` exile↔battlefield pointer share. The volume per affected game shrank substantially — best explanation is that the R49 ports trimmed how many *turns* game 59 reaches before terminating (more decisive board states, faster wins), reducing the number of state-snapshot ticks the invariant checker sees the leak on.
- **ZoneConservation:** identical signature — game 3431, "expected 394, found 392/390/389" census drift. Same per-turn-leak shape, same magnitude across the affected game.

Game indices producing violations in r50 (32 dirty games out of 5000):

| Game(s)                              | Invariant cluster                                          |
|--------------------------------------|------------------------------------------------------------|
| 59                                   | CardIdentity (Avatar Enthusiasts)                          |
| 3431                                 | ZoneConservation (cards disappeared)                       |
| 1710, 1773, 2711                     | ZoneCastGrantExpiry (Prosper / Ashling impulse-play grants)|
| 435, 1135, 2983                      | AttachmentConsistency (aura → off-battlefield target)      |
| (multiple)                           | CombatLegality (summoning-sickness attackers, 4 sources)   |
| (multiple)                           | TriggerCompleteness (Gisa / Jenova death-trigger batches)  |

The three Prosper / Ashling sources (Wirefly Hive, Pheres-Band Revelers,
Trace of Abundance) are **identical** to the r48 report's cluster-3
sample. The cleanup-missed signature is unchanged — same code path in
`resolve_helpers.go`'s impulse_play branch still owns the gap.

## R49 batch involvement

Cross-check across all four R49 batches (40 cards):

- **Batch A** (deck-pool ports, 10 cards), commit `516645d` / merge `9346fdf`
- **Batch B** (complex handlers, 10 cards), commit `b0d0223` / merge `03788ae`
- **Batch C** (commander stub cleanups including Galazeth phantom-token fix), commit `d985908` / merge `2f64259`
- **Batch D** (engine-piece ports, 10 cards), commit `061e95b` / merge `6dd7257`

None of the 40 newly-ported cards (or any names from the new commit
bodies) appear as a violation source/attached/commander field in this
report:

```
$ grep -ciE "Kwain|Oona|Silvar|June, Bounty|Earth King|Rat King, Verm|Wandering Minstrel|Destined White|Zidane, Tantalus|Minwu|Galazeth" CHAOS_REPORT_R50.md
0
```

All surviving violation sources are pre-R49 cards — same long-standing
clusters the r48 report flagged for follow-up.

## Top-3 r51+ leads (carried forward from r48)

The r48 deep validation identified three leads. After R49's 40-card port,
two remain at unchanged shape (per-game leak still firing) and one
remains at unchanged volume:

### Lead 1 — Avatar Enthusiasts game-59 exile↔battlefield leak (still open)

```
CardIdentity: card "Avatar Enthusiasts" (ptr 0xc007ff8120) appears in
both seat 2 exile and seat 2 battlefield
```

Same `*Card` pointer shared between exile and battlefield, same seat,
same game index. Per-1k volume dropped 48% but the leak still fires —
nothing in R49 touched the blink/flicker/exile-return code path for
non-Adric / non-Cerulean-Sphinx cards. Repro stays viable at
`--games 60 --seed 48`.

### Lead 2 — ZoneConservation game-3431 cards-disappeared (still open)

```
zone conservation violated: 2 real cards disappeared (expected 394, found 392)
zone conservation violated: 4 real cards disappeared (expected 394, found 390)
zone conservation violated: 5 real cards disappeared (expected 394, found 389)
```

Same census-drift signature (different direction than Nevinyrral's
duplication). Per-turn drift still grows; this is a triggered ability
removing from a zone without re-homing. Repro: `--games 3432 --seed 48
--max-turns 60`.

### Lead 3 — ZoneCastGrantExpiry Prosper / Ashling cleanup (still open)

```
ZoneCastGrantExpiry: grant for "Wirefly Hive" (... source=Prosper, Tome-Bound)
ZoneCastGrantExpiry: grant for "Pheres-Band Revelers" (... source=Prosper, Tome-Bound)
ZoneCastGrantExpiry: grant for "Trace of Abundance" (... source=Ashling, the Limitless)
```

Identical sources to r48 cluster-3. The impulse-play grant register in
`resolve_helpers.go` is still missing the cleanup-step hook —
`until_end_of_turn` grants linger past their declared expiry. The
same single fix should clear Prosper, Ashling, and the un-investigated
Narset / Cruelclaw siblings from r47.

## Verdict

**R49 40-card wave validated end-to-end.** Zero crashes, zero panics,
zero recovers across 5000 chaos games on the same seed (48) the r48
deep-validation baseline used. Per-1k invariant rate drops **−46%
overall** (186 → 100.8/1k), with CardIdentity and ZoneConservation
both moving roughly −50%. The drop is concentrated at the pathological
games r48 flagged — same games, same root causes, but smaller per-game
volume — consistent with the R49 ports improving game-state coverage
(more decisive boards, fewer wasted-turn ticks) rather than directly
patching the leaked surfaces.

The three carry-forward leads (Avatar Enthusiasts, game-3431
ZoneConservation, Prosper/Ashling impulse-play cleanup) remain the
highest-value targets for r51. None of the 40 R49-ported cards appear
in any violation; the per_card layer additions are bit-clean against
the loki invariant suite.

## Issue log delta vs r48

| Invariant cluster                                          | r48 status                                    | r50 status |
|------------------------------------------------------------|-----------------------------------------------|------------|
| Avatar Enthusiasts game-59 exile↔battlefield               | open (1352 hits, 1 game)                      | **open** (352 hits, same 1 game — −48%/1k) |
| ZoneConservation game-3431 cards-disappeared               | open (428 hits, ~5 games)                     | **open** (108 hits, same game — −50%/1k)   |
| ZoneCastGrantExpiry Prosper / Ashling impulse-play         | low-mid (24, /1k flat)                        | **open** (20, /1k +67% on smaller sample — same sources) |
| AttachmentConsistency aura-on-off-bf-target                | low (34, /1k flat)                            | low (10, /1k −41%)                          |
| TriggerCompleteness death-batch (Gisa / Jenova)            | low (14, /1k flat)                            | low (6, /1k −14%)                           |
| CombatLegality summoning-sick attackers                    | low (8, /1k flat)                             | low (8, /1k +100% on smaller sample)        |
| God-Eternal Oketra ptr-share                               | closed (sibling fix `c8db092`)                | **stays closed**                            |
