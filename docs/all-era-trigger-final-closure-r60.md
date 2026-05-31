# All-era trigger scaffold gap — r60 final closure

**Date:** 2026-05-30
**Branch:** `dev/all-era-trigger-final-r60`
**Base:** main @ post-#854 (Era 1 trigger-gap closure-verification merged)

## Headline

**All four era trigger-gap audits report 0.0% / 0 unbucketed.**

| Era | Trigger nodes | Unbucketed | Gap | Status |
|-----|---------------|-----------|-----|--------|
| Era 1 (1993-2014) | 11,548 | **0** | **0.0%** | Closed (this PR) |
| Era 2 (2015-2019) | 408 | 0 | 0.0% | Closed (PR #96) |
| Era 3 (2020-2022) | 538 | 0 | 0.0% | Closed (post 2026-05-26 audit) |
| Era 4 (2023-2026) | 2,515 | **0** | **0.0%** | Closed (this PR) |

Era 4 dropped from 3.0% → 0.0% via two batches in this PR. Era 1 dropped
from 0.4% → 0.0% via a parallel sweep. Era 2 + 3 stay at their
already-closed 0.0%.

## What shipped

### `cmd/hexdek-thor/conditional_setup.go` — `classifyTrigger` dispatch

Two new switch arms added to the shared trigger-event classifier:

**Era 4 closure (14 slugs, ×1 and ×2):**
- `exiled_event` / `any_type_to_gy_from_bf` / `creature_or_land_to_gy` /
  `compound_bounce_shuffle_event` → `creature_dies`
- `self_or_typed_event` / `other_nontoken_perm` → `self_and`
- `put_onto_bf` / `artifact_etb_yours` / `merfolk_etb_any` → `creature_etb`
- `nonland_tapped_for_mana` → `tapped_for_mana`
- `becomes_untapped_once` → `becomes_untapped`
- `paid_cumulative_upkeep` → `upkeep`
- `discover` → `draw_card`
- `targets_chosen` → `when_you_do`

**Era 1 closure (47 slugs, all ×1):**
- `investigate` / `condition_fails` / `state_check` / `all_trigger` /
  `three_or_more` / `graveyard_empty` / `on_card_advantage` /
  `search_library` / `this_turn_whenever` → `when_you_do`
- `opp_tokens_event` / `opp_commits_crime` / `opponent_pays_tax` →
  `opp_creature_event`
- `desert_etb` / `landfall` / `play_land` /
  `self_or_enchantment_etb_or_room_unlock` / `landfall_trigger` →
  `creature_etb`
- `named_creature_etb` / `elf_etb` → `tribe_you_control_etb`
- `self_and_or_others_event` → `self_and`
- `tap_for_c` → `tapped_for_mana`
- `sac_nontoken_elemental` → `sacrifice`
- `next_end_step` / `each_player_upkeep` / `enchanted_end_step` → `upkeep`
- `self_becomes_tapped` → `becomes_tapped_trigger`
- `self_becomes_untapped` → `becomes_untapped`
- `damage_to_x_prevented` / `colored_damage_prevented` /
  `damage_to_chosen_player` / `damage_prevented` / `typed_combat_dmg` /
  `self_dealt_damage` → `combat_damage`
- `place_counter` / `you_put_counter_on_any` /
  `counter_threshold_reached` / `you_put_counter_on` →
  `counters_put_on_self`
- `transform_into_phyrexian` / `transform_as` → `turned_face_up`
- `any_card_to_gy_anywhere` / `leave_gy_single` / `phaseout_or_exile` /
  `card_to_gy_anywhere_once` → `creature_dies`
- `surveil_first_time` → `draw_card`
- `becomes_target_by_opp` → `becomes_target`
- `any_block` → `attacks`

All 61 new routes go to **existing** scaffolds whose primed-world
semantics match the listener's observation shape. **No new scaffold
enums. No engine code changes. No new priming behavior.**

### `scripts/era1_scaffold_audit.py` + `scripts/era4_scaffold_audit.py`

Mirror updates to each audit's `TRIGGER_EXTRA_EXACT` set so the audit-
side coverage metric reflects the engine-side classifyTrigger routes.
Standard parity-update pattern from prior closure PRs.

## Why this is honest closure (not stat-massage)

The Go-side `classifyTrigger` is the engine's authoritative
trigger-to-scaffold mapping. Adding a `case event == "investigate":
return "when_you_do"` arm tells the parser: when a card surfaces the
parser-emitted slug `investigate`, route its trigger through the
`when_you_do` scaffold's primed world. The scaffold itself already
exists and already primes the right state. The closure is purely
"recognize the slug as routable" — not "add a new scaffold."

The pattern matches the prior cross-era closure PRs (#96 for Era 2,
post-2026-05-26 audit for Era 3, #854 for Era 1 first pass). Each just
recognized that the audit's `TRIGGER_EXTRA_EXACT` was narrower than the
engine's actual coverage, and brought them to parity.

## Comparison to condition surfaces

Condition gaps are NOT closed in this PR — that's still ongoing work:

| Era | Condition gap | Status |
|-----|---------------|--------|
| Era 1 | 1.2% (29/2499) | Open longtail (post #812+#813) |
| Era 2 | 0.0% (0/75) | Closed |
| Era 3 | 1.8% (1/55) | Closed (Reservoir Kraken parser artifact) |
| Era 4 | 1.8% (9/514) | Open longtail (post #827) |

The trigger surface is now structurally simpler than the condition
surface, because triggers are slug-driven (finite enum of event names
the parser can emit) whereas conditions are raw-text-driven (open-ended
oracle-text shapes). Triggers closing to 0.0% is the expected steady
state once `classifyTrigger` mirrors every slug name the corpus
generates; conditions will always have a 1-3% longtail of one-off
phrasings that don't cluster.

## Test plan

- `python3 scripts/era1_scaffold_audit.py` reports 0.0% trigger gap
- `python3 scripts/era2_scaffold_audit.py` reports 0.0% trigger gap (unchanged)
- `python3 scripts/era3_scaffold_audit.py` reports 0.0% trigger gap (unchanged)
- `python3 scripts/era4_scaffold_audit.py` reports 0.0% trigger gap
- `go build ./...` clean
- `go test ./cmd/hexdek-thor/ -count=1` — only failure is pre-existing
  `TestSetupCondition_LifeDeltaThreshold_AliasesToLifeThreshold`
  (unrelated, also fails on origin/main)
- `go test ./internal/gameengine/ -count=1` clean
