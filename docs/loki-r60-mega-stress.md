# Loki R60 Mega Stress

Date: 2026-05-24
Branch: `dev/loki-r60-mega-stress-r60`
Goal: after PR #178 (SBA-cap mandatory-loop-draw cleanup) closed the
seed-1337 residual and all 5 prior stress seeds returned 0, run a
"wide-seed" stress test across 5 fresh seeds × 2,000 chaos games
each. If every seed returns 0, declare the engine officially clean
at the wide-seed level.

## TL;DR

| Seed | Chaos (2K games) | Nightmare (10K boards) | Verdict |
|:----:|:----------------:|:----------------------:|:-------:|
| 2024 | **0** violations | **0** | ✅ clean |
| 2025 | **0** violations | **0** | ✅ clean |
| 31415 (π) | **2** in 1 game | **2** | ⚠️ residuals |
| 271828 (e) | **0** | **2** | ⚠️ residuals |
| 161803 (φ) | **0** | **0** | ✅ clean |

- Across 10,000 chaos games + 50,000 nightmare boards: **6 violations
  total** (2 chaos in 1 game / 4 nightmare across ≤4 boards), 0 crashes.
- Per-game chaos rate: **0.02%** (2 / 10,000).
- Per-board nightmare rate: **≤0.008%** (4 / 50,000).
- 3 of 5 seeds were fully clean across both phases.
- **NOT officially clean at the wide-seed level.** Two distinct
  residual signatures surfaced: one chaos (Gisa TriggerCompleteness
  on own-sacrifice) and one nightmare (CardIdentity).

## Detailed runs

### Seed 2024 — fully clean

```
chaos:     0 violations / 0 crashes / 2000 clean games (74 g/s, 27.18s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (11059 b/s, 904ms)
```

### Seed 2025 — fully clean

```
chaos:     0 violations / 0 crashes / 2000 clean games (67 g/s, 29.90s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (8704 b/s, 1.15s)
```

### Seed 31415 (π) — 4 total residuals (2 chaos / 2 nightmare)

```
chaos:     2 violations / 0 crashes / 1999 clean games (65 g/s, 30.94s)
nightmare: 2 violations / 0 crashes / 9999 clean boards (11307 b/s, 884ms)
```

#### Chaos: TriggerCompleteness × 2 — Gisa, Glorious Resurrector on own-creature sacrifice

- **Game**: 237 (seed 2401416, perm 0)
- **Turn**: 55 cleanup
- **Commanders**: Old Rutstein / Kodama of the West Tree / Gut, Furious
  Fanatic / Khârn the Betrayer
- **Message**: "death event 'sacrifice' at index 2728 with trigger-bearer
  [{Gisa, Glorious Resurrector 0}] on battlefield, but no subsequent
  trigger/effect event found"

Event 2728 is `sacrifice seat=0 source=Blackbloom Rogue` — seat 0
sacrificing their own creature (Blackbloom Rogue is a creature) as
part of a Birthing Ritual chain (2720–2727). Gisa, Glorious
Resurrector's oracle text triggers only on **opponent's** nontoken
creatures dying, so she SHOULD NOT trigger on a same-controller
sacrifice. The invariant's `checkTriggerCompleteness` already filters
non-creature sacrifices (the 2026-05-24 TriggerCompleteness 8 → 0
fix) but does not yet have a controller-of-deceased filter for
opponent-only triggers like Gisa. Likely fix: when looking for a
trigger-bearer match on a death event, gate `creature_dies_opp`
class triggers (Gisa, Bastion of Remembrance, Diregraf Captain,
Marauding Blight-Priest's "opponent loses" family) on
`death_event.controller != trigger_bearer.controller`.

#### Nightmare: CardIdentity × 2

No game-state context emitted for nightmare violations beyond the
total. The CardIdentity invariant family caught two card-instance
inconsistencies during random-board fuzzing; without per-board
breadcrumbs it can't be isolated to a single card from this run.
The cluster has historical precedent (Adric, Oketra, Dread, etc. —
all closed in prior r60 waves) but the signature is bounded enough
in nightmare boards that no per-game investigation is possible from
the report alone.

### Seed 271828 (e) — 2 nightmare residuals, chaos clean

```
chaos:     0 violations / 0 crashes / 2000 clean games (75 g/s, 26.56s)
nightmare: 2 violations / 0 crashes / 9999 clean boards (10502 b/s, 952ms)
```

Same CardIdentity nightmare signature as seed 31415 — 2 hits, no
chaos surface. The nightmare phase tests random board states (not
playthrough), so this is plausibly a single fragile combination of
cards that happens to surface across multiple seed-dependent random
draws.

### Seed 161803 (φ) — fully clean

```
chaos:     0 violations / 0 crashes / 2000 clean games (60 g/s, 33.19s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (14249 b/s, 702ms)
```

## Residual surface

Two distinct signatures emerged:

### 1. TriggerCompleteness on Gisa, Glorious Resurrector (chaos)

**Frequency**: 1 chaos game out of 10,000 (seed 31415 only).
**Root cause hypothesis**: TriggerCompleteness invariant lacks an
opponent-controller filter for the `creature_dies_opp` trigger class.
Gisa's actual gameplay-correct behavior (don't trigger on own
sacrifice) is being mis-flagged as a missed trigger.
**Severity**: Likely false-positive in the invariant, not an engine
bug. Confirmable by reading `checkTriggerCompleteness`.

