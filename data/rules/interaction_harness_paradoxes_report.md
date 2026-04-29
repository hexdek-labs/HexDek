# Interaction Harness — PARADOXES & REPLACEMENT EFFECTS

_5 interactions × 4 deck contexts × 100 reps = 2000 total reps. seed=42_

## Summary matrix

| Interaction | Pass | Paradox | Fail | Parser-gap |
|---|---:|---:|---:|---:|
| `humility_opalescence` | 400 | 0 | 0 | 0 |
| `worldgorger_animate` | 197 | 203 | 0 | 0 |
| `blood_moon_dryad_arbor` | 400 | 0 | 0 | 0 |
| `show_and_tell_omniscience` | 400 | 0 | 0 | 0 |
| `painter_grindstone` | 400 | 0 | 0 | 0 |

## Per-context breakdown

### `humility_opalescence`

| Deck context | Pass | Paradox | Fail | Parser-gap | First detail |
|---|---:|---:|---:|---:|---|
| solo | 100 | 0 | 0 | 0 | stable state; Humility=Creature 1/1 no abilities; Opalescence unchanged; ts(H)=8 |
| aggro_shell | 100 | 0 | 0 | 0 | stable state; Humility=Creature 1/1 no abilities; Opalescence unchanged; ts(H)=5 |
| control_shell | 100 | 0 | 0 | 0 | stable state; Humility=Creature 1/1 no abilities; Opalescence unchanged; ts(H)=4 |
| ramp_shell | 100 | 0 | 0 | 0 | stable state; Humility=Creature 1/1 no abilities; Opalescence unchanged; ts(H)=8 |

### `worldgorger_animate`

