# Hat EvalWeights Reference — Dimensions, Per-Archetype Profiles, and Tuning Playbook (r60)

> **Reference for future hat tuning work.** Catalogs all 20 `EvalWeights` dimensions consumed by `GameStateEvaluator.EvaluateDetailed`, the per-archetype weight profiles configured in `internal/hat/eval_weights.go`, when each dimension fires (early/mid/late-game multiplicative rescaling), and the empirical tuning playbook accumulated across PRs #793 → #958.

The doc is descriptive — it captures the current state — and prescriptive in the playbook section (what is known to work, what is known to misfire). It's meant as the first thing a future-self or peer reads before opening `eval_weights.go`.

---

## Section 1 — How the dispatch works at runtime

`NewEvaluator(sp *StrategyProfile)` resolves weights in this order (`internal/hat/evaluator.go:64-72`):

1. **`sp.Weights != nil`** → use the strategy-provided weights directly. The Freya pipeline ships `eval_weights` in every `.strategy.json` containing the **first 8 of 20 dimensions** (BoardPresence through GraveyardValue). The loader at `strategy_loader.go:351-366` overlays these 8 onto the archetype-dispatched profile, so the remaining 12 dims always come from `DefaultWeightsForArchetype`.
2. **`sp != nil && sp.Weights == nil`** → fall through to `DefaultWeightsForArchetype(sp.Archetype)`.
3. **`sp == nil`** → fall through to `DefaultWeightsForArchetype(ArchetypeMidrange)`.

**`LegacyMidrangeOnly`** (toggled by `--legacy-hat-weights`) short-circuits `DefaultWeightsForArchetype` to always return the midrange profile. This is the canonical A/B harness used in PRs #923 / #943 / #958 to ablate the archetype-dispatch contribution.

At evaluation time the rescaler at `evaluator.go:4131` (`rescaleWeights`) multiplies the loaded weights by stage-of-game factors (early / mid / late), board-position factors (ahead / behind), and plan-state factors (PlanAssemble / PlanExecute / PlanDefend / PlanDevelop). Stage modulation is detailed in Section 4.

---

## Section 2 — The 20 dimensions

The first 8 are the **Freya-overlay dims** (shipped per-deck in `strategy.json`). The remaining 12 are **archetype-dispatch dims** (set only by `DefaultWeightsForArchetype`).

### Freya-overlay dims (1–8)

