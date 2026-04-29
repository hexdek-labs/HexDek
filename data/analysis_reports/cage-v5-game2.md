# Game `cage-v5-game2` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 1 (`Coram, the Undertaker`)
- **End reason**: seat 1 is the last one standing
- **Turns played**: 17
- **Events captured**: 2821
- **Findings**: 2 CRITICAL / 6 WARN / 9 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Ashling, Rekindled // Ashling, Rimebound | recursion_graveyard | -7 | LOST (combat damage) |
| 1 | Coram, the Undertaker | recursion_graveyard | 23 | **WINNER** |
| 2 | Oloro, Ageless Ascetic | storm | -5 | LOST (combat damage) |
| 3 | Varina, Lich Queen | recursion_graveyard | -13 | LOST (combat damage) |

## CRITICAL flags

1. **[Artifact mana inertness]** Talisman of Dominance on seat 3 generated no mana for 1 turn(s)

   - Seats: `3`, Turns: `13`
   - group_key: `artifact_inert::Talisman of Dominance`

   <details><summary>Details</summary>

   - `artifact`: `Talisman of Dominance`
   - `expected`: `any2`
   - `present_range`: `[13, None]`
   - `inert_turns`: `[13]`

   </details>
2. **[Artifact mana inertness]** Talisman of Impulse on seat 1 generated no mana for 1 turn(s)

   - Seats: `1`, Turns: `6`
   - group_key: `artifact_inert::Talisman of Impulse`

   <details><summary>Details</summary>

   - `artifact`: `Talisman of Impulse`
   - `expected`: `any2`
   - `present_range`: `[6, None]`
   - `inert_turns`: `[6]`

   </details>

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'debt_deathless_each_opp_2x_drain_you_gain' fired 1 times

   - Seats: `—`, Turns: `10`
   - group_key: `unhandled::debt_deathless_each_opp_2x_drain_you_gain`
   - Event seqs: `1335`

   <details><summary>Details</summary>

   - `slug`: `debt_deathless_each_opp_2x_drain_you_gain`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
2. **[Per-card handler missing]** per_card_unhandled for slug 'flare_of_fortitude_alt_cost_sac_white' fired 1 times

   - Seats: `—`, Turns: `12`
   - group_key: `unhandled::flare_of_fortitude_alt_cost_sac_white`
   - Event seqs: `1544`

   <details><summary>Details</summary>

   - `slug`: `flare_of_fortitude_alt_cost_sac_white`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
3. **[Per-card handler missing]** per_card_unhandled for slug 'flare_of_fortitude_life_locked_perms_hexproof_indestructible' fired 1 times

   - Seats: `—`, Turns: `12`
   - group_key: `unhandled::flare_of_fortitude_life_locked_perms_hexproof_indestructible`
   - Event seqs: `1545`

   <details><summary>Details</summary>

   - `slug`: `flare_of_fortitude_life_locked_perms_hexproof_indestructible`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
4. **[Per-card handler missing]** per_card_unhandled for slug 'life' fired 1 times

   - Seats: `—`, Turns: `6`
   - group_key: `unhandled::life`
   - Event seqs: `499`

   <details><summary>Details</summary>

   - `slug`: `life`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
5. **[Per-card handler missing]** per_card_unhandled for slug 'stunning_reversal' fired 1 times

   - Seats: `—`, Turns: `11`
   - group_key: `unhandled::stunning_reversal`
   - Event seqs: `1455`

   <details><summary>Details</summary>

   - `slug`: `stunning_reversal`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
