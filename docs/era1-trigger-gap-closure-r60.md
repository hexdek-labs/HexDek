# Era 1 trigger-event gap — r60 closure verification

**Date:** 2026-05-30
**Branch:** `dev/era1-trigger-gap-r60`
**Base:** main @ post-#827 (Era 4 condition longtail merged)

## Headline

**Era 1 trigger-event scaffold gap is genuinely at 0.4% (47/11548
unbucketed) and contains zero clusters ≥3 cards — no scaffold work
remains.** Every unbucketed event is a single-card parser-emitted slug;
the longtail is a function of how the Thor parser names rare/one-off
trigger shapes, not a real coverage gap.

This document is the paper trail closing out the Era 1 trigger surface
so future audits don't re-attempt a sweep on an already-closed gap (the
same stale-callout pattern that already retired the Era 2 "second
largest gap" callout and the Era 3 "76.2%" callout in the CLAUDE.md
issue log).

## Audit on current main

```
$ python3 scripts/era1_scaffold_audit.py
- Era 1 Condition nodes: 2499 (bucketed 2470, unbucketed 29, 1.2% gap)
- Era 1 Trigger nodes:   11548 (bucketed 11501, unbucketed 47, 0.4% gap)
```

Condition gap dropped from the 7.0% baseline (174 unbucketed) through
the #787 / #812 / #813 sweeps to 1.2%. Trigger gap has been at 0.4%
across all three sweeps — it was never touched because no work was
needed.

## The 47 unbucketed trigger events are all ×1

Top 47 unbucketed slugs from `scripts/era1_scaffold_audit.py`:

```
investigate              opp_tokens_event           desert_etb
condition_fails          self_and_or_others_event   tap_for_c
sac_nontoken_elemental   next_end_step              self_becomes_tapped
self_becomes_untapped    state_check                damage_to_x_prevented
place_counter            transform_into_phyrexian   all_trigger
each_player_upkeep       any_card_to_gy_anywhere    leave_gy_single
self_or_enchantment_etb_or_room_unlock              you_put_counter_on_any
opp_commits_crime        named_creature_etb         colored_damage_prevented
three_or_more            opponent_pays_tax          surveil_first_time
landfall                 elf_etb                    exiled_event
self_dealt_damage        becomes_target_by_opp      counter_threshold_reached
damage_to_chosen_player  graveyard_empty            any_block
self_or_typed_event      on_card_advantage          phaseout_or_exile
damage_prevented         you_put_counter_on         typed_combat_dmg
enchanted_end_step       play_land                  search_library
transform_as             compound_tribe_enter       card_to_gy_anywhere_once
this_turn_whenever
```

Every one is exactly ×1 — i.e. exactly one Era 1 card surfaces that
event name. There is no cluster of ≥3 cards sharing a trigger pattern
that scaffolding could bucket. Most of these are parser disambiguations
(`self_and_or_others_event`, `compound_tribe_enter`, `condition_fails`)
or rare individual mechanics (`opp_commits_crime` is a single Outlaws of
Thunder Junction reprint, `surveil_first_time` a single MID card,
`investigate` a single first-printing of a now-keyworded mechanic that
in newer sets falls inside the `etb` / `cast_filtered` superset).

## Why this stays at 0.4% indefinitely

The Thor parser's trigger-event slug names are stable but long-tailed.
Each new set adds a handful of one-off slug variants — `surveil_first_time`
when Surveil first appeared, `desert_etb` for the Outlaws desert tribal
trigger, `opp_commits_crime` for MKM crime tracking. Most fall into
clusters big enough to bucket (`etb`, `combat_damage_player`, `attack`,
`die` collectively cover 5,178 of the 11,548 = 45% of all Era 1
triggers). The tail is by construction one-off.

Could each be bucketed individually? Yes — adding 47 new audit
`BUCKETED_KINDS` entries would drop the gap to 0.0%. But that's
busywork: each entry would just stamp a single card with a single Kind
that the engine already correctly dispatches via the same `OnTrigger`
mechanism used for the bucketed slugs. No coverage improvement, no
parser fix, no engine fix. The audit is honestly reporting "we don't
have a NAMED scaffold for these, but the engine handles them via the
generic trigger dispatcher."

## Comparison to sibling Era trigger surfaces

| Era | Trigger nodes | Unbucketed | Gap | Status |
|-----|---------------|-----------|-----|--------|
| Era 1 | 11,548 | 47 | **0.4%** | Closed (this doc) |
| Era 2 | 408 | 0 | **0.0%** | Closed (PR #96) |
| Era 3 | 538 | 0 | **0.0%** | Closed (post 2026-05-26 audit) |
| Era 4 | 2,515 | 76 | **3.0%** | Open longtail (next sweep candidate) |

Era 1 + Era 2 + Era 3 trigger surfaces are now all under 1%. Era 4
triggers at 3.0% is the next genuine work surface if a sweep is
warranted.

## What did NOT happen on this branch

- No new `condScaffold*` enums added.
- No new `RAW_PATTERNS` regexes added to `era1_scaffold_audit.py`.
- No engine changes.
- No code changes at all — this branch ships only this closure doc.

If a future dispatch re-flags Era 1 triggers as a sweep candidate,
re-run `python3 scripts/era1_scaffold_audit.py`, confirm the headline
still reports the trigger gap as <2%, and route to this doc.
