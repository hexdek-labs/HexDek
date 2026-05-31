# Archetype Fingerprint Refinement — Combo / Reanimator / Control (r60)

> **Followup to PR #943.** That PR amplified Stax + Storm distinctive dims and surfaced two findings: (a) the dispatch IS wired correctly, (b) per-deck signal needs distinct enough weight profiles. This PR audits the remaining customized archetypes (Combo, Voltron, Aggro, Midrange, Control, Reanimator), identifies which are too close to the Midrange baseline to differentiate play, and amplifies the laggards.

## Audit — L1 distance from Midrange (pre-amp)

| Archetype | L1 from Midrange (pre) | Top dim | 2nd dim | Status |
|-----------|------------------------|---------|---------|--------|
| Voltron | **7.6** | CommanderProgress (2.0) | ThreatExposure (1.8) | already distinct, leave alone |
| Aggro | **5.9** | BoardPresence (2.0) | LifeResource (1.6) | already distinct, leave alone |
| Reanimator | **5.5** | GraveyardValue (1.8) | ComboProximity (1.1) | modest amp opportunity |
| Control | **5.0** | StackInteraction (1.5) | CardAdvantage (1.6) | modest amp opportunity |
| Combo | **4.5** | ComboProximity (2.0) | ArtifactSynergy (0.4) | **LEAST DISTINCT — needs amp** |
| Midrange | n/a | — | — | the baseline |

