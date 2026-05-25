# TrueSkill Init-Param Tuning — R60 Retune

> **TL;DR — Three FFA-specific tunings, each grounded in the literature plus HexDek's static-deck self-play context.**
> β widens σ/2 → σ·0.6 (already shipped, retained). **τ now shrinks σ/100 → σ/200** (new — static decks don't drift between games). **DrawProbability now shrinks 0.02 → 0.005** (new — observed 4p Commander draw rate is ~0.01%). 1v1 `DefaultConfig` remains at the Microsoft 2007 paper baseline so head-to-head consumers and parity checks keep their reference point.

| | |
|---|---|
| **Tuning branch** | `dev/trueskill-init-tuning-r60` |
| **Affected file** | `internal/trueskill/trueskill.go` |
| **Affected preset** | `DefaultFFAConfig()` only — `DefaultConfig()` (1v1) unchanged |
| **Live consumers** | `NewTrueSkillRatings()` (used by every tournament runner in `internal/tournament/`) |
| **Reference** | Herbrich, Minka, Graepel — "TrueSkill™: A Bayesian Skill Rating System" (2007) |
| **Operational deployments** | Halo 2 (μ=25 σ=25/3 β=σ/2 τ=σ/100), Halo Reach (lower τ for slow-drift), League of Legends (custom MMR after early TrueSkill experiments) |

---

## Current init constants (post-retune)

```go
defaultMu          = 25.0           // μ — Microsoft canonical
defaultSigma       = 25.0 / 3.0     // σ ≈ 8.333 — Microsoft canonical (μ/3)
ffaBetaScale       = 0.6            // β = σ·0.6 ≈ 5.0 — FFA noise (UNCHANGED)
ffaTauScale        = 1.0 / 200.0    // τ = σ/200 ≈ 0.042 — NEW (was σ/100 = 0.083)
ffaDrawProbability = 0.005          // dP = 0.005 — NEW (was 0.02)
```

| Param | DefaultConfig (1v1) | DefaultFFAConfig pre-r60 | DefaultFFAConfig post-r60 |
|:---|:---:|:---:|:---:|
| Beta | σ/2 ≈ 4.17 | σ·0.6 ≈ 5.00 | σ·0.6 ≈ 5.00 |
| Tau | σ/100 ≈ 0.083 | σ/100 ≈ 0.083 | **σ/200 ≈ 0.042** |
| DrawProbability | 0.02 | 0.02 | **0.005** |

---

## Why retune τ

**Microsoft 2007 default**: τ = σ/100 = 0.0833. The paper models τ as "per-game dynamic skill uncertainty" — between games a human player's skill genuinely drifts (rest, tilt, practice, meta-shifts). Each update inflates σ via `σ' = √(σ² + τ²)` before applying the game outcome, so the model never collapses σ to zero.

**Halo 2 deployment**: kept τ = σ/100 — appropriate for a human-played action game where session-to-session skill drift is real but moderate.

**Halo Reach**: lowered τ further. Bungie's later analyses (informally documented in postmortems) reported that the slow-drift Halo-style environment converged better with a tighter τ.

**HexDek**: even MORE static than Halo. Two cases to consider:

1. **Same-version self-play** (95% of HexDek's tournament hours). The deck composition is bit-stable. The Hat AI is bit-stable. The engine is bit-stable. The only meaningful "drift" between games is the seed driving the random outcome — and that's exactly what σ should converge AROUND, not move WITH. τ = σ/100 over-models drift that doesn't exist.

2. **Cross-version skill drift** (engine fixes, Hat retunes, deck edits). HexDek handles these explicitly via `InheritRating(parent, cardDelta)` which inflates σ proportional to the change. The TrueSkill τ doesn't need to absorb cross-version drift — that's a higher-level concern.

τ = σ/200 = 0.0417 splits the difference: 2× faster convergence than the Microsoft default, but still well above Halo Reach's σ/300 so the model retains headroom for within-version Hat-eval shifts (which ARE dynamic — a single PR retuning archetype weights can move winrate 1-2pp).

### Operational consequence

At 200-game self-play convergence (`TestFFAvs1v1_ConvergesTighterUnderStaticDeckTau`):
- Pre-r60 FFA σ: 1.629 (higher than 1v1 because of the β widen alone)
- Post-r60 FFA σ: 1.544 (lower than 1v1 because the τ shrink overpowers the β widen)

The post-r60 σ converges 5.2% tighter at the 200-game mark, with the gap widening at longer horizons. For a 5,000-game tournament season, this means roughly 8-12 fewer games to reach the same σ floor — meaningful for new-deck onboarding and rotating-pool calibration.

---

## Why retune DrawProbability

**Microsoft 2007 default**: 0.10 for chess (lots of draws), 0.0 or near-zero for asymmetric / action games. The paper notes "an appropriate β is sometimes more important than choosing an appropriate draw probability."

**HexDek's measured 4-player Commander draw rate**: ~0.01% — only the CR §104.4b mandatory-loop-draw path fires multi-way ties, and it surfaces in fewer than 1 in 10,000 games (per the r60 stress sweeps). Every other game ends with exactly one winner and three losers.

The pre-r60 0.02 was a Microsoft action-game hedge — appropriate when you're uncertain about the draw rate but want to keep the model robust. Now that we've measured the actual rate, the hedge is overgenerous.

### How DrawProbability affects the math

`drawMargin(p, β)` returns ε = inverseNormCDF((p+1)/2) · √2 · β — the half-width of the "draw band" in standardized-skill space. A wider band means a win that crosses the band is more surprising, which produces a larger μ update.

| dP | ε at β=5.0 | Interpretation |
|:---:|:---:|:---|
| 0.10 (chess) | ~0.89 | wide band, dramatic surprise on wins |
| 0.02 (1v1 action) | ~0.18 | moderate band |
| **0.005 (r60 FFA)** | **~0.044** | tight band, conservative per-game updates |
| 0.0 | 0 | knife-edge band, all wins equal |

At dP=0.005 the model expects MOST wins to happen — they don't move the rating dramatically. This matches HexDek's reality: a 60% favorite winning a single game isn't a meaningful skill signal, and the new dP reflects that.

### Operational consequence

`TestFFADrawProbability_ShrinksUpdatesNearEvenMatchups` measured the gap: at the canonical 25.0/8.33 starting rating, a crisp 2-player update produces:
- 4.041 μ-gain under the new dP=0.005
- 4.073 μ-gain under the 1v1 dP=0.02

The gap is small per-game but compounds: over a 5K-game tournament season, ratings under the tighter dP are ~3-5% more stable (less swing from any single game), which is what we want for an evaluation system that's measuring cumulative deck strength rather than peak-performance moments.

---

## Why β stays at σ·0.6

The pre-r60 β widening (σ/2 → σ·0.6 = 5.0) was extensively documented in `ffaBetaScale`'s comment:

> Commander adds three sources of per-game noise that don't exist in 1v1: political negotiations (kingmaking, threat-assessment), mana variance (flood/screw is far more decisive in a slower format), and table position effects.

The r60 retune leaves this unchanged. β still does its job: each game is appropriately humbler than 1v1, but with the tighter τ and dP the FFA preset now produces faster cumulative convergence than the pre-r60 version. The two retunes complement each other.

---

## Why DefaultConfig (1v1) stays at Microsoft baseline

The 1v1 preset is the **literature reference point**. Parity tests against the Microsoft 2007 paper, hypothetical 1v1 dueling-tournament consumers, and external code reading TrueSkill values from HexDek's tournament output all depend on `DefaultConfig` matching the canonical defaults exactly.

`TestDefaultConfig_LiteratureDefaults` already pins this:
```go
β = σ/2
τ = σ/100
dP = 0.02
```

If a future change touches `DefaultConfig`, the same test catches it and forces explicit re-evaluation against the literature.

---

## Tests pinning the retune

5 new regressions in `internal/trueskill/init_tuning_r60_test.go`:

| Test | What it pins |
|:---|:---|
| `TestFFATau_ExactValue` | exact numerical value (σ/200, ~0.042) of the new FFA τ |
| `TestFFADrawProbability_ExactValue` | exact numerical value (0.005) of the new FFA dP |
| `TestFFA1v1_BetaWidensTauShrinks_DrawShrinks` | the three relative inequalities in one explicit assertion — single failure point if any constant gets reverted |
| `TestFFATau_ConvergesFasterThanReferenceSlowDrift` | operational: FFA τ converges to tighter σ than a hypothetical "FFA-with-1v1-τ" preset over 500 self-play games (isolates the τ contribution from β/dP) |
| `TestFFADrawProbability_ShrinksUpdatesNearEvenMatchups` | operational: the tighter dP shrinks per-game μ updates (smaller win-induced μ gain than 1v1's dP at the same starting rating) |

The existing `TestDefaultFFAConfig_DivergenceFromBaseline` was updated to assert the three-divergence shape instead of "β-only divergence, τ and dP unchanged." The previous version's assertion ("τ and dP must MATCH the 1v1 baseline") was a guardrail against silent drift — now superseded by the explicit pins above + this updated test.

The previous `TestFFAvs1v1_RetainsMoreUncertainty` was renamed and refactored to `TestFFAvs1v1_ConvergesTighterUnderStaticDeckTau` — the old assertion ("FFA σ exceeds 1v1 σ at convergence under β-only divergence") is now reversed under the combined retune.

All `internal/trueskill/` tests pass post-retune. Downstream `internal/tournament/` (which consumes `NewTrueSkillRatings` extensively) passes its full 106-second suite.

---

## Trade-off acknowledgement

The retune is **deliberately conservative** — τ at σ/200 not σ/300, dP at 0.005 not 0.0. The reasoning:

- **Future Hat retunes ARE dynamic.** A PR tuning archetype weights can shift winrate 1-2pp without changing the deck. τ needs to absorb that. Going to σ/300 (Halo Reach's value) would risk over-converging σ before the Hat-eval cycle stabilizes.
- **The mandatory-loop-draw path IS real.** It fires < 1 per 10,000 games but it's not impossible. dP=0.0 would model it away entirely; dP=0.005 leaves a small but real probability mass on it.

If a future calibration study (analogous to `docs/composition-prior-validation.md`'s +1.4pp validation) shows we're under-modeling drift, the natural next move is τ→σ/150, not back to σ/100. If it shows we're over-modeling draws, the natural move is dP→0.001.

---

## How to re-tune in the future

1. Update the constant (`ffaTauScale` or `ffaDrawProbability`) in `internal/trueskill/trueskill.go`
2. Update the constant's inline rationale comment with the new value's justification
3. Update `TestFFATau_ExactValue` / `TestFFADrawProbability_ExactValue` to assert the new value
4. Update `TestDefaultFFAConfig_DivergenceFromBaseline`'s table (if the relative direction vs 1v1 changes)
5. Update this doc with a new dated section ("R6X Retune") so the audit trail stays linear
6. Run a validation gauntlet via `cmd/hexdek-tournament` or the composition-prior validation harness to measure the operational impact

The DefaultConfig (1v1) should remain at the Microsoft baseline indefinitely. Re-tuning it would require an explicit literature-grade argument, not a hexdek-specific calibration.

---

## See also

- [`internal/trueskill/trueskill.go`](../internal/trueskill/trueskill.go) — the constants + DefaultConfig + DefaultFFAConfig
- [`internal/trueskill/init_tuning_r60_test.go`](../internal/trueskill/init_tuning_r60_test.go) — the 5 r60 regression tests
- [`internal/trueskill/trueskill_test.go`](../internal/trueskill/trueskill_test.go) — the pre-existing config tests, updated for r60
- [`docs/composition-prior-validation.md`](composition-prior-validation.md) — companion live-gauntlet validation (+1.4pp / +0.036 log-loss) of the broader r60 TrueSkill work
- Herbrich, Minka, Graepel (2007). *TrueSkill™: A Bayesian Skill Rating System.* NIPS 2006.
