# Chaos Gauntlet Report

Generated: 2026-05-24T17:01:23-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 2000 |
| Seed | 31415 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 25.118s |
| Throughput | 80 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 0 (in 0 games) |
| Clean Games | 2000 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 801ms |
| Throughput | 12483 boards/sec |
| Crashes | 0 |
| Invariant Violations | 2 |
| Clean Boards | 9999 |

## Invariant Violations (Nightmare Boards)

| Invariant | Count |
|-----------|-------|
| CardIdentity | 2 |

## Verdict: ISSUES FOUND

**2 total issues** across 2000 chaos games and 10000 nightmare boards.
- 0 crashes in chaos games
- 0 invariant violations in chaos games
- 0 crashes in nightmare boards
- 2 invariant violations in nightmare boards

Review the details above to identify which cards and interactions are problematic.
