# Precon Vibes-Bracket Calibration Baseline (R60)

## Why

Freya's `estimateMeasuredBracket` (renamed from `estimateBracket` as part of the bracket-vs-measured-bracket refactor; see CLAUDE.md) was tuned against the 16-deck `data/decks/test/` corpus (14/16 exact, 16/16 within ±1 per `bracket_calibration_test.go`), a corpus dominated by mid-to-high-power decks (lots of B3/B4, several cEDH-leaning B5). It has had no calibration pressure from the OTHER end of the distribution: **unedited WotC precons** — product whose explicit design intent is Bracket 2 (Core). Without a precon baseline we cannot tell whether the bracket-estimator's *floor* behaves correctly, only its ceiling.

**Naming convention (post-refactor):** the new `bracket` field on `DeckProfile` is the **declared / rubber-stamp** bracket (B2 by default for any deck under `data/decks/wizards/`, user-editable). The `measured_bracket` field is what Freya's signal estimator computes. The Δ column below tabulates `measured_bracket − bracket` so precons that play hotter than their B2 stamp surface as negative deltas.

This doc fixes that. It imports 15 unedited Moxfield uploads of WotC precons across 5 release eras (C13 → DSK 2024) and tabulates the mechanical-bracket call alongside the secondary metrics that should track "vibes" (commander_synergy, win_lines, combo_density, value_chain depth, recursion ratio). The point is to find the deltas — precons that mechanically score above or below WotC's stated B2 intent — so we know where the algorithm needs work.

## Methodology

