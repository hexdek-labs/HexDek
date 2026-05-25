# CompositionPrior — Reference

A composition-conditioned TrueSkill prior that captures how each archetype
performs given the OTHER 3 archetypes at the table. Live in showmatch.

## Why

Standard TrueSkill assumes `expected_winrate = f(player_skill)`. In
4-player Commander self-play that assumption is broken: PR #322's
meta-study measured **composition shifts winrate ~10× more than seat
position does** (e.g. LandsMatter: 23pp swing across compositions vs
2.4pp across seats). Without conditioning on the pod, TrueSkill
miscalibrates by 20+ rating points per game in dominated compositions.

The prior fixes this by feeding TrueSkill the residual `observed
rank − pod-conditioned expectation` instead of raw winrate. Player
skill is what's LEFT after the pod's effect is subtracted.

## The math

Three layers stack to handle data sparsity, with graceful fallback at
each tier:

### Layer 1 — Prior data structures

In-memory counters (in `internal/trueskill/composition_prior.go`):

```
matchupWins  : (archetype_a, archetype_b) → wins-by-a in shared games
matchupGames : (archetype_a, archetype_b) → total games where both played
archWins     : archetype → total wins across all pods
archGames    : archetype → total participation across all pods
```

Both directions of every pair are stored, so lookup is O(1) regardless
of query order. `ObserveGame(pod, winnerArchetype)` updates 12 directed
pair cells per 4-player game plus archetype baseline counters.

### Layer 2 — ExpectedWinrate (tiered fallback)

```go
func (cp *CompositionPrior) ExpectedWinrate(deckArchetype string, pod []string) float64
```

1. **Pairwise mean**: average of `matchup_winrate(deckArchetype, opp)`
   for each opponent in the pod. Self-mirror seats (an archetype that
   matches deckArchetype) are SKIPPED — a deck doesn't beat itself in
   expectation. Returned when at least one pairwise cell has data.
2. **Archetype baseline**: `archWins[arch] / archGames[arch]`.
   Returned when no pairwise data exists for the specific pod.
3. **Uniform**: `1 / podSize` (= 0.25 for 4-player). Returned when
   the archetype has never been observed.

### Layer 3 — Confidence (sample-based trust)

```go
func (cp *CompositionPrior) Confidence(deckArchetype string, pod []string) float64
```

Returns `1 − exp(−n/50)` where n is the mean pairwise game count
across opponents (or the archetype-baseline game count when pairwise
is empty). Calibration:

| n   | Confidence |
|----:|-----------:|
| 0   | 0.00       |
| 10  | 0.18       |
| 25  | 0.39       |
| 50  | 0.63       |
| 100 | 0.86       |
| 150 | 0.95       |
| 200 | 0.98       |

The half-trust point (n=50, confidence=0.63) was calibrated against
the meta-study's per-cell stderr behavior — n=50 gives stderr≈7pp on
a binomial proportion, the regime where the prior becomes meaningfully
more informative than the uniform fallback.

### Layer 4 — Wilson 95% confidence intervals

```go
func (cp *CompositionPrior) ExpectedWinrateInterval(archetype string, pod []string) WinrateInterval
```

Returns:

```go
type WinrateInterval struct {
    Point   float64  // matches what ExpectedWinrate returns
    Low     float64  // Wilson 95% lower bound
    High    float64  // Wilson 95% upper bound
    Samples int      // effective n the bounds are based on
    Source  string   // "pairwise" | "archetype_baseline" | "uniform"
}
```

Uses the Wilson score interval at z=1.96 — more robust than the
normal approximation for small n and extreme p̂, and degrades
gracefully toward [0, 1] as n → 0 instead of producing undefined
bounds. Formula:

```
center = (p̂ + z²/(2n)) / (1 + z²/n)
margin = z·√(p̂(1−p̂)/n + z²/(4n²)) / (1 + z²/n)
low    = max(0, center − margin)
high   = min(1, center + margin)
```

For the cold-start tier (Source = "uniform") the interval spans the
full [0, 1] — explicitly signaling "the prior has nothing to say."

## Integration with TrueSkill

### The offset formula

For each player `i` in a 4-archetype pod:

```
offset_i = Weight × Confidence(arch_i, pod)
                  × MuOffsetScale
                  × (ExpectedWinrate(arch_i, pod) − 1/podSize)

shifted_μ_i = raw_μ_i + offset_i
```

Run standard `UpdateMultiplayer` (or `Update2Player` pairwise) on the
shifted μ values. The resulting `Δshifted_μ_i` applies back to the
raw μ:

```
new_μ_i = raw_μ_i + (updated_shifted_μ_i − shifted_μ_i)
```