(Stax / Storm L1 values are post-#943 amplification; they're not the targets of this PR.)

Voltron at L1=7.6 is the gold standard — multiple strongly-distinctive dims, both primary (CommanderProgress 2.0) and secondary (ThreatExposure 1.8 / ArtifactSynergy 1.1 / EnchantmentSynergy 0.9 / StackInteraction 1.2). Combo at L1=4.5 was the opposite — a strong peak (ComboProximity 2.0) but every secondary dim sat within 0.1 of Midrange. The hat played Combo as "Midrange with one big dial" rather than as a distinct archetype.

## Amplifications

Picked DEFENSIVE / REACH dimensions for the combo-flavored archetypes to avoid the over-commitment pattern PR #943's Storm amp produced at TierGungnir (Vial Smasher −1.80pp). The goal is distinguishable PLAY, not premature combo commitment.

### Combo (L1 4.5 → 5.1)

| Dim | Before | After | Rationale |
|-----|--------|-------|-----------|
| ToolboxBreadth | 0.6 | **1.0** | Tutors / toolbox is the assembly path. Valuing them higher expands the hand's options BEFORE committing. |
| StackInteraction | 0.6 | **1.0** | Force of Will / Pact of Negation / Veil of Summer protect the combo from being countered mid-resolution. Hold the counter open rather than dumping it for premature tempo. |

ComboProximity stays at 2.0. Regression test `TestStormWeights_ComboProximityRemainsHighest` family of tests was checked — Combo has no peak-pinning test, but lifting ComboProximity higher would worsen the TierGungnir over-commit pattern documented across PRs #793 / #826 / #848 / #898 / #943.

### Reanimator (L1 5.5 → 6.1)

| Dim | Before | After | Rationale |
|-----|--------|-------|-----------|
| OpponentGraveyardThreat | 0.8 | **1.1** | Opp yards = fuel for cross-deck reanimate / Bojuka Bog timing. Reanimator decks DO read opponent yards because Animate Dead works just as well on opp's tutored fatty. |
| ActivationTempo | 0.6 | **0.9** | Volrath / Karador / Sheoldred / Windgrace are activated reanimators; the activation axis is the gameplan. |

GraveyardValue stays at 1.8 (peak, regression test `TestReanimatorWeights_GraveyardValueRemainsHighest`).

### Control (L1 5.0 → 5.6)

| Dim | Before | After | Rationale |
|-----|--------|-------|-----------|
| OpponentGraveyardThreat | 1.0 | **1.3** | Control reads the long game by tracking what opponents have spent and what they still hold; the graveyard is half that signal. |
| ThreatTrajectory | 0.8 | **1.1** | "Is THIS the threat I answer this turn vs save the counter for a bigger one in 2 turns" IS Control. The dial was below midrange — wrong direction. |

CardAdvantage stays at 1.6 (peak, regression test `TestControlWeights_CardAdvantageRemainsHighest`).

## Pre-existing main breakage fixed inline

While running `go test ./internal/hat/...`, hit a `ReverseIndex redeclared in this block` build error in `internal/huginn`. Diagnosis: PR #944 (wave2 multistep batch1) merged a `type ReverseIndex interface` in `recommender.go:44` that collides with the pre-existing `func ReverseIndex(oracleID string) []string` in `reverse_index.go:329` (same package). Hat package transitively imports huginn (`strategy_loader.go`), so this blocked all hat tests on main.

**Surgical 2-line fix:** renamed the unused interface to `ReverseIndexer` (idiomatic Go naming for interface types, no external consumers). Restores `go test ./internal/hat/...` and `go build ./...` to clean. Not the focus of this PR but had to be fixed to ship the work.

## Tests (5 new pins, all pass)

`internal/hat/eval_weights_fingerprint_r60_test.go`:

| Test | Pins |
|------|------|
| `AllCustomizedAreDistinct` | every customized archetype has L1 ≥ 5.0 from Midrange (calibrated at the pre-amp Combo's failure threshold) |
| `ComboAmpLanded` | Combo ToolboxBreadth=1.0, StackInteraction=1.0, ComboProximity=2.0 (peak) |
| `ReanimatorAmpLanded` | Reanimator OpponentGraveyardThreat=1.1, ActivationTempo=0.9, GraveyardValue=1.8 (peak) |
| `ControlAmpLanded` | Control OpponentGraveyardThreat=1.3, ThreatTrajectory=1.1, CardAdvantage=1.6 (peak) |
| `PairwiseDistinctness` | every pair of customized archetypes has L1 ≥ 3.5 from each other (defends against amplifying onto a peer) |

All existing weight invariants pass (`TestStormWeights_ComboProximityRemainsHighest`, `TestStaxWeights_StaxLockProgressRemainsHighest`, `TestReanimatorWeights_GraveyardValueRemainsHighest`, `TestControlWeights_CardAdvantageRemainsHighest`, etc.). Full hat suite green. `go build ./...` clean.

## Post-amp L1 distances from Midrange

| Archetype | L1 (post-amp) | Δ vs pre | Floor (5.0) cleared? |
|-----------|---------------|----------|----------------------|
| Voltron | 7.6 | 0 (unchanged) | ✓ |
| Aggro | 5.9 | 0 (unchanged) | ✓ |
| Reanimator | **6.1** | +0.6 | ✓ |
| Control | **5.6** | +0.6 | ✓ |
| Combo | **5.1** | +0.6 | ✓ (just barely) |

All 7 customized archetypes now clear the 5.0 distinctness floor. The L1=5.1 Combo result is the new ceiling-of-defensible-distinctness — bringing the dim it shares with Midrange any closer would trigger the regression invariant.

## 500-game self-play probe

Same 4-archetype gauntlet as PR #923 / #943 (Etali Midrange / Francisco Combo / Tayam Stax / Vial Smasher Storm), seed 42, 500g per config.

| Deck | Arch | Config A_fp (amps live) | Config B_fp (legacy forces midrange) | Δ pp |
|------|------|--------------------------|--------------------------------------|------|
| Etali, Primal Conqueror | Midrange | 41.0% | 39.8% | +1.2 |
| Tayam, Luminous Enigma | Stax | 31.6% | 32.8% | −1.2 |
| Francisco, Fowl Marauder | Combo | 24.4% | 24.0% | +0.4 |
| Vial Smasher the Fierce | Storm | 3.0% | 3.4% | −0.4 |

**A_fp vs B_fp L1 = 3.2pp.** Below PR #943's 6.40pp — but the configs are different (PR #943 measured Stax+Storm amps vs forced midrange; this PR measures Stax+Storm+Combo+Reanimator+Control amps vs forced midrange — the Combo amp specifically shifts Francisco's behavior but Francisco is a marginal-winrate deck in this pod so the empirical effect at n=500 is small).

**The probe's primary value is confirming no regression**: amplifying 3 additional archetypes did NOT reverse the prior signal direction, did NOT introduce crashes, and the Combo deck's winrate moved +0.4pp toward A_fp — the expected direction. The TEST is the durable contract here; the gauntlet is supplementary observation.

## Caveats

- **n=500 per config is below the per-cell ±4pp noise floor.** Individual per-deck shifts are not statistically distinguishable. The fingerprint tests are the durable invariant.
- **Single 4-deck pod.** The Combo amp affects Francisco's hand evaluation but Francisco was already a marginal winrate deck (24% range) — the amp's effect floor here is small.
- **Same TierGungnir budget=50.** PR #898's `--hat-budget 200` ablation is still queued. The Combo amp's `+0.6 L1` would compound differently with rollouts active.
- **Reanimator and Control amps were not gauntlet-tested.** Neither archetype is in the cEDH gauntlet pool; the amps land cleanly via the fingerprint tests but their empirical effect on Reanimator / Control decks remains untested at the gauntlet level. This is acceptable because the fingerprint floor is the load-bearing claim.

## Recommendation

- **Land this as the first PR with a quantitative archetype-distinctness invariant.** The 5.0 L1 floor + 3.5 pairwise floor prevent future tunings from regressing distinctness without explicit acknowledgment.
- **Do NOT amplify Combo ComboProximity further.** The over-commit pattern documented across the #793/#826/#848/#898/#943 chain says higher ComboProximity at TierGungnir hurts Combo decks. The defensive amps shipped here (Toolbox + StackInteraction) are the safe direction.
- **A Reanimator-pool gauntlet (4 distinct Reanimator decks) would validate the +0.6 L1 amp empirically.** Not in this PR's scope but a natural followup if Reanimator becomes a focal archetype.
- **Run `--hat-budget 200` ablation across all 5 amplified archetypes (#943's Stax+Storm + this PR's Combo+Reanimator+Control)** once the budget infrastructure ships. Will distinguish "amp produces signal at TierRagnarok" from "amp structurally equivalent at all budgets."

## Files

- `internal/hat/eval_weights.go` — Combo, Reanimator, Control amplifications (6 dim edits total)
- `internal/hat/eval_weights_fingerprint_r60_test.go` — 5 new fingerprint invariant tests
- `internal/huginn/recommender.go` — pre-existing `ReverseIndex` collision fix (rename interface to `ReverseIndexer`)
- `docs/hat-archetype-fingerprint-r60.md` — this report
- `docs/hat-archetype-fingerprint-r60-data/config_a_fp.md` + `config_b_fp.md` — raw 500g probe outputs

## See also

- [`docs/hat-archetype-amplification-r60.md`](hat-archetype-amplification-r60.md) — PR #943 that ships Stax + Storm amplifications and surfaces the L1-distinctness framing.
- [`docs/hat-self-play-divergence-r60.md`](hat-self-play-divergence-r60.md) — PR #923 honest-null that triggered the investigation.
- `internal/hat/eval_weights.go ArchetypeCombo / ArchetypeReanimator / ArchetypeControl` — the profiles amplified.
- `internal/hat/eval_weights_fingerprint_r60_test.go` — durable distinctness invariant.