- **Corpus:** 15 Moxfield community uploads of stock WotC precon decklists, in `data/decks/wizards/`, owner-tagged `wizards`. Sources are the "Commander Precons" Moxfield namespace (the canonical stock-list uploader). 3 precons per era × 5 eras.
- **Import:** `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards`. Freya auto-runs on import; per-deck output at `data/decks/wizards/freya/<slug>.strategy.json` and `<slug>_freya.md`.
- **Metrics pulled from strategy.json:**
  - `measured_bracket` ← Freya's `estimateMeasuredBracket` signal output (was `mechanical_bracket` in r60 pre-refactor)
  - `plays_like` ← `plays_like_label` (Freya's secondary "how it actually plays" call, separate from mechanical)
  - `commander_synergy_pct` ← `commander_synergy × 100`
  - `win_lines` ← `len(win_lines)` (Freya-detected win-line entries — note: high counts on tribal/token decks reflect "every X-tribal creature is a win-line piece", not multiplicity of distinct kill paths)
  - `combo_density` ← `len(combo_notes)` (entries from `KnownCombos` partial-match list; "have X, missing Y for determined" lines)
  - `value_chain_depth` ← max / avg `depth` across `value_chains[]` entries
  - `recursion_ratio` ← fraction of value-chains with `recursion_depth ∈ {infinite, deep}`
  - `game_changers`, `power_pct`, `mana_base_grade` for additional cross-reference
- **declared_bracket** (formerly `predicted_vibes_bracket`) is a per-row judgment call against WotC's stated B2 intent, informed by the metrics above. This column is the predecessor of the formal `bracket` field — the precon corpus is now stamped to B2 automatically by `isWizardsPrecon`, so post-refactor this column is always B2 for every row and the human-judged adjustments below are advisory only:
  - **B1 (Exhibition):** power_pct < 25 AND no win_lines AND no combos
  - **B2 (Core):** WotC default for unedited precon — no overriding signal
  - **B3 (Upgraded):** power_pct ≥ 60, OR combo_density ≥ 4, OR game_changers ≥ 2
  - **B4 (Optimized):** power_pct ≥ 80, OR game_changers ≥ 4
  - Rule of thumb chosen as the minimum-surprise prior; every precon ships with WotC's "this is a Bracket 2 product" expectation, and we only depart on visible mechanical evidence the bar has moved.

## Ranked Table

Sorted chronologically by era, then by release window.

| # | Era | Precon | Commander | Archetype | Measured Brkt | Plays-Like | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | **Declared Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---------:|:----------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:--------------:|:-:|
| 1 | C13 | Mind Seize | Jeleva, Nephalia's Scourge | midrange | 2 Core | Exhibition | 69.5 | 9 | 1 | 2.33 / 3 | 0.67 | 0 | 58 | B | **2** | ✓ |
| 2 | C16 | Breed Lethality | Atraxa, Praetors' Voice | tribal | 2 Core | Exhibition | 56.7 | 12 | 2 | 2.50 / 3 | 0.25 | 0 | 50 | A | **2** | ✓ |
| 3 | C18 | Subjective Reality | Aminatou, the Fateshifter | midrange | 2 Core | Core | 54.2 | 15 | 1 | 2.25 / 3 | 0.50 | 0 | 38 | F | **2** | ✓ |
| 4 | C19 | Mystic Intellect | Sevinne, the Chronoclasm | midrange | **1 Exhibition** | Exhibition | 74.6 | 5 | 3 | 2.50 / 3 | 0.25 | 0 | 58 | C | **2** | **−1** |
| 5 | C20 | Symbiotic Swarm | Kathril, Aspect Warper | counters | 2 Core | Core | 68.9 | 8 | 2 | 3.00 / 3 | 0.00 | 0 | 50 | B | **2** | ✓ |
| 6 | C21 | Quantum Quandrix | Adrix and Nev, Twincasters | counters | 2 Core | Exhibition | 52.5 | 12 | 1 | 2.00 / 2 | 0.50 | 0 | 50 | B | **2** | ✓ |
| 7 | AFR | Draconic Rage | Vrondiss, Rage of Ancients | tribal | 2 Core | Exhibition | 11.7 | 11 | 1 | 2.00 / 2 | 0.50 | 0 | 48 | B | **2** | ✓ |
| 8 | NEO | Buckle Up | Kotori, Pilot Prodigy | artifacts | 2 Core | Exhibition | 88.7 | 4 | 1 | 2.60 / 3 | 0.40 | 0 | 60 | A | **3** | +1 |
| 9 | BRO | Urza's Iron Alliance | Urza, Chief Artificer | midrange | **4 Optimized** | Exhibition | 87.3 | 15 | 1 | 2.40 / 3 | 0.60 | 0 | 58 | C | **2** | **−2** |
| 10 | 40K | Necron Dynasties | Szarekh, the Silent King | artifacts | 2 Core | Core | 95.2 | **103** | 1 | 2.71 / 3 | 0.57 | 0 | 73 | A | **3** | +1 |
| 11 | LTR | Riders of Rohan | Éowyn, Shieldmaiden | tribal | 2 Core | Core | 37.7 | 24 | 3 | 2.20 / 3 | 0.40 | 0 | 50 | B | **2** | ✓ |
| 12 | WHO | Blast from the Past | The Fourth Doctor + Sarah Jane | combo | **4 Optimized** | Core | 87.1 | 14 | 1 | 2.67 / 3 | 0.33 | 0 | 50 | D | **2** | **−2** |
| 13 | MH3 | Eldrazi Incursion | Ulalek, Fused Atrocity | midrange | 2 Core | Exhibition | 24.6 | 10 | 1 | 2.67 / 3 | 0.00 | 0 | 48 | B | **2** | ✓ |
| 14 | BLB | Animated Army | Bello, Bard of the Brambles | combo | 2 Core | Core | 70.5 | **46** | 1 | 2.50 / 3 | 0.00 | 0 | 58 | B | **2** | ✓ |
| 15 | DSK | Death Toll | Winter, Cynical Opportunist | midrange | 2 Core | Core | 59.0 | 6 | 1 | 3.00 / 3 | 0.50 | 0 | 55 | B | **2** | ✓ |

**Δ column:** measured_bracket − declared_bracket. `✓` = match. Sign convention: `−2` means the engine measured the precon TWO brackets HOTTER than its declared B2 stamp warrants (the deck plays at B4 mechanics behind a B2 label); `+1` means the engine measured it ONE bracket COOLER than declared (rare; mostly B1-leaning Exhibition calls).

**Aggregate:** 11/15 exact match (73%), 2/15 vibes-cooler-than-mechanical (false-positive B4 calls on stock precons), 2/15 mechanical-cooler-than-vibes (Buckle Up + Necron Dynasties as B3 candidates). Within ±1: 13/15.

## Findings

### 1. **False-positive B4 calls on stock precons** (severity: HIGH)

Two of fifteen unedited precons score B4 ("Optimized" — the bracket reserved for tuned high-power non-cEDH decks). Both are mechanically intelligible as misses, not random noise:

- **Urza's Iron Alliance (BRO)** → B4 with raw 0 Game Changers, power_pct 58, no infinite combos. The B4 call cannot be coming from the score ladder (no GCs, modest finishers); it is almost certainly coming from one of the floor lifts added in the May 24 calibration. The `finishers≥8 AND fastMana≥6` "tuned-redundancy floor" was added for voja-tribal-style decks; an Urza precon ships with a lot of low-CMC artifacts (Sol Ring, signets, Mind Stone, the precon's stock fast-mana suite) AND a long tail of artifact-creature finishers, and that combination may be tripping the floor. **Action:** run `--explain-bracket` on this deck and check whether the tuned-redundancy floor fires; if so, tighten the predicate (e.g. require concurrent commander_synergy>50 AND an infinite line, or move the floor from B4 to B3-cap).

- **Blast from the Past (WHO)** → B4 with 0 GCs, archetype=combo. Combo-archetype detection on this deck is itself suspicious (the Doctor Who precon is a flicker/value pile, not a true combo deck); a follow-up worth tracing is whether the combo-archetype classification is feeding a bracket lift that wasn't intended for "the engine called this combo because it happens to have a Doctor partner + a few flicker pieces."

