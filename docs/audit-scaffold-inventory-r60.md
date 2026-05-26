# Phase 1C Scaffold Inventory Audit

**Branch:** `dev/audit-scaffold-inventory-r60`
**Date:** 2026-05-25
**Scope:** Cross-reference of `cmd/hexdek-thor/conditional_setup.go` + 
`cmd/hexdek-thor/goldilocks.go` scaffold dispatchers against the AST corpus 
(`data/rules/ast_dataset.jsonl`).

**Output:** findings only — no scaffolds deleted, no dispatch consolidated, 
no corpus added. Phase 1C is inventory-only; any cleanup happens in a follow-up PR.

---

## Corpus baseline

- Cards processed: **31,963**
- Distinct trigger event slugs in corpus: **416**
- Distinct condition kinds in corpus (incl. raw wrappers): **44**
- Distinct structured condition kinds (excl. raw/conditional/if): **42**

## Dispatcher inventory

### Trigger side — `cmd/hexdek-thor/conditional_setup.go`
- `triggerConditionActions` registered scaffold keys: **46**
- `classifyTrigger` distinct return slugs: **46**
- Duplicate registry keys: **NONE**

### Condition side — two-dispatcher architecture
- `cmd/hexdek-thor/conditional_setup.go::detectConditionScaffold` case-strings: **126**
- `cmd/hexdek-thor/goldilocks.go` switch case-strings: **98**
- Union (Go-handled total): **222**

---

## Finding 1 — Trigger orphan scaffolds
_Registered in `triggerConditionActions` but ZERO corpus events route to them 
via a Python mirror of `classifyTrigger` (opponent-actor heuristic included)._

**Count: 2**

- `begin_combat` — phase-style; only fires when `phase: combat_start | begin_combat | beginning of combat`; corpus has these as phase events, not literal `event` slugs.
- `lose_life` — Python mirror substring catch `lose` requires `life` in event; no corpus event currently contains both.

**Verification needed before deletion:** the Python mirror does not model every 
actor variant. Recommended next step is instrumenting `classifyTrigger` in Go 
with a per-slug hit counter on a full corpus pass to confirm zero matches.

## Finding 2 — Trigger orphan parse-targets
_`classifyTrigger` returns slug X, but `triggerConditionActions[X]` is missing — 
`primeTriggerCondition` would fall through without priming._

**Count: 0**

_(none — every slug `classifyTrigger` returns has a registry handler)_

## Finding 3 — Unhandled corpus trigger events
_Event slugs the parser emits that fall through every exact-match and substring 
catch in `classifyTrigger`. These nodes get no priming and contribute to the 
per-era unbucketed-trigger residuals._

**Total: 2174 unhandled trigger nodes across 79 distinct slugs.**

Top 30 by count:

- `phase` × 1998
- `self_put_into_graveyard_from_bf` × 27
- `class_becomes_level` × 13
- `ally_type_to_gy_from_bf` × 12
- `specialize_from_zone` × 10
- `type_to_gy_from_bf` × 8
- `ring_tempts_you` × 7
- `to_gy_from_bf` × 6
- `opp_type_to_gy_from_bf` × 5
- `ally_typed_to_gy` × 5
- `opp_creature_to_gy` × 3
- `exiled_event` × 3
- `self_to_gy` × 3
- `self_or_typed_event` × 3
- `modified_creature_event` × 2
- `compound_tribe_enter` × 2
- `face_down_creature_event` × 2
- `nontoken_type_to_gy` × 2
- `it_state_change` × 2
- `tribal_to_gy_from_bf` × 2
- `self_die_or_ally_gy` × 1
- `train` × 1
- `put_onto_bf` × 1
- `investigate` × 1
- `opp_tokens_event` × 1
- `desert_etb` × 1
- `condition_fails` × 1
- `self_and_or_others_event` × 1
- `tap_for_c` × 1
- `sac_nontoken_elemental` × 1

_(... and 49 more low-count slugs — full list available by re-running the audit)_

## Finding 4 — Truly-unhandled corpus condition kinds
_Structured condition kinds the parser emits that NEITHER `detectConditionScaffold` 
NOR `goldilocks.go` cases. After cross-checking both dispatchers — only **5** 
kinds remain truly unhandled._

**Total: 15 unhandled condition nodes across 5 distinct kinds.**

