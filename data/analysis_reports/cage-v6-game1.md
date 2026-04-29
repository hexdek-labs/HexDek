# Game `cage-v6-game1` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 1 (`Coram, the Undertaker`)
- **End reason**: seat 1 is the last one standing
- **Turns played**: 18
- **Events captured**: 2448
- **Findings**: 2 CRITICAL / 4 WARN / 5 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Ashling, Rekindled // Ashling, Rimebound | recursion_graveyard | -2 | LOST (combat damage) |
| 1 | Coram, the Undertaker | recursion_graveyard | 40 | **WINNER** |
| 2 | Oloro, Ageless Ascetic | storm | 0 | LOST (combat damage) |
| 3 | Varina, Lich Queen | recursion_graveyard | -7 | LOST (combat damage) |

## CRITICAL flags

1. **[Artifact mana inertness]** Talisman of Hierarchy on seat 3 generated no mana for 1 turn(s)

   - Seats: `3`, Turns: `7`
   - group_key: `artifact_inert::Talisman of Hierarchy`

   <details><summary>Details</summary>

   - `artifact`: `Talisman of Hierarchy`
   - `expected`: `any2`
   - `present_range`: `[7, None]`
   - `inert_turns`: `[7]`

   </details>
2. **[Artifact mana inertness]** Talisman of Resilience on seat 1 generated no mana for 1 turn(s)

   - Seats: `1`, Turns: `10`
   - group_key: `artifact_inert::Talisman of Resilience`

   <details><summary>Details</summary>

   - `artifact`: `Talisman of Resilience`
   - `expected`: `any2`
   - `present_range`: `[10, None]`
   - `inert_turns`: `[10]`

   </details>

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'angels_grace_cant_lose_or_win' fired 1 times

   - Seats: `—`, Turns: `5`
   - group_key: `unhandled::angels_grace_cant_lose_or_win`
   - Event seqs: `344`

   <details><summary>Details</summary>

   - `slug`: `angels_grace_cant_lose_or_win`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
2. **[Per-card handler missing]** per_card_unhandled for slug 'angels_grace_damage_floor_1' fired 1 times

   - Seats: `—`, Turns: `5`
   - group_key: `unhandled::angels_grace_damage_floor_1`
   - Event seqs: `345`

   <details><summary>Details</summary>

   - `slug`: `angels_grace_damage_floor_1`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
3. **[Per-card handler missing]** per_card_unhandled for slug 'stunning_reversal' fired 1 times

   - Seats: `—`, Turns: `12`
   - group_key: `unhandled::stunning_reversal`
   - Event seqs: `1361`

   <details><summary>Details</summary>

   - `slug`: `stunning_reversal`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
4. **[Storm deck never fires]** Seat 2 (Oloro, Ageless Ascetic) has storm payoffs ['Aetherflux Reservoir'] but never cast them (game had 52 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `payoffs_present`: `['Aetherflux Reservoir']`
   - `payoffs_never_cast`: `['Aetherflux Reservoir']`
   - `total_casts`: `52`

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
2. **[Dead in hand]** Seat 0 ended with 1 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `1`
   - `dead_cards`: `['Muldrotha, the Gravetide']`

   </details>
3. **[Dead in hand]** Seat 2 ended with 4 cards in hand that were never cast

   - Seats: `2`, Turns: `—`
   - group_key: `dead_hand::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `hand_size`: `4`
   - `dead_cards`: `['Felidar Sovereign', 'Aetherflux Reservoir', 'Beacon of Immortality', 'Sphere of Safety']`

   </details>
4. **[Dead in hand]** Seat 3 ended with 2 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `2`
   - `dead_cards`: `['Army of the Damned', 'Zombie Apocalypse']`

   </details>
5. **[Mana floated]** Seat 1 floated ≥3 mana at phase boundary for 10 turns

   - Seats: `1`, Turns: `9-18`
   - group_key: `mana_float::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `float_count`: `10`
   - `total_wasted`: `84`

   </details>

## Per-seat overview

### Seat 0: Ashling, Rekindled // Ashling, Rimebound

- Final life: **-2**, hand=1, library=75, graveyard=4
- Commander damage taken: 16 from seat 3 (Varina, Lich Queen)
- Commander casts: Ashling, Rekindled // Ashling, Rimebound×2

### Seat 1: Coram, the Undertaker

- Final life: **40**, hand=0, library=63, graveyard=10
- Battlefield (27): Balor, Colossal Grave-Reaver, Coram, the Undertaker, Dauthi Voidwalker, Demolition Field, Evolving Wilds, Forest, Forest, Forest, Forest, Ignoble Hierarch, Lightning Skelemental, Millikin, Mountain, Mountain, Mountain, Mountain, Overgrown Tomb, Smoldering Marsh, Sol Ring, Swamp, Swamp, Swamp, Swamp, Talisman of Resilience, The Balrog of Moria, Viridescent Bog
- Commander casts: Coram, the Undertaker×1

### Seat 2: Oloro, Ageless Ascetic

- Final life: **0**, hand=4, library=79, graveyard=9
- Commander casts: Oloro, Ageless Ascetic×0

### Seat 3: Varina, Lich Queen

- Final life: **-7**, hand=2, library=76, graveyard=7
- Commander casts: Varina, Lich Queen×2

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 2 |  |
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
| `dead_in_hand` | 3 |  |
| `mana_floated` | 1 |  |
| `commander_never_cast` | 1 |  |
| `untapped_artifact_never_activated` | 0 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
