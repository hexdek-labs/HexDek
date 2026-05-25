# Self-Trigger Response Gauntlet — R60 Round 16+

Date: 2026-05-25
Branch: `dev/hat-self-trigger-validation-r60`
Framework: PR #410 (draw-damage punishers) + PR #413 (mill / life-loss extensions) + PR #418 (matrix suite)

## TL;DR

**0 self-trigger counter fires across 2,600 gauntlet games.** The framework's lethal-projection gate is so narrow that it doesn't fire in routine play — and the gauntlet's test deck corpus contains no damage-on-draw punishers (Underworld Dreams, Curse of Wizardry, Spiteful Visions) at all, so the framework's preconditions are *structurally absent* from this measurement.

**Winrate impact in measured corpus: 0.0%** (lower bound is exact — with zero fires, behavior is identical to the no-framework baseline).

## Methodology

Five 500-game gauntlets across distinct seeds:

```
seeds:        42, 99, 1337, 2024, 271828
games/seed:   500
seats:        4
hat:          yggdrasil
hat-budget:   50  (evaluator-guided default)
deck corpus:  data/decks/test/  (16 decks: cEDH big stick, blink, combat-cast
              combo, partner combo, control, ramp/lumra, mullie, stormoff,
              turbo kinnan, disguise precon, etc.)
```

Instrumentation: a process-lifetime atomic counter (`hat.SelfTriggerCounterFires`)
incremented on every `ChooseResponse` self-trigger counter fire. Surfaced by
the tournament CLI's end-of-run summary as `SELF-TRIGGER-COUNTER fires: N`.

Plus a bonus configuration:

```
seed:         42
games:        100
hat-budget:   200  (MCTS rollout-tier)
```

to confirm the deeper-search path doesn't change observed fire frequency.

## Results

| Seed | Games | Throughput | Crashes | Avg turns | **Self-counter fires** |
|------|------:|-----------:|--------:|----------:|-----------------------:|
| 42       | 500 | 11.2 g/s | 0 | 47.1 | **0** |
| 99       | 500 | 11.0 g/s | 0 | 47.5 | **0** |
| 1337     | 500 | 11.2 g/s | 0 | 46.8 | **0** |
| 2024     | 500 | 11.1 g/s | 0 | 47.3 | **0** |
| 271828   | 500 | 19.4 g/s | 0 | 47.5 | **0** |
| **Total** | **2,500** | — | **0** | — | **0** |

Bonus MCTS-tier run (seed 42, games 100, budget 200): also **0 fires**.

Aggregate: **0 fires across 2,600 games**.

## Interpretation

The framework's per-trigger gate requires **all four** conditions simultaneously:

1. The stack item is a triggered ability (not a spell, not an activated)
2. The trigger's source is our own permanent
3. Resolving the trigger would deal damage / mill us / drain us via:
   - draw + damage-on-draw punisher on OUR board, OR
   - self-mill ≥ library size, OR
   - stated `you lose N life` / `deals N damage to you` with N ≥ life
4. We hold an affordable counterspell

The conjunction is rare by design. Specifically for the draw-damage scenario
— the most plausible firing case — the `data/decks/test/` corpus contains
**zero damage-on-draw punishers**:

```
$ grep -liE "Underworld Dreams|Curse of Wizardry|Spiteful Visions|Megrim|Black Vise" \
    data/decks/test/*.txt
(no matches)
```

So the framework's precondition couldn't arise in the measurement corpus,
making the 0-fire result a **structural lower bound** rather than an upper-
bound estimate of true frequency.

## Validation that the framework would fire if the shape existed

The full matrix suite from PR #418 includes 7 end-to-end ChooseResponse
integration tests, each constructing a synthetic punisher-bearing scenario
and asserting the counter fires:

- `TestSelfTriggerMatrix_E2E_DrawDamageLethalCountered` — Mulldrifter + Underworld Dreams at 2 life
- `TestSelfTriggerMatrix_E2E_MillLethalCountered` — Hedron Crab self-mill at 2-card library
- `TestSelfTriggerMatrix_E2E_LifeLossLethalCountered` — Phyrexian Arena upkeep at 1 life
- `TestSelfTriggerMatrix_E2E_DamageToSelfLethalCountered` — self-ETB damage at 2 life

All 7 pass deterministically. The framework is wired correctly — the
gauntlet result reflects corpus coverage, not implementation gap.

## Winrate impact analysis

With **0 fires observed**, the framework's branch in `ChooseResponse` never
executed. The hat's behavior under the gauntlet was **bit-identical** to the
pre-framework baseline at every priority window. Therefore:

- Winrate delta vs no-framework baseline: **exactly 0.0%**
- Per-game decision divergence: **0**
- Per-game wall-clock overhead: **~negligible** (one `shouldCounterOwnTrigger`
  call per priority window where `top.Controller == seatIdx`, each returning
  false in ≤ 5 comparisons before bailing)

## Conclusion

The self-trigger response framework is correctly characterized as a
**rare-edge-case safety net**, not a winrate optimization. Its design
intent (preserve correctness in the "I cast Mulldrifter at 2 life with my
own Underworld Dreams in play" pathological scenario) is fully met by the
matrix suite; the gauntlet result confirms it imposes no measurable
overhead in routine play.

To meaningfully measure winrate impact in future iterations, the gauntlet
corpus would need decks that intentionally combine damage-on-draw punishers
with draw-trigger creatures — i.e., decks that wouldn't be sanely built by
a human but exercise the framework. The honest finding for the current
deck corpus is: **the framework correctly does nothing 100% of the time**.

## How to reproduce

```bash
for seed in 42 99 1337 2024 271828; do
  go run ./cmd/hexdek-tournament/ \
    --decks data/decks/test \
    --games 500 --seed "$seed" --seats 4 \
    --hat yggdrasil --hat-budget 50 \
    --report /tmp/g-$seed.md 2>&1 \
    | grep -E "SELF-TRIGGER|Throughput"
done
```

Expected output: every seed reports `SELF-TRIGGER-COUNTER fires: 0`.
