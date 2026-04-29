# Game `octo-smoke-001` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 2 (`Fire Lord Azula`)
- **End reason**: seat 2 is the last one standing
- **Turns played**: 18
- **Events captured**: 1950
- **Findings**: 6 CRITICAL / 4 WARN / 7 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Ardenn, Intrepid Archaeologist | artifact_ramp | 6 | LOST (21+ commander damage from Fire Lord Azula (CR 704.6c)) |
| 1 | Yarok, the Desecrated | artifact_ramp | 0 | LOST (combat damage) |
| 2 | Fire Lord Azula | artifact_ramp | 30 | **WINNER** |
| 3 | Kraum, Ludevic's Opus | artifact_ramp | -3 | LOST (combat damage) |

## CRITICAL flags

1. **[Artifact mana inertness]** Artifact mana inertness — seats [0,2], turns [2-18] (2 findings)

   - Seats: `0, 2`, Turns: `2-18`
   - group_key: `artifact_inert::Mox Opal`

   <details><summary>Details</summary>

   - `artifact`: `Mox Opal`
   - `expected`: `any1`
   - `present_range`: `[2, None]`
   - `inert_turns`: `[2, 4, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17]`

   </details>

   <details><summary>Subfindings (2)</summary>

   - Mox Opal on seat 0 generated no mana for 14 turn(s)
   - Mox Opal on seat 2 generated no mana for 17 turn(s)

   </details>
2. **[Artifact mana inertness]** Artifact mana inertness — seats [0,3], turns [5, 7, 9, 14-17] (2 findings)

   - Seats: `0, 3`, Turns: `5, 7, 9, 14-17`
   - group_key: `artifact_inert::Mox Amber`

   <details><summary>Details</summary>

   - `artifact`: `Mox Amber`
   - `expected`: `any1`
   - `present_range`: `[14, None]`
   - `inert_turns`: `[14, 15, 16, 17]`

   </details>

   <details><summary>Subfindings (2)</summary>

   - Mox Amber on seat 0 generated no mana for 4 turn(s)
   - Mox Amber on seat 3 generated no mana for 3 turn(s)

   </details>
3. **[Artifact mana inertness]** Everflowing Chalice on seat 0 generated no mana for 7 turn(s)

   - Seats: `0`, Turns: `11-17`
   - group_key: `artifact_inert::Everflowing Chalice`

   <details><summary>Details</summary>

   - `artifact`: `Everflowing Chalice`
   - `expected`: `any_variable`
   - `present_range`: `[11, None]`
   - `inert_turns`: `[11, 12, 13, 14, 15, 16, 17]`

   </details>
4. **[Commander damage SBA miss]** Seat 3 took 24 commander damage from Fire Lord Azula (dealer seat 2) but sba_704_6c never fired

   - Seats: `3`, Turns: `—`
   - group_key: `cmd_dmg_miss::Fire Lord Azula`

   <details><summary>Details</summary>

   - `victim`: `3`
   - `dealer`: `2`
   - `commander`: `Fire Lord Azula`
   - `damage`: `24`
   - `victim_lost_anyway`: `True`

   </details>
5. **[Reactive trigger missed]** Esper Sentinel on seat 0 — 7/11 opponent casts produced no draw or pay-mana reaction

   - Seats: `0`, Turns: `6-9`
   - group_key: `reactive_missed::Esper Sentinel`
   - Event seqs: `431, 442, 549, 668, 674, 769, 802`

   <details><summary>Details</summary>

   - `trigger`: `Esper Sentinel`
   - `descr`: `whenever opponent casts first noncreature, draw unless pay {X}`
   - `missed_count`: `7`
   - `total_opportunities`: `11`
   - `sample_misses`: `[{'cast_card': "Thassa's Oracle", 'caster_seat': 2, 'turn': 6, 'seq': 431}, {'cast_card': 'Windfall', 'caster_seat': 2, 'turn': 6, 'seq': 442}, {'cast_card': "Yawgmoth's Will", 'caster_seat': 2, 'turn': 7, 'seq': 549}, {'cast_card': 'Leyline of Anticipation', 'caster_seat': 2, 'turn': 8, 'seq': 668}, {'cast_card': "An Offer You Can't Refuse", 'caster_seat': 1, 'turn': 8, 'seq': 674}]`

   </details>
