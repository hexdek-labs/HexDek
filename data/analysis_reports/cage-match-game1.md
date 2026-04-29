# Game `cage-match-game1` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 1 (`Kaust, Eyes of the Glade`)
- **End reason**: seat 1 is the last one standing
- **Turns played**: 18
- **Events captured**: 1899
- **Findings**: 0 CRITICAL / 4 WARN / 7 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Coram, the Undertaker | recursion_graveyard | -1 | LOST (combat damage) |
| 1 | Kaust, Eyes of the Glade | unknown | 58 | **WINNER** |
| 2 | Oloro, Ageless Ascetic | storm | -4 | LOST (combat damage) |
| 3 | Varina, Lich Queen | recursion_graveyard | -1 | LOST (combat damage) |

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'angels_grace_cant_lose_or_win' fired 1 times

   - Seats: `—`, Turns: `5`
   - group_key: `unhandled::angels_grace_cant_lose_or_win`
   - Event seqs: `295`

   <details><summary>Details</summary>

   - `slug`: `angels_grace_cant_lose_or_win`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
2. **[Per-card handler missing]** per_card_unhandled for slug 'angels_grace_damage_floor_1' fired 1 times

   - Seats: `—`, Turns: `5`
   - group_key: `unhandled::angels_grace_damage_floor_1`
   - Event seqs: `296`

   <details><summary>Details</summary>

   - `slug`: `angels_grace_damage_floor_1`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
3. **[Per-card handler missing]** per_card_unhandled for slug 'stunning_reversal' fired 1 times

   - Seats: `—`, Turns: `17`
   - group_key: `unhandled::stunning_reversal`
   - Event seqs: `1760`

   <details><summary>Details</summary>

   - `slug`: `stunning_reversal`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
4. **[Storm deck never fires]** Seat 2 (Oloro, Ageless Ascetic) has storm payoffs ['Aetherflux Reservoir'] but never cast them (game had 43 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `payoffs_present`: `['Aetherflux Reservoir']`
   - `payoffs_never_cast`: `['Aetherflux Reservoir']`
   - `total_casts`: `43`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 2 (Oloro, Ageless Ascetic) never cast their commander across 18 turns

   - Seats: `2`, Turns: `—`
   - group_key: `cmd_never_cast::Oloro, Ageless Ascetic`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `total_turns`: `18`

   </details>
2. **[Dead in hand]** Seat 0 ended with 5 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `5`
   - `dead_cards`: `['Kulrath Zealot', 'The Balrog of Moria', 'Too Greedily, Too Deep', 'Final Act', 'Moldgraf Monstrosity']`

   </details>
3. **[Dead in hand]** Seat 1 ended with 1 cards in hand that were never cast

   - Seats: `1`, Turns: `—`
   - group_key: `dead_hand::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `hand_size`: `1`
   - `dead_cards`: `['Akroma, Angel of Fury']`

   </details>
4. **[Dead in hand]** Seat 2 ended with 4 cards in hand that were never cast

   - Seats: `2`, Turns: `—`
   - group_key: `dead_hand::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `hand_size`: `4`
   - `dead_cards`: `['Felidar Sovereign', 'Aetherflux Reservoir', 'Beacon of Immortality', 'Generous Gift']`

   </details>
5. **[Dead in hand]** Seat 3 ended with 3 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `3`
   - `dead_cards`: `["Lich's Mastery", 'Army of the Damned', 'Zombie Apocalypse']`

   </details>
6. **[Mana floated]** Seat 1 floated ≥3 mana at phase boundary for 4 turns

   - Seats: `1`, Turns: `12, 15-17`
   - group_key: `mana_float::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `float_count`: `4`
   - `total_wasted`: `21`

   </details>
7. **[Mana floated]** Seat 3 floated ≥3 mana at phase boundary for 3 turns

   - Seats: `3`, Turns: `11-12, 14`
   - group_key: `mana_float::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `float_count`: `3`
   - `total_wasted`: `11`

   </details>

## Per-seat overview

### Seat 0: Coram, the Undertaker

- Final life: **-1**, hand=5, library=72, graveyard=9
- Commander damage taken: 16 from seat 3 (Varina, Lich Queen); 4 from seat 1 (Kaust, Eyes of the Glade)
- Commander casts: Coram, the Undertaker×1

### Seat 1: Kaust, Eyes of the Glade

- Final life: **58**, hand=1, library=75, graveyard=3
- Battlefield (21): Ashcloud Phoenix, Exotic Orchard, Forest, Furycalm Snarl, Jungle Shrine, Kaust, Eyes of the Glade, Kessig Wolf Run, Lifecrafter's Bestiary, Master of Pearls, Mountain, Ohran Frostfang, Plains, Plains, Ransom Note, Sakura-Tribe Elder, Salt Road Ambushers, Scattered Groves, Shrine of the Forsaken Gods, Sidar Kondo of Jamuraa, Tesak, Judith's Hellhound, Ugin's Mastery
- Commander casts: Kaust, Eyes of the Glade×2

### Seat 2: Oloro, Ageless Ascetic

- Final life: **-4**, hand=4, library=83, graveyard=7
- Commander damage taken: 8 from seat 1 (Kaust, Eyes of the Glade)
- Commander casts: Oloro, Ageless Ascetic×0

### Seat 3: Varina, Lich Queen

- Final life: **-1**, hand=3, library=78, graveyard=6
- Commander damage taken: 4 from seat 1 (Kaust, Eyes of the Glade)
- Commander casts: Varina, Lich Queen×2

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 0 |  |
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
| `per_card_unhandled` | 3 |  |
| `storm_deck_never_fires` | 1 |  |
| `ramp_deck_no_treasure` | 0 |  |
| `combo_pieces_stranded` | 0 |  |
| `muldrotha_no_gy_cast` | 0 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 4 |  |
| `mana_floated` | 2 |  |
| `commander_never_cast` | 1 |  |
| `untapped_artifact_never_activated` | 0 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
