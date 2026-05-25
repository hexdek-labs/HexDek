# Era 3 (2020-2022) Scaffold-Gap Sweep — R60

**Branch:** `dev/era3-scaffold-sweep-r60`
**Date:** 2026-05-25
**Audit script:** `scripts/era3_scaffold_audit.py`
**Prior era sweeps:** Era 2 PR #447 (closed 14.5% → 0.0%), Era 1 / Era 4
prior r60 work.

## Before

The Era 3 audit was the highest-gap era per CLAUDE.md (76.2% in the
original 2026-05-08 corpus snapshot). Pre-sweep state after porting the
trigger-bucketing logic from `era2_scaffold_audit.py`:

```
- Era 3 cards: 798
- Era 3 Condition nodes: 55  (bucketed 44,  unbucketed 11, 20.0% gap)
- Era 3 Trigger nodes:   538 (bucketed 414, unbucketed 124, 23.0% gap)
- Combined: 135 / 593 unbucketed nodes (22.8%)
```

Top unbucketed trigger events (the dominant share of the gap):

| Event slug | Count | Era 3 mechanic |
|---|---:|---|
| `mutates` | 25 | Ikoria mutate |
| `turned_face_up` | 20 | morph / disguise / foretell flip |
| `exploits_creature` | 20 | DTK exploit |
| `specialize_creature` | 8 | Capenna specialize |
| `becomes_target` | 6 | ward triggers |
| `ally_exploits` | 4 | DTK exploit (ally variant) |
| `dealt_damage` | 3 | damage listener |
| `remove_counter` | 2 | counter manipulation |
| `card_put_into_zone` | 2 | zone-change wrapper |
| `becomes_blocked` | 2 | combat listener |
| `unlock_door` | 2 | OTJ / Duskmourn rooms |
| 30 distinct singletons | 30 | foretell, transforms, day/night, cycle_card, commit_crime, conjure, etc. |

Top unbucketed conditions (each a single-card raw fragment):

| Raw text | Card |
|---|---|
| judgment counter (×3 fragments) | Faithbound Judge / Sinner's Judgment |
| `if X was bargained` | Talion's Throneguard |
| `saddled and a creature was dealt damage` | Switchgrass Grazer |
| `if excess damage was dealt this way` | Mephit's Enthusiasm |
| `it didn't have decayed` | Wilhelt, the Rotcleaver |
| `you cast it` | Lutri, the Spellchaser |
| `creature spell and a noncreature spell this turn` | Eshki Dragonclaw |
| `perpetually gains` (×2 cards) | Switchgrass Grazer / Talion's Throneguard |
| Rith excess-damage opponent variant | Rith, Liberated Primeval |

## Root causes

Same shape as the Era 2 sweep: the parser canonicalises Era 3 mechanic
events as bespoke slugs that no substring catch in `classifyTrigger`
matched. Five mechanics (mutate, exploit, specialize, turned-face-up,
unlock-door) each have a materially different priming world than any
existing scaffold; the remaining 30 long-tail slugs reduce cleanly to
existing scaffolds whose primed world fits.

On the condition side, every unbucketed entry is a raw-text fragment
from a one-off card; the audit's `RAW_PATTERNS` list never enumerated
the Era 3 keyword vocabulary (`judgment counter`, `was bargained`,
`saddled`, `excess damage`, `perpetually`, `you cast it`, `decayed`).

## Fix

### Five new trigger scaffolds (`cmd/hexdek-thor/conditional_setup.go`)

| Slug | Primes | Logs event |
|---|---|---|
| `mutates` | `srcPerm.Flags["mutated"] = 1` | `mutate` |
| `turned_face_up` | `srcPerm.Flags["turned_face_up"] = 1`, clear `face_down` | `turned_face_up` |
| `exploits_creature` | place "Exploit Victim" friendly creature | `exploits` |
| `specialize_creature` | `srcPerm.Flags["specialized"] = 1` | `specialize` |
| `unlock_door` | `srcPerm.Flags["room_unlocked"] = srcPerm.Flags["fully_unlocked"] = 1` | `unlock_door` |

### 30 long-tail routing cases