- `life_threshold` × 5
- `life_vs_half_starting` × 4
- `repeat_any_optional` × 3
- `life_delta_threshold` × 2
- `life_threshold_both` × 1

All remaining unhandled kinds are **life-comparison variants** that the Python audit 
script (`scripts/era1_scaffold_audit.py`) already buckets via `BUCKETED_KINDS` but 
that lack explicit Go-side dispatch. Likely safe to add as aliases to an existing 
`life_threshold`-style handler.

## Finding 5 — Orphan condition handlers
_Declared as `case "X":` in a Go-side dispatcher but ZERO corpus nodes have that kind. 
Most are defensive scaffolds added during r60 era sweeps for kinds the parser MIGHT emit 
on future corpus updates. The list below identifies each by which dispatcher owns it 
(deletion candidates need verification against the parser's actual emit vocabulary)._

**In `detectConditionScaffold` only: 91**

- `all_lands_subtype`
- `all_lands_type`
- `another_typed_etb_this_turn`
- `attacked_this_turn`
- `bargained`
- `card_type_reveal`
- `cast_n_spells_this_turn`
- `colored_mana_spent`
- `control_n_creatures`
- `control_n_lands`
- `control_n_tapped_creatures`
- `counter_doubler`
- `counter_replacement_boost`
- `counters_on_self_ge`
- `creature_etb_last_turn`
- `damage_dealt_to_self_this_turn`
- `damage_taken_this_turn`
- `damaged_creature_died`
- `didnt_die`
- `didnt_enter_this_turn`
- `discarded_nonland`
- `dragon_beheld`
- `drawn_n_cards_this_turn`
- `enters_as`
- `enters_with`
- `equipment_attached`
- `equipped`
- `etb_as`
- `exiled_this_turn`
- `exiled_with_count_ge`
- `first_combat_phase`
- `first_time_resolved_this_turn`
- `full_party`
- `gained_n_life_this_turn`
- `hand_size_ge`
- `hand_size_le`
- `hand_size_lt`
- `hand_size_threshold`
- `it_was_cast`
- `library_empty`
- `life_above_starting`
- `lost_life_last_turn`
- `main_phase`
- `mana_spent`
- `more_cards_in_hand_than_opponents`
- `more_cards_than_each_opponent`
- `more_life_than_opponent`
- `no_counters`
- `no_damage_since_last_turn`
- `no_named_counter`
- `no_named_tokens`
- `no_time_counters`
- `not_their_turn`
- `not_your_turn`
- `opp_more_creatures`
- `opponent_more_creatures`
- `opponent_more_life`
- `permanent_mana_value_le`
- `permanent_mv_le`
- `persist_check`
- `player_no_creatures`
- `put_counter_this_turn`
- `quest_counters`
- `revealed_card_type`
- `self_counter_threshold`
- `self_didnt_etb_this_turn`
- `self_has_no_counter`
- `self_is_suspended`
- `self_is_untapped`
- `shares_creature_type`
- `starting_player`
- `surge_cost_paid`
- `time_counter_on_self`
- `token_doubler`
- `token_replacement_boost`
- `total_toughness`
- `tribute_not_paid`
- `tribute_wasnt_paid`
- `undying_check`
- `was_attacking`
- `was_bargained`
- `was_cast`
- `wasnt_blocking`
- `wasnt_cast`
- `wasnt_creature_type`
- `wasnt_sacrificed`
- `wasnt_subtype`
- `you_control_n_creatures`
- `you_control_n_lands`
- `you_more_life`
- `you_were_starting_player`

**In `goldilocks.go` only: 89**

- `abilities`
- `ability_word`
- `activated`
- `adapt`
- `add_mana`
- `additional_cost`
- `afterlife`
- `artifact`
- `attacking`
- `aura_buff`
- `bestow`
- `blocking`
- `buff`
- `convoke`
- `copy_permanent`
- `copy_spell`
- `counter_mod`
- `counter_spell`
- `create_token`
- `creature`
- `creatures`
- `crew`
- `cycling`
- `damage`
- `destroy`
- `dethrone`
- `devotion`
- `devour`
- `discard`
- `draw`
- `embalm`
- `enchanted`
- `enchantment`
- `equip_buff`
- `escape`
- `etb_with_counters`
- `eternalize`
- `evolve`
- `extra_combat`
- `extra_turn`
- `fabricate`
- `fight`
- `flying`
- `foretell`
- `gain_control`
- `gain_life`
- `grant_ability`
- `instant`
- `kicker`
- `land`
- `legendary`
- `look_at`
- `lose_life`
- `mill`
- `modification_effect`
- `modular`
- `ninjutsu`
- `non`
- `optional`
- `parsed_tail`
- `permanent`
- `persist`
- `planeswalker`
- `prevent`
- `reanimate`
- `regenerate`
- `replacement`
- `revolt`
- `riot`
- `sacrifice`
- `saga_chapter`
- `scry`
- `self_calculated_pt`
- `set_life`
- `shuffle`
- `sorcery`
- `spell`
- `suspend`
- `tap`
- `tapped`
- `token`
- `triggered`
- `tutor`
- `undying`
- `unearth`
- `untapped`
- `upkeep`
- `win_game`
- `you`

## Finding 6 — Duplicate condition dispatch
_Same kind handled in BOTH `detectConditionScaffold` AND `goldilocks.go`. 
Either intentional layering (different priming responsibilities) or accidental 
duplication — needs human review before consolidation._

**Count: 2**

- `paid_optional_cost`
- `you_attacked_this_turn`

---

## Summary table

| Finding | Side | Count | Severity |
|---|---|---:|---|
| Orphan scaffolds (registered, zero corpus) | Trigger | 2 | low (deletion candidates; verify in Go first) |
| Orphan parse-targets (classifier slug, no handler) | Trigger | 0 | n/a |
| Unhandled corpus events | Trigger | 2174 nodes / 79 slugs | medium (per-era residual gap) |
| Truly-unhandled corpus kinds | Condition | 15 nodes / 5 kinds | low (all life-comparison; alias to existing) |
| Orphan handlers (declared, zero corpus) | Condition | 180 | low (defensive scaffolds; verify before deletion) |
| Duplicate dispatch | Condition | 2 | medium (intentional or accidental — needs human review) |
| Duplicate registry keys | Trigger | 0 | n/a |

## Audit methodology

1. Parsed `triggerConditionActions` map literal for registered keys and `classifyTrigger` 
   function body for return slugs. Built a Python mirror of the Go routing logic 
   (exact-event matches, substring catches, opponent-actor heuristic, phase fallback).
2. Walked the AST corpus collecting every `Trigger.event` / `Trigger.phase` / 
   `Trigger.actor.base` and every `Condition.kind`. Tallied counts per slug.
3. Routed each corpus event through the Python mirror; recorded which registry 
   slug it lands on. Unrouted events → unhandled. Slugs with zero hits → orphans.
4. For condition dispatchers, extracted case-strings from `case "X":` and 
   `case "X", "Y":` lines in both `detectConditionScaffold` and `goldilocks.go`. 
   Cross-referenced with corpus kinds; computed orphans and unhandled separately 
   per dispatcher, then union/intersection for duplicates.

**Known limitations:**
- Python mirror of `classifyTrigger` doesn't model every actor-base variant; the 
  2 trigger orphan candidates need Go-level confirmation.
- Condition dispatch from helper functions (`detectPaidOptionalCost`, 
  `detectForEach`, `detectETBAs`, `detectDidPriorAction`) is captured via their 
  delegating `case` lines in `detectConditionScaffold` — kinds these helpers 
  process internally are credited to the parent dispatcher.

## Followups (out of scope for Phase 1C — separate PRs)
- **Trigger orphan scaffolds** — instrument `classifyTrigger` in Go to confirm zero 
  hits across the full corpus before deletion.
- **Unhandled trigger events** — route the top-count slugs to existing scaffolds via 
  the same routing pattern as PRs #447, #451, #453 (era 2/3/1 sweeps).
- **Truly-unhandled condition kinds** — add 5 life-comparison aliases (1-line each) 
  to the existing `life_threshold` dispatch arm.
- **Orphan condition handlers** — bisect against parser commit history to determine 
  which are obsolete (parser emit removed) vs forward-defensive (parser COULD emit).
- **Duplicate dispatch** — review whether `paid_optional_cost` and `you_attacked_this_turn` 
  layered priming is intentional or duplication; consolidate or comment.
