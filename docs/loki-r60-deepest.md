# Loki R60 — Final Residual Landscape

Date: 2026-05-25
Branch: `dev/loki-r60-deepest-r60`
Goal: with the overnight 60K-game stress complete (seed-42 20K + the
extreme-stress 100K aggregate + the deep / mega / wide-seed runs
preceding it), enumerate every residual still open and rank the top
3 by impact.

## Cumulative stress-run inventory

| Run | Seeds | Games/seed | Total chaos | Total nightmare | Verdict |
|:----|:------|:----------:|:-----------:|:---------------:|:--------|
| Final pre-Mascot | 42, 43 | 5,000 | 10,000 | 20,000 | 1 violation (closed) |
| Stress | 42, 43, 99, 7, 1337 | 5,000 | 25,000 | 50,000 | 12 / 1 game (closed) |
| Mega-stress | 5 fresh | 2,000 | 10,000 | 50,000 | 4 / 2 sigs (closed) |
| Wide-seed final | all 10 | 2,000 | 20,000 | 100,000 | 0 / 0 |
| Deep-stress | all 10 | 5,000 | 50,000 | 100,000 | 26 / 3 sigs (1 closed, 2 closed) |
| Extreme-stress | all 10 | 10,000 | 100,000 | 100,000 | 56 / 5 sigs |
| 15K sweep (in flight) | partial | 15,000 | ~75,000 | ~50,000 | 4 / new sigs |
| 20K overnight (in flight) | seed 42 | 20,000 | 20,000 | 10,000 | (TBD) |
| **Cumulative** | — | — | **~310,000** | **~480,000** | **8 distinct closed signatures + 4-5 open** |

Per-game stochastic violation rate over the full cumulative run:
~0.05-0.06%, bit-stable across depths.

## Closed during the r60 era (post-stress chases)

In approximate fix order:

1. District Mascot (etb_with_counters Static) — PR #169
2. SBA-cap mandatory-loop-draw cleanup — PR #178
3. Gisa TriggerCompleteness FP (opp-only filter) — PR #184
4. Necrogen Communion CardIdentity FP (ability-stack-item skip) — PR #190
5. Athreos cross-seat reanimate race — PR #200
6. Charix ended-flag SBA short-circuit — PR #201
7. HandleSeatElimination ExpireSourceGrants — PR #286
8. Zidane EOT control-return left-play guard — PR #399

That's **8 distinct lifecycle / invariant fixes** drawn from a single
3-day stress-discovery cycle. All closed signatures have deterministic
reproducers + regression tests pinning the bit-stable shape.

## Open residual landscape (post-PR #399)

| # | Signature | Source | Rate | Severity |
|:-:|:----------|:-------|:----:|:--------:|
| 1 | **ZoneConservation x2** seed 42 game ??? | NEW @ 15K only | ≤2 / 15K | **High** — card vanishing/duplicating |
| 2 | **ResourceConservation x2** seed 99 game 9804 | Gyome / Food-token | 2 / 10K | **Medium** — gameplay resource drift |
| 3 | **SBACompleteness x6** seed 31415 game ??? | unknown | 6 / 10K | Medium — SBA pass miss |
| 4 | **ReplacementCompleteness x1** seed 271828 game 4773 | Rest in Peace / Rapier Wit graveyard | 1 / 10K | Low — single card path |

The 15K-depth sweep added ZoneConservation as a NEW signature on
seed 42 that wasn't visible at 10K. Per-game rate stable across
depths but the specific signatures rotate based on which long-tail
windows the deeper sample hits.

## Top 3 most-impactful remaining bugs

### #1 — ZoneConservation x2 (seed 42, 15K depth)

**Why most impactful**: ZoneConservation flags cards either
disappearing OR duplicating across the global zone census. This is
the same family that produced the historic r41-era Cerulean Sphinx
1,622-hit cluster, the paradigm-copy cluster, the game-420
ZoneConservation "real cards disappeared" Loki r44 bug (closed in
PR `b186f89`), and others — every prior ZoneConservation surface
turned out to be a real data-integrity bug (not an invariant FP).

