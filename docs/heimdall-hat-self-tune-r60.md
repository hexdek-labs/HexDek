# Heimdall → hat self-tune loop — closed loop (R60)

## Why this exists

Through r60 the system had a one-way arrow: Heimdall observed games
(`analytics.GameAnalysis`) and reported to humans; hat's per-archetype
`EvalWeights` (`internal/hat/eval_weights.go`) were hand-tuned by engineers
reading those reports. Every retune required: read the report, eyeball
the weights, edit the Go file, recompile, re-run a gauntlet, eyeball
again. A reasonable workflow but slow, biased toward the engineer's
intuitions, and impossible to run unattended.

This document covers the **closed-loop wiring** — the data flow that
lets a gauntlet's analytics produce the next gauntlet's weights with no
human in the middle.

## What ships in this PR vs. predecessors vs. followups

| PR | Status | Scope |
|---|---|---|
| #947 | merged | foundation: `HeimdallFeedback` struct, `SetActiveFeedback`, apply overlay, bounds, safety clamps |
| #949 | rebasing | applier polish (per-archetype downgrade, etc.) |
| #955 | rebasing | actual attribution model (replaces `ComputeWeightDeltas` stub) |
| this PR (#TBD) | foundational | **end-to-end wiring** — tournament CLI consumes + emits feedback; `scripts/self-tune-loop.sh` orchestrates a single iteration; this doc |
| future | not yet | convergence study (multi-iteration loop, stability analysis, drift detection) |

The wiring ships even though the attribution model is still stubbed.
`ComputeWeightDeltas(nil, _)` returns an empty `HeimdallFeedback`; the
apply path treats empty feedback as no-op; the next gauntlet runs with
baseline weights. The loop is then proven to work end-to-end (no panic,
no schema mismatch, no orphan files), and when #955 lands the real
attribution body slots in without renegotiating any contract.

## Architecture

```
   ┌─────────────────────────────────────────────────────────────────┐
   │                          Gauntlet A                              │
   │  --decks <path>  --games 500  --feedback-out <dir>               │
   │                                                                  │
   │  ┌────────┐    ┌────────────┐    ┌────────────────────────────┐  │
   │  │ engine │ →  │ Analytics  │ →  │ analytics.ComputeWeight    │  │
   │  │ games  │    │ per game   │    │ Deltas(analyses, prov)     │  │
   │  └────────┘    └────────────┘    └────────────────────────────┘  │
   │                                                ↓                 │
   │                            ┌──────────────────────────────────┐  │
   │                            │ analytics.SaveHeimdallFeedback   │  │
   │                            │ → <dir>/heimdall_feedback.json   │  │
   │                            └──────────────────────────────────┘  │
   └─────────────────────────────────────────────────────────────────┘
                                                ↓
   ┌─────────────────────────────────────────────────────────────────┐
   │                          Gauntlet B                              │
   │  --apply-feedback <dir>  --result-json result_b.json             │
   │                                                                  │
   │  ┌──────────────────────────────────┐                            │
   │  │ analytics.LoadHeimdallFeedback   │                            │
   │  └──────────────────────────────────┘                            │
   │                ↓                                                 │
   │  ┌──────────────────────────────────┐                            │
   │  │ hat.SetActiveFeedback(fb, pol)   │                            │
   │  │ → fills activeByArchetype map    │                            │
   │  └──────────────────────────────────┘                            │
   │                ↓                                                 │
   │  ┌──────────────────────────────────┐    ┌──────────────────┐    │
   │  │ hat.NewEvaluator(strategy)       │ →  │ Game outcomes    │    │
   │  │ → DefaultWeightsForArchetype     │    │ shift vs A       │    │
   │  │   applies overlay                │    │                  │    │
   │  └──────────────────────────────────┘    └──────────────────┘    │
   └─────────────────────────────────────────────────────────────────┘
```

## Data flow contract

### Egress (gauntlet A)

`hexdek-tournament` after the run completes:

1. If `--feedback-out <dir>` is set, call
   `analytics.ComputeWeightDeltas(result.Analyses, provenance)`.
   `provenance` is set to `"tournament:<report-basename>:games=<N>"` so
   downstream consumers can audit where the feedback came from.
2. Call `analytics.SaveHeimdallFeedback(dir, fb)` — atomic write via
   tmp+rename, mkdir-s the dir if needed.

The egress runs unconditionally (no minimum-games gate) so a 10-game
smoke test can still flow end-to-end through the pipeline. The
applier-side `MinSampleSize=50` per-archetype gate handles the
"too-small to act on" case downstream.

### Ingress (gauntlet B)

`hexdek-tournament` before the gauntlet starts:

1. If `--apply-feedback <path>` is set:
   - Accept either a directory or a direct JSON path
     (`filepath.Dir(path)` if the latter); resolves to a dir.
   - `analytics.LoadHeimdallFeedback(dir)` returns `(nil, nil)` for
     "no file exists" — the CLI logs and falls through to baseline
     weights instead of erroring (so a first-time gauntlet can use
     the flag idempotently).
   - Schema-version mismatch IS an error and aborts the run; additive
     fields don't require a bump.
   - On success, `hat.SetActiveFeedback(fb, policy)` installs the
     overlay. Default policy is `direct`; `--feedback-policy` accepts
     `direct | confidence-weighted | advisory | skip`.
2. Per-archetype apply happens inside `DefaultWeightsForArchetype` on
   every evaluator construction — zero-cost lookup when no overlay is
   installed.

### Comparison (script-side)

`scripts/self-tune-loop.sh` runs two back-to-back gauntlets with the
same deck pool / seed / seat count and diffs:

- `result.games`, `result.draws`, `result.avg_turns` — coarse health
  metrics.
- `result.wins_by_commander` — per-deck winrate delta.
- Sum of `|Δ wins|` across all commanders as a single "total shift"
  scalar.

The script emits a **VERDICT** with three buckets:

1. **`feedback file empty (attribution still STUBBED)`** — the expected
   current state until PR #955 lands. Confirms the integration is
   wired; the apply path correctly no-ops empty feedback; gauntlet B is
   bit-identical to gauntlet A. **The loop works as an integration;
   it has no signal yet.**
2. **`feedback computed but produced no observable shift`** — the
   attribution model emitted deltas but they were too small to bite
   (clamped by `MaxDimensionDelta=2.0`, gated by `MinSampleSize=50`,
   or simply within MCTS noise). Suggests either gauntlet too small or
   noise floor too high.
3. **`gauntlet B differs from A — the closed loop has BITE`** — the
   end-to-end signal worked. Single iteration is sufficient evidence
   that the wiring is sound; convergence behavior is the next study.

## Why a single iteration is enough for this PR

Spec was explicit: **convergence study is a follow-up**. A single
iteration answers the load-bearing question — "does the data path
work end-to-end?" — and produces a verdict the next worker can act on.
Multi-iteration loops, drift detection, stability analysis, anti-
overfitting safeguards, and tuning-rate scheduling all build on this
foundation but require their own PRs because their failure modes are
fundamentally different (numerical, not integration).

## Operational notes

### Running it

```bash
scripts/self-tune-loop.sh \
    --decks data/decks/cage_match \
    --games 500 \
    --seed 42 \
    --out /tmp/heimdall-self-tune
```

Artifacts after a run:

```
/tmp/heimdall-self-tune/
  hexdek-tournament              # built binary
  heimdall_feedback.json         # gauntlet A's output
  result_a.json                  # gauntlet A's TournamentResult
  report_a.md                    # gauntlet A's markdown report
  result_b.json                  # gauntlet B's TournamentResult
  report_b.md                    # gauntlet B's markdown report
```

### Adversarial input

A hand-edited `heimdall_feedback.json` cannot:

- Push a weight negative (would invert MCTS sign) — floored at 0 by
  `applyFeedbackOverlay` (`internal/hat/heimdall_feedback_apply.go`).
- Push a weight unbounded — clamped to `±MaxDimensionDelta` on load
  AND on apply.
- Cross-archetype-pollute — the apply path is keyed on the exact
  archetype string; an aggro delta does not bleed into control.
- Bypass the sample-size gate — per-archetype `SampleSize <
  MinSampleSize` is silently skipped (logged, not applied).

### Concurrency

`hat.SetActiveFeedback` mirrors `LegacyMidrangeOnly`: set once at
gauntlet startup, before the first `NewEvaluator`. Mid-gauntlet flips
are safe in the data-race sense (RWMutex-protected) but produce
inconsistent per-deck weights across games in the same run; don't do
that.

### Reverting

Run a gauntlet with `--feedback-policy=skip` to ingest the feedback
metadata for logging/reporting but skip the actual weight overlay.
Useful for A/B-comparing "feedback on" vs "feedback off" gauntlets
without rebuilding the binary.

## Followups (NOT in this PR)

- **PR #955 attribution body** — replace the
  `analytics.ComputeWeightDeltas` stub with real per-archetype
  regression: per-dimension contribution vs. win-rate residuals,
  significance filter (p < 0.05), noise floor on `abs(delta)`.
- **Convergence study** — iterate the loop N times, plot the delta
  vector's L2 norm per iteration, identify whether it converges,
  oscillates, or diverges; tune the per-iteration learning rate.
- **Per-deck (not per-archetype) feedback** — current schema is
  per-archetype because dev-4's attribution model targets archetype
  primitives; later iterations may move to per-deck or per-(deck,
  matchup) granularity.
- **Drift detection** — long-running production gauntlets should
  alarm when a single iteration's deltas exceed a threshold (suggests
  the underlying meta shifted, not just a per-game noise update).
- **Result-JSON schema** — `--result-json` currently emits the raw
  `TournamentResult`; the script-side diff should eventually be
  generalized into a `hexdek-heimdall --diff a.json b.json`
  subcommand instead of inline Python.
