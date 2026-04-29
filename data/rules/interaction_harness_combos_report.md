# Interaction Harness — Canonical Win-Condition Combos

_Each interaction run against 4 deck contexts × 100 iterations = 400 runs per combo, 2000 total runs._

## Summary

| Interaction | Runs | Pass (win) | Pass (paradox) | Fail | Crash | Skip (parser) |
|---|---:|---:|---:|---:|---:|---:|
| thoracle_consult | 400 | 400 | 0 | 0 | 0 | 0 |
| thoracle_pact | 800 | 0 | 0 | 0 | 0 | 400 |
| labman_brainstorm | 400 | 400 | 0 | 0 | 0 | 0 |
| jace_wielder_ult | 400 | 400 | 0 | 0 | 0 | 0 |
| painter_grindstone | 400 | 400 | 0 | 0 | 0 | 0 |

## Sub-results (interaction × deck context)

| Interaction | Context | Runs | Pass (win) | Pass (paradox) | Fail | Crash | Skip | Notes |
|---|---|---:|---:|---:|---:|---:|---:|---|
| thoracle_consult | Burn | 100 | 100 | 0 | 0 | 0 | 0 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_consult | Control | 100 | 100 | 0 | 0 | 0 | 0 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_consult | Creatures | 100 | 100 | 0 | 0 | 0 | 0 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_consult | Ramp | 100 | 100 | 0 | 0 | 0 | 0 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_pact | Burn | 100 | 0 | 0 | 0 | 0 | 100 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_pact | Control | 100 | 0 | 0 | 0 | 0 | 100 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_pact | Creatures | 100 | 0 | 0 | 0 | 0 | 100 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| thoracle_pact | Ramp | 100 | 0 | 0 | 0 | 0 | 100 | Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or  |
| labman_brainstorm | Burn | 100 | 100 | 0 | 0 | 0 | 0 | Laboratory Maniac: Mod(if_intervening_tail) args=('you would draw a ca |
| labman_brainstorm | Control | 100 | 100 | 0 | 0 | 0 | 0 | Laboratory Maniac: Mod(if_intervening_tail) args=('you would draw a ca |
| labman_brainstorm | Creatures | 100 | 100 | 0 | 0 | 0 | 0 | Laboratory Maniac: Mod(if_intervening_tail) args=('you would draw a ca |
| labman_brainstorm | Ramp | 100 | 100 | 0 | 0 | 0 | 0 | Laboratory Maniac: Mod(if_intervening_tail) args=('you would draw a ca |
| jace_wielder_ult | Burn | 100 | 100 | 0 | 0 | 0 | 0 | Jace, Wielder of Mysteries: Mod(if_intervening_tail) args=('you would  |
| jace_wielder_ult | Control | 100 | 100 | 0 | 0 | 0 | 0 | Jace, Wielder of Mysteries: Mod(if_intervening_tail) args=('you would  |
| jace_wielder_ult | Creatures | 100 | 100 | 0 | 0 | 0 | 0 | Jace, Wielder of Mysteries: Mod(if_intervening_tail) args=('you would  |
| jace_wielder_ult | Ramp | 100 | 100 | 0 | 0 | 0 | 0 | Jace, Wielder of Mysteries: Mod(if_intervening_tail) args=('you would  |
| painter_grindstone | Burn | 100 | 100 | 0 | 0 | 0 | 0 | Painter's Servant: Triggered(etb) → UnknownEffect: '<per-card:painters |
| painter_grindstone | Control | 100 | 100 | 0 | 0 | 0 | 0 | Painter's Servant: Triggered(etb) → UnknownEffect: '<per-card:painters |
| painter_grindstone | Creatures | 100 | 100 | 0 | 0 | 0 | 0 | Painter's Servant: Triggered(etb) → UnknownEffect: '<per-card:painters |
| painter_grindstone | Ramp | 100 | 100 | 0 | 0 | 0 | 0 | Painter's Servant: Triggered(etb) → UnknownEffect: '<per-card:painters |

## Parser / engine gaps blocking coverage

### thoracle_consult

- Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or equal to the number of cards in your library', 'you win the game')
- Demonic Consultation: spell_effect→Mod(parsed_effect_residual) args=('choose:card_name',)
- Demonic Consultation: spell_effect→Mod(parsed_effect_residual) args=('exile the top six cards of your library, then reveal cards from the top of you
- Demonic Consultation: spell_effect→Mod(parsed_effect_residual) args=('put that card into your hand and exile all other cards revealed this way',)

### thoracle_pact

- Thassa's Oracle: Mod(if_intervening_tail) args=('x is greater than or equal to the number of cards in your library', 'you win the game')
- Tainted Pact: spell_effect→Mod(exile_top_library) args=(1,)
- Tainted Pact: spell_effect→Mod(parsed_effect_residual) args=('you may put that card into your hand unless it has the same name as another ca
- Tainted Pact: spell_effect→Mod(parsed_effect_residual) args=('repeat this process',)

### labman_brainstorm

- Laboratory Maniac: Mod(if_intervening_tail) args=('you would draw a card while your library has no cards in it', 'you win the game instead')

### jace_wielder_ult

- Jace, Wielder of Mysteries: Mod(if_intervening_tail) args=('you would draw a card while your library has no cards in it', 'you win the game instead')
- Jace, Wielder of Mysteries: Activated loyalty cost '+1' — engine has no loyalty tracking
- Jace, Wielder of Mysteries: Activated loyalty cost '−8' — engine has no loyalty tracking

### painter_grindstone

- Painter's Servant: Triggered(etb) → UnknownEffect: '<per-card:painters_servant_choose_color>'
- Painter's Servant: Mod(custom) args=('painters_servant_color_wash',)
- Grindstone: Mod(if_intervening_tail) args=('two cards that share a color were milled this way', 'repeat this process')

## Files

- Harness: `scripts/interaction_harness_combos.py`
- Helpers added to `scripts/playloop.py` (`setup_board_state`, `run_scripted_sequence`)
- This report: `data/rules/interaction_harness_combos_report.md`
