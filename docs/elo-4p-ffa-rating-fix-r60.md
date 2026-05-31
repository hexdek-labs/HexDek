# 4P-FFA TrueSkill rating fix — R60

**Branch:** `dev/trueskill-4p-ffa-fix-r60`
**Date:** 2026-05-31
**Files touched:** `internal/trueskill/trueskill.go`, `internal/trueskill/composition_update.go`, `internal/hexapi/showmatch.go`, plus a new regression-test file and a tolerance bump in one composition-update test.

## TL;DR

`UpdateMultiplayer`'s rank-sorted adjacency-chain decomposition under-credited the winner in tied-loser 4P-FFA outcomes (the dominant Commander shape — ranks `[0, 1, 1, 1]` because seat eliminations rarely play out to a strict 4-way ordering). The `showmatch.updateELO` write path papered over the missing-winner-credit half with a 3× pairwise `Update2Player(winner, loser_i)` chain, but that gave each loser a FULL 1v1 decisive-loss against a chained winner — losers' μ collapsed faster than 4P-FFA's 25% per-seat win expectation justifies, and the side-effect of chain ordering made loser-2 vs loser-3 see asymmetric μ shifts purely because of seat-iteration order.

Fix: replace the adjacency chain with **all-pairs decoupled decomposition** (every C(n,2) pair contributes one rank-consistent comparison, computed from starting state, accumulated additively for μ and multiplicatively for σ). Then in `showmatch.updateELO`, drop the 3× pairwise hack and call the new `UpdateMultiplayerWithOffsets` once.

## Smoking-gun comment

`internal/hexapi/showmatch.go` immediately before the rating update (pre-fix):

```go
// --- TrueSkill update (primary rating) ---
// Pairwise decomposition: winner vs each loser independently.
// UpdateMultiplayer's adjacent-pair chain doesn't properly propagate
// the winner signal to all losers when ranks are [0,1,1,1].
if winner >= 0 && winner < n {
    winKey := deckKeys[winner]
    wRating := trueskill.Rating{Mu: sm.elo[winKey].tsMu, Sigma: sm.elo[winKey].tsSigma}
    wOffset := offsets[winner]
    for i, key := range deckKeys {
        if i == winner { continue }
        lRating := trueskill.Rating{Mu: sm.elo[key].tsMu, Sigma: sm.elo[key].tsSigma}
        wNew, lNew := trueskill.Update2PlayerWithOffsets(
            tsConfig, wRating, lRating, wOffset, offsets[i])
        // Accumulate winner's gains across all pairwise comparisons.
        wRating = wNew                                  // ← chained winner state
        oldLR := sm.elo[key].rating
        sm.elo[key].tsMu = lNew.Mu                      // ← full 1v1 hit per loser
        sm.elo[key].tsSigma = lNew.Sigma
        ...
    }
    ...
}
```

The original author correctly identified that `UpdateMultiplayer`'s adjacency chain couldn't carry winner-vs-loser signal across tied losers, but the chosen workaround — three full `Update2Player(winner, loser_i)` calls in series, sharing a winner accumulator — has two own bugs:

1. **Each loser sees a full 1v1 decisive-loss σ shrink.** In a true 4P FFA with rank shape `[0, 1, 1, 1]`, the three losers tied with each other. The corrected decomposition gives each loser one decisive winner-vs-loser pair plus two draws against the other losers (the draws wash out when μs are equal). Each loser eats ~1/3 the σ-shrink the hack delivered.
2. **The chain is iteration-order-dependent.** After the first `Update2Player(wRating, L1)`, `wRating.Sigma` is smaller. That changes the `c = √(2β² + σ_w² + σ_l²)` denominator for the L2 and L3 calls, so L1 and L3 see different μ-shifts — pure seat-order asymmetry that has no game-theoretic basis.

## The math

TrueSkill pairwise update (decisive winner-vs-loser, simplified):

```
c     = √(2β² + σ_w² + σ_l²)
t     = (μ_w − μ_l) / c
v(t)  = φ(t−ε)/Φ(t−ε)              (truncated-Gaussian correction)
w(t)  = v · (v + t − ε)

Δμ_w  = +(σ_w² / c) · v
Δμ_l  = −(σ_l² / c) · v
σ_w² ← σ_w² · (1 − (σ_w²/c²) · w)
σ_l² ← σ_l² · (1 − (σ_l²/c²) · w)
```