6. **[Storm deck never fires]** Seat 2 (Oloro, Ageless Ascetic) has storm payoffs ['Aetherflux Reservoir'] but never cast them (game had 41 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `payoffs_present`: `['Aetherflux Reservoir']`
   - `payoffs_never_cast`: `['Aetherflux Reservoir']`
   - `total_casts`: `41`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 2 (Oloro, Ageless Ascetic) never cast their commander across 17 turns

   - Seats: `2`, Turns: `—`
   - group_key: `cmd_never_cast::Oloro, Ageless Ascetic`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Oloro, Ageless Ascetic`
   - `total_turns`: `17`

   </details>
2. **[Commander never cast]** Seat 3 (Varina, Lich Queen) never cast their commander across 17 turns

   - Seats: `3`, Turns: `—`
   - group_key: `cmd_never_cast::Varina, Lich Queen`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `commander`: `Varina, Lich Queen`
   - `total_turns`: `17`

   </details>
3. **[Dead in hand]** Seat 0 ended with 1 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `1`
   - `dead_cards`: `['Lamentation']`

   </details>
4. **[Dead in hand]** Seat 2 ended with 2 cards in hand that were never cast

   - Seats: `2`, Turns: `—`
   - group_key: `dead_hand::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `hand_size`: `2`
   - `dead_cards`: `['Angelic Chorus', 'Mangara, the Diplomat']`

   </details>
5. **[Dead in hand]** Seat 3 ended with 7 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `7`
   - `dead_cards`: `['Diregraf Captain', "Lich's Mastery", 'Living Death', 'Diregraf Colossus', 'Fatestitcher', "Necromancer's Covenant", "Urza's Incubator"]`

   </details>
6. **[Mana floated]** Seat 0 floated ≥3 mana at phase boundary for 3 turns

   - Seats: `0`, Turns: `9, 15, 17`
   - group_key: `mana_float::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `float_count`: `3`
   - `total_wasted`: `10`

   </details>
7. **[Mana floated]** Seat 1 floated ≥3 mana at phase boundary for 9 turns

   - Seats: `1`, Turns: `6, 8, 10-16`
   - group_key: `mana_float::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `float_count`: `9`
   - `total_wasted`: `50`

   </details>
8. **[Mana floated]** Seat 2 floated ≥3 mana at phase boundary for 10 turns

   - Seats: `2`, Turns: `1-10`
   - group_key: `mana_float::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `float_count`: `10`
   - `total_wasted`: `53`

   </details>
9. **[Mana floated]** Seat 3 floated ≥3 mana at phase boundary for 5 turns

   - Seats: `3`, Turns: `4, 8-10, 13`
   - group_key: `mana_float::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `float_count`: `5`
   - `total_wasted`: `19`

   </details>

## Per-seat overview

### Seat 0: Ashling, Rekindled // Ashling, Rimebound

- Final life: **-7**, hand=1, library=76, graveyard=4
- Commander casts: Ashling, Rekindled // Ashling, Rimebound×3

### Seat 1: Coram, the Undertaker

- Final life: **23**, hand=0, library=73, graveyard=11
- Battlefield (16): Chainer, Nightmare Adept, Coram, the Undertaker, Corpse Connoisseur, Forest, Forest, Forest, Forest, Mountain, Perpetual Timepiece, Purple Worm, Riveteers Overlook, Swamp, Talisman of Impulse, Talisman of Resilience, Timeless Witness, Treasure Vault
- Commander casts: Coram, the Undertaker×2

### Seat 2: Oloro, Ageless Ascetic

- Final life: **-5**, hand=2, library=82, graveyard=5
- Commander casts: Oloro, Ageless Ascetic×0

### Seat 3: Varina, Lich Queen

- Final life: **-13**, hand=7, library=76, graveyard=8
- Commander casts: Varina, Lich Queen×0

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
| `per_card_unhandled` | 5 |  |
| `storm_deck_never_fires` | 1 |  |
| `ramp_deck_no_treasure` | 0 |  |
| `combo_pieces_stranded` | 0 |  |
| `muldrotha_no_gy_cast` | 0 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 3 |  |
| `mana_floated` | 4 |  |
| `commander_never_cast` | 2 |  |
| `untapped_artifact_never_activated` | 0 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