6. **[Reactive trigger missed]** Mystic Remora on seat 3 — 31/33 opponent casts produced no draw or pay-mana reaction

   - Seats: `3`, Turns: `9-18`
   - group_key: `reactive_missed::Mystic Remora`
   - Event seqs: `735, 769, 868, 906, 916, 925, 934, 1033, 1042, 1109` (+10 more)

   <details><summary>Details</summary>

   - `trigger`: `Mystic Remora`
   - `descr`: `whenever opponent casts nonc, draw unless they pay {4}`
   - `missed_count`: `31`
   - `total_opportunities`: `33`
   - `sample_misses`: `[{'cast_card': "Illusionist's Bracers", 'caster_seat': 0, 'turn': 9, 'seq': 735}, {'cast_card': 'Deathrite Shaman', 'caster_seat': 1, 'turn': 9, 'seq': 769}, {'cast_card': 'Puresteel Paladin', 'caster_seat': 0, 'turn': 10, 'seq': 868}, {'cast_card': 'Culling the Weak', 'caster_seat': 1, 'turn': 10, 'seq': 906}, {'cast_card': 'Llanowar Elves', 'caster_seat': 1, 'turn': 10, 'seq': 916}]`

   </details>

## WARN flags

1. **[Per-card handler missing]** per_card_unhandled for slug 'sacrifice_add_B_per_mana_value' fired 1 times

   - Seats: `—`, Turns: `15`
   - group_key: `unhandled::sacrifice_add_B_per_mana_value`
   - Event seqs: `1565`

   <details><summary>Details</summary>

   - `slug`: `sacrifice_add_B_per_mana_value`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
2. **[Per-card handler missing]** per_card_unhandled for slug 'sacrifice_alt_cost_sac_creature' fired 1 times

   - Seats: `—`, Turns: `15`
   - group_key: `unhandled::sacrifice_alt_cost_sac_creature`
   - Event seqs: `1564`

   <details><summary>Details</summary>

   - `slug`: `sacrifice_alt_cost_sac_creature`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
