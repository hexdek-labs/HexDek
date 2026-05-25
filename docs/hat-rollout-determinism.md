# Hat Rollout Determinism Audit (R60 round 5)

Date: 2026-05-24
Branch: `dev/hat-rollout-determinism-r60`
Scope: every RNG source touched during a hat decision rollout —
`MCTSHat.simulateRollout`, `YggdrasilHat.simulateRollout`,
`YggdrasilHat.multiRolloutForCard`, and any default-source `rand`
call that influences MCTS branch selection.

## TL;DR

**One real determinism gap found and fixed.** The per-rollout RNG
seed was derived from a package-level mutable global
(`var rolloutSeedCounter int64` in `internal/hat/rollout.go`),
which made hat behavior depend on the order tests / games / parallel
goroutines fired rollouts in the same process. Replaced with a
per-hat `rolloutSeed int64` field, reset on `game_start` in
`ObserveEvent`.

**One residual non-determinism documented, not fixed.**
`YggdrasilHatWithNoise` seeds its `noiseRNG` from `rand.Int63()` on
the default global source, which is auto-seeded by `time.Now()` since
Go 1.20. This is intentional: live-play hats are *supposed* to vary
their tiebreaker noise across games so the AI isn't predictable.
Replays and tests that need reproducible noise should construct the
hat with a fixed-seed RNG (see "Replay reproducibility" below).

## The determinism gap

`internal/hat/rollout.go` pre-fix:

```go
// rolloutSeedCounter is bumped per-rollout within a decision to give each
// candidate a different RNG stream.
var rolloutSeedCounter int64

func (h *MCTSHat) simulateRollout(gs *gameengine.GameState, seatIdx int, actionFn func(clone *gameengine.GameState)) float64 {
    rolloutSeedCounter++
    rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + rolloutSeedCounter))
    ...
}
```

Three concrete failure modes the package global produced:

1. **Test-order dependence.** A test that ran rollouts mutated the
   counter; a later test starting from the same configured
   `GameState` sampled a *different* RNG stream → potentially
   different chosen action → flaky CI. The Go test runner does not
   guarantee deterministic test ordering across runs (especially
   with `-shuffle`), so two CI runs could produce different test
   outcomes from byte-identical code.

2. **Data race under parallel runners.** The tournament gauntlet
   runs games on multiple goroutines; the `++` is not atomic and
   the var has no mutex. Under contention this could produce
   duplicate seeds (two rollouts in lockstep) or torn writes on
   32-bit platforms. `go test -race` would flag this on any parallel
   test that exercised rollouts.

3. **Cross-game leakage within a process.** Game N's rollout seed
   carried into game N+1. Two "fresh" games starting from the same
   `GameState` and same configured hats produced different rollout
   sequences depending on how many rollouts the previous game ran.

The same anti-pattern existed in three call sites — all of them
bumping the same global:

- `internal/hat/rollout.go:77` — `MCTSHat.simulateRollout`
- `internal/hat/information_set.go:55` — `YggdrasilHat.multiRolloutForCard`
- `internal/hat/yggdrasil.go:8670` — `YggdrasilHat.simulateRollout`

## The fix

Per-hat seed counter, reset on `game_start`. Both `MCTSHat` and
`YggdrasilHat` now carry a `rolloutSeed int64` field. The package
global is gone — `grep rolloutSeedCounter internal/hat/` returns
empty.

Seed formula is otherwise unchanged:

```go
h.rolloutSeed++
rng := rand.New(rand.NewSource(
    int64(gs.Turn)*1000 + int64(seatIdx)*100 + h.rolloutSeed,
))
```

The reset hook is the same `game_start` event that already resets
`actionStats` and `totalVisits` — see `MCTSHat.ObserveEvent`
(`mcts.go`) and `YggdrasilHat.ObserveEvent` (`yggdrasil.go`). So a
hat that observes `game_start` between games will reproduce identical
rollout sequences for game N+1 as it did for game N given identical
inputs.

Regressions in `internal/hat/rollout_determinism_r60_test.go` pin:

1. Two independent `MCTSHat` instances have independent counters
   (bumping A's seed doesn't move B's).
2. `MCTSHat.ObserveEvent({Kind: "game_start"})` resets
   `rolloutSeed` to 0.
3. Same for `YggdrasilHat` on both axes.
4. End-to-end: two fresh hats from an identical starting state
   produce the same rollout score regardless of intervening rollouts
   by an unrelated hat in the same process.

## Residual non-determinism (intentional)

`internal/hat/yggdrasil.go:312`:

```go
noiseRNG: rand.New(rand.NewSource(rand.Int63())),
```

`YggdrasilHatWithNoise` constructs a per-hat `noiseRNG` whose seed
comes from `rand.Int63()` on the global default source. Since Go
1.20, that source is auto-seeded by `time.Now()` at process start.
Same process → same seed; new process → different seed.

This is **intentional** for live tournament play: the noise RNG
breaks ties between near-equivalent choices ("we have three Mountains
to tap; which one?") and exists specifically so the AI doesn't play
identically every game when paired into a long match. The current
auto-seeding gives "different process, different sequence" which is
the right shape for live play but the wrong shape for replay or
fixture-based tests.

**Replay reproducibility recipe.** Callers that need bit-identical
behavior across processes should construct the hat without noise:

```go
h := NewYggdrasilHat(strategy, budget)  // no-noise constructor
```

Or seed deterministically:

```go
h := NewYggdrasilHatWithNoise(strategy, budget, 0.2)
h.noiseRNG = rand.New(rand.NewSource(42))  // fixture seed
```

(The field is package-private; a setter could be added if external
callers need it. Today only the tournament + Goldilocks harness
construct hats, and both have access to the package.)

## Other audit findings (clean)

| Surface | Verdict | Notes |
|---|---|---|
| `MCTSHat.actionStats` | clean | per-hat map, reset on game_start, turn-scoped keys prevent cross-turn UCB inflation |
| `MCTSHat.totalVisits` | clean | per-hat, reset on game_start |
| `YggdrasilHat.actionStats` / `totalVisits` | clean | same shape as MCTSHat |
| `YggdrasilHat.planState` / opponent observation arrays | clean | per-hat, all reset on game_start |
| `simulateRollout` rng usage in `CloneForRollout` | clean | rng is passed in; clone uses only the passed rng for determinization |
| `determinize` opponent-hand sampling | clean | uses the passed-in rng exclusively |
| `selectAmongTop` (YggdrasilHat) | clean | uses `noiseRNG` (intentional non-determinism documented above) |
| Map iteration order in candidate scoring | clean | candidates collected into a slice before sorting; map iteration only affects log ordering, not chosen card |
| `sort.SliceStable` on equal UCB values | clean | stable sort + deterministic input order → deterministic output |

## How to re-run the audit

```bash
# 1. No package globals storing per-rollout state:
grep -rn 'var rolloutSeedCounter' internal/hat/  # must be empty

# 2. No call to rand.* on the default source from a rollout path:
grep -rn 'rand\.\(Int\|Float\|Read\|Seed\)' internal/hat/ | \
    grep -v _test.go | grep -v noiseRNG

# 3. Race-free under parallel:
go test ./internal/hat/... -count=1 -race

# 4. Order-independent under shuffle:
go test ./internal/hat/... -count=10 -shuffle=on
```
