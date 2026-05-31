# Era 4 (2023-2026) Scaffold-Gap Audit

- Total cards in dataset: **31963**

- Era distribution: era1=26932, era2=537, era3=798, era4=3696

- Era 4 cards: **3696**

- Era 4 Condition nodes: **514** (bucketed 505, unbucketed 9, 1.8% gap)

- Era 4 Trigger nodes: **2515** (bucketed 2439, unbucketed 76, 3.0% gap)


## Top unbucketed condition Kinds

- `conditional` × 6
- `if` × 3

## Top unbucketed raw-text fragments (kind in raw/intervening_if/as_long_as)

- × 1: `all ten digits are crossed out`  _(e.g. Duelists' Convocation International)_
- × 1: `the gift was promised and that creature isn't legendary`  _(e.g. Coiling Rebirth)_
- × 1: `if it entered under your control, put a +1/+1 counter on it. otherwise, tap it`  _(e.g. Hallowed Respite)_
- × 1: `if it's a spacecraft, put ten charge counters on it. if you do, remove ten charg`  _(e.g. Systems Override)_
- × 1: `if a land was destroyed this way, its controller may search their library for up`  _(e.g. Dire-Strain Rampage)_
- × 1: `if damage is prevented this way, create that many 1/1 colorless phyrexian mite a`  _(e.g. Ria Ivor, Bane of Bladehold)_
- × 1: `one of that creature's creature types is on your buddy list`  _(e.g. Champion of the Hareish)_
- × 1: `if an opponent protects it, remove a defense counter from it. otherwise, put a d`  _(e.g. Portent Tracker)_
- × 1: `if this artifact turns over completely at least once during the flip, destroy al`  _(e.g. Chaos Orb)_

## Bucketed condition Kinds (sanity)

- `if` × 165
- `paid_optional_cost` × 101
- `conditional` × 76
- `for_each` × 51
- `did_prior_action` × 46
- `metalcraft` × 24
- `it_was_a_creature` × 8
- `no_creatures_on_battlefield` × 7
- `threshold` × 6
- `landfall` × 5
- `spell_mastery` × 3
- `you_descended_this_turn` × 3
- `etb_if` × 2
- `morbid` × 1
- `ferocious` × 1
- `raid` × 1
- `domain` × 1
- `life_delta_threshold` × 1
- `life_vs_half_starting` × 1
- `no_mana_spent_to_cast` × 1

## Top unbucketed trigger events

- `self_deals_damage_player` × 5
- `another_creature_or_artifact_event` × 5
- `this_card_event` × 3
- `tap_for_mana` × 3
- `ally_typed_etb` × 3
- `creature_etb_any` × 3
- `upkeep` × 3
- `land_etb_any` × 2
- `block_creature` × 2
- `exiled_event` × 2
- `self_blocks` × 2
- `one_or_more_other_ally_event` × 2
- `self_or_typed_event` × 2
- `lose_control_of` × 2
- `creature_cards_to_zone` × 2
- `one_or_more_lands` × 2
- `opp_activate` × 1
- `you_surveil` × 1
- `you_proliferate` × 1
- `one_or_more_other_creatures` × 1
- `you_put_counters_on_any` × 1
- `creature_cards_leave_gy` × 1
- `another_typed_etb` × 1
- `compound_tribe_die_or_leave` × 1
- `you_dealt_damage` × 1
- `put_onto_bf` × 1
- `legend_ally_event` × 1
- `self_phase_inout` × 1
- `targets_chosen` × 1
- `any_player_sacs` × 1
- `opp_landfall` × 1
- `saga_final_chapter` × 1
- `nonland_tapped_for_mana` × 1
- `you_scry` × 1
- `you_exert_creature` × 1
- `artifact_etb_yours` × 1
- `merfolk_etb_any` × 1
- `one_or_more_ally_with_x_enter` × 1
- `creature_or_land_to_gy` × 1
- `other_nontoken_perm` × 1
- `discover` × 1
- `counter_threshold` × 1
- `compound_bounce_shuffle_event` × 1
- `any_type_to_gy_from_bf` × 1
- `becomes_untapped_once` × 1
- `paid_cumulative_upkeep` × 1
- `becomes_blocked_by` × 1
- `self_card_zone_to_zone` × 1
- `you_put_counters_on` × 1

## Top trigger events (bucketed + unbucketed)

- `etb` × 753
- `phase` × 286
- `die` × 143
- `attack` × 142
- `type_leaves_battlefield` × 111
- `combat_damage_player` × 80
- `when_you_do` × 60
- `cast_any` × 49
- `cast_filtered` × 45
- `beginning_of_ordinal_step` × 42
- `to_graveyard` × 40
- `tribe_you_control_etb` × 30
- `self_put_into_graveyard_from_bf` × 27
- `unlock_door` × 27
- `enter_or_attack` × 24
- `ltb` × 24
- `etb_as` × 20
- `cast_spell` × 20
- `self_leaves_battlefield` × 18
- `you_attack` × 16
- `creature_dies` × 16
- `one_or_more_typed_event` × 16
- `group_combat_damage_player` × 15
- `self_and` × 14
- `another_typed_enters` × 12
- `ally_type_to_gy_from_bf` × 12
- `misc_when` × 12
- `you_whenever` × 12
- `sacrifice_filtered` × 10
- `turned_face_up` × 10
- `misc_whenever_a` × 10
- `deals_damage` × 10
- `tribe_you_control_dies` × 10
- `specialize_from_zone` × 10
- `becomes_tapped` × 9
- `etb_or_another` × 9
- `nontoken_creature_event` × 9
- `type_to_gy_from_bf` × 8
- `combat_damage_player_or_battle` × 8
- `nontoken_ally_event` × 8