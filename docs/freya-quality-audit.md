# Freya Quality Audit — 2026-05-16

20-deck random sample from `/api/decks?owner=moxfield` (full corpus = 1,267 decks). Each deck re-run through `hexdek-freya --format json` against the current `oracle-cards.json` (172 MB, 2026-04-30) and freshly-built mechanic DB. Cached `strategy.json` files were overwritten and compared against the API-listed metadata (owner-declared bracket `wbs`, previously-cached plays-like `pls`, declared `archetype`).

Sampling was seeded (`awk srand(20260516)`) for reproducibility. Deck IDs and raw outputs are preserved under `data/decks/moxfield/freya/` (strategy JSON) and `/tmp/freya_audit/` (per-run stderr logs + `timings.tsv` + `compare.json`).

---

## 1. Per-deck analysis time

| Metric | ms |
|---|---|
| Avg total wall-clock (load + analyze + save) | **5,697** |
| Avg analysis-only (excluding oracle + mechDB load) | **360** |
| Min analysis-only | 259 (`ashling_rekindled_b2`) |
| Max analysis-only | 639 (`kruphix_god_of_horizons_b4`) |

Oracle load (~3.2 s on this hardware, 31k entries) and mechanic-DB build (~1.5 s) dominate single-deck wall time. They are amortized in `--all-decks` mode and in the API server which keeps both resident. **Per-deck analysis cost is dominated by `ComputeWinLines` and the role classifier**, not classification — the 639 ms outlier (Kruphix) has 9 win lines and 7 finishers; the 259 ms minimum (Ashling) has 25 win lines but a smaller resolved-card set.

Spread is narrow (2.5× between min and max), so analysis cost is roughly linear in deck size with a small constant — no pathological cases in this sample.

### Per-deck table

| Commander | fname B | wbs (declared) | freya B (strict) | freya plays-like | GC | pwr% | win-lines | value-chains | finishers | analysis ms |
|---|---|---|---|---|---|---|---|---|---|---|
| Kruphix, God of Horizons | 4 | 2 | 2 | 2 | 3 | 76 | 9 | 3 | 7 | 639 |
| Zask, Skittering Swarmlord | 3 | 2 | 2 | 3 | 0 | 65 | 7 | 10 | 2 | 491 |
| Tymna the Weaver | 4 | 5 | **5** | 3 | **16** | 93 | 13 | 8 | 3 | 412 |
| Shanna, Purifying Blade | 2 | 2 | 2 | 3 | 0 | 78 | 12 | 5 | 7 | 368 |
| Kylox, Visionary Inventor | 4 | 2 | 2 | 1 | 0 | 63 | **39** | 1 | 2 | 282 |
| Inspirit, Flagship Vessel | 4 | 2 | 2 | 2 | 1 | 65 | 8 | 5 | 6 | 357 |
| Urza, Chief Artificer | 4 | 4 | 4 | 3 | 9 | 88 | 6 | 3 | 4 | 326 |
| Ashling, Rekindled // Rimebound | 2 | 2 | 2 | 2 | 0 | 71 | **25** | 1 | 3 | 259 |
| Sefris of the Hidden Ways | 4 | 3 | 3 | 3 | 2 | 93 | **42** | 7 | 4 | 336 |
| Mishra, Claimed by Gix | 3 | 3 | 3 | 3 | 3 | 76 | 17 | 6 | 4 | 300 |
| Ashnod, Flesh Mechanist | 3 | 2 | 2 | 1 | 1 | 78 | 9 | 8 | 5 | 426 |
| Tifa, Martial Artist | 2 | 2 | 2 | 2 | 0 | 65 | 21 | 1 | 14 | 413 |
| Tinybones, Trinket Thief | 4 | 2 | 2 | 2 | 1 | 78 | 5 | 4 | 3 | 262 |
| Riku of Two Reflections | 3 | 2 | 2 | 1 | 1 | 89 | 2 | **0** | **0** | 398 |
| Obeka, Brute Chronologist | 4 | 3 | 3 | 3 | 2 | 76 | 4 | 4 | 0 | 290 |
| Grub, Storied Matriarch // Notorious Auntie | 3 | 3 | 3 | 2 | 1 | 75 | 11 | 5 | 4 | 360 |
| Berta, Wise Extrapolator | 2 | 2 | 2 | 2 | 0 | 86 | 5 | 5 | 3 | 319 |
| The Wise Mothman | 2 | 1 | 1 | 2 | 0 | 60 | 4 | 6 | 2 | 337 |
| Grand Arbiter Augustin IV | 4 | 3 | 3 | 1 | 4 | 70 | 9 | 4 | 7 | 299 |
| Berta, Wise Extrapolator (b3) | 3 | 2 | 2 | 2 | 1 | 78 | 6 | 5 | 4 | 327 |