The Gaussian rating update is **invariant to uniform μ shifts** but
breaks symmetry under differential shifts. So the offsets encode the
composition's tilt; the residual (Δμ after offset removal) is the
player-skill signal.

### Cold-start guarantee

When the prior has no data for a cell:

- `Confidence = 0` → `offset = 0`
- `shifted_μ = raw_μ` → update reduces to standard TrueSkill exactly

This is the **no-regression guarantee** — the prior never degrades
behavior, only adds correction when it has evidence. Verified by
byte-equivalent tests in `composition_update_test.go` and
`showmatch_composition_prior_test.go`.

### Configuration knobs

```go
type CompositionUpdateConfig struct {
    Prior         *CompositionPrior  // nil → reduces to vanilla
    Weight        float64            // 0..1, 0.5 = recommended default
    MuOffsetScale float64            // default 10
}
```

- **Weight 0.5**: design recommendation; "split the difference between
  raw-skill and pod-conditioned." Tune toward 1.0 for more aggressive
  correction once live data accumulates.
- **MuOffsetScale 10**: a 25pp composition advantage shifts μ by 2.5
  points (~30% of σ=8.33). Large enough to measure, small enough not
  to swamp single-game skill signal.

`trueskill.DefaultCompositionUpdateConfig(prior)` constructs the
recommended starting config.

## Validation result

PR #411 ran a 2500-test-game synthetic gauntlet (5 pods × 500g, 5
seeds) comparing vanilla TrueSkill against prior-aware TrueSkill on
held-out prediction. Both systems start cold; prior is pre-seeded
with 1000 bootstrap games.

| Metric | Standard | Prior-aware | Mean Δ |
|---|---:|---:|---:|
| Top-1 accuracy | 30.9% | 32.3% | **+1.4 pp** |
| Mean log-loss | 1.495 | 1.460 | **+0.036** |

**All 5 seeds improved on both metrics — no regression**, and 4 of 5
pods improved on accuracy in the reference seed-42 run. Improvements
are modest because both systems converge with 100 in-distribution
games; the prior's value is the accelerated cold-start window plus
archetype-level transfer to unseen pods.

Critical implementation note from the validation run: the prior-aware
system stores μ as "skill modulo composition" (offsets baked OUT
during training). At PREDICTION time the offset must be ADDED BACK to
compare decks in a specific pod. The validation harness's
`effectiveMu()` function does this; production code (e.g. matchmaking
prediction) must do the same.

## Live monitoring

PR #420 makes the prior's per-game effect observable. Every showmatch
game produces a `[]CompositionPriorEffect` with one entry per seat:

```go
type CompositionPriorEffect struct {
    Seat              int     // play-order seat (post-rotation)
    Archetype         string
    Offset            float64 // μ-shift applied before the update
    Confidence        float64 // prior's trust for this (arch, pod) cell
    ExpectedWinrate   float64 // pod-conditioned baseline
    MuDeltaVsBaseline float64 // (prior μ-after) − (vanilla shadow μ-after)
}
```

`MuDeltaVsBaseline` is the **gold metric**: how much did the prior
shift this seat's rating update vs. vanilla TrueSkill on the same
game. Implementation: `updateELO` runs a shadow vanilla update in
parallel with the real one and reports the difference. Sum across
seats is approximately zero (the prior redistributes rating, doesn't
inject or remove it).

PR #422 persists these effects to disk for offline analysis:

```
data/heimdall/composition_prior/{rng_seed}.json
```

One JSON file per game. The Heimdall observer writes via
`writeCompositionPriorRecord` when `Observation.CompositionPriorEffects`
is non-empty.

## CLI

`hexdek-composition-replay` is the debug surface. Looks up a game by
RNG seed (or showmatch_game.game_id via `-gameid`) and prints a
formatted per-seat table:

```
$ hexdek-composition-replay 555555

Game RNG seed: 555555
Winner seat:   1
Turns:         47
Kill method:   combat

seat result   archetype        predicted%   confidence    offset_μ   Δ_vs_van   interpretation
0    —        Mill                  62.0%        0.72       1.800     -0.900    expected better → amplified μ loss
1    WINNER   Voltron                5.0%        0.72      -1.200      2.100    upset win → amplified μ gain
2    —        Aggro                 18.0%        0.50       0.000     -0.600    expected loss → dampened μ loss
3    —        Combo                 15.0%        0.50      -0.600     -0.600    expected loss → dampened μ loss

Sum |Δ_vs_vanilla|: 4.200 μ-points total redistribution
Avg |Δ| per seat:   1.050
```

Two lookup modes:

