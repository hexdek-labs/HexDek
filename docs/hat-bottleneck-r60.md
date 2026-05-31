# Hat Bottleneck Investigation — Why cEDH Games Were Slow (r60)

> **Investigates why the four-PR cEDH combo-assembly chain (#793 → #826 → #848) produced a 2500-game honest-null result (PR #863).** A fresh-eyes read of `internal/hat/` and the gauntlet workflow finds three bottlenecks that all stack on each other, ranked by impact. The largest one is a strategy-loader path bug that silently disabled the entire architecture for every prior gauntlet — the other two PRs' code never even fired. This PR ships the surgical fix for the load bug; the other two bottlenecks are documented for the next iteration.

**Verdict: a single 6-line loader fix unlocks the entire investigation chain. With the fix shipped here, avg cEDH game length drops from 48.4 → 43.5 turns (−4.9 turns, 10% compression) at 400-game validation.**

## TL;DR

1. **Bottleneck #1 (root cause): `LoadStrategyFromFreya` could not find the strategy.json files.** The gauntlet's deck pool was staged at `<root>/batch_a/*.txt` and `<root>/batch_b/*.txt`, but the Freya analysis (`<deck>.strategy.json`) was at `<root>/freya/`. The pre-fix loader only checked `<deckdir>/freya/<deck>.strategy.json` (i.e. `<root>/batch_a/freya/...` — doesn't exist). It returned nil. The tournament logged `WARNING: deck X has no Freya analysis — Hat will play without strategy intelligence` for every deck and every game. `Strategy.ComboPieces` was empty, `comboPieceSet` was empty, `valueEngineSet` was empty. Every architectural change in PRs #793 + #826 + #848 depended on these being populated. None fired. **The shipped fix: a parent-dir fallback in `LoadStrategyFromFreya`. 6 LoC + 4 tests.**

2. **Bottleneck #2: MCTS budget=50 is in TierGungnir (evaluator-guided UCB1), not TierRagnarok (rollout).** Rollouts — actual multi-turn forward simulation — fire only at `Budget >= 200` (`rolloutBudgetGe` constant at `internal/hat/rollout.go:12`). PR #848's lift to 75/100 stayed below the rollout threshold. At budget 50, the hat does single-state evaluation; it cannot simulate "cast Demonic Tutor → fetch Thassa's Oracle → cast Oracle → cast Demonic Consultation → win" because that requires forward simulation. The gauntlet ran without rollouts for any of the four PRs.

3. **Bottleneck #3: Even at budget≥200, `rollout.go resolveStack` is a state approximation, not an effect simulator.** Permanent spells go straight to battlefield (no ETB triggers fire); instants/sorceries drop to graveyard (no targeting, no modes, no effect resolution). So even with rollouts enabled and a fixed loader, the simulator at lines 133–165 wouldn't see Demonic Tutor actually fetching a piece, wouldn't see Thassa's Oracle's win-the-game ETB, wouldn't see Demonic Consultation exiling the library. The rollout is a board-shape approximation suited for "how does the position look in 4 turns under heuristic play" — it is not a combo-line simulator. This is the actual ceiling on combo-assembly quality in HexDek's current AI.

## Bottleneck #1 in detail — the root cause

### What broke

`internal/hat/strategy_loader.go LoadStrategyFromFreya` searches at:

```
<dir(deckPath)>/freya/<base>.strategy.json
<dir(deckPath)>/freya/<base>_freya.json   (fallback format)
```

The cEDH gauntlet stages decks at:

```
/tmp/cedh-seat-bias/
  freya/<deck>.strategy.json    ← analysis here
  freya/<deck>_freya.json
  batch_a/<deck>.txt            ← decks here
  batch_b/<deck>.txt
```

When the tournament calls `LoadStrategyFromFreya("/tmp/cedh-seat-bias/batch_a/francisco_b5.txt")`, it looks at `/tmp/cedh-seat-bias/batch_a/freya/francisco_b5.strategy.json` — which doesn't exist. The function returns nil. The hat factory at `cmd/hexdek-tournament/main.go:277` then constructs `NewYggdrasilHatWithNoise(nil, ...)` and the tournament prints `WARNING: deck X has no Freya analysis`.

### Why this silently disabled the entire investigation chain

Every PR in the investigation depended on `Strategy.ComboPieces` being populated:

| PR | Change | Required field | Status with profile=nil |
|----|--------|----------------|-------------------------|
| #793 | scoreCombo multi-tutor leaf credit | `e.Strategy.ComboPieces` | Falls through to `scoreComboHardcoded` (legacy `knownCombos` list — not deck-specific) |
| #826 | cardHeuristic combo-priority cast-order bias | `h.planState.Current ∈ {Assemble, Execute}` AND `h.comboPieceSet[name]` | planState never flips (no ComboPieces → comboSeq is nil → Evaluate returns zero ComboAssessment); comboPieceSet is empty. Bias never fires. |
| #826 | Broader ComboAssessment.Assembling gate | `comboSeq.Lines` non-empty | comboSeq IS nil. |
| #848 | refreshPlanState mid-turn hook | `h.comboSeq` non-nil | refreshPlanState early-returns at the nil-comboSeq guard. |
| #848 | effectiveBudget Assemble/Execute lift | `h.planState.Current` flipped to Assemble/Execute | Never flips. |

**All four architectural surfaces no-op when ComboPieces is empty.** The investigation was tuning code paths that never executed for the very decks it was supposed to help.

The tournament was logging this — line `WARNING: deck X has no Freya analysis` printed 8× per gauntlet — but the warning was buried in a 2000-line log and the gauntlet still ran, producing apparently meaningful per-deck winrates that nobody recognized as the "no strategy" fallback path. PR #784's baseline was *also* on the no-strategy path; every subsequent PR's "vs baseline" comparison was apples-to-apples but at the wrong fidelity tier.

### The fix

Six lines in `LoadStrategyFromFreya`: iterate over `[deckdir, dirof(deckdir)]` as candidate roots before falling back to nil. 4 regression tests in `strategy_loader_parent_fallback_r60_test.go` pin:

- Deck-local strategy still loads (pre-existing behavior preserved).
- Parent-dir strategy now loads (new behavior).
- Deck-local wins over parent-dir when both exist (per-batch override).
- Nil result still returns when nothing is staged anywhere.

The deck-local-wins-when-both-exist contract matters because some gauntlet workflows ship per-batch tuned variants that should win over the shared root analysis.

### Impact validation (400-game gauntlet)

Same pool, seeds, pod compositions as PR #784. The only delta is the loader fix.

| Metric | PR #784 baseline (no strategy) | **This PR (strategy loads)** | Δ |
|---|---|---|---|
| Avg turns per game | 48.40 | **43.50** | **−4.90 (−10%)** |
| Francisco (Combo) winrate | 36.4% | 36.0% | −0.4 (flat) |
| Storm avg winrate | 5.55% | 6.50% | +0.95 |
| Stax avg winrate | 30.27% | 27.83% | **−2.43** |
| Midrange avg winrate | 30.80% | 33.75% | **+2.95** |
| Throughput | 74–91 g/s | 8.4 g/s | 9–11× slower (signal firing) |

**Avg game length compresses by 10% — by far the largest movement across the entire investigation chain.** Stax loses 2.43pp because the now-faster Storm/Combo decks reach win conditions before Stax assembles its prison. Midrange gains 2.95pp (Etali's MDFC back-face combo is now actually recognized as a combo line). Combo (Francisco specifically) is still flat — see Bottleneck #3 below for why even with strategy loaded the actual win execution depends on rollout/effect-resolution depth that isn't yet in place.

The single 6-line loader fix delivers a larger empirical effect than the combined three PRs of architectural refinement that preceded it.

### What this means for PRs #793, #826, #848

The architectural changes in those PRs are *correct* and *should* land — they just couldn't measure their own impact without a populated `Strategy.ComboPieces`. With this loader fix shipped, those PRs' contributions can finally be validated. PR #848's "null result" verdict at 2500 games (`docs/cedh-gauntlet-rerun-postplanstate-r60.md`) was correct as data but wrong as diagnosis — the chain wasn't starved, it was *unwired*.

The 5+ engineering days spent on PR #793 + #826 + #848 + #863 represent a real cost from a single missing fallback path in `LoadStrategyFromFreya`. The next iteration should re-run the gauntlet on this PR's fix to measure the *actual* contribution of each architectural change.

## Bottleneck #2 in detail — TierGungnir vs TierRagnarok

### What it is

The hat has three decision tiers (`yggdrasil.go:306-315`):

| Tier | Budget | What runs |
|------|--------|-----------|
| TierMjolnir | 0 | Heuristic only — no evaluator, no rollout |
| TierGungnir | 1–199 | Evaluator-guided scoring + UCB1 exploration |
| TierRagnarok | ≥200 | MCTS rollout (clone game state, forward-sim N turns, evaluate terminal) |

`rolloutBudgetGe = 200` (constant at `internal/hat/rollout.go:12`). The cEDH gauntlet's default `--hat-budget=50` is TierGungnir. PR #848's lift took budget to 75 (Assemble) / 100 (Execute) — still TierGungnir. The rollout path at `yggdrasil.go:4596-4665` requires `eb >= rolloutBudgetGe` (200). Never satisfied at the gauntlet's default budget.

### Why this matters for cEDH

Combo execution is fundamentally a multi-turn lookahead problem: "if I cast Demonic Tutor this turn, fetch Thassa's Oracle, and then cast Demonic Consultation next turn after holding mana, I win." Single-state evaluation at TierGungnir can score "Demonic Tutor in hand → tutor in graveyard" as marginally good (Demonic Tutor is a tutor card, +0.30 bonus), but it can't see "Thassa's Oracle on the battlefield with library = 0 → instant win." That terminal state requires forward simulation.

TierGungnir's mode of play is "score candidate spells by single-state evaluator, pick the best." For Combo decks that mode is structurally midrange-flavored — the eval can credit ComboProximity but can't actually see the wincon resolve. Hence the avg 47–50 turn game length even when the leaf eval is well-tuned.

### Why we don't ship a budget bump in this PR

Two reasons:

1. Bottleneck #1 dominates. Until strategy loads, no budget setting matters because the comboSeq is nil regardless. The validation at 400 games above is at budget=50 (TierGungnir) and STILL drops avg game length by 4.9 turns from baseline — because just having `ComboPieces` populated means cardHeuristic finally sees combo pieces as combo pieces.
2. The rollout simulator at Bottleneck #3 is too approximate to handle cEDH wincon resolution even at budget=200. Bumping budget without fixing the simulator would burn 4–5× the wall-clock per game without proportionally improving decision quality.

The right follow-up is a `--hat-budget 200` ablation specifically on Francisco *after* the loader fix lands, to measure how much TierRagnarok actually adds on top of TierGungnir with a populated strategy. That experiment is meaningful for the first time post-this-PR.

## Bottleneck #3 in detail — rollout simulator is approximate

### What `resolveStack` actually does

From `internal/hat/rollout.go:133-165`:

```go
func resolveStack(gs *gameengine.GameState) {
    for len(gs.Stack) > 0 {
        top := gs.Stack[len(gs.Stack)-1]
        gs.Stack = gs.Stack[:len(gs.Stack)-1]
        // ... skip nil / countered ...
        card := top.Card
        if isPermanentCard(card) {
            // build a Permanent and append to battlefield
        } else {
            // append to graveyard
        }
    }
}
```

**No effect resolution.** Demonic Tutor pops off the stack and is placed in graveyard — but the "search your library for a card, put it into your hand" effect never fires. No library is searched. No card moves to hand. Thassa's Oracle on a top-of-library cast goes to battlefield but the ETB ability doesn't fire — no win condition. Dramatic Reversal's "untap all nonland permanents" never happens.

### Why this is the actual cEDH ceiling

For a Storm or Combo deck, the value of "cast Demonic Tutor" in a forward simulation is entirely contained in the *effect that fires after the tutor resolves*. The simulator skipping effect resolution means the rollout sees:

- Turn N: Demonic Tutor enters graveyard. (No piece fetched.)
- Turn N+1: Cast Vampiric Tutor. Enters graveyard. (No piece fetched.)
- Turn N+2: Combo pieces still not in hand. No win.

Versus reality:

- Turn N: Demonic Tutor → search library, put Thassa's Oracle in hand. Cast Oracle. ETB: if library empty, you win. Library not empty yet — stays in play.
- Turn N+1: Cast Demonic Consultation. Effect: exile library until you find a card not in your deck. Library empties. Oracle ETB triggers (or re-evaluates state). You win.

The simulator can never see this story because the effects don't fire. Even a perfectly-tuned cardHeuristic + scoreCombo + planState + budget-200-rollout would not actually reach the wincon node in the search tree.

### What this means for the next iteration

Fixing this is a significantly larger engineering project than Bottleneck #1's 6-line loader fix. Two plausible directions:

1. **Lightweight effect simulator inside `resolveStack`.** Hand-code a small set of cEDH-critical effects (search/exile-library, win-the-game ETBs, draw-N, untap-nonland-perms) that the rollout simulator can actually execute against the cloned state. Scoped to the canonical ~20 cEDH wincon effects. Estimated 3–500 LoC + heavy testing.
2. **Outsource rollout to the engine's actual resolve path.** The engine package already resolves spells correctly — the rollout could call the engine's normal resolution loop instead of `resolveStack`'s approximation. Performance trade-off: real resolution is ~10× slower per turn than the approximation, but the rollout would actually see the wincon. Estimated 100 LoC at the rollout call site + a tournament flag to gate it (because real-resolution rollouts at budget=200 in 1250-game gauntlets would be untenable wall-clock).

Either is bounded engineering but neither is a 6-line fix. Both should wait until Bottleneck #1's loader fix has been re-measured at 2500 games on the corrected gauntlet so we know what the ceiling actually is at TierGungnir-with-strategy.

## Recommendation

- **Land this PR.** The loader fix is bounded (6 LoC + 4 tests + the report) and unlocks a 10% game-length compression empirically.
- **Re-run the cEDH 2500-game gauntlet** on this branch's fix to produce the *first valid* baseline + measure each of the four previous PRs' actual contribution.
- **Bottleneck #2** (budget 50 → 200 ablation): meaningful experiment once the loader is fixed. Until then, untestable.
- **Bottleneck #3** (rollout effect simulator): defer. Real engineering. Schedule only after Bottleneck #1's re-measurement clarifies what TierRagnarok actually needs to do to add value.

## Files

- `internal/hat/strategy_loader.go` — `LoadStrategyFromFreya` parent-dir fallback
- `internal/hat/strategy_loader_parent_fallback_r60_test.go` — 4 regression tests
- `docs/hat-bottleneck-r60.md` — this report
- `docs/hat-bottleneck-r60-data/batch_a_200g_with_strategy.md` — validation batch A (avg 43.2 turns, strategy loaded)
- `docs/hat-bottleneck-r60-data/batch_b_200g_with_strategy.md` — validation batch B (avg 43.8 turns, strategy loaded)

## See also

- [`docs/cedh-gauntlet-rerun-postplanstate-r60.md`](cedh-gauntlet-rerun-postplanstate-r60.md) — PR #863 honest-null verdict that triggered this investigation. Should be re-read with this PR's verdict attached: "null result was correct as data but wrong as diagnosis — chain wasn't starved, it was unwired."
- [PR #793](https://github.com/hexdek-labs/HexDek/pull/793), [#826](https://github.com/hexdek-labs/HexDek/pull/826), [#848](https://github.com/hexdek-labs/HexDek/pull/848) — the four-PR investigation chain whose architectural changes are correct but were untestable until this loader fix.
- `internal/hat/strategy_loader.go LoadStrategyFromFreya` — the function changed.
- `internal/hat/rollout.go` — Bottleneck #2 + #3 surface for the next iteration.
- `internal/hat/yggdrasil.go effectiveBudget` (~line 2110) — Bottleneck #2's TierGungnir/Ragnarok gate.