| Event(s) | Routes to | Why that scaffold fits |
|---|---|---|
| `face_up_as`, `as_transform` | `turned_face_up` | same flip semantics |
| `ally_exploits` | `exploits_creature` | same victim+log world |
| `fully_unlock_room` | `unlock_door` | same room state |
| `becomes_target`, `ally_targeted_by_opp`, `becomes_blocked` | `attacks` | combat-style world (opp creature + untapped source) |
| `dealt_damage`, `deals_damage`, `damage_prevented_this_way`, `ally_source_damage` | `combat_damage` | damage listener world |
| `remove_counter`, `counter_put_on_actor`, `counters_put_on_self_any`, `counters_put_on_actor`, `creature_modified_event` | `counters_put_on_self` | counter on source |
| `card_put_into_zone`, `permanent_to_gy`, `card_milled_via`, `compound_card_zone_event` | `creature_dies` | zone-change → graveyard |
| `foretell_card` | `cast_spell` | foretell = cast-from-exile |
| `attached_as`, `equipped_trigger`, `day_night_flip`, `transforms`, `next_time_one_or_more_enter` | `creature_etb` | enter-style listener |
| `cycle_card` | `discard` | cycling = discard+draw |
| `you_commit_crime`, `commit_crime`, `pay_cost_multiple`, `misc_whenever_a`, `you_conjure_one_or_more`, `you_mechanic` | `when_you_do` | reflexive action wrapper |
| `self_or_another_when` | `etb_or_another` | etb-pair scaffold |
| `becomes_state` | `becomes_tapped_trigger` | state-toggle world |
| `player_land_play` | `upkeep` | phase-style no-op |

### Condition raw-text patterns (`scripts/era3_scaffold_audit.py`)

8 new entries in `RAW_PATTERNS`: `judgment_counter`, `was_bargained`,
`saddled_and_damage`, `excess_damage`, `perpetual_effect`,
`didnt_have_keyword`, `you_cast_it`, `cast_creature_and_noncreature`.

## After

```
- Era 3 cards: 798
- Era 3 Condition nodes: 55  (bucketed 54,  unbucketed 1, 1.8% gap)
- Era 3 Trigger nodes:   538 (bucketed 538, unbucketed 0, 0.0% gap)
- Combined: 1 / 593 unbucketed nodes (0.17%)
```

| Side | Before | After | Δ |
|---|---:|---:|---:|
| Conditions | 11 / 55 (20.0%) | 1 / 55 (1.8%) | **−91%** |
| Triggers | 124 / 538 (23.0%) | 0 / 538 (0.0%) | **−100%** |
| Combined | 135 / 593 (22.8%) | 1 / 593 (0.17%) | **−99.3%** |

**Era 3 trigger gap closed completely; condition gap reduced to one
parser-oddity edge case** (Reservoir Kraken: `"if they do, tap this
creature and create a 1/1 blue fish creature token"` — the parser wrapped
an effect clause as a conditional; not worth a one-card pattern).

## Tests

- `cmd/hexdek-thor/era3_r60_trigger_routing_test.go`:
  - `TestClassifyTrigger_Era3R60SweepRouting` — 39 routing assertions
    (5 new-scaffold routes + 30 long-tail routes) with paired "scaffold
    still registered" checks.
  - `TestTriggerConditionActions_Era3R60Apply` — verifies each of the 5
    new scaffolds stamps the expected flag and logs the expected event.
- `go test ./cmd/hexdek-thor/ -count=1 -timeout 180s` — clean.

## Files

- `cmd/hexdek-thor/conditional_setup.go`:
  - 5 new `triggerConditionActions` entries (mutates, turned_face_up,
    exploits_creature, specialize_creature, unlock_door).
  - 36 new event-slug routing cases in `classifyTrigger`.
- `cmd/hexdek-thor/era3_r60_trigger_routing_test.go` — new file.
- `scripts/era3_scaffold_audit.py`:
  - Trigger-bucketing block ported from `era2_scaffold_audit.py`.
  - 5 new scaffold slugs added to `TRIGGER_EVENT_EXACT`.
  - 30 long-tail slugs added to `TRIGGER_EXTRA_EXACT`.
  - 8 new condition `RAW_PATTERNS` entries.
- `data/rules/era3_scaffold_audit.md` — regenerated.
- `docs/era3-scaffold-sweep-r60.md` — this report.
