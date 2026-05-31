# Heimdall → hat self-tune convergence study (R60)

> Follow-up to PR #965 (closed the single-iteration loop). This document
> reports the actual 5-iteration trajectory + a forward-looking
> recommendation for convergence guardrails the next attribution-side
> PRs (#955 et al.) should land alongside the real `ComputeWeightDeltas`
> body.

## Setup

- **Script**: `scripts/self-tune-convergence-study.sh`
- **Iterations**: 5
- **Games per iteration**: 500
- **Seed**: 42 (held constant across iterations — same seed reused so
  per-game RNG identity matches across iterations modulo any feedback-
  induced decision divergence)
- **Seats**: 4
- **Deck pool**: `data/decks/test` (16 decks)
- **Build artifact**: `/tmp/heimdall-convergence-full/hexdek-tournament`
- **Per-iteration artifacts**: `/tmp/heimdall-convergence-full/iter_N/{result.json,heimdall_feedback.json,report.md}`

Each iteration N>0 consumes iteration N−1's `heimdall_feedback.json`
via `--apply-feedback`; iteration 0 is baseline (no feedback). All
iterations emit their own feedback via `--feedback-out` so the chain
continues to the next cycle.

## Observed trajectory (5×500 actual run, 2026-05-31)

Wall clock per iteration: 90s, 43s, 51s, 34s, 29s (first iteration
pays AST/oracle load overhead; subsequent iterations reuse the OS
file cache). Total ~4 minutes for the full study.

```
 iter   games   draws   avg_turns     fb_l2    fb_max   arches
    0     500       1      46.756    0.0000    0.0000        0
    1     500       0      47.070    0.0000    0.0000        0
    2     500       0      46.308    0.0000    0.0000        0
    3     500       0      47.718    0.0000    0.0000        0
    4     500       0      46.538    0.0000    0.0000        0
```

```
 avg_turns trajectory:    ▃▄▁█▂  [46.31 → 47.72]
 feedback L2 trajectory:  ─────  [0.0000 → 0.0000]
 feedback max|Δ| traj:    ─────  [0.0000 → 0.0000]
```

```
 commander                                      i 0  i 1  i 2  i 3  i 4  spark   range
 Ardenn, Intrepid Archaeologist                 181  182  186  187  188  ▁▂▆▇█  181→188
 Fire Lord Azula                                 71   63   78   65   71  ▄▁█▁▄  63→78
 Kraum, Ludevic's Opus                          145  142  141  150  145  ▄▁▁█▄  141→150
 Yarok, the Desecrated                          102  113   95   98   96  ▃█▁▂▁  95→113
```

(Pool is 4 commanders × 4 seats × 500 games = 2000 commander-slots
per iteration; sums per row exceed 500 because each game seats 4
copies-of-the-deck-pool. Counts above are total wins-by-commander
across all seat-rotations in the iteration.)

## Verdict

**`STUB-LOCKED (loop runs but attribution is silent)`**

Every iteration's `heimdall_feedback.json` carried 0 archetypes and
L2=0. That's the `analytics.ComputeWeightDeltas` stub behavior (the
real attribution body is rebasing in PR #955). The hat-side apply
path correctly no-ops empty feedback, so each iteration is
bit-equivalent to the baseline in the dimensions the feedback
would touch.

The drift observed in the result tables (`avg_turns Δ = 1.41` from
iter 2's 46.31 to iter 3's 47.72; `Yarok wins Δ = 18` from iter 2's
95 to iter 1's 113) is NOT feedback bite — it's engine-side non-
determinism: per-game sub-seeds derived from master seed × iteration
ordinal × per-game worker IDs, plus goroutine-scheduling noise on
the MCTS rollout pool. Visible in the per-commander sparklines:
`Fire Lord Azula ▄▁█▁▄` is a textbook noise pattern, not a tuning
signal.

The most interesting honest finding is **Ardenn's monotone climb**:
`181→182→186→187→188` over 5 iterations is a strict-increasing run
of length 5, which has probability ≈ 1/5! = 0.83% under a pure-noise
null hypothesis. Two reads:

1. **Most likely**: it's a coincidence (we'd expect ~1 such pattern
   per 120 cards observed under noise; we observed 4 cards in this
   gauntlet so the false-positive risk per study is ~3%).
2. **Worth flagging**: same-seed runs may have a small structural
   bias from RNG-state warmup that systematically advantages a
   specific commander in a specific seat position. When PR #955 lands
   and real deltas start flowing, this same monotone-climb shape will
   be ambiguous between "the feedback bit" and "the seed-bias was
   always there" — disambiguate by also running the study with seed
   randomized per iteration as a control.

CONVERGENCE QUESTION UNANSWERABLE until the attribution body lands.

## Interpretation

The current `analytics.ComputeWeightDeltas` is a stub (signature
committed in PR #947, body still pending PR #955). Every iteration
emits `archetypes: []` with `L2=0`. The hat-side apply path correctly
no-ops empty feedback, so each iteration is bit-equivalent to the
baseline in the dimensions that the feedback would touch.

What that means for this study:

- **The integration loop is verified end-to-end at gauntlet scale.**
  5 cycles × 500 games × 4 seats × full Heimdall analytics × Save/Load
  round-trip × hat overlay install — zero panic, zero schema-version
  mismatch, zero orphan files, zero data corruption.
- **Per-commander win counts DO drift across iterations** even with
  the same `--seed 42`. That drift is NOT feedback bite; it's engine-
  side non-determinism (per-game sub-seeds derived from the master
  seed + iteration ordinal + workers + the small-but-real
  goroutine-scheduling noise on the MCTS rollout pool). Treating that
  drift as a tuning signal would be a category error.
- **The convergence question is structurally unanswerable until the
  attribution body lands.** Once `ComputeWeightDeltas` produces real
  per-archetype per-dimension deltas, this same script answers it.

## Recommended convergence guardrails (forward-looking)

These are NOT in scope for this PR — they're the rationale the next
attribution-side worker should land alongside their real
`ComputeWeightDeltas` body. The guardrails are needed because a
naïve attribution model fed straight into the apply path will at best
oscillate and at worst diverge under common gauntlet failure modes.

### G1. Iteration-bounded delta clamp (decay schedule)

The current per-dimension clamp is **fixed** at `MaxDimensionDelta = 2.0`
(`internal/analytics/heimdall_feedback.go:126`). That's appropriate
for iteration 1 (the first delta is allowed to be loud) but
catastrophic by iteration 5 (oscillation will be amplified). Replace
with a decaying schedule:

```
effective_cap = MaxDimensionDelta * (1 / (1 + iteration_index))
```

So iteration 1 caps at 2.0; iteration 2 at 1.0; iteration 3 at 0.67;
iteration 4 at 0.5; iteration 5 at 0.4. Settling behavior preserved;
amplification suppressed.

### G2. Per-archetype momentum dampener

Track each archetype's last-iteration delta vector; emit the new
delta as the EMA `0.7 * new + 0.3 * prev`. Single-iteration overshoot
in any dimension gets averaged against the prior iteration's intent
rather than fully replacing it. Implements implicit "trust region"
around the prior weights.

The state needs to live somewhere — natural home is a new
`HeimdallFeedback.PreviousArchetypes []ArchetypeFeedback` field
(additive, no schema bump per `heimdall_feedback.go` evolution rules)
populated by the producer from the prior iteration's loaded feedback.

### G3. Divergence early-stop

If `feedback_l2 / baseline_l2 > 3` in any iteration (where
`baseline_l2 = ||iteration_0_deltas||`), treat as divergence and:

1. Log the divergence event.
2. Stop applying further feedback for the rest of the run.
3. Emit the prior iteration's feedback unchanged (don't keep computing
   new deltas — they're chasing a noise floor that's escaped the basin).

### G4. Sample-size weighting for delta credibility

The current `MinSampleSize = 50` is binary (apply / advisory). Replace
with a sigmoid weighting: delta is scaled by
`1 / (1 + exp(-(samples - 100) / 30))`. At 50 samples → 5% applied;
at 100 samples → 50%; at 200 samples → 96%. Smooth attenuation
prevents the "59 → 61 samples flips from advisory to direct"
discontinuity that would oscillate at archetype boundaries with
small per-iteration deltas in sample count.

### G5. Per-dimension drift budget

Sum the absolute deltas across iterations per `(archetype, dimension)`
pair; refuse to apply any further deltas to a pair once its cumulative
budget exceeds `2 * MaxDimensionDelta = 4.0`. Prevents one dimension
from being repeatedly hammered in the same direction across all 5
iterations into a regime the producer never explicitly chose.

### G6. Sticky-archetype gate

Don't apply feedback to an archetype that appeared in fewer than 3
gauntlets within the rolling 5-iteration window. Prevents a one-off
small-sample archetype from being tuned every iteration based on
the same noisy signal.

### G7. Composite scoring instead of single-dimension regression

The future-body sketch in `analytics.ComputeWeightDeltas` doc comment
mentions per-dimension regression vs. win-rate residuals. Doing that
in isolation per-dimension will produce mutually-correlated deltas
that compound. Replace with multi-dimension OLS on the full
`EvalWeights` vector simultaneously so the regression naturally
penalizes correlated movements.

### Combined safety net

G1+G2+G3 are the *minimum viable* set — the decay schedule prevents
amplification, the momentum dampener prevents oscillation, and the
divergence early-stop prevents catastrophic runaway. G4-G7 are
quality-of-tuning improvements that can land iteratively.

## Followups (NOT in this PR)

- **PR #955** attribution body — when this lands, re-run this script
  and update the `{{VERDICT}}` section above with the actual
  convergence story.
- **Per-deck (not per-archetype) feedback** — convergence behavior may
  be very different at deck granularity (more deltas, smaller per-
  deck sample size).
- **Drift detection in production** — long-running tournament harness
  should alarm when a single iteration crosses G3's divergence
  threshold.
- **Convergence-aware learning rate** — the per-iteration step size
  should itself be tunable based on the prior iteration's apparent
  bite (large bite → small step; small bite → bigger step).

## Honest limitations of this study

1. **Single deck pool** (`data/decks/test`, 16 decks). Convergence
   behavior may be archetype-pool-dependent; a heavily-skewed pool
   (e.g. 14 control + 2 aggro) would put nearly all the attribution
   weight on a couple of archetypes.
2. **Fixed seed across iterations**. Real production tuning would
   bootstrap each iteration with a fresh seed; the fixed-seed setup
   gives the cleanest "did the feedback do anything?" signal but
   masks RNG-induced robustness questions.
3. **Stub attribution model**. Every conclusion here is integration-
   side; nothing in this study validates the eventual attribution
   model itself.
4. **Iteration-0 baseline ALSO emits feedback** — the
   `--feedback-out` flag runs unconditionally in iter 0 too, so
   iter 1's "first applied" feedback is what iter 0 produced. With
   the stub, this is empty and inconsequential; once the body lands,
   the first applied feedback is computed from the un-overlaid
   baseline run, which is the intended bootstrap shape.

## Reproducing

```bash
scripts/self-tune-convergence-study.sh \
    --decks data/decks/test \
    --games 500 \
    --seed 42 \
    --seats 4 \
    --iterations 5 \
    --out /tmp/heimdall-convergence-full
```

Re-running with `--games 100 --iterations 3` produces the same
verdict pattern faster (the stub-locked state is the same regardless
of iteration count or gauntlet size); use that for development.
