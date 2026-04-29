# Game `cage-match-game2` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 0 (`Coram, the Undertaker`)
- **End reason**: seat 0 is the last one standing
- **Turns played**: 18
- **Events captured**: 2099
- **Findings**: 1 CRITICAL / 2 WARN / 5 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Coram, the Undertaker | recursion_graveyard | 39 | **WINNER** |
| 1 | Kaust, Eyes of the Glade | unknown | -2 | LOST (combat damage) |
| 2 | Oloro, Ageless Ascetic | storm | 0 | LOST (combat damage) |
| 3 | Varina, Lich Queen | recursion_graveyard | -1 | LOST (combat damage) |

## CRITICAL flags

1. **[Artifact mana inertness]** Arcane Signet on seat 1 generated no mana for 7 turn(s)

   - Seats: `1`, Turns: `10-16`
   - group_key: `artifact_inert::Arcane Signet`

   <details><summary>Details</summary>

   - `artifact`: `Arcane Signet`
   - `expected`: `any1`
   - `present_range`: `[10, None]`
   - `inert_turns`: `[10, 11, 12, 13, 14, 15, 16]`

   </details>

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'debt_deathless_each_opp_2x_drain_you_gain' fired 1 times

   - Seats: `—`, Turns: `10`
   - group_key: `unhandled::debt_deathless_each_opp_2x_drain_you_gain`
   - Event seqs: `941`

   <details><summary>Details</summary>

   - `slug`: `debt_deathless_each_opp_2x_drain_you_gain`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
2. **[Storm deck never fires]** Seat 2 (Oloro, Ageless Ascetic) has storm payoffs ['Aetherflux Reservoir'] but never cast them (game had 51 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `payoffs_present`: `['Aetherflux Reservoir']`
   - `payoffs_never_cast`: `['Aetherflux Reservoir']`
   - `total_casts`: `51`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 3 (Varina, Lich Queen) never cast their commander across 18 turns

   - Seats: `3`, Turns: `—`
   - group_key: `cmd_never_cast::Varina, Lich Queen`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `commander`: `Varina, Lich Queen`
   - `total_turns`: `18`

   </details>
2. **[Dead in hand]** Seat 1 ended with 3 cards in hand that were never cast

   - Seats: `1`, Turns: `—`
   - group_key: `dead_hand::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `hand_size`: `3`
   - `dead_cards`: `['Showstopping Surprise', 'Austere Command', 'Temur War Shaman']`

   </details>
3. **[Dead in hand]** Seat 2 ended with 3 cards in hand that were never cast

   - Seats: `2`, Turns: `—`
   - group_key: `dead_hand::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `hand_size`: `3`
   - `dead_cards`: `['Angelic Chorus', 'Approach of the Second Sun', 'Skullclamp']`

   </details>
4. **[Dead in hand]** Seat 3 ended with 7 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `7`
   - `dead_cards`: `["Lich's Mastery", 'Living Death', "Necromancer's Covenant", 'Raise the Palisade', 'Undead Warchief', 'Endless Ranks of the Dead', 'Vindictive Lich']`

   </details>
5. **[Mana floated]** Seat 1 floated ≥3 mana at phase boundary for 3 turns

   - Seats: `1`, Turns: `10, 14, 16`
   - group_key: `mana_float::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `float_count`: `3`
   - `total_wasted`: `11`

   </details>

## Per-seat overview

### Seat 0: Coram, the Undertaker

- Final life: **39**, hand=0, library=70, graveyard=12
- Battlefield (17): Blood Crypt, Coram, the Undertaker, Dakmor Salvage, Forest, Kulrath Zealot, Lightning Greaves, Moldgraf Monstrosity, Mosswort Bridge, Mountain, Perpetual Timepiece, Purple Worm, Sol Ring, Swamp, Talisman of Impulse, The Balrog of Moria, Treasure Vault, Troll of Khazad-dûm
- Commander casts: Coram, the Undertaker×2

### Seat 1: Kaust, Eyes of the Glade

- Final life: **-2**, hand=3, library=76, graveyard=4
- Commander casts: Kaust, Eyes of the Glade×3

### Seat 2: Oloro, Ageless Ascetic

- Final life: **0**, hand=3, library=73, graveyard=7
- Commander damage taken: 10 from seat 1 (Kaust, Eyes of the Glade)
- Commander casts: Oloro, Ageless Ascetic×1

### Seat 3: Varina, Lich Queen

- Final life: **-1**, hand=7, library=72, graveyard=11
- Commander casts: Varina, Lich Queen×0

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 1 |  |
| `storm_copies_empty` | 0 |  |
| `bolas_citadel_no_life_change` | 0 |  |
| `reactive_trigger_missed` | 0 |  |
| `draw_reactive_trigger_missed` | 0 |  |
| `commander_damage_sba_miss` | 0 |  |
| `thoracle_no_win` | 0 |  |
| `mana_pool_persists_across_phase` | 0 |  |
| `winner_consistency` | 0 |  |
| `crashed_effects` | 0 |  |
| `cap_hits` | 0 |  |
| `cast_failed_spam` | 0 |  |
| `per_card_unhandled` | 1 |  |
| `storm_deck_never_fires` | 1 |  |
| `ramp_deck_no_treasure` | 0 |  |
| `combo_pieces_stranded` | 0 |  |
| `muldrotha_no_gy_cast` | 0 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 3 |  |
| `mana_floated` | 1 |  |
| `commander_never_cast` | 1 |  |
| `untapped_artifact_never_activated` | 0 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