### 2. CardIdentity in nightmare boards

**Frequency**: 4 nightmare boards out of 50,000 across 2 seeds.
**Root cause**: unknown from report alone; nightmare phase emits no
per-board context.
**Severity**: Bounded (0.008% of nightmare boards). Same invariant
family that closed multiple times in prior r60 waves.

## Trajectory

| Run | Seeds | Total Chaos | Total Nightmare | Verdict |
|:---:|:------|:-----------:|:---------------:|:--------|
| Final (post-Mascot, PR #169) | 42, 43 | 10,000 | 20,000 | 0/0 |
| Stress (post-SBA-cap, pre-#178) | 99, 7, 1337 | 15,000 | 30,000 | 12/0 (1 game) |
| Post-#178 verification | 42, 43, 99, 7, 1337 | 25,000 | 50,000 | 0/0 |
| **Mega-stress (this run)** | **2024, 2025, 31415, 271828, 161803** | **10,000** | **50,000** | **2/4** |

Cumulative since r41 baseline (1,652 in 5,000 games / seed 41 = 33%
per game), the engine's per-game stochastic violation rate has
fallen by **~99.94%**.

## Conclusion

Engine status: **substantively clean but NOT officially clean at the
wide-seed level**.

- 3 of 5 mega-stress seeds returned 0/0.
- The two seeds that surfaced residuals returned 2 violations each
  on single games / single signatures.
- Both signatures are bounded long-tail bugs (one likely-false-
  positive in an invariant, one nightmare-only CardIdentity).
- Per-game stochastic rate across mega-stress: 0.02% per chaos game,
  0.008% per nightmare board.

The "wide-seed officially clean" declaration that this run was
designed to confirm requires zero violations across all 5 seeds.
That bar was not met. r60 systemic-cluster work is complete; what
remains is a small number of long-tail signatures that need
individual investigation.

The Gisa TriggerCompleteness signature is the most-tractable next
chase: it has a deterministic reproducer (game 237, seed 31415) and
the suspected fix is in the invariant, not the engine.

## How to reproduce all five runs

```bash
go run ./cmd/hexdek-loki --games 2000 --seed 2024     # expect: 0/0
go run ./cmd/hexdek-loki --games 2000 --seed 2025     # expect: 0/0
go run ./cmd/hexdek-loki --games 2000 --seed 31415    # expect: 2 chaos / 2 nightmare
go run ./cmd/hexdek-loki --games 2000 --seed 271828   # expect: 0 chaos / 2 nightmare
go run ./cmd/hexdek-loki --games 2000 --seed 161803   # expect: 0/0
```

Narrow seed-31415 reproducer (game 237 only, single signature):

```bash
go run ./cmd/hexdek-loki --games 240 --seed 31415 --invariant trigger-completeness
```

---

## Post-#184 confirmation re-run (2026-05-24)

After PR #184 (Gisa-opp-only TriggerCompleteness false-positive fix —
`opponentOnlyCreatureDiesTriggers` map in invariants.go) shipped, the
mega-stress was re-run identically (5 seeds × 2,000 chaos games + 10K
nightmare boards each):

| Seed | Chaos (2K games) | Δ vs original | Nightmare (10K boards) | Δ |
|:----:|:----------------:|:-------------:|:----------------------:|:-:|
| 2024 | **0** | flat | **0** | flat |
| 2025 | **0** | flat | **0** | flat |
| 31415 (π) | **0** | **2 → 0 (−100%)** | **2** | flat (separate residual) |
| 271828 (e) | **0** | flat | **2** | flat (separate residual) |
| 161803 (φ) | **0** | flat | **0** | flat |

### Headline

- **Chaos phase: 0 violations across all 5 seeds × 10,000 chaos games
  = 0 in 10,000.** Every chaos-phase signature this mega-stress run
  surfaced is now closed.
- **Nightmare phase: 4 violations across 2 seeds × 10,000 boards each
  = 4 in 50,000 (0.008% per board).** Same CardIdentity signature
  as the original run — unchanged because PR #184 fixed the chaos-
  side Gisa false positive, not the nightmare CardIdentity family.
- **4 of 5 seeds now fully clean both phases** (up from 3/5 in the
  original mega-stress).

### Status update vs prior verdict

The original mega-stress concluded "substantively clean but NOT
officially clean at the wide-seed level" because of two signatures.
With PR #184 merged:

- Gisa TriggerCompleteness false-positive: **CLOSED** (chaos goes
  from 2-in-10,000 → 0-in-10,000 across the wide-seed sweep)
- Nightmare CardIdentity: **STILL OPEN** (4-in-50,000 / 2 boards
  per affected seed)

The "wide-seed officially clean" bar requires zero violations across
all 5 seeds. Chaos phase clears that bar; nightmare does not. The
verdict is now: **officially clean at the chaos level across the
wide seed sweep; nightmare phase still has one bounded long-tail
signature**.

### Reproducer (post-#184)

```bash
go run ./cmd/hexdek-loki --games 2000 --seed 2024     # expect: 0/0
go run ./cmd/hexdek-loki --games 2000 --seed 2025     # expect: 0/0
go run ./cmd/hexdek-loki --games 2000 --seed 31415    # expect: 0 chaos / 2 nightmare
go run ./cmd/hexdek-loki --games 2000 --seed 271828   # expect: 0 chaos / 2 nightmare
go run ./cmd/hexdek-loki --games 2000 --seed 161803   # expect: 0/0
```