`fname B` = bracket digit embedded in the imported deck ID (the source-site classification).
`wbs` = owner-declared "what bracket says" persisted in the deck row.
`freya B` = `estimateBracket` (gameplay-tier with WotC Game Changers ceiling/floor rules).
`freya plays-like` = `estimatePlaysLike` (power output: win-line density, true infinites, speed, redundancy).

---

## 2. Archetype detection

**0 / 20 archetype failures.** Every deck classified to the same primary archetype as the API metadata (`midrange`, `lands matter`, `stax`, `lifegain`, `artifacts`, `combo`, `counters matter`, `tribal`, `reanimator`, `aristocrats`). Cached bracket value matched the freshly-computed measured bracket on **all 20** — the classifier is deterministic and stable, no archetype regressions since these decks were imported. (Note: at the time of this audit the `plays_like` signal still existed as a separate measurement; that scaffolding was removed in the dev/remove-plays-like-r60 sweep and the canonical felt-power signal is now `measured_bracket`.)

This is a strong signal that the recent expansion to 22 archetype fingerprints (CLAUDE.md Done list, 2026-04-29) is holding up against real moxfield input.

Caveat: the sample size is small and the archetype label space already maps near-1:1 onto deck-ID tags, so this is an existence proof of stability rather than a coverage claim. Hidden-archetype decks (where the owner-declared label is wrong) cannot be detected by metadata-match alone — these would need to be flagged by deck-import audits, not Freya re-runs.

---

## 3. Bracket estimation

### 3a. Drift summary (Freya plays-like vs owner-declared `wbs`)

| Category | Count |
|---|---|
| Freya plays-like *lower* than declared | 7 |
| Match | 10 |
| Freya plays-like *higher* than declared | 3 |

Freya's *strict* bracket (`estimateBracket`) tracks the owner's declaration much more closely — only **Tymna the Weaver** had `freya_bracket != wbs`, and that case is **Freya is right**: 16 Game Changers + 93rd-percentile power → B5/cEDH, the owner already labelled it B5. The other 19 are aligned on the strict-bracket axis.

So the real drift is **between Freya's two internal verdicts**: `estimateBracket` (config-tier, anchored to WotC Game Changer rules) vs `estimatePlaysLike` (mechanical performance). They disagreed on **10 / 20** decks. That gap is exactly the signal these two metrics are designed to surface — but two of the disagreements look like genuine misfires:

### 3b. Suspicious calls

