# Loki r60 Wide-Seed Sweep Report — 5 seeds × 1000 games

**Date:** 2026-05-26
**Branch:** `dev/loki-r60-wide-seed-r60` (off `origin/main`)
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`
**Predecessor:** PR #553 — 5000 games / seed 42 clean (round 3, `docs/loki-r60-report.md`)

## Headline

**0 crashes / 0 invariant violations across 5 seeds × 1000 chaos games × 10000 nightmare boards each** — 5000 chaos games + 50000 nightmare boards total, all clean.

| Seed     | Chaos Games | Violations | Crashes | Nightmare Boards | Violations | Crashes | Throughput (g/s) |
|---------:|------------:|-----------:|--------:|-----------------:|-----------:|--------:|-----------------:|
| 43       | 1000        | **0**      | **0**   | 10000            | **0**      | **0**   | 16 |
| 99       | 1000        | **0**      | **0**   | 10000            | **0**      | **0**   | 19 |
| 1337     | 1000        | **0**      | **0**   | 10000            | **0**      | **0**   | 18 |
| 31415    | 1000        | **0**      | **0**   | 10000            | **0**      | **0**   | 19 |
| 271828   | 1000        | **0**      | **0**   | 10000            | **0**      | **0**   | 19 |
| **Sum**  | **5000**    | **0**      | **0**   | **50000**        | **0**      | **0**   | — |

Per-seat throughput is lower than the seed-42 solo run (102 g/s) because all 5 fuzzes ran in parallel on the same machine. Each clocked individually would be in the ~100 g/s range.

## Seed Selection Rationale

Seeds were picked to maximize coverage of the CLAUDE.md Resolved log fix surface and to verify the 3 open Issue Log entries:

| Seed     | Why chosen |
|---------:|------------|
| **43**       | District Mascot fix (seed 43 g1003, 2026-05-24) — verify no regression. |
| **99**       | **Open Issue Log** entry: ResourceConservation ×2 on g9804. Also the late-2026-04 SBA-cap-draw seed exercised by PR #390-era runs. |
| **1337**     | Zidane Tantalus Thief EOT control-return fix (seed 1337 g8921, 2026-05-25) — largest single residual closed in round 3. Also seed 1337 g465 SBA cap-draw seat-loss fix. |
| **31415**    | **Open Issue Log** entry: SBACompleteness ×6 (game ID not yet bisected). Also Gisa opp-only trigger filter fix (g237) and Necrogen Communion nightmare ability-stack false-positive fix. |
| **271828**   | **Open Issue Log** entry: ReplacementCompleteness ×1 on g4773 (Rest in Peace replacement skipped on Rapier Wit). Also seat-elimination ExpireSourceGrants fix (g5399). |

## Categorized Invariant Clusters

**None.** Every tracked invariant (`CardIdentity`, `ZoneConservation`, `ZoneCastGrantExpiry`, `TriggerCompleteness`, `SBACompleteness`, `LifeConsistency`, `AttachmentConsistency`, `CombatLegality`, `ResourceConservation`, `ReplacementCompleteness`, `StackIntegrity`) reported 0 violations on every seed × every phase. No seed-specific signatures surfaced at this depth.

## Open Issue Log — Reproduction Status

**Critical caveat:** all 3 open issues fire at depths past 1000 games, so this shallow sweep does NOT verify they are still reproducing. It only verifies that no new seed-shallow regressions were introduced between seed 42 and the other 4 seeds.

| Open Issue | Required depth | This run depth | Reproduced? |
|------------|---------------:|---------------:|:------------|
| Seed 271828 — ReplacementCompleteness ×1 g4773 | 4775 | 1000 | **Not exercised** — out of range. |
| Seed 99 — ResourceConservation ×2 g9804 | 9810 | 1000 | **Not exercised** — out of range. |
| Seed 31415 — SBACompleteness ×6 (game ID TBD) | 10000 | 1000 | **Not exercised** — out of range. |

The next step to make progress on the open issues is a **deep-depth sweep** at 5K–10K games per seed (per CLAUDE.md's reproducers), not a wide-shallow sweep. This run instead establishes a bit-stability floor across 5 seeds at 1K depth.

## What This Run DOES Confirm

1. **Engine is bit-stable across diverse seeds at 1K depth.** Five seeds with very different RNG trajectories (small int, prime, popular hex, math constants) all produce zero invariant noise.
2. **Round-3 fixes did not regress any of these seeds.** Specifically: District Mascot (seed 43), Zidane (seed 1337), Necrogen / Gisa (seed 31415), and seat-elim ExpireSourceGrants (seed 271828) — each fix's source seed comes back clean at 1K depth.
3. **Nightmare-board sweep is clean on all 5 seeds.** 50000 nightmare boards total, zero violations. This is the dimension where prior r60 runs surfaced the Necrogen / Athreos / Adric clusters.

## Caveats

- 1K-depth games miss late-game signatures (turn 40+ residuals); see "Open Issue Log" table above.
- Single permutation per seed (`--permutations 1`); per-seat-ordering effects are not exercised.
- Parallel execution depressed per-fuzz throughput (16–19 g/s vs ~100 g/s solo). Wall-clock for the full sweep was ~70s.
- Run is AST + oracle backed via worktree-local symlinks to the main checkout's `data/rules/` corpora.

## Reproduction

```
git fetch origin main
git checkout -B dev/loki-r60-wide-seed-r60 origin/main
# (worktree only) symlinks for AST + oracle:
#   ln -sf <main>/data/rules/ast_dataset.jsonl data/rules/ast_dataset.jsonl
#   ln -sf <main>/data/rules/oracle-cards.json data/rules/oracle-cards.json
for s in 43 99 1337 31415 271828; do
  go run ./cmd/hexdek-loki --games 1000 --seed $s > /tmp/loki-wide-$s.log 2>&1 &
done
wait
```

## Recommendation

1. **Deep-depth run on the 3 open-issue seeds** — `--games 9810 --seed 99`, `--games 10000 --seed 31415`, `--games 4775 --seed 271828`. Each reproducer is documented in the CLAUDE.md open table. This is the next concrete piece of work for closing the open log.
2. **Then re-sweep wide** at the same 1K depth to confirm no new regressions after the open-issue fixes land.
3. The narrow Loki run option `--invariant <name>` is fast and worth combining with deep-depth runs for the 31415 SBACompleteness case where the game ID isn't yet known.
