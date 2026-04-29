# Game `cage-v2-game2` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 2 (`Sin, Spira's Punishment`)
- **End reason**: seat 2 is the last one standing
- **Turns played**: 15
- **Events captured**: 1693
- **Findings**: 0 CRITICAL / 1 WARN / 7 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Coram, the Undertaker | recursion_graveyard | -3 | LOST (combat damage) |
| 1 | Kaust, Eyes of the Glade | unknown | -2 | LOST (combat damage) |
| 2 | Sin, Spira's Punishment | recursion_graveyard | 19 | **WINNER** |
| 3 | Varina, Lich Queen | recursion_graveyard | 14 | LOST (21+ commander damage from Sin, Spira's Punishment (CR 704.6c)) |

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'swallowed_by_leviathan' fired 1 times

   - Seats: `—`, Turns: `5`
   - group_key: `unhandled::swallowed_by_leviathan`
   - Event seqs: `363`

   <details><summary>Details</summary>

   - `slug`: `swallowed_by_leviathan`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 3 (Varina, Lich Queen) never cast their commander across 15 turns

   - Seats: `3`, Turns: `—`
   - group_key: `cmd_never_cast::Varina, Lich Queen`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `commander`: `Varina, Lich Queen`
   - `total_turns`: `15`

   </details>
2. **[Dead in hand]** Seat 0 ended with 4 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `4`
   - `dead_cards`: `['The Balrog of Moria', 'Moldgraf Monstrosity', 'Purple Worm', 'Troll of Khazad-dûm']`

   </details>
3. **[Dead in hand]** Seat 1 ended with 3 cards in hand that were never cast

   - Seats: `1`, Turns: `—`
   - group_key: `dead_hand::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `hand_size`: `3`
   - `dead_cards`: `['Showstopping Surprise', 'Austere Command', 'Temur War Shaman']`

   </details>
4. **[Dead in hand]** Seat 3 ended with 6 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `6`
   - `dead_cards`: `['Diregraf Captain', "Lich's Mastery", 'Living Death', 'Diregraf Colossus', 'Fatestitcher', "Necromancer's Covenant"]`

   </details>
5. **[Mana floated]** Seat 1 floated ≥3 mana at phase boundary for 3 turns

   - Seats: `1`, Turns: `10, 12, 15`
   - group_key: `mana_float::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `float_count`: `3`
   - `total_wasted`: `9`

   </details>
6. **[Mana floated]** Seat 2 floated ≥3 mana at phase boundary for 5 turns

   - Seats: `2`, Turns: `8-9, 13-15`
   - group_key: `mana_float::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `float_count`: `5`
   - `total_wasted`: `32`

   </details>
7. **[Simultaneous SBA (cmdr damage + life)]** Seat 0 took 21 commander damage from Sin, Spira's Punishment AND life total reached ≤0 in the same damage batch. Both §704.5a and §704.6c apply simultaneously per CR §704.3. Engine recorded §704.5a as loss_reason; this is a valid display choice, not a bug.

   - Seats: `0`, Turns: `—`
   - group_key: `cmd_dmg_simultaneous::Sin, Spira's Punishment`

   <details><summary>Details</summary>

   - `victim`: `0`
   - `dealer`: `2`
   - `commander`: `Sin, Spira's Punishment`
   - `damage`: `21`
   - `simultaneous_with_life_loss`: `True`
   - `rule`: `704.3 — simultaneous SBAs all apply`

   </details>

## Per-seat overview

### Seat 0: Coram, the Undertaker

- Final life: **-3**, hand=4, library=75, graveyard=10
- Commander damage taken: 21 from seat 2 (Sin, Spira's Punishment)
- Commander casts: Coram, the Undertaker×1

### Seat 1: Kaust, Eyes of the Glade

- Final life: **-2**, hand=3, library=77, graveyard=4
- Commander damage taken: 14 from seat 2 (Sin, Spira's Punishment)
- Commander casts: Kaust, Eyes of the Glade×3

### Seat 2: Sin, Spira's Punishment

- Final life: **19**, hand=0, library=77, graveyard=6
- Battlefield (17): Alchemist's Refuge, Awaken the Honored Dead, Breeding Pool, Colossal Grave-Reaver, Command Tower, Deadeye Navigator, Endless Sands, Foreboding Landscape, Forest, Horizon Explorer, Icetill Explorer, Island, Island, Island, Simic Growth Chamber, Sin, Spira's Punishment, Woodland Cemetery
- Commander damage taken: 4 from seat 1 (Kaust, Eyes of the Glade)
- Commander casts: Sin, Spira's Punishment×1

### Seat 3: Varina, Lich Queen

- Final life: **14**, hand=6, library=80, graveyard=6
- Commander damage taken: 21 from seat 2 (Sin, Spira's Punishment)
- Commander casts: Varina, Lich Queen×0

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 0 |  |
| `storm_copies_empty` | 0 |  |
| `bolas_citadel_no_life_change` | 0 |  |
| `reactive_trigger_missed` | 0 |  |
| `draw_reactive_trigger_missed` | 0 |  |
| `commander_damage_sba_miss` | 1 |  |
| `thoracle_no_win` | 0 |  |
| `mana_pool_persists_across_phase` | 0 |  |
| `winner_consistency` | 0 |  |
| `crashed_effects` | 0 |  |
| `cap_hits` | 0 |  |
| `cast_failed_spam` | 0 |  |
| `per_card_unhandled` | 1 |  |
| `storm_deck_never_fires` | 0 |  |
| `ramp_deck_no_treasure` | 0 |  |
| `combo_pieces_stranded` | 0 |  |
| `muldrotha_no_gy_cast` | 0 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 3 |  |
| `mana_floated` | 2 |  |
| `commander_never_cast` | 1 |  |
| `untapped_artifact_never_activated` | 0 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
