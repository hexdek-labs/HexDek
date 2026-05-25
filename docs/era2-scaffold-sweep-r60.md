# Era 2 (2015-2019) Scaffold-Gap Sweep — R60

**Branch:** `dev/era2-scaffold-sweep-r60`
**Date:** 2026-05-25
**Audit script:** `scripts/era2_scaffold_audit.py`

## Before

```
- Era 2 cards: 537
- Era 2 Condition nodes: 75 (bucketed 75, unbucketed 0, 0.0% gap)
- Era 2 Trigger nodes:   408 (bucketed 349, unbucketed 59, 14.5% gap)
```

Conditions were already fully bucketed coming into the sweep (the older
2026-05-24 r60 work on era 2 closed the condition side). The remaining
gap was entirely on the trigger side.

Top unbucketed trigger events:

| Event slug | Count | Share |
|---|---:|---:|
| `combat_damage_player` | 22 | 37.3% |
| `die` | 9 | 15.3% |
| `beginning_of_ordinal_step` | 4 | 6.8% |
| `group_combat_damage_player` | 4 | 6.8% |
| `etb_as` | 2 | 3.4% |
| `combat_damage_player_or_pw` | 1 | 1.7% |
| `token_event` | 1 | 1.7% |
| `to_graveyard` | 1 | 1.7% |
| `spend_this_mana` | 1 | 1.7% |
| `nontoken_ally_event` | 1 | 1.7% |
| `block` | 1 | 1.7% |
| `conditional_state` | 1 | 1.7% |
| `compound_opp_tribe_event` | 1 | 1.7% |
| `misc_when` | 1 | 1.7% |
| `self_combat_damage` | 1 | 1.7% |
| `one_or_more_typed_event` | 1 | 1.7% |
| `ally_explore` | 1 | 1.7% |
| `self_and_another` | 1 | 1.7% |
| `cycle` | 1 | 1.7% |
| `coin_flip_result` | 1 | 1.7% |
| `nontoken_creature_event` | 1 | 1.7% |
| `combat_damage_opponent` | 1 | 1.7% |
| `lose_game` | 1 | 1.7% |

## Root cause

The dominant 32/59 (54%) gap was a **substring-format mismatch**:
`classifyTrigger`'s substring catches were written for prose ("combat
damage" with a space), but the parser canonicalizes events with
underscores (`combat_damage_player`, `group_combat_damage_player`, etc.).
The prose substring never matched the underscore slug, so the entire
combat-damage family fell out of bucketing.

The remaining ~46% came from:
- Singular forms (`die`, `to_graveyard`) the parser emits alongside the
  plural `dies` substring catch.
- Modal/variant ETB slugs (`etb_as`, `token_event`, `nontoken_*_event`,
  `one_or_more_typed_event`).
- Long-tail Era 2 mechanics (`cycle`, `block`, `coin_flip_result`,
  `lose_game`, `spend_this_mana`, `beginning_of_ordinal_step`).

## Fix

Routing-only — no new scaffolds were registered. Each new event slug
maps to an existing `triggerConditionActions` entry whose priming
semantics already match the slug's runtime needs:

| Event slug(s) | Routes to | Rationale |
|---|---|---|
| `combat_damage_*` (5 variants) | `combat_damage` | same world: opponent creature as target |
| `die`, `to_graveyard` | `creature_dies` | same world: friendly Setup Victim creature |
| `etb_as` | `creature_etb` | same world: ETB Buddy + source ETB |
| `cycle` | `discard` | cycling = discard + draw; hand+library priming covers it |
| `block` | `attacks` | block requires the attacks priming (opp creature + untapped source) |
| `coin_flip_result` | `player_wins_coin_flip` | existing flip scaffold logs the event |
| `lose_game` | `sacrifice` | sac-fodder creature feeds the "if a player would lose" redirect |
| `beginning_of_ordinal_step` | `upkeep` | phase no-op (fireTriggerEvent advances) |
| `token_event` | `creature_etb` | token entering = ETB |
| `nontoken_ally_event`, `ally_explore` | `ally_etb` | ally ETB world |
| `nontoken_creature_event` | `creature_etb` | friendly creature ETB |
| `compound_opp_tribe_event` | `opp_creature_event` | opp-tribal scaffold |
| `one_or_more_typed_event` | `tribe_you_control_etb` | typed ETB listener |
| `self_and_another` | `self_and` | existing pair scaffold |
| `conditional_state`, `misc_when` | `when_you_do` | reflexive flag carrier |
| `spend_this_mana` | `you_get_energy` | resource-spend event logger |

The audit's Python-side `TRIGGER_SUBSTRING_CATCHES` was updated to
include the underscore-form `combat_damage` mirror; the long-tail slugs
were added to a new `TRIGGER_EXTRA_EXACT` set so the audit reflects the
Go-side routing.

## After

```
- Era 2 cards: 537
- Era 2 Condition nodes: 75 (bucketed 75,  unbucketed 0, 0.0% gap)
- Era 2 Trigger nodes:   408 (bucketed 408, unbucketed 0, 0.0% gap)
```

**59 → 0 unbucketed trigger events. Trigger gap 14.5% → 0.0% (−100%).**

Both condition and trigger gaps for Era 2 are now closed.

## Tests

- `cmd/hexdek-thor/era2_r60_trigger_routing_test.go` — pins all 23 new
  event-slug → scaffold routes with a paired "scaffold still registered"
  assertion so a future refactor that deletes a scaffold can't silently
  re-open the gap.
- `go test ./cmd/hexdek-thor/ -count=1` — clean (no regressions on the
  existing 19-slug `era2_r60_trigger_scaffolds_test.go` from the earlier
  sweep).

## Files

- `cmd/hexdek-thor/conditional_setup.go` — added 23 new event-slug cases
  to `classifyTrigger`'s switch, all routing to existing scaffolds.
- `cmd/hexdek-thor/era2_r60_trigger_routing_test.go` — new regression file.
- `scripts/era2_scaffold_audit.py` — added underscore-form
  `combat_damage` substring catch and `TRIGGER_EXTRA_EXACT` set.
- `docs/era2-scaffold-sweep-r60.md` — this report.