3. **[Storm deck never fires]** Seat 2 (Fire Lord Azula) has storm payoffs ['Brain Freeze'] but never cast them (game had 50 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Fire Lord Azula`
   - `payoffs_present`: `['Brain Freeze']`
   - `payoffs_never_cast`: `['Brain Freeze']`
   - `total_casts`: `50`

   </details>
4. **[Storm deck never fires]** Seat 3 (Kraum, Ludevic's Opus) has storm payoffs ['Grapeshot', 'Tendrils of Agony', 'Brain Freeze'] but never cast them (game had 50 total casts)

   - Seats: `3`, Turns: `—`
   - group_key: `storm_no_fire::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `commander`: `Kraum, Ludevic's Opus`
   - `payoffs_present`: `['Grapeshot', 'Tendrils of Agony', 'Brain Freeze']`
   - `payoffs_never_cast`: `['Grapeshot', 'Tendrils of Agony', 'Brain Freeze']`
   - `total_casts`: `50`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 1 (Yarok, the Desecrated) never cast their commander across 18 turns

   - Seats: `1`, Turns: `—`
   - group_key: `cmd_never_cast::Yarok, the Desecrated`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `commander`: `Yarok, the Desecrated`
   - `total_turns`: `18`

   </details>
2. **[Commander never cast]** Seat 3 (Kraum, Ludevic's Opus) never cast their commander across 18 turns

   - Seats: `3`, Turns: `—`
   - group_key: `cmd_never_cast::Kraum, Ludevic's Opus`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `commander`: `Kraum, Ludevic's Opus`
   - `total_turns`: `18`

   </details>
3. **[Dead in hand]** Seat 0 ended with 1 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `1`
   - `dead_cards`: `['Bruenor Battlehammer']`

   </details>
4. **[Dead in hand]** Seat 1 ended with 6 cards in hand that were never cast

   - Seats: `1`, Turns: `—`
   - group_key: `dead_hand::s1`

   <details><summary>Details</summary>

   - `seat`: `1`
   - `hand_size`: `6`
   - `dead_cards`: `['Fellwar Stone', 'Diabolic Intent', 'Snapcaster Mage', 'Displacer Kitten', 'Rhystic Study', 'Sylvan Library']`

   </details>
5. **[Dead in hand]** Seat 3 ended with 7 cards in hand that were never cast

   - Seats: `3`, Turns: `—`
   - group_key: `dead_hand::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `hand_size`: `7`
   - `dead_cards`: `["Mind's Desire", 'Soul Shatter', 'Laboratory Maniac', 'Imperial Recruiter', 'Deflecting Swat', 'Sheoldred, the Apocalypse', 'Rhystic Study']`

   </details>
6. **[Mana floated]** Seat 2 floated ≥3 mana at phase boundary for 9 turns

   - Seats: `2`, Turns: `9, 12-18`
   - group_key: `mana_float::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `float_count`: `9`
   - `total_wasted`: `43`

   </details>
7. **[Mana permanent never activated]** Seat 2 ended with Mox Opal on the battlefield having never generated mana

   - Seats: `2`, Turns: `—`
   - group_key: `unactivated::Mox Opal`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `card`: `Mox Opal`
   - `expected`: `any1`

   </details>

## Per-seat overview

### Seat 0: Ardenn, Intrepid Archaeologist

- Final life: **6**, hand=1, library=75, graveyard=3
- Commander damage taken: 24 from seat 2 (Fire Lord Azula)
- Commander casts: Ardenn, Intrepid Archaeologist×1

### Seat 1: Yarok, the Desecrated

- Final life: **0**, hand=6, library=74, graveyard=9
- Commander damage taken: 16 from seat 0 (Ardenn, Intrepid Archaeologist); 4 from seat 2 (Fire Lord Azula)
- Commander casts: Yarok, the Desecrated×0

### Seat 2: Fire Lord Azula

- Final life: **30**, hand=0, library=74, graveyard=14
- Battlefield (12): Ancient Tomb, Arena of Glory, Badlands, City of Traitors, Fire Lord Azula, Gemstone Caverns, Mana Vault, Mistrise Village, Mox Diamond, Mox Opal, Thassa's Oracle, Urza's Saga
- Commander casts: Fire Lord Azula×1

### Seat 3: Kraum, Ludevic's Opus

- Final life: **-3**, hand=7, library=81, graveyard=2
- Commander damage taken: 24 from seat 2 (Fire Lord Azula)
- Commander casts: Kraum, Ludevic's Opus×0

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 5 |  |
| `storm_copies_empty` | 0 |  |
| `bolas_citadel_no_life_change` | 0 |  |
| `reactive_trigger_missed` | 2 |  |
| `draw_reactive_trigger_missed` | 0 |  |
| `commander_damage_sba_miss` | 1 |  |
| `thoracle_no_win` | 0 |  |
| `mana_pool_persists_across_phase` | 0 |  |
| `winner_consistency` | 0 |  |
| `crashed_effects` | 0 |  |
| `cap_hits` | 0 |  |
| `cast_failed_spam` | 0 |  |
| `per_card_unhandled` | 2 |  |
| `storm_deck_never_fires` | 2 |  |
| `ramp_deck_no_treasure` | 0 |  |
| `combo_pieces_stranded` | 0 |  |
| `muldrotha_no_gy_cast` | 0 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 3 |  |
| `mana_floated` | 1 |  |
| `commander_never_cast` | 2 |  |
| `untapped_artifact_never_activated` | 1 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