| # | Dim | What it scores | Function |
|---|------|---------------|----------|
| 1 | `BoardPresence` | Sum of board power normalized vs opponent average; multi-body width bonus for tokens. | `scoreBoard` (`evaluator.go:264`) |
| 2 | `CardAdvantage` | Hand size, draw engines (Rhystic / Mystic Remora / Esper Sentinel), 4-player table multiplier. | `scoreCards` (`evaluator.go:355`) |
| 3 | `ManaAdvantage` | Mana source count relative to average + color coverage. | `scoreMana` (`evaluator.go:826`) |
| 4 | `LifeResource` | Own life normalized to starting 40. Life-payment decks (Bolas's Citadel, K'rrik) intentionally not penalized when payoffs are on board. Aggro reads this as opponent-life pressure. | `scoreLife` (`evaluator.go:1312`) |
| 5 | `ComboProximity` | How close we are to assembling a known wincon. Multi-tutor credit (PR #793), graveyard-piece-with-recursion (#793), cost-reducer castable-this-turn bonus. Off-class combo plans dampened ×0.7. | `scoreCombo` (`evaluator.go:1598`) |
| 6 | `ThreatExposure` | Negative — average opponent board power relative to our life, plus hoser awareness, vulnerability-aware penalties from Freya threat assessment. | `scoreThreat` (`evaluator.go:2255`) |
| 7 | `CommanderProgress` | Commander combat damage dealt + commander zone status. Voltron's dominant axis. | `scoreCommander` (`evaluator.go:2686`) |
| 8 | `GraveyardValue` | Self-mill payoff scaling (Uurg / Sidisi / Muldrotha / Splinterfright). Delirium thresholds, escape fuel. | `scoreGraveyard` (`evaluator.go:2986`) |

### Archetype-dispatch dims (9–20)

| # | Dim | What it scores | Function |
|---|------|---------------|----------|
| 9 | `DrainEngine` | Aristocrats-style death-trigger payoffs (Blood Artist / Zulaport Cutthroat / Dina) + sac outlets + fodder availability. | `scoreDrainEngine` (`evaluator.go:3433`) |
| 10 | `ArtifactSynergy` | Artifacts on battlefield + treasure tokens + artifact-matters commander payoffs. | `scoreArtifactSynergy` (`evaluator.go:3535`) |
| 11 | `EnchantmentSynergy` | Enchantments on battlefield + enchantress draw triggers. | `scoreEnchantmentSynergy` (`evaluator.go:3598`) |
| 12 | `OpponentGraveyardThreat` | Negative — reanimation targets in opp yards, flashback/escape spells, cheat-into-play creatures. | `scoreOpponentGraveyard` (`evaluator.go:3628`) |
| 13 | `PartnerSynergy` | Partner pair value + on-field interaction. 0 for non-partner decks. | `scorePartnerSynergy` (`evaluator.go:3694`) |
| 14 | `ActivationTempo` | Untapped activated abilities on battlefield. Repeatable engines, mana sinks. | `scoreActivationTempo` (`evaluator.go:3801`) |
| 15 | `ToolboxBreadth` | Diversity of available lines — tutors in hand, modal spells, MDFC flexibility, non-mana activations. | `scoreToolboxBreadth` (`evaluator.go:3875`) |
| 16 | `ThreatTrajectory` | Forward-looking threat assessment — projects each opp's next-turn power from hand size, mana availability, recent cadence. | `scoreThreatTrajectory` (`evaluator.go:3929`) |
| 17 | `StackInteraction` | Counterspells in hand × available mana. Soft "can I respond?" measure. | `scoreStackInteraction` (`evaluator.go:3969`) |
| 18 | `PlaneswalkerProgress` | Planeswalkers on battlefield + loyalty. | `scorePlaneswalkerProgress` (`evaluator.go:4021`) |
| 19 | `ExileZoneAssets` | Cards in exile + enabler that lets us cast them (Cascade-style "exile then cast", Bolas's Citadel, foretell). | `scoreExileAssets` (`evaluator.go:4064`) |
| 20 | `StaxLockProgress` | Battlefield permanents matching stax-lock oracle patterns (nonland-don't-untap, can't-cast, additional-cost-pay). | `scoreStaxLock` (`evaluator.go:4101`) |

---

## Section 3 — Per-archetype weight matrix

Values shown for the 7 customized archetypes currently in the cEDH/self-play test pool. **Bold = at least 0.5 distance from Midrange baseline (the "load-bearing" dim for that archetype).**

| Dim | Midrange | Aggro | Combo | Control | Reanim | Stax | Storm | Voltron |
|---|---|---|---|---|---|---|---|---|
| BoardPresence | 1.0 | **2.0** | **0.4** | 1.0 | 0.8 | 0.7 | **0.2** | 0.8 |
| CardAdvantage | 1.0 | **0.4** | 0.8 | **1.6** | 0.6 | 1.2 | 1.3 | **0.5** |
| ManaAdvantage | 0.8 | 0.8 | 0.7 | **1.3** | 0.5 | 1.0 | **1.3** | 0.5 |
| LifeResource | 0.7 | **1.6** | 0.3 | 0.6 | 0.4 | 0.5 | 0.2 | 0.6 |
| ComboProximity | 0.5 | 0.1 | **2.0** | 0.4 | **1.1** | 0.3 | **2.2** | 0.2 |
| ThreatExposure | 0.8 | 0.6 | 0.5 | **1.3** | 1.2 | **1.5** | **0.3** | **1.8** |
| CommanderProgress | 0.7 | 0.9 | 0.6 | 0.5 | 0.6 | 0.8 | 0.4 | **2.0** |
| GraveyardValue | 0.5 | 0.2 | 0.5 | 0.4 | **1.8** | 0.4 | 0.6 | 0.3 |
| DrainEngine | 0.3 | 0.2 | 0.3 | 0.2 | 0.4 | 0.2 | 0.4 | 0.1 |
| ArtifactSynergy | 0.3 | 0.2 | 0.4 | 0.3 | 0.2 | **1.8** | 0.3 | **1.1** |
| EnchantmentSynergy | 0.3 | 0.2 | 0.3 | 0.3 | 0.2 | 0.5 | 0.2 | **0.9** |
| OpponentGraveyardThreat | 0.6 | 0.3 | 0.5 | **1.3** | **1.1** | 0.8 | 0.3 | 0.3 |
| PartnerSynergy | 0.5 | 0.4 | 0.2 | 0.4 | 0.3 | 0.3 | 0.2 | 0.3 |
| ActivationTempo | 0.4 | 0.2 | 0.3 | 0.7 | **0.9** | 0.5 | **1.8** | 0.5 |
| ToolboxBreadth | 0.5 | 0.2 | **1.0** | 0.7 | 0.4 | 0.4 | 0.5 | 0.4 |
| ThreatTrajectory | 0.5 | 0.3 | 0.5 | **1.1** | 0.5 | 0.7 | 0.3 | 0.6 |
| StackInteraction | 0.7 | 0.2 | 1.0 | **1.5** | 0.4 | **1.6** | **2.0** | **1.2** |
| PlaneswalkerProgress | 0.6 | 0.4 | 0.3 | 0.8 | 0.3 | 0.5 | 0.2 | 0.2 |
| ExileZoneAssets | 0.5 | 0.6 | 0.4 | 0.4 | 0.2 | 0.2 | 0.3 | 0.2 |
| StaxLockProgress | 0.2 | 0.1 | 0.3 | 0.6 | 0.2 | **3.0** | 0.1 | 0.1 |

**L1 distance from Midrange** (the distinctness invariant pinned by `TestArchetypeFingerprint_AllCustomizedAreDistinct` at floor 5.0):

| Archetype | L1 from Midrange | Peak dim |
|-----------|------------------|----------|
| Storm | 9.10 | ComboProximity (2.2) |
| Stax | 8.70 | StaxLockProgress (3.0) |
| Voltron | 7.60 | CommanderProgress (2.0) |
| Reanimator | 6.10 | GraveyardValue (1.8) |
| Aggro | 5.90 | BoardPresence (2.0) |
| Control | 5.60 | CardAdvantage (1.6) |
| Combo | 5.10 | ComboProximity (2.0) |

The other 17 archetype profiles (`Ramp`, `Spellslinger`, `Tribal`, `Aristocrats`, `Selfmill`, `Enchantress`, `Artifacts`, `Lifegain`, `LandsMatter`, `CountersMatter`, `Mill`, `Superfriends`, `Blink`, `ExtraCombats`, `GroupHug`, `Burn`) are not gauntlet-tested; their distinctness is unaudited. Treat them as "exists, untested" rather than "validated."

---

## Section 4 — When each dimension fires (rescaleWeights stage modulation)

`rescaleWeights(gs, seatIdx)` in `evaluator.go:4131` modulates the loaded weights based on game state before they're applied to the dimension scores. Key shifts:

### Early-game amplifications (turn ≤ 5; `earlyFactor = max(0, 1 − stage×2)`)

| Dim | Multiplier | Why |
|-----|-----------|-----|
| ManaAdvantage | × (1 + earlyFactor × 0.3) | Ramp matters most when fresh mana stretches the next 3 turns. |
| CardAdvantage | × (1 + earlyFactor × 0.2) | Draw early scales into more decisions later. |
| PartnerSynergy | × (1 + earlyFactor × 0.15) | Partner pairs need both halves deployed early. |

### Late-game amplifications (turn > 12; `lateFactor = max(0, stage×2 − 1)`)

| Dim | Multiplier | Why |
|-----|-----------|-----|
| ComboProximity | × (1 + lateFactor × 0.3) | Closing power matters. |
| ThreatExposure | × (1 + lateFactor × 0.2) | Lethal windows are closer. |
| BoardPresence | × (1 + lateFactor × 0.15) | Anti-air for late finishers. |
| DrainEngine | × (1 + lateFactor × 0.25) | Drain math compounds against fewer remaining seats. |
| GraveyardValue | × (1 + lateFactor × 0.2) | Recursion engines hit critical mass. |
| CommanderProgress | × (1 + lateFactor × 0.15) | Voltron's clock running. |
| ThreatTrajectory | × (1 + lateFactor × 0.15) | Forward-looking matters most when win-this-turn is plausible. |
| OpponentGraveyardThreat | × (1 + lateFactor × 0.2) | Reanimate / flashback windows. |
| StackInteraction | × (1 + lateFactor × 0.25) | Counter-the-wincon is highest-leverage decision. |
| LifeResource | × (1 + lateFactor × 0.15) | Life gap between seats decides the swing. |

### Mid-game peak (turn 6–10; `midFactor = 1 − abs(stage − 0.5) × 2`)

| Dim | Multiplier | Why |
|-----|-----------|-----|
| ActivationTempo | × (1 + midFactor × 0.2) | Activations peak when boards are populated but not finished. |
| ArtifactSynergy | × (1 + midFactor × 0.15) | Equipment / treasure value compounds mid-curve. |

### Plan-state amplifications (from `PlanState.PlanWeightMultipliers` in `gameplan.go`)

| Plan | Dimensions amplified | Magnitude |
|------|----------------------|-----------|
| `PlanAssemble` | ComboProximity, ToolboxBreadth | ~1.4–1.6 (per `gameplan.go` `Multipliers`) |
| `PlanExecute` | ComboProximity (very high), StackInteraction, life-preserve | sharp peak |
| `PlanDefend` | LifeResource, ThreatExposure | ~1.4 |
| `PlanDevelop` | (no change — baseline) | 1.0 |

Refresh cadence: PR #848 wired `refreshPlanState` into `ChooseCastFromHand` entry so a mid-turn tutor / draw / recursion that flips the Assembling gate is visible to the same turn's cast decisions, not the next upkeep.

### Position-aware shifts (rescaler reads `positionSignal = (myPow − oppPowAvg) / (myPow + oppPowAvg)`)

- When ahead: BoardPresence, CommanderProgress amplified — press the advantage.
- When behind: StackInteraction, CardAdvantage amplified — stabilize and dig.

---

## Section 5 — Tuning playbook (what we know works, what misfires)

Curated from the empirical evidence across PRs #793, #826, #848, #888, #898, #923, #943, #958.

### What works

1. **Anchor a single signature dim at 2.0+.** Every successfully-differentiating archetype has exactly one peak ≥ 2.0: Voltron CommanderProgress 2.0, Stax StaxLockProgress 3.0, Aggro BoardPresence 2.0, Storm ComboProximity 2.2, Combo ComboProximity 2.0, Reanimator GraveyardValue 1.8. The peak is the archetype's *fingerprint*. Without one, the profile collapses toward midrange.
2. **Amplify 2–3 secondary dims to 1.0–1.8.** The L1 distinctness floor of 5.0 (`TestArchetypeFingerprint_AllCustomizedAreDistinct`) cannot be cleared by a single peak alone. Stax (8.7) and Storm (9.1) clear it strongly because they have 3+ secondary dims above midrange. Combo barely clears (5.1) because it only has 2 secondary amps (ToolboxBreadth, StackInteraction).
3. **Pick DEFENSIVE / REACH dims for combo-flavored archetypes.** PR #943's Storm amp on ComboProximity / ActivationTempo / StackInteraction produced **Vial Smasher −1.80pp** at TierGungnir — the hat over-committed to combo lines it couldn't close at budget=50. PR #958's Combo amp deliberately picked ToolboxBreadth + StackInteraction (defensive) to avoid the same trap. The rule of thumb: combo decks at TierGungnir benefit from "expand options + protect" amps; they regress from "execute harder" amps.
4. **Run `--legacy-hat-weights` as the A/B harness.** It short-circuits `DefaultWeightsForArchetype` to midrange uniformly. The L1 distance between full and legacy gauntlet outcomes IS the dispatch's signal. PR #923 measured L1=3.6pp (below noise); PR #943 measured 6.4pp (signal visible); PR #958 added secondary amps for Combo / Reanimator / Control. Repeat the harness whenever changing any archetype profile.
5. **Pin the load-bearing invariants with regression tests.** `TestStormWeights_ComboProximityRemainsHighest`, `TestStaxWeights_StaxLockProgressRemainsHighest`, `TestReanimatorWeights_GraveyardValueRemainsHighest`, `TestControlWeights_CardAdvantageRemainsHighest`, plus the fingerprint floor + pairwise distinctness in `eval_weights_fingerprint_r60_test.go`. Any future amp that breaks an invariant should be explicit (lower the floor in the test alongside the weight change, with a comment).

### What misfires

1. **Amplifying ComboProximity past the regression-test pin at TierGungnir hurts combo decks.** PR #943's Storm ComboProximity 1.8 → 2.2 was kept under the regression invariant; pushing it further (or amplifying Combo's 2.0 higher) reproduces the over-commit pattern.
2. **The Freya-overlay path bypasses the archetype dispatch for dims 1–8.** When `strategy.json` ships `eval_weights` (which it always does for cEDH decks), the 8 Freya-emitted dims override the archetype's defaults. So tuning Combo's BoardPresence in `eval_weights.go` does nothing for cEDH gauntlets — Freya wins. The dispatch matters only for the 12 archetype-only dims (9–20).
3. **`LoadStrategyFromFreya` originally only searched `<deckdir>/freya/`.** PR #888 added the parent-dir fallback after PR #863's honest-null revealed every prior gauntlet had been running with `profile=nil`. Any new gauntlet workflow that stages decks in batched subdirs needs the loader to find the analysis — verify with the tournament log line `loaded Freya strategy (archetype=X, combos=N)`.
4. **Sample size matters a lot.** At n=500/config the per-cell 95% Wilson CI is ±4pp; aggregate L1 across 4 decks has a ~16pp band. Any A/B signal below that band is noise, not evidence. PR #943's 6.4pp signal was at the edge — durable at n=2500 (PR #898's protocol), tentative at n=500.
5. **Weight magnitude is non-monotonic for combo-flavored archetypes at TierGungnir.** "More combo weight = more combo wins" is FALSE at budget=50 because the hat commits to lines it can't close. The relationship may flip at `--hat-budget 200` (TierRagnarok with rollouts active) — PR #898's queued ablation.

### Open questions

1. **Will higher `--hat-budget` (200+, TierRagnarok) flip the combo over-commit story?** Best experiment to run on this code: re-run PR #898's gauntlet at budget=200 with current weights.
2. **Do the 17 non-gauntlet-tested archetypes (Ramp, Spellslinger, Tribal, Aristocrats, …) have any per-deck signal in their respective pools?** None has been validated empirically.
3. **Does `EmergentSynergy` (from Huginn) eventually justify a 21st dim?** The infrastructure is there (`StrategyProfile.EmergentSynergies`); whether it adds eval signal is untested.

---

## Section 6 — Recommended tuning workflow for the next contributor

1. **Identify the archetype to tune.** Run `internal/hat/eval_weights_fingerprint_r60_test.go` and check the L1 distance from Midrange. If < 5.0, the fingerprint test FAILS — that's the floor.
2. **Look at the per-archetype matrix in Section 3.** Identify which of the 20 dims actually correspond to the archetype's gameplan. Anchor the signature dim at 2.0 if not already; consider secondary amps to 1.0–1.6.
3. **Mirror Section 5's playbook.** Defensive amps for combo-flavored archetypes; offense for race archetypes. Do NOT push ComboProximity past the regression-test pin.
4. **Write a pin test** in `eval_weights_*_test.go` for the specific dim values you're shipping. Defends against revert.
5. **Run the A/B harness.** `go run ./cmd/hexdek-tournament --decks <pod> --games 500 --seed 42` twice — once with full strategy, once with `--legacy-hat-weights`. Compute L1 + per-deck deltas. If L1 < 4pp at n=500, the change has no measurable effect; either amplify further or accept it as a fingerprint refinement only.
6. **Document in a `docs/hat-*-r60.md` companion.** Pattern: PR #943 (Stax + Storm), PR #958 (Combo + Reanimator + Control). Cross-reference the empirical evidence and the over-commit caveat from #793/#826/#848/#898.

---

## See also

- [`docs/hat-archetype-amplification-r60.md`](hat-archetype-amplification-r60.md) — PR #943 Stax + Storm amp.
- [`docs/hat-archetype-fingerprint-r60.md`](hat-archetype-fingerprint-r60.md) — PR #958 Combo + Reanimator + Control amp, L1 fingerprint test pin.
- [`docs/hat-self-play-divergence-r60.md`](hat-self-play-divergence-r60.md) — PR #923 honest-null that triggered the amplification series.
- [`docs/hat-bottleneck-r60.md`](hat-bottleneck-r60.md) — PR #888 loader-path fix (the root cause of PR #863's false null).
- [`docs/cedh-gauntlet-real-baseline-r60.md`](cedh-gauntlet-real-baseline-r60.md) — PR #898 first real cEDH baseline.
- `internal/hat/eval_weights.go` — the dispatch tables.
- `internal/hat/eval_weights_fingerprint_r60_test.go` — the distinctness invariants.
- `internal/hat/evaluator.go` `NewEvaluator` + `EvaluateDetailed` + `rescaleWeights` — the runtime application path.
- `internal/hat/strategy_loader.go` `LoadStrategyFromFreya` + `buildFromStrategyJSON` — the Freya-overlay merge.
- `internal/hat/gameplan.go` `PlanState.PlanWeightMultipliers` — the plan-state-aware modulation.
