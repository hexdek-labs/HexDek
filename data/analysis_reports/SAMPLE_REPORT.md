# Game `long-001` Forensic Analysis

> _"What we see vs what is reported should be eyes on anything that looks like it could be an error. Those are the flags."_  — 7174n1c, 2026-04-16

## Summary

- **Winner**: seat 3 (`Kinnan, Bonder Prodigy`)
- **End reason**: seat 3 is the last one standing
- **Turns played**: 19
- **Events captured**: 2087
- **Findings**: 5 CRITICAL / 5 WARN / 6 INFO

### Seats

| Seat | Commander | Archetype | Final life | Status |
| ---- | --------- | --------- | ---------- | ------ |
| 0 | Kraum, Ludevic's Opus | artifact_ramp | -1 | LOST (combat damage) |
| 1 | Muldrotha, the Gravetide | cedh_turbo | 6 | LOST (21+ commander damage from Kraum, Ludevic's Opus (CR 704.6c)) |
| 2 | Ral, Monsoon Mage // Ral, Leyline Prodigy | artifact_ramp | 0 | LOST (combat damage) |
| 3 | Kinnan, Bonder Prodigy | artifact_ramp | 38 | **WINNER** |

## CRITICAL flags

1. **[Artifact mana inertness]** Artifact mana inertness — seats [0,1], turns [1, 3, 11] (2 findings)

   - Seats: `0, 1`, Turns: `1, 3, 11`
   - group_key: `artifact_inert::Mox Diamond`

   <details><summary>Details</summary>

   - `artifact`: `Mox Diamond`
   - `expected`: `any1`
   - `present_range`: `[11, None]`
   - `inert_turns`: `[11]`

   </details>

   <details><summary>Subfindings (2)</summary>

   - Mox Diamond on seat 0 generated no mana for 1 turn(s)
   - Mox Diamond on seat 1 generated no mana for 2 turn(s)

   </details>
2. **[Artifact mana inertness]** Artifact mana inertness — seats [0,3], turns [16-19] (2 findings)

   - Seats: `0, 3`, Turns: `16-19`
   - group_key: `artifact_inert::Mox Amber`

   <details><summary>Details</summary>

   - `artifact`: `Mox Amber`
   - `expected`: `any1`
   - `present_range`: `[17, None]`
   - `inert_turns`: `[17, 18]`

   </details>

   <details><summary>Subfindings (2)</summary>

   - Mox Amber on seat 0 generated no mana for 2 turn(s)
   - Mox Amber on seat 3 generated no mana for 3 turn(s)

   </details>
3. **[Draw-reactive trigger missed]** Orcish Bowmasters on seat 0 — 19/19 opponent draws produced no reaction

   - Seats: `0`, Turns: `10-15`
   - group_key: `draw_reactive_missed::Orcish Bowmasters`
   - Event seqs: `741, 771, 787, 884, 925, 942, 1019, 1052, 1070, 1143` (+9 more)

   <details><summary>Details</summary>

   - `trigger`: `Orcish Bowmasters`
   - `descr`: `whenever opp draws extra/card, create goblin + ping 1`
   - `missed_count`: `19`
   - `total_opportunities`: `19`

   </details>
4. **[Reactive trigger missed]** Esper Sentinel on seat 0 — 8/11 opponent casts produced no draw or pay-mana reaction

   - Seats: `0`, Turns: `15-17, 19`
   - group_key: `reactive_missed::Esper Sentinel`
   - Event seqs: `1506, 1569, 1574, 1682, 1749, 1865, 1874, 2065`

   <details><summary>Details</summary>

   - `trigger`: `Esper Sentinel`
   - `descr`: `whenever opponent casts first noncreature, draw unless pay {X}`
   - `missed_count`: `8`
   - `total_opportunities`: `11`
   - `sample_misses`: `[{'cast_card': "Thassa's Oracle", 'caster_seat': 1, 'turn': 15, 'seq': 1506}, {'cast_card': 'The One Ring', 'caster_seat': 3, 'turn': 15, 'seq': 1569}, {'cast_card': 'Mental Misstep', 'caster_seat': 1, 'turn': 15, 'seq': 1574}, {'cast_card': 'Demonic Consultation', 'caster_seat': 1, 'turn': 16, 'seq': 1682}, {'cast_card': 'Windfall', 'caster_seat': 3, 'turn': 16, 'seq': 1749}]`

   </details>
5. **[Reactive trigger missed]** Reactive trigger missed — seats [0,3], turns [3, 6, 8-19] (2 findings)

   - Seats: `0, 3`, Turns: `3, 6, 8-19`
   - group_key: `reactive_missed::Mystic Remora`
   - Event seqs: `619, 648, 746, 890, 900, 1026, 1151, 1169, 1178, 1229` (+30 more)

   <details><summary>Details</summary>

   - `trigger`: `Mystic Remora`
   - `descr`: `whenever opponent casts nonc, draw unless they pay {4}`
   - `missed_count`: `21`
   - `total_opportunities`: `33`
   - `sample_misses`: `[{'cast_card': 'Fierce Guardianship', 'caster_seat': 1, 'turn': 9, 'seq': 619}, {'cast_card': 'Elvish Spirit Guide', 'caster_seat': 1, 'turn': 9, 'seq': 648}, {'cast_card': 'Intuition', 'caster_seat': 1, 'turn': 10, 'seq': 746}, {'cast_card': 'Diabolic Intent', 'caster_seat': 1, 'turn': 11, 'seq': 890}, {'cast_card': 'Birds of Paradise', 'caster_seat': 1, 'turn': 11, 'seq': 900}]`

   </details>

   <details><summary>Subfindings (2)</summary>

   - Mystic Remora on seat 0 — 21/33 opponent casts produced no draw or pay-mana reaction
   - Mystic Remora on seat 3 — 30/31 opponent casts produced no draw or pay-mana reaction

   </details>

## WARN flags

1. **[Muldrotha no graveyard cast]** Muldrotha on seat 1 but no graveyard recurse or cast-from-graveyard observed

   - Seats: `1`, Turns: `—`
   - group_key: `muldrotha_no_gy`
2. **[Per-card handler missing]** per_card_unhandled for slug 'animist_awakening_reveal_x_lands_to_battlefield' fired 1 times

   - Seats: `—`, Turns: `17`
   - group_key: `unhandled::animist_awakening_reveal_x_lands_to_battlefield`
   - Event seqs: `1887`

   <details><summary>Details</summary>

   - `slug`: `animist_awakening_reveal_x_lands_to_battlefield`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
3. **[Per-card handler missing]** per_card_unhandled for slug 'animist_awakening_spell_mastery_untap_lands' fired 1 times

   - Seats: `—`, Turns: `17`
   - group_key: `unhandled::animist_awakening_spell_mastery_untap_lands`
   - Event seqs: `1888`

   <details><summary>Details</summary>

   - `slug`: `animist_awakening_spell_mastery_untap_lands`
   - `hits`: `1`
   - `sites`: `['spell']`

   </details>
4. **[Storm deck never fires]** Seat 0 (Kraum, Ludevic's Opus) has storm payoffs ['Grapeshot', 'Brain Freeze'] but never cast them (game had 58 total casts)

   - Seats: `0`, Turns: `—`
   - group_key: `storm_no_fire::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `commander`: `Kraum, Ludevic's Opus`
   - `payoffs_present`: `['Grapeshot', 'Tendrils of Agony', 'Brain Freeze']`
   - `payoffs_never_cast`: `['Grapeshot', 'Brain Freeze']`
   - `total_casts`: `58`

   </details>
5. **[Storm deck never fires]** Seat 2 (Ral, Monsoon Mage) has storm payoffs ['Grapeshot', 'Brain Freeze'] but never cast them (game had 58 total casts)

   - Seats: `2`, Turns: `—`
   - group_key: `storm_no_fire::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Ral, Monsoon Mage`
   - `payoffs_present`: `['Grapeshot', 'Brain Freeze']`
   - `payoffs_never_cast`: `['Grapeshot', 'Brain Freeze']`
   - `total_casts`: `58`

   </details>

## INFO flags

1. **[Commander never cast]** Seat 2 (Ral, Monsoon Mage // Ral, Leyline Prodigy) never cast their commander across 19 turns

   - Seats: `2`, Turns: `—`
   - group_key: `cmd_never_cast::Ral, Monsoon Mage // Ral, Leyline Prodigy`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `commander`: `Ral, Monsoon Mage // Ral, Leyline Prodigy`
   - `total_turns`: `19`

   </details>
2. **[Dead in hand]** Seat 0 ended with 1 cards in hand that were never cast

   - Seats: `0`, Turns: `—`
   - group_key: `dead_hand::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `hand_size`: `1`
   - `dead_cards`: `["Mind's Desire"]`

   </details>
3. **[Dead in hand]** Seat 2 ended with 7 cards in hand that were never cast

   - Seats: `2`, Turns: `—`
   - group_key: `dead_hand::s2`

   <details><summary>Details</summary>

   - `seat`: `2`
   - `hand_size`: `7`
   - `dead_cards`: `['Snap', 'Faithless Looting', 'Hidden Strings', 'Tavern Scoundrel', 'Brain Freeze', 'Mox Diamond', 'Merchant Scroll']`

   </details>
4. **[Mana floated]** Seat 0 floated ≥3 mana at phase boundary for 4 turns

   - Seats: `0`, Turns: `14-15, 17, 19`
   - group_key: `mana_float::s0`

   <details><summary>Details</summary>

   - `seat`: `0`
   - `float_count`: `4`
   - `total_wasted`: `18`

   </details>
5. **[Mana floated]** Seat 3 floated ≥3 mana at phase boundary for 4 turns

   - Seats: `3`, Turns: `16-19`
   - group_key: `mana_float::s3`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `float_count`: `4`
   - `total_wasted`: `23`

   </details>
6. **[Mana permanent never activated]** Seat 3 ended with Mox Amber on the battlefield having never generated mana

   - Seats: `3`, Turns: `—`
   - group_key: `unactivated::Mox Amber`

   <details><summary>Details</summary>

   - `seat`: `3`
   - `card`: `Mox Amber`
   - `expected`: `any1`

   </details>

## Per-seat overview

### Seat 0: Kraum, Ludevic's Opus

- Final life: **-1**, hand=1, library=70, graveyard=11
- Commander damage taken: 18 from seat 3 (Kinnan, Bonder Prodigy)
- Commander casts: Kraum, Ludevic's Opus×1

### Seat 1: Muldrotha, the Gravetide

- Final life: **6**, hand=0, library=0, graveyard=11
- Commander damage taken: 24 from seat 0 (Kraum, Ludevic's Opus)
- Commander casts: Muldrotha, the Gravetide×1

### Seat 2: Ral, Monsoon Mage // Ral, Leyline Prodigy

- Final life: **0**, hand=7, library=74, graveyard=9
- Commander damage taken: 12 from seat 1 (Muldrotha, the Gravetide); 8 from seat 0 (Kraum, Ludevic's Opus)
- Commander casts: Ral, Monsoon Mage // Ral, Leyline Prodigy×0

### Seat 3: Kinnan, Bonder Prodigy

- Final life: **38**, hand=0, library=67, graveyard=9
- Battlefield (24): Basalt Monolith, Bloom Tender, Candelabra of Tawnos, Exploration, Flooded Strand, Forest, Freed from the Real, Ghostly Pilferer, Inventors' Fair, Kinnan, Bonder Prodigy, Lion's Eye Diamond, Lotus Petal, Mox Amber, Multani's Harmony, Mystic Remora, Pemmin's Aura, Polluted Delta, Scalding Tarn, Sol Ring, Tezzeret the Seeker, Tropical Island, Urza's Bauble, Wooded Foothills, Yavimaya Coast
- Commander casts: Kinnan, Bonder Prodigy×1

## Detector coverage

| Detector | Findings | Error |
| -------- | -------- | ----- |
| `artifact_mana_inertness` | 4 |  |
| `storm_copies_empty` | 0 |  |
| `bolas_citadel_no_life_change` | 0 |  |
| `reactive_trigger_missed` | 3 |  |
| `draw_reactive_trigger_missed` | 1 |  |
| `commander_damage_sba_miss` | 0 |  |
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
| `muldrotha_no_gy_cast` | 1 |  |
| `werewolf_day_night_static` | 0 |  |
| `dead_in_hand` | 2 |  |
| `mana_floated` | 2 |  |
| `commander_never_cast` | 1 |  |
| `untapped_artifact_never_activated` | 1 |  |

---

_Generated by `scripts/analyze_single_game.py` — file under `scripts/analyze_utils/`._