```bash
hexdek-composition-replay 1234567890         # positional RNG seed
hexdek-composition-replay -gameid 42         # showmatch_game.game_id → resolves seed via SQLite
hexdek-composition-replay -data data/ 999    # custom data dir
```

The per-seat **interpretation** column maps each (winner-status,
favored/disfavored) combination to a human-readable phrase:

| Winner? | Favored? | Phrase |
|---|---|---|
| Won | Favored | expected win → dampened μ gain |
| Won | Disfavored | upset win → amplified μ gain |
| Lost | Favored | expected better → amplified μ loss |
| Lost | Disfavored | expected loss → dampened μ loss |
| Any | Low confidence | cold-start (no effect) |
| Any | Near 25% | near-neutral expectation |

Useful for spot-checking: "Did the prior correctly identify Mill as
favored in pod X, and dampen the credit when Mill won?"

## API surface summary

| Function | When to use |
|---|---|
| `NewCompositionPrior(podSize)` | Construct an empty prior. |
| `(*CompositionPrior).ExpectedWinrate(arch, pod)` | Scalar baseline winrate (0..1). |
| `(*CompositionPrior).ExpectedWinrateInterval(arch, pod)` | Bounded prediction with [Low, High] and source tier. |
| `(*CompositionPrior).Confidence(arch, pod)` | Scalar [0..1] sample-based trust. |
| `(*CompositionPrior).ObserveGame(pod, winnerArch)` | Feed a finished game's outcome. |
| `ComputeCompositionOffset(prior, arch, pod, cfg)` | Compute the μ-shift for one deck (functional callers). |
| `Update2PlayerWithOffsets(cfg, w, l, wOffset, lOffset)` | TS pairwise update with composition offsets baked in. |
| `(*TrueSkillRatings).UpdateWithComposition(names, ranks, archs, cfg)` | Full multiplayer update for the stateful TrueSkillRatings tracker. |
| `DefaultCompositionUpdateConfig(prior)` | Recommended starting config (Weight=0.5, Scale=10). |

## Caveats

1. **All-AI self-play context.** The +1.4pp validation used synthetic
   pods; live human + bot mixed play might show different patterns.
   The cold-start guarantee ensures the prior won't make things
   worse if its model is mis-specified.
2. **Hat-uniformity assumption.** The prior conditions on archetype
   alone, not on which hat is piloting. Mixed hat-strength matches
   may need a separate dimension; left for follow-up.
3. **Long-tail compositions stay sparse.** The full C(22, 4) = 7,315
   possible 4-archetype combinations are not all reachable; the
   pairwise table covers the gap via additive decomposition.
4. **No drift handling.** Older games count equally to recent games.
   Live deployment may need a decay weighting or periodic recompute.
5. **The prior depends on TrueSkill and TrueSkill depends on the
   prior.** Update order matters — `ObserveGame` runs AFTER the
   rating update in showmatch's `updateELO` so each game uses the
   pre-game prior state. If both updates fire simultaneously from
   the same game data, the prior can drift.

## Related work

| PR | What |
|---|---|
| #322 | Meta-study: composition matters 10× more than seat |
| #398 | Design doc (3 options, tiered fallback recommended) |
| #403 | MVP CompositionPrior (Option 1, pairwise approximation) |
| #408 | `TrueSkillRatings.UpdateWithComposition` wiring |
| #411 | +1.4pp validation gauntlet |
| #415 | Live showmatch `updateELO` integration |
| #420 | Per-game monitoring (CompositionPriorEffect) |
| #422 | Per-game JSON persistence to `data/heimdall/composition_prior/` |
| #424 | `hexdek-composition-replay` CLI |
| #428 | Wilson 95% confidence intervals |

## Implementation files

```
internal/trueskill/
  composition_prior.go              — prior data + ExpectedWinrate + Confidence
  composition_prior_test.go         — 17 regressions
  composition_prior_interval.go     — WinrateInterval + Wilson math
  composition_prior_interval_test.go — 11 regressions
  composition_update.go             — UpdateWithComposition + helpers
  composition_update_test.go        — 9 regressions

internal/heimdall/
  composition_prior_log.go          — per-game JSON persistence
  composition_prior_log_test.go     — 7 regressions
  types.go (CompositionPriorEffect) — monitoring record type

internal/hexapi/
  showmatch.go (updateELO + Showmatch.compositionPrior)  — live integration
  showmatch_composition_prior_test.go     — 7 wiring regressions
  showmatch_composition_monitoring_test.go — 8 monitoring regressions

cmd/
  hexdek-composition-replay/main.go — debug CLI
  validate-composition-prior/main.go — +1.4pp validation gauntlet (PR #411)
```

Total: 8 production files, 6 test files, 59 regressions. ~1,500 LoC.