Both decks read **vibes B2** to a human (these are unedited precons; WotC's design floor); calling them B4 is the same magnitude of error as PR #139's pre-fix cEDH false-positives but in the opposite direction. The deltas (−2 each) are the largest in the corpus.

### 2. **B1 call on Mystic Intellect** (severity: MEDIUM)

Sevinne precons are widely regarded as among the weakest WotC ever shipped, so the B1 call is defensible — but the supporting metrics (commander_synergy 74.6%, combo_density 3, depth 2.5/3) say it has more shape than B1's "barely functional" framing suggests. The estimator likely undershoots when low GC count + modest avg CMC + low tutor density compound; consider whether the score floor for "has at least one named combo + has at least one win line" should be B2-cap.

### 3. **Buckle Up and Necron Dynasties read +1 on vibes** (severity: LOW — engine could plausibly be right)

- **Buckle Up (NEO):** power_pct 60, commander_synergy 88.7%, A-grade mana, archetype=artifacts, 3-step value chains with 40% recursive. The metrics genuinely point to a tuned-feeling deck despite the B2 call. This is the precon that famously plays better than its bracket; if anything the mechanical bracket here may be the false-NEGATIVE, not the vibes prediction.
- **Necron Dynasties (40K):** power_pct 73, commander_synergy 95.2%, A-grade mana, 103 win_lines. The 103 is itself a calibration smell — Szarekh's "Necron token" payoff probably gets every Necron creature classified as a win-line piece (token-counting pollution). Real win-line count is probably 5-10. Even ignoring the inflated count, power_pct 73 with A mana base is the strongest stock precon in the corpus.

### 4. **`measured_bracket` and `plays_like` disagree on 9/15 decks** (severity: MEDIUM — investigate)

The two values come from different code paths (bracket from score-ladder + floors/ceilings, plays_like from the deck-feel simulator) and were never expected to match exactly, but a 60% disagreement rate across the unedited-precon floor suggests one of them is systematically miscalibrated for the bottom of the distribution. The mismatches lean one direction — `plays_like` calls Exhibition where `bracket` calls Core — which is consistent with the bracket estimator being right for the average deck and the plays_like simulator under-rating "stuff happens" precons that don't have a clean win condition. A focused study of the 9 mismatches would clarify which path needs adjustment.

### 5. **Mana base grades range A → F across stock precons** (severity: LOW — informational)

- A-grade: Breed Lethality (C16 Atraxa), Buckle Up (NEO), Necron Dynasties (40K). All three contain the precon's full premium dual cycle plus utility lands; the grader works as intended on these.
- F-grade: Subjective Reality (C18 Aminatou). 3-color esper with mostly basics + taplands; the grader correctly flags it as the worst mana base in the corpus.
- D-grade: Blast from the Past (WHO). 2-color WB Doctor Who; the F-side outlier on a 2-color deck is informative — the precon legitimately has a rough mana base.

These look right and don't warrant action; logged for the record.

## Suggested Next Steps

1. **Add this corpus to the bracket regression test.** New invariant: every deck under `data/decks/wizards/` should land at bracket ≤ 3 (Upgraded) unless the test explicitly opts it into a higher bracket. Currently 2/15 violate.
2. **Trace `--explain-bracket` on Urza's Iron Alliance and Blast from the Past.** Identify which floor/ceiling rule fires and tighten it; expected pattern is mirror to the May 24 B5 confirmation gate.
3. **Fix the 103-win-line count on Necron Dynasties.** Token-class creatures shouldn't each count as a discrete win-line entry; the win-line dedupe needs a tribal/token coalescer.
4. **Run the same exercise on the 16-deck `data/decks/test/` corpus** with the new precon corpus appended, and report the combined exact-match rate. Should be ≥ 25/31 to land.

## Reproducing

```bash
# Re-import any precon (Moxfield cache lives at ~/.cache/hexdek/moxfield):
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Re-run Freya standalone on a saved deck:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt --json
```

15 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace):

| Precon | Moxfield ID |
|--------|-------------|
| Mind Seize (C13) | `caPxDQM6Zk6R7MNuhVVXqA` |
| Breed Lethality (C16) | `ycs0QP5BTkWbs7hXNcTUdw` |
| Subjective Reality (C18) | `7Eph6jxhJkekY-as0pl7XA` |
| Mystic Intellect (C19) | `HQVV0USd-ECJa_7kyXdsnw` |
| Symbiotic Swarm (C20) | `VdxcZ7n6skOrhuElFAa5bg` |
| Quantum Quandrix (C21) | `hNhQ07wNf0e7423P6S1P1g` |
| Draconic Rage (AFR) | `Z0Cz_IU2mkC5Qp9y8Nxgog` |
| Buckle Up (NEO) | `QTZDODqtBUOQGIoEbEDbCQ` |
| Urza's Iron Alliance (BRO) | `yvrb9gEkuUSVBT0ulhjxfA` |
| Necron Dynasties (40K) | `8ufvofa2ZkWCvXRR1lCFKQ` |
| Riders of Rohan (LTR) | `wWL4dez0i0euOvWSpmQ7UQ` |
| Blast from the Past (WHO) | `EDdBpyjUFUOBH2ZoOzAkxA` |
| Eldrazi Incursion (MH3) | `guLGf5HBmUyrqbttAnng-A` |
| Animated Army (BLB) | `GAnCfVPj7EGXBf4ftLgn-A` |
| Death Toll (DSK) | `pdTbH1kXzUuDht7kgBm-1g` |