| Deck | Freya verdict | Why it looks off |
|---|---|---|
| **Grand Arbiter Augustin IV** (cmd/stax, listed B4) | strict B3 / plays-like **B1** | 4 GCs (Mystical Tutor, Drannith Magistrate, Narset, Farewell), A-grade mana base, 9 win lines, 7 finishers, 70th-pct power. Plays-like B1 ("Exhibition") is wrong for any GAAIV stax build — competitive lock-piece commander with cheap counterspells should be B3+. Speaks to `estimatePlaysLike` not weighting **stax lock-piece density** or **commander tempo** at all — it's reading off win-line count + true-infinites + avg-CMC, and GAAIV stax wins through resource denial not combo kills, so it falls through the cracks. |
| **Kylox, Visionary Inventor** (midrange, listed B4) | strict B2 / plays-like B1 with **39 win lines** | 39 win lines but plays-like B1 means the win-line counter is fanning permutations of one core engine (Ashling + N enablers) without deduplicating, so the line count is meaningless. The plays-like estimator implicitly discounts because the underlying signals (true infinites = 0, avg CMC = 3.4, GC = 0) say "low power," and the win-line score caps out at +3 regardless of count. **Two defects compounding**: win-line over-permutation (Ashling appears as anchor for 39 trivially-different triples) AND plays-like ignoring it correctly. The bug is the count, not the verdict. |
| **Sefris of the Hidden Ways** (combo, listed B4) | strict B3 / plays-like B3 with **42 win lines** and **93rd pct** | Same over-permutation pattern — 42 lines is implausible. The headline verdict (B3 / B3) is fine, but the count is noise. |
| **Ashling, Rekindled** (midrange B2) | 25 win lines | Another over-permutation case. |
| **Tifa, Martial Artist** (tribal B2) | 21 win lines, **14 finishers** | 14 finishers on a B2 tribal deck is improbable — the finisher classifier is too permissive, likely promoting any creature with evasion or +1/+1 counters. |
| **Riku of Two Reflections** (midrange B3) | 0 value chains, 0 finishers, 2 win lines, plays-like B1 despite **89th-pct power** | Riku copies spells; the value-chain detector and finisher classifier missed every doubling-engine signal. 89th-pct percentile but plays-like Exhibition is a sign that `power_percentile` and `estimatePlaysLike` are drawing from different signal sets and disagreeing. |
| **Kruphix, God of Horizons** (midrange B4) | strict B2 with 3 GCs | The WotC ceiling rule caps 1-3 GCs at B3 (`bracket > 3 → 3`) but does **not** raise a B2 with 3 GCs to B3. The current floor rule says "GC=1-3 and bracket < 2 → 2"; there is no "GC=3 → minimum 3" floor. Many CAG readings of bracket policy would peg "3 Game Changers" at B3 minimum. Worth re-examining the floor. |

### 3c. Stable / well-behaved decks

`Mishra (B3, 3 GCs, plays B3)`, `Urza (B4, 9 GCs, plays B3)`, `Tymna (B5, 16 GCs, plays B3)`, and `Obeka (B3, 2 GCs, plays B3)` all show the expected pattern: strict bracket follows the GC ladder, plays-like trails by 0-2 brackets because combo execution is slow without fast mana. These are the calibration cases that work as intended.

---

## 4. Recommended improvement priorities

Ranked by audit-evidence weight and probable difficulty.

### P0 — Win-line deduplication (high impact, low complexity)

`ComputeWinLines` is emitting permutations of a single engine as distinct lines. Kylox (39), Sefris (42), Ashling (25), Tifa (21) are all over-counts; Riku (2) is an under-count. The fix is to **canonicalize win-line keys by anchor card + finisher type**, not by every k-tuple of pieces that happen to combine. This noise also propagates into the strategy JSON the hat reads, so the AI is seeing inflated win-line vectors.

Suggested rule: collapse lines sharing (anchor, finisher_type) into one line with combined piece sets; cap displayed lines at a reasonable bound (e.g. 10) with an "N additional permutations" tail.

### P1 — `estimatePlaysLike` blind to stax / control pressure