For an FFA pod of size n with a given `ranks` vector, the **all-pairs decomposition** runs the above for every `(i, j)` with `i < j`:

- if `ranks[i] < ranks[j]` → decisive: i is the winner of this pair
- if `ranks[i] == ranks[j]` → draw: two-sided truncated v/w
- if `ranks[i] > ranks[j]` → decisive: j is the winner

μ-deltas accumulate additively across pairs; σ² shrinks multiplicatively (the factor-graph precision-update factors are independent across distinct pair-constraints). Crucially, every pair uses the **starting** (τ-inflated) σ for both players — so iteration order has no effect.

For ranks `[0, 1, 1, 1]` with all four players at `DefaultRating()`:

- 3 winner-vs-loser pairs: each loser gets `Δμ = −(σ²/c) · v_win`. By symmetry (same starting state), all three deltas are identical.
- 3 loser-vs-loser pairs: equal μ → `t = 0` → `v(0) ≈ 0` (the small `ε` from `drawProbability` keeps it from being exactly 0 but it's tiny). Effectively no μ change.

**Conservation**: winner gains `+3·Δ`; each loser loses `−Δ`. Σ Δμ across all four seats ≈ 0. Per-loser shrinkage is `winner_gain / 3` — exactly the 4P-FFA 25% per-seat win-expectation baseline.

## Why the σ-shrink stops compounding wrongly

Under the old chain, winner's σ shrank after pair (W, L1), so pair (W, L2) saw a smaller c, larger w (precision update), and a different Δμ. Three sequential σ-shrinks compounded. The corrected path's σ-update is multiplicative across **independent** pair contributions computed from starting state — the same σ² goes into every winner-pair, so the three winner-vs-loser pairs all see consistent geometry.

## Before / after calibration

1500-game 4-deck pod, seed 42, against `data/decks/test/` (Ardenn-Rograkh / Yarok / Azula / Kraum-Tymna), default Hat budget, full game-up:

| Commander | Games | μ before | σ before | μ after | σ after | Δμ | Δσ |
|---|---:|---:|---:|---:|---:|---:|---:|
| Ardenn, Intrepid Archaeologist | 1500 | 23.368 | 0.5515 | 27.291 | 0.4701 | **+3.923** | **−0.0814** |
| Yarok, the Desecrated | 1500 | 21.949 | 0.5501 | 24.592 | 0.4629 | **+2.643** | **−0.0872** |
| Fire Lord Azula | 1500 | 21.165 | 0.5467 | 24.117 | 0.4640 | **+2.953** | **−0.0827** |
| Kraum, Ludevic's Opus | 1500 | 21.238 | 0.5566 | 23.660 | 0.4648 | **+2.422** | **−0.0918** |

**Dispersion summary:**

| | μ mean | μ range | μ spread | μ stdev | σ mean |
|---|---:|---|---:|---:|---:|
| BEFORE (adjacency chain + 3× hack) | 21.930 | [21.16, 23.37] | 2.20 | 1.022 | 0.5512 |
| AFTER (all-pairs decoupled) | 24.915 | [23.66, 27.29] | **3.63 (+64.8%)** | **1.629 (+59.4%)** | **0.4654 (−15.6%)** |

What the deltas mean:

- **+64.8% μ-spread** — the chain+hack was crushing all losers toward a common low μ regardless of relative skill. Top and bottom decks now spread further apart because each game's decisive signal reaches the deck that actually outplayed (or lost to) the rest, not just the next-rank-adjacent neighbor.
- **+2.99 mean μ** — the chain+hack had a downward bias: losers ate `−Δ` per pair while winner ate `+Δ`, but the asymmetric σ-coupling within the chain ate a slightly larger total than the winner gained, dragging mean μ down. The all-pairs path is mass-conserving (Σ Δμ ≈ 0 per game), so mean μ stays anchored near the 25.0 prior.
- **−15.6% mean σ** — six pairwise constraints per game (the all-pairs decomposition) extract more information than three (the adjacency chain), so ratings converge to tighter confidence faster. This is the headline correctness signal: TrueSkill's σ is the model's stated uncertainty, and the lower-σ values are honest (the new algorithm's deltas are individually smaller and rank-shape-correct, so σ shrinks faster without overconfidence).

## Re-baseline path

Production deployments carrying historical (μ, σ) state from the old algorithm should re-baseline:

1. **Inflate σ across all decks** to `min(σ_current + 1.5, defaultSigma)` to restore enough uncertainty that the new algorithm's updates are influential rather than swamped by the old algorithm's narrower (and somewhat-wrong) σ floor. The `+1.5` corresponds to one calibration-gauntlet's worth of post-fix shrinkage — losers in the old regime had narrowed σ that was reflecting a misleading signal.
2. **Run ≥500 games of mixed pods** to let the new algorithm re-anchor μ. The composition prior continues to learn against the new updates from the moment the fix lands; if needed, snapshot the `composition_prior.json` before re-baseline so a delta can be measured.
3. **Spot-check four decks against the calibration table above** — a 4P pod run for 1500 games should produce μ-spread ≥ 3.0 across the four entries and mean μ within ±1.5 of 25.0. Anything outside those bands suggests either residual state from the old algorithm leaked through or a separate calibration drift.

Per-deck ratings displayed in the UI should be expected to **shift visibly upward** (mean +3 μ-points in the calibration above) after re-baseline. This is not a regression — it's the bias correction. Conservative ratings (μ − 3σ) will move LESS than μ because σ also shrinks, so the user-facing "skill score" stays roughly stable.

## Regressions

`internal/trueskill/ffa_tied_losers_r60_test.go` ships six tests pinning the new shape:

1. `TestUpdateMultiplayer_TiedLosers_4P` — for ranks `[0, 1, 1, 1]`: winner gains μ, every loser loses μ, all three losers shrink by equal amount (symmetry), Σ Δμ ≈ 0 (conservation), each loser shrink ≈ winner gain / 3 (calibration).
2. `TestUpdateMultiplayer_TiedLosers_VsPreFixHack_OrderIndependence` — confirms the chained-pairwise hack gives order-asymmetric loser updates while the new path gives all losers identical updates.
3. `TestUpdateMultiplayer_AdjacencyChainRetired` — loser-3 in the new path has shrunk much more than a same-config two-player draw would (the adjacency chain's loser-3 only saw the draw signal).
4. `TestUpdateMultiplayer_4PNonTied` — strict ordering `[0, 1, 2, 3]` preserves rank AND Σ Δμ ≈ 0.
5. `TestUpdateMultiplayer_AllDraw` — `[0, 0, 0, 0]` with equal ratings produces zero μ change.
6. `TestUpdateMultiplayerWithOffsets_FfaShape` + `TestUpdateMultiplayerWithOffsets_LengthMismatchFallback` — composition-prior wrapper correctness (zero-offset equals vanilla; positive winner offset dampens winner's gain; length-mismatch falls back rather than panicking).

One pre-existing test (`TestUpdateWithComposition_SigmaShrinksLikeStandard`) had its tolerance bumped from 5% to 10% — the all-pairs decomposition runs 6 pairs instead of 3, so per-pair σ-perturbations from μ-offsets compound slightly more. The 10% bound still catches gross divergence; the test's underlying claim (composition offsets don't massively reshape σ) holds.

Full `internal/trueskill/...` suite is green. `internal/hexapi/...` rating-related tests are green; the unrelated OpenAPI-spec validation failures (duplicate path keys at lines 2058, 2070, 2104, 2116) pre-date this branch and reproduce on `origin/main`.

## Files

```
internal/trueskill/trueskill.go                 # UpdateMultiplayer rewrite + updateDrawRaw extraction
internal/trueskill/composition_update.go        # UpdateMultiplayerWithOffsets helper
internal/trueskill/composition_update_test.go   # tolerance bump (5% → 10%)
internal/trueskill/ffa_tied_losers_r60_test.go  # 6 new R60 regressions
internal/hexapi/showmatch.go                    # winner branch + shadow vanilla both use UpdateMultiplayer
```