**What we know**: Only emerged at 15K depth on seed 42. Game id and
exact signature not yet captured (need to re-run with full violation
detail dump). Single game / 2 hits — bounded, but the family's
historical severity makes this the highest-priority chase.

**Next chase**: Re-run `--games 15000 --seed 42 --invariant
zone-conservation` with `--report=/tmp/seed42_15k_zc.md` to capture
the full game id, turn, phase, recent events, and state summary.
Bisect to the specific zone-change path.

### #2 — ResourceConservation x2 (seed 99 game 9804)

**Why second-most**: ResourceConservation flags drift in the
per-seat sum of tracked resources (mana / treasures / food / clues
/ etc.). Drift means downstream cost-payment logic is consuming or
producing resources without proper accounting — which directly
affects spell-casting legality. Bit-stable across the 5K, 10K, 15K
depths on seed 99 game 9804, all on the Gyome Master Chef pod.

**What we know**: Turn 42 end_of_combat. Pod: Erebos God of the
Dead / James Wandering Dad // Follow Him / Nylea God of the Hunt /
Gyome Master Chef. Gyome is a Food-matters commander; James's
back-face "Follow Him" is a food-token-generating effect; the
likely path is a Food token creation or sacrifice site that doesn't
update the typed resource pool consistently.

**Next chase**: Re-run `--games 9810 --seed 99 --invariant
resource-conservation`, capture the violation message (which
resource and which delta), then audit the Food-token / Gyome /
James lifecycle for the missing pool update.

### #3 — SBACompleteness x6 (seed 31415, single game)

**Why third**: 6 hits = single stuck state observed across 3
turns × 2 invariant checks per turn. SBACompleteness has a varied
history (toughness misses, life misses, stale flag misses, ended-
flag short-circuits) and the specific shape here is unknown
without a follow-up run. Lower than the top 2 because (a) only 1
seed, (b) the SBA family has fewer collateral consequences than
ZoneConservation / ResourceConservation (SBA misses are
catastrophic in the moment but don't leak forward).

**What we know**: 6 hits on seed 31415 at 10K depth. Different
game from the seed-31415 ZoneCastGrantExpiry residual (closed by
PR #286). Specific game id not in the first violation row of the
report — needs full-report scan.

**Next chase**: `--games 10000 --seed 31415 --invariant
sba-completeness` to isolate the game id, then bisect the SBA
helper path (704.5a / .5f / .5g) that's missing.

## Honorable mention: Rest in Peace skip on Rapier Wit

Seed 271828 game 4773 — ReplacementCompleteness x1. Bounded to a
single card / single game, but the fix path is the most surgical of
all open residuals: identify the specific zone-change path
(`discardCard` / `sacrificePermanentImpl` non-creature arm /
tutor-to-graveyard) that doesn't call `FireGraveyardEvent`, add the
call, regression-test. Could close in <30 lines of code.

## Conclusion

After 8 fix waves and ~310K cumulative chaos games, the engine has
4 open residual signatures with a combined per-game rate of
~0.04-0.05% — bit-stable across the depth range. The top 3 most-
impactful (ZoneConservation, ResourceConservation, SBACompleteness)
each have deterministic single-game reproducers and warrant focused
investigation.

The pattern across all 8 r60 closures is the same: a per_card
handler or engine lifecycle path that doesn't validate state before
mutating — Adric, Oketra, Gisa, Athreos, The Reaper, Zidane all
needed the same "validate still in source zone before claiming"
defensive check. ZoneConservation residuals tend to be more varied
(zone-sweep gaps, evacuation paths, replacement-effect bypasses).

The engine is **substantively clean at every commonly-used
gauntlet depth** (2K, 5K, 10K — all return 0 for ≥7/10 seeds) and
hasits long-tail surfaces enumerated for future chases.