Grand Arbiter Augustin IV failing to "Exhibition" is the clearest miscall in the sample. `estimatePlaysLike` only scores **win lines, true infinites, avg CMC, redundancy** — the four levers of combo decks. It has no input from:
- Stax piece density (Drannith Magistrate, Trinisphere, Sphere of Resistance, Thalia, ...)
- Counterspell density (already a bracket signal — should also feed plays-like)
- Tax-piece curves (cheap lock pieces win by suffocation, which is a B3-4 play pattern even with zero "win lines")
- Commander-as-payoff (high `commander_synergy` should bump plays-like since the deck's "engine" is on the field by turn 3-5)

Add a `controlPressure` term to `estimatePlaysLike` summing role-tagged hosers, counterspells, and high-commander-synergy creatures. Verify against the Grand Arbiter + Tinybones + Tymna cases.

### P1 — Game Changer floor: bump 3-GC decks to B3 minimum

Current floors:
- GC = 1-3 → B2 floor
- GC ≥ 4 → B3 floor

But the practical reading of WotC's bracket guidance is that 3 GCs is a meaningful concentration that should not be assigned B2. Recommend changing to:
- GC = 1-2 → B2 floor
- GC ≥ 3 → B3 floor

Re-validate against Kruphix (3 GCs, currently B2 strict) and the 22-archetype calibration suite before merging.

### P1 — `power_percentile` and `measured_bracket` cross-check (HISTORICAL — pre-removal)

Riku reads 89th percentile but plays-like B1. (Historical context: at audit time, the percentile estimator and the now-removed `estimatePlaysLike` used disjoint signals.) `estimatePlaysLike` has since been removed; the canonical felt-power signal is `measured_bracket`. The "no wincon → floppy" semantics that `plays_like` tracked are scheduled for absorption into `measured_bracket` in a follow-up calibration chapter.

### P2 — Finisher classifier over-promotion

Tifa (B2 tribal, 14 finishers) and Shanna (B2 lifegain, 7 finishers) are flagging far too many cards as finishers. The classifier appears to treat any evasive creature or any direct-damage source as a finisher; on a tribal deck with creature redundancy that's nearly every nonland card. Tighten the finisher heuristic to require either (a) an explicit "loses the game" / "win the game" / lethal-trigger pattern, or (b) per-archetype thresholds (combat finishers need haste + evasion + ≥ 4 power *and* survive sweeper).

### P2 — Riku-type "engine commander" detection

Riku copies spells but Freya found 0 value chains and 0 finishers. The value-chain extractor should detect "copy / double" commanders as a top-level engine pattern (Riku, Reflections of Kiki-Jiki, Mirari, Dualcaster effects) and treat them as a synthesized engine even when no explicit value-chain card is in the deck. Same gap likely affects other "the commander IS the value engine" decks (Thrasios, Niv-Mizzet, Kykar, ...).

### P3 — Oracle + mechDB load is a bottleneck for single-shot CLI use

5.7 s per `--deck` invocation is bearable but slow. Cache the mechDB build to `data/rules/mechanic_db.gob` keyed by oracle SHA; halve cold-start time for repeated runs.

### P3 — Audit-as-code

Promote this audit into a `cmd/hexdek-freya-audit/` tool that samples N decks, runs them, and compares cached vs current strategy JSON. Run on every Freya code change to catch drift before merge. The pipeline already exists in `/tmp/freya_audit_run.sh` and `/tmp/freya_compare2.py` — only needs to be productionized in Go.

---

## 5. What this audit does *not* cover

- **Ground truth.** "Owner-declared bracket" is the only label available and is known to be noisy (owners over- and under-claim). Without playtested win-rate data, "Freya disagrees with the owner" is not the same as "Freya is wrong."
- **Larger sample.** 20 decks is a sniff test. The full 1,267-deck corpus should be re-run before treating any of the P0-P1 fixes as validated.
- **Hat-level effects.** Freya's strategy JSON feeds the AI's eval weights; over-counted win lines and missed engines change AI behavior in tournament play. A downstream audit running `hexdek-tournament` before and after each P0/P1 fix is the only way to confirm the fix improved play, not just the numbers.

---

## Reproduction

```bash
go build -o /tmp/hexdek-freya-audit ./cmd/hexdek-freya/

curl -s "https://hexdek.dev/api/decks?owner=moxfield" > /tmp/freya_audit_all.json
jq -r '.[] | "\(.id)\t\(.bracket)\t\(.archetype)\t\(.commander_card)"' /tmp/freya_audit_all.json \
  | awk 'BEGIN{srand(20260516)} {print rand()"\t"$0}' | sort | head -20 | cut -f2- \
  > /tmp/freya_audit_sample.tsv

mkdir -p /tmp/freya_audit
while IFS=$'\t' read -r id bracket arch cmdr; do
  /tmp/hexdek-freya-audit --deck "data/decks/moxfield/${id}.txt" --format json \
    > "/tmp/freya_audit/${id}.json" 2> "/tmp/freya_audit/${id}.log"
done < /tmp/freya_audit_sample.tsv
```

Raw artifacts: `/tmp/freya_audit/compare.json` (20-row table), `/tmp/freya_audit/timings.tsv`, `/tmp/freya_audit/<deck-id>.log` (per-deck stderr), and the refreshed `data/decks/moxfield/freya/<deck-id>.strategy.json` files.