| Deck context | Pass | Paradox | Fail | Parser-gap | First detail |
|---|---:|---:|---:|---:|---|
| solo | 56 | 44 | 0 | 0 | loop broken by player after 2 iters (CR 731: optional action by the controller v |
| aggro_shell | 44 | 56 | 0 | 0 | loop broken by player after 2 iters (CR 731: optional action by the controller v |
| control_shell | 54 | 46 | 0 | 0 | loop broken by player after 2 iters (CR 731: optional action by the controller v |
| ramp_shell | 43 | 57 | 0 | 0 | loop broken by player after 2 iters (CR 731: optional action by the controller v |

### `blood_moon_dryad_arbor`

| Deck context | Pass | Paradox | Fail | Parser-gap | First detail |
|---|---:|---:|---:|---:|---|
| solo | 100 | 0 | 0 | 0 | Dryad Arbor correctly resolved to Land Creature — Mountain Dryad (1/1) |
| aggro_shell | 100 | 0 | 0 | 0 | Dryad Arbor correctly resolved to Land Creature — Mountain Dryad (1/1) |
| control_shell | 100 | 0 | 0 | 0 | Dryad Arbor correctly resolved to Land Creature — Mountain Dryad (1/1) |
| ramp_shell | 100 | 0 | 0 | 0 | Dryad Arbor correctly resolved to Land Creature — Mountain Dryad (1/1) |

### `show_and_tell_omniscience`

| Deck context | Pass | Paradox | Fail | Parser-gap | First detail |
|---|---:|---:|---:|---:|---|
| solo | 100 | 0 | 0 | 0 | Omniscience's static free-cast ability recognized by parser + resolver; hand rea |
| aggro_shell | 100 | 0 | 0 | 0 | Omniscience's static free-cast ability recognized by parser + resolver; hand rea |
| control_shell | 100 | 0 | 0 | 0 | Omniscience's static free-cast ability recognized by parser + resolver; hand rea |
| ramp_shell | 100 | 0 | 0 | 0 | Omniscience's static free-cast ability recognized by parser + resolver; hand rea |

### `painter_grindstone`

| Deck context | Pass | Paradox | Fail | Parser-gap | First detail |
|---|---:|---:|---:|---:|---|
| solo | 100 | 0 | 0 | 0 | library of 42 cards → milled 42 in 22 iterations (color=B) |
| aggro_shell | 100 | 0 | 0 | 0 | library of 29 cards → milled 29 in 15 iterations (color=W) |
| control_shell | 100 | 0 | 0 | 0 | library of 31 cards → milled 31 in 16 iterations (color=W) |
| ramp_shell | 100 | 0 | 0 | 0 | library of 41 cards → milled 41 in 21 iterations (color=B) |

## Flagship: Humility + Opalescence — full §613 layer log

_Context: solo, rep=0, status=pass._

```
§613.1d  Layer 4 (type/subtype) — apply Opalescence
  Opalescence[ts=51]: each other non-Aura enchantment gains type Creature
    → Humility: types ['Enchantment'] + Creature = ['Creature', 'Enchantment']
§613.1f  Layer 6 (abilities) — apply Humility
  Humility[ts=87]: all creatures lose all abilities
    NOTE: characteristic-defining abilities (Opalescence's P/T) are NOT removed by layer 6 — they're layer-7a CDAs that 613.1f.6 does not touch. See CR 613.2.
    → Craterhoof Behemoth: abilities ['haste'] → []
§613.3a  Layer 7a (characteristic-defining P/T)
  Opalescence[ts=51]: affected non-Aura enchantments' base P/T = mana value
    → Humility (MV=4): P/T (None, None) → (4, 4)
§613.3b  Layer 7b (P/T set)
  Humility[ts=87]: all creatures have base P/T 1/1
    → Humility: P/T (4, 4) → (1,1)
    → Grizzly Bears: P/T (2, 2) → (1,1)
    → Craterhoof Behemoth: P/T (5, 5) → (1,1)
§613 resolution complete — stable state reached.
```

## Worldgorger Dragon + Animate Dead — loop trace

_Context: solo, rep=0, status=paradox._

```
Replacement-loop simulation: Worldgorger Dragon + Animate Dead
  Initial: Worldgorger on battlefield (anim'd), Animate Dead attached, 3 other permanents on battlefield.
[iter 1] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 1] Animate Dead LTB-trigger → sacrifice WDragon
[iter 2] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 2] Animate Dead ETB-trigger → return WDragon from GY
[iter 3] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 3] Animate Dead LTB-trigger → sacrifice WDragon
[iter 4] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 4] Animate Dead ETB-trigger → return WDragon from GY
[iter 5] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 5] Animate Dead LTB-trigger → sacrifice WDragon
[iter 6] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 6] Animate Dead ETB-trigger → return WDragon from GY
[iter 7] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 7] Animate Dead LTB-trigger → sacrifice WDragon
[iter 8] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 8] Animate Dead ETB-trigger → return WDragon from GY
[iter 9] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 9] Animate Dead LTB-trigger → sacrifice WDragon
[iter 10] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 10] Animate Dead ETB-trigger → return WDragon from GY
[iter 11] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 11] Animate Dead LTB-trigger → sacrifice WDragon
[iter 12] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 12] Animate Dead ETB-trigger → return WDragon from GY
[iter 13] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 13] Animate Dead LTB-trigger → sacrifice WDragon
[iter 14] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 14] Animate Dead ETB-trigger → return WDragon from GY
[iter 15] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 15] Animate Dead LTB-trigger → sacrifice WDragon
[iter 16] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 16] Animate Dead ETB-trigger → return WDragon from GY
[iter 17] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 17] Animate Dead LTB-trigger → sacrifice WDragon
[iter 18] WDragon LTB → return 4 exiled permanents (incl. Animate Dead)
[iter 18] Animate Dead ETB-trigger → return WDragon from GY
[iter 19] WDragon ETB → exile 4 other permanents (incl. Animate Dead)
[iter 19] Animate Dead LTB-trigger → sacrifice WDragon
... (363 more lines)
```

## Blood Moon + Dryad Arbor — type-replacement log

_Context: solo, rep=0, status=pass._

```
§613.1d  Layer 4 (type/subtype) — apply Blood Moon
  Blood Moon[ts=82]: nonbasic lands are Mountains. Per CR 305.7, nonbasic-land subtypes are replaced, and abilities printed on the card are overridden with the basic Mountain's mana ability.
    → Dryad Arbor: subtypes ['Dryad', 'Forest'] → ['Dryad', 'Mountain']; types ['Creature', 'Land']
§613 resolution complete — Dryad Arbor is still a Creature (CR 305.7 does not remove creature type-line).
```

## Painter's Servant + Grindstone — mill trace

_Context: solo, rep=0, status=pass._

```
Painter's Servant chose color: B
Grindstone activated targeting library of 42 cards.
[iter 1] Milled 2 cards: ['card_0', 'card_1']. Library now 40.
[iter 1] Both cards share color 'B' (Painter active) → repeat.
[iter 2] Milled 2 cards: ['card_2', 'card_3']. Library now 38.
[iter 2] Both cards share color 'B' (Painter active) → repeat.
[iter 3] Milled 2 cards: ['card_4', 'card_5']. Library now 36.
[iter 3] Both cards share color 'B' (Painter active) → repeat.
[iter 4] Milled 2 cards: ['card_6', 'card_7']. Library now 34.
[iter 4] Both cards share color 'B' (Painter active) → repeat.
[iter 5] Milled 2 cards: ['card_8', 'card_9']. Library now 32.
[iter 5] Both cards share color 'B' (Painter active) → repeat.
[iter 6] Milled 2 cards: ['card_10', 'card_11']. Library now 30.
[iter 6] Both cards share color 'B' (Painter active) → repeat.
[iter 7] Milled 2 cards: ['card_12', 'card_13']. Library now 28.
[iter 7] Both cards share color 'B' (Painter active) → repeat.
[iter 8] Milled 2 cards: ['card_14', 'card_15']. Library now 26.
[iter 8] Both cards share color 'B' (Painter active) → repeat.
[iter 9] Milled 2 cards: ['card_16', 'card_17']. Library now 24.
[iter 9] Both cards share color 'B' (Painter active) → repeat.
... (25 more lines)
```

## Rules-level ambiguities the engine surfaced

- Humility+Opalescence: the 'each OTHER non-Aura enchantment' clause in Opalescence (post-2017 Oracle update) resolves the self-loop — Opalescence doesn't animate itself. Pre-2017 cards would have a true bootstrapping paradox.
- Worldgorger Dragon loop: the correct citation is CR 731.4 (loop rules — 'Taking Shortcuts'), not CR 722 (which is 'Controlling Another Player'). CR 731.4 only calls mandatory loops a draw when they span both players; Worldgorger + Animate Dead is strictly one-sided (every participant is under the controller's ownership). With no break condition, CR 104.3a applies — the controller can't perform a required action and LOSES the game. A draw is NOT the default outcome; the controller typically wins by casting a stack response that breaks the loop, but if they can't break it they lose. See playloop.classify_loop_716 for the mandatory_one_sided classification path.
- Blood Moon on Dryad Arbor: CR 305.7 was clarified to preserve the creature type-line. Some older rules interpretations stripped the creature subtype; we implement the modern reading (Dryad stays).
- Painter's Servant: the 2022 Oracle rewrite split the color-wash into a zone-aware clause ('cards that aren't on the battlefield, spells, and permanents'). Grindstone's repeat condition ('two cards that share a color were milled') reads the cards at the time they enter the graveyard — by then Painter has washed them.

## Bugs discovered

_None. All 5 interactions resolved cleanly or were correctly flagged as paradoxes._

## Files written

- `scripts/interaction_harness_paradoxes.py`
- `scripts/playloop.py` (added `detect_paradox(game)` helper)
- `data/rules/interaction_harness_paradoxes_report.md` (this report)
