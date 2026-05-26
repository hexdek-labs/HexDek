# Precon Vibes-Bracket Calibration — R2 Findings Summary (R60)

## Purpose

Concise summary of what the two-wave precon calibration study (PRs #508, #513, #523) has established about Freya's bracket-estimator behavior on unedited WotC product. Written as a decision-support doc — separating "the algorithm has a bug" from "this specific deck looks weird" — so 7174n1c can pick the next move (engine fix vs more data vs a different scope) without re-reading three docs.

The actual ranked tables and per-deck metrics live in:
- `docs/precon-vibes-baseline-r60.md` (R1, 15 decks)
- `docs/precon-vibes-baseline-r2-r60.md` (R2, 15 different decks)
- `docs/precon-b4-tracing-r60.md` (R1 root-cause trace for the two B4 false-positives)

## What the R2 wave was for

R1 (PR #508) shipped a 15-deck unedited-precon corpus and reported 2/15 B4 false-positives. That number could mean either:
- **(a) The bracket estimator has a systemic predicate problem** — every WotC precon that looks a particular way will mis-call, OR
- **(b) The R1 sample drew two outlier decks** — the next 15 precons would all land correctly.

R2 (PR #523) ran the same exercise on a completely disjoint 15 precons to disambiguate. Result: **(a)**. A third precon trips the same predicate via the same execution path. The reading is settled.

## What's now established

### 1. The B4 false-positive root cause is real, reproducible, and traced to a single predicate

**3 of 30 stock precons** mis-call as B4 ("Optimized"). All three hit the same code:
- Urza's Iron Alliance (BRO) — R1
- Blast from the Past (WHO) — R1
- Family Matters (BLB) — R2

All three lifted to B4 by `Tuned-redundancy floor` at `cmd/hexdek-freya/archetype.go:1229`:
```go
tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6
if tunedRedundancy && bracket < 4 {
    bracket = 4
    label = "Optimized"
}
```

Two of the three (Blast, Family Matters) ALSO exhibit the secondary ordering bug PR #513 documented: the `GC=0 ceiling` had explicitly demoted them to B2 with reason *"no Game Changers and no true-infinite combo"*, and the floor ran unconditionally after and silently undid the ceiling. The rationale text on those decks literally contains two adjacent contradictory verdicts and the floor wins.

The predicate is over-broad because WotC ships precons that naturally satisfy both conjuncts:
- `finishers ≥ 8` — Freya's finisher classifier counts each mass-pump anthem as a distinct finisher line AND separately counts each anthem-pairing-with-token-maker as another line. 4 anthems × 2 pairings = 8 lines from 1 conceptual finisher pattern.
- `fastMana ≥ 6` — the standard precon ramp suite (Sol Ring + 5 signets/talismans + Mind Stone) trips this threshold by design.

PR #513's recommended fix tightens the predicate to require an additional independent "this is actually B4" signal AND respects a ceiling-applied "no-GC + no-real-combo" reason. Both fixes were validated against all three known false-positives in PR #523's writeup — predicate alone would also demote Family Matters.

**Status:** root cause traced, fix designed, awaiting 7174n1c's go-ahead to implement.

### 2. The `plays_like` simulator under-rates Core precons as Exhibition (NEW HYPOTHESIS — not yet root-caused)

Both waves show ~55-60% disagreement between `mechanical_bracket` and `plays_like` (R1: 9/15 = 60%, R2: 8/15 = 53%, combined: 17/30 = 57%). The DIRECTION of disagreement is consistent: nearly every mismatch is `plays_like=Exhibition vs bracket=Core`. The simulator is biased against precons whose engine is "stuff happens slowly" rather than a clean win condition.

This is more speculative than finding #1 — it's a pattern in the disagreement stats, not a traced predicate. But the direction is too consistent to be noise across 30 decks. Likely fix surface: `estimatePlaysLike()` at `cmd/hexdek-freya/archetype.go:1246`. Not yet investigated at the depth PR #513 took for `estimateBracket()`.

**Status:** hypothesis from the data; not traced.

### 3. The cycling-loop combo detector has a separate bug — confirmed on two decks

R1 §1B (PR #513) flagged Blast from the Past's "4 combo lines" as false positives — Scattered Groves + Irrigated Farmland + Jo Grant treated as a renewable loop when cycling consumes the land permanently. R2 found a far more extreme case in the same family: **Timeless Wisdom (C20 Gavi cycling commander) reports 27,879 win_lines** (27,323 "determined" + 550 "infinite"). The Gavi deck has ~25 cycling cards; the false-positive detector explodes combinatorially across pair / triple / quad combinations, plus a separate stream of "X produces card for Y, Y produces card back | NO OUTLET" lines that the classifier itself acknowledges have no outlet but counts anyway.

This does NOT affect the bracket call directly (the score table caps at "+3 for 5+ combo lines") but:
- the headline win_line counts are meaningless on cycling-heavy decks
- the underlying detector treats consume-once triggers (cycling, kicker, suspend, encore) as renewable producers

Fix surface: the loop detector's "renewable producer" check needs to exclude one-shot trigger keywords.

**Status:** bug confirmed across multiple decks, fix surface identified, not yet investigated.

## What's now ruled out

- **"R1's 2/15 was a sampling fluke."** Ruled out — finding 1 reproduced with the same execution path on a disjoint deck.
- **"The disagreement between mech and plays_like is random noise."** Ruled out — direction is consistent across both waves.
- **"Cycling-deck combo detection might just be a Doctor Who quirk."** Ruled out — Gavi shows the same shape, far more extreme.

## What's still open

- **Whether to implement PR #513's recommended fix as-designed, modified, or not at all.** Decision belongs to 7174n1c. The fix is small (≤30 lines) and the regression-test fixture is sketched in PR #513 §4.
- **Whether to trace `estimatePlaysLike()` for finding #2.** Reasonable next step but lower priority — that path is informational/user-facing and doesn't gate engine behavior the way `estimateBracket()` does.
- **Whether to fix the cycling-loop detector now or batch with other combo-detector cleanup.** The 27,879 win-line count is cosmetic for bracket purposes but ugly for any downstream UI consumer.

## Cost-benefit on a hypothetical R3 (NOT a recommendation)

The Round-1-vs-Round-2 cross-validation rates landed within statistical noise (B4 false-positive 13% → 7%; within-±1 87% → 93%; plays-like-disagree 60% → 53%) across 30 decks. A third wave would refine the point estimates but is very unlikely to expose a fourth finding category — the residual signal in the data has been mostly extracted. The remaining unknowns (will the predicate fix actually eliminate ALL precon B4 false-positives in production? does fix-design B cover edge cases the corpus doesn't show?) are answered by **shipping the fix and gating it on the existing 30-deck corpus**, not by collecting another 15 decks. 7174n1c is the right call on whether more data, the fix, or both is the right move.

## Reproducing any of this

```bash
# Single deck trace:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 15 'Bracket rationale'

# Full 30-deck rerun (no flags needed — Freya auto-runs on import; just re-import any deck):
go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards

# Aggregate metrics across all 30 stock precons — see the R1/R2 baseline docs
# for the JSON extraction script.
```
