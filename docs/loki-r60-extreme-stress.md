# Loki R60 Extreme Stress

Date: 2026-05-24
Branch: `dev/loki-r60-extreme-stress-r60`
Goal: take the wide-seed gauntlet from 5,000 chaos games per seed up
to **10,000 chaos games per seed** (2x deep-stress, 5x mega-stress).
Same 10 seeds. Looking for residuals only visible at extreme sample
depth.

## TL;DR

**5 of 10 seeds clean, 5 surfaced new residuals at the 10K sample.**

| Seed | Chaos (10K games) | Nightmare (10K boards) | Verdict |
|:----:|:-----------------:|:----------------------:|:-------:|
| 42 | **2 in 1 game** (ZoneCastGrantExpiry) | 0 | ⚠️ new |
| 43 | **0** | **0** | ✅ clean |
| 99 | **2 in 1 game** (ResourceConservation) | 0 | ⚠️ new |
| 7 | **0** | **0** | ✅ clean |
| 1337 | **40 in 1 game** (CardIdentity) | 0 | ⚠️ new |
| 2024 | **0** | **0** | ✅ clean (was 24 at 5K post-#200) |
| 2025 | **0** | **0** | ✅ clean (was 1 at 5K post-#201) |
| 31415 (π) | **8 in 2 games** (ZoneCastGrantExpiry 2 + SBACompleteness 6) | 0 | ⚠️ new |
| 271828 (e) | **4 in 2 games** (ZoneCastGrantExpiry) | 0 | ⚠️ new |
| 161803 (φ) | **0** | **0** | ✅ clean |

Aggregate at the deepest sample:

- **100,000 chaos games + 100,000 nightmare boards** total
- **56 violations across 7 games** (5 distinct signatures)
- **0 nightmare violations across all 10 seeds × 10K boards each**
- **0 crashes / 0 panics**

Per-game stochastic rate: **0.056%** chaos (56 / 100K), **0%** nightmare.

## Statistical context

| Sample depth | Total chaos games | Violations | Per-game rate |
|:------------:|:-----------------:|:----------:|:-------------:|
| 2K / seed | 20,000 | 0 | 0.000% |
| 5K / seed | 50,000 | 26 | 0.052% |
| **10K / seed** | **100,000** | **56** | **0.056%** |

The per-game rate is consistent across the two deeper samples (~0.05%).
The 10K depth doesn't reveal a worse engine — it reveals more of the
SAME long-tail surface that 2x more games surfaces statistically.
The empirical estimate is now tight enough that the next residual
chase is bug-by-bug, not sweep-by-sweep.

## Residual breakdown

### Seed 42 game 5480 — ZoneCastGrantExpiry x2

- **Turn**: 38 upkeep
- **Commanders**: Kess Dissident Mage / Syr Armont the Redeemer /
  Faldorn Dread Wolf Herald / Faithful Squire // Kaiso Memory of
  Loyalty
- **Signature**: `ZoneCastGrantExpiry` — graveyard-cast / exile-cast
  grant survived its declared expiry. Family already partially closed
  in earlier r60 waves (PR #106 source-LTB + game-end, the heist /
  may_play_exiled_free expiry stamping, the residual-deep fix). The 2
  hits here are a NEW source — possibly Kess's "cast then exile"
  permission outliving an interaction the prior fixes didn't cover.

### Seed 99 game 9804 — ResourceConservation x2

- **Turn**: 42 end_of_combat
- **Commanders**: Erebos God of the Dead / James Wandering Dad //
  Follow Him / Nylea God of the Hunt / Gyome Master Chef
- **Signature**: `ResourceConservation` — total resource accounting
  (mana / treasures / food / etc.) drifts from the per-seat sum.
  Bit-stable family that had several prior r60 fixes around treasure
  / clue / food token paths.

### Seed 1337 game 8921 — CardIdentity x40

- **Turn**: 41 cleanup
- **Commanders**: Magda Brazen Outlaw / Shiko Paragon of the Way /
  Spider-Gwen Free Spirit / Zidane Tantalus Thief
- **Signature**: `CardIdentity` — same *Card pointer in two zones. 40
  hits on ONE game means a deep persistent leak that the invariant
  re-detects each turn (8-10 turns × 4-5 hits per turn). Magda /
  Shiko / Zidane all have card-stealing or copy-creating effects;
  candidates for the leak source are the per_card handlers for
  Magda (treasure-into-permanent), Shiko (manifest-style), or Zidane
  (steal-from-hand).

### Seed 31415 — 2 games

**Game 5610**: ZoneCastGrantExpiry x2 (turn 33 draw, different
commander pod than seed 42's). Same family.

**Game ???**: SBACompleteness x6 (different game from 5610 — the
report shows 2 games affected). Probably another rare-window SBA
miss; needs the full report for game-id.

### Seed 271828 — 2 games

**Game 5399**: ZoneCastGrantExpiry x4 (turn 50 upkeep). Commanders
include Gonti Canny Acquisitor — Gonti's "cast face-down from exile"
permission is a known long-clock grant. Likely a Gonti-style
grant surviving past its declared expiry.

## Cross-seed pattern

**ZoneCastGrantExpiry shows up 3 times** (seeds 42, 31415, 271828)
across the wide sweep — that's the dominant signature at 10K depth.
The earlier r60 ZoneCastGrant work (PR #106, follow-up) cleared the
high-frequency surfaces (Yawgmoth's Agenda / Cruelclaw / Narset /
heist arm / may_play_exiled_free arm); the remaining hits are the
long tail of rare cast-from-zone permission lifecycles. Likely
candidates from the new signatures:

- Kess Dissident Mage (cast-and-exile from graveyard)
- Gonti Canny Acquisitor (cast face-down from exile)
- A Kaiso / Faldorn variant of cast-from-some-zone

## Conclusion

At 10K depth the engine has a **bounded long-tail surface of ~0.05%
per game** consisting of 5 distinct signatures across the 10-seed
sweep. None of the post-Athreos / post-Charix "officially clean"
trajectory is broken; the deeper sample just exposes more rare-
window bugs that exist at every depth.

Next chases:

1. **ZoneCastGrantExpiry remainder** — most-frequent signature
   across 3 seeds; concentrated investigation in
   `zone_cast.go` lifecycle paths (Kess / Gonti / Kaiso permissions)
   may close all three with one fix wave.
2. **Seed 1337 game 8921 CardIdentity x40** — single deep leak,
   high-multiplier; likely a Magda / Shiko / Zidane per_card
   handler stealing without the established "validate-still-in-
   source-zone" defensive check from Athreos / Gisa / Adric.
3. **Seed 99 game 9804 ResourceConservation x2** + **seed 31415
   game X SBACompleteness x6** — lower priority; one-game / few-hit
   surfaces.

The r60 era status is now: **clean across 5/10 seeds at 10K; 5
seeds surface 5 distinct bounded long-tail residuals at extreme
sample depth.** Per-game violation rate is bit-stable at ~0.05%
across the 5K and 10K depths — the engine is not regressing, the
gauntlet is just deep enough to enumerate the rare-window long-tail.

## How to reproduce

```bash
for seed in 42 43 99 7 1337 2024 2025 31415 271828 161803; do
  go run ./cmd/hexdek-loki --games 10000 --seed "$seed"
done
```

Narrow per-seed (the affected game numbers above):

```bash
go run ./cmd/hexdek-loki --games 5485  --seed 42      # ZoneCast expiry game 5480
go run ./cmd/hexdek-loki --games 9810  --seed 99      # ResourceConservation game 9804
go run ./cmd/hexdek-loki --games 8930  --seed 1337    # CardIdentity x40 game 8921
go run ./cmd/hexdek-loki --games 5620  --seed 31415   # ZoneCast game 5610
go run ./cmd/hexdek-loki --games 5410  --seed 271828  # ZoneCast game 5399
```

Total runtime for the full 10-seed extreme sweep ≈ 25-30 minutes on
the reference machine.
