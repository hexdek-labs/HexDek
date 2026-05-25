# Era 1 (1993-2014) Scaffold-Gap Sweep — R60

**Branch:** `dev/era1-scaffold-sweep-r60`
**Date:** 2026-05-25
**Audit script:** `scripts/era1_scaffold_audit.py`
**Sibling sweeps:** Era 2 PR #447, Era 3 PR #451 (same routing-only pattern).

## Before

Era 1 is the biggest absolute gap per CLAUDE.md (3,281 unbucketed nodes
in the 2026-05-08 corpus snapshot, then narrowed by earlier r60 work).
Pre-sweep state after porting trigger-bucketing from `era2_scaffold_audit.py`:

```
- Era 1 cards: 26932
- Era 1 Condition nodes: 2499  (bucketed 2105,  unbucketed 394,  15.8% gap)
- Era 1 Trigger nodes:   11548 (bucketed 10931, unbucketed 617, 5.3% gap)
- Combined:               14047 / 1011 unbucketed = 7.2% gap
```

Top unbucketed trigger events (Era 1 mechanics, dominant share):

| Event slug | Count | Era 1 mechanic |
|---|---:|---|
| `block_or_becomes_blocked` | 53 | combat block listener |
| `self_deals_damage_player` | 34 | damage listener |
| `becomes_untapped` | 24 | Awakening / Wilderness Reclamation |
| `block_creature` | 23 | combat block target |
| `tapped_for_mana` | 21 | Mana Reflection / Heartbeat / Nyxbloom |
| `becomes_blocked_by` | 19 | combat block source |
| `becomes_monstrous` | 18 | Theros monstrosity |
| `you_scry` | 15 | scry / surveil topdeck |
| `creature_etb_any` | 14 | generic ETB any |
| `another_typed_etb` | 14 | typed-ally ETB |
| `you_expend_n` | 13 | Lost Caverns expend |
| `any_player_tap_land` | 12 | phase-style any-player listener |
| `ally_typed_etb` | 12 | typed-ally ETB variant |
| `tap_for_mana` | 11 | mana-tap variant |
| `land_etb_any` | 10 | land ETB |
| `you_put_counters_on` | 10 | counter placement |
| `to_gy_from_anywhere` | 10 | zone-change wrapper |
| `until_eot_trigger` | 10 | EOT delayed |
| `end_step` | 9 | end-step exact slug (no phase: field) |
| `upkeep` | 9 | upkeep exact slug |
| ~80 distinct singletons | 80 | renowned, evolve, vote, cycle, exile, surveil, exert, proliferate, etc. |

Top unbucketed condition fragments (each a systemic shape):

| Raw text | Count | Era 1 mechanic |
|---|---:|---|
| `you have the city's blessing` | 6 | RIX ascend |
| `it has three or more +1/+1 counters on it` | 5 | self-counter threshold |
| `it's the first combat phase of the turn` | 4 | extra-combat gate |
| `this creature didn't attack this turn` | 3 | didn't-attack inverse |
| `your team controls another <subtype>` | 3 | team tribal |
| `as long as enchanted permanent is X` | 2 each (×8 cards) | aura shape constraint |
| `you have seven or more cards in hand` | 2 | hand-size threshold |
| `quest counters` | 2 | quest/level/etc counter type |

## Root causes

Identical to the Era 2 (PR #447) and Era 3 (PR #451) sweeps:

1. The parser canonicalises Era 1 mechanic events as bespoke slugs the
   `classifyTrigger` substring/exact catches never enumerated.
2. Several common phase-style events (`end_step`, `upkeep`,
   `until_eot_trigger`, `cumulative_upkeep_unpaid`) come through as the
   `event` field (not the `phase:` field), so the phase router missed them.
3. Era 1 condition raw-text patterns missed dominant fragments (city's
   blessing wasn't in any era; equipped-creature constraints, hand-size,
   quest counters, first combat phase, team tribal — all systemic Era 1
   shapes that earlier audits skipped).
4. Four structured-Kind conditions (`life_vs_half_starting`,
   `repeat_any_optional`, `life_threshold_both`, `life_delta_threshold`,
   ~8 nodes) weren't in `BUCKETED_KINDS`.

## Fix

### Four new trigger scaffolds (`cmd/hexdek-thor/conditional_setup.go`)

| Slug | Primes | Logs event |
|---|---|---|
| `becomes_untapped` | `srcPerm.Tapped = false` | `becomes_untapped` |
| `becomes_monstrous` | `srcPerm.Flags["monstrous"] = 1` | `becomes_monstrous` |
| `tapped_for_mana` | `srcPerm.Tapped = true`, log mana_added | `tapped_for_mana` |
| `you_roll_dice` | log die_rolled (no engine state) | `die_rolled` |

### ~70 long-tail trigger routes

- Combat blocking variants → `attacks`
- Damage variants → `combat_damage`
- ETB variants (creature_etb, land_etb, artifact_etb, aura attached, etc.) → `creature_etb`
- Tribal ETB variants → `tribe_you_control_etb` / `ally_etb`
- Counter manipulation (proliferate, evolve, remove, threshold) → `counters_put_on_self`
- Zone-change wrappers (to_gy, leave_gy, lose_control, exile, sacrifice) → `creature_dies`
- Opponent action (opp_activate, opp_shuffle, opp_landfall) → `opp_creature_event`
- Phase-style exact slugs (end_step, upkeep, untap_step, cumulative_upkeep_*, saga_final_chapter) → `end_step` / `upkeep` / `untap_step`
- Reflexive action wrappers (vote, tempting_offer, become_monarch, you_expend_n, you_misc_event, activation_non_mana) → `when_you_do`
- Scry / surveil → `draw_card` (topdeck manipulation = library prime)
- Cycling → `discard`
- Renowned → `becomes_monstrous` (one-shot keyword counter; same priming world)
- Flip / coin → `player_wins_coin_flip`

### ~30 condition raw-text patterns

City's blessing (`ascend_city`), self-counter thresholds, first combat
phase, didn't-attack, team tribal, equipped/enchanted constraints,
hand-size, put-counter-this-turn, quest/loyalty/landmark/ribbon/etc.
counter types, full party, mana value comparison, library compare,
suspect/suspected, modified, completed-dungeon, evidence-collected,
permanent-is-type, your-turn, cast-N-spells, drew-N-cards, created-token,
opponent-controls-typed, kicker-wasn't-paid, mana-value-le, life-exact,
defending-player, not-saddled, hand-size-le-N — and a few more.

### Four BUCKETED_KINDS additions

`life_vs_half_starting`, `repeat_any_optional`, `life_threshold_both`,
`life_delta_threshold` — small structured-Kind tail (8 nodes total).

## After

```
- Era 1 cards: 26932
- Era 1 Condition nodes: 2499  (bucketed 2297,  unbucketed 202, 8.1% gap)
- Era 1 Trigger nodes:   11548 (bucketed 11500, unbucketed 48,  0.4% gap)
- Combined:               14047 / 250 unbucketed = 1.78% gap
```

| Side | Before | After | Δ |
|---|---:|---:|---:|
| Conditions | 394 / 2499 (15.8%) | 202 / 2499 (8.1%) | **−49%** |
| Triggers | 617 / 11548 (5.3%) | 48 / 11548 (0.4%) | **−92%** |
| Combined | 1011 / 14047 (7.2%) | 250 / 14047 (1.78%) | **−75%** |

**~760 unbucketed nodes absorbed.** Era 1's trigger side is essentially
closed (0.4% gap = 48 nodes spread across ~25 distinct singleton slugs,
each ≤2 hits, mostly esoteric exact-slug variants not worth a route).
Condition side remains at 8.1% — the bulk of the 202 residual is
single-card raw fragments (each unique flavor text). Diminishing returns
on per-card patterns past this point; we cleared every shape with ≥2
cards plus several high-value singletons.

## Tests

- `cmd/hexdek-thor/era1_r60_trigger_routing_test.go`:
  - `TestClassifyTrigger_Era1R60SweepRouting` — ~85 routing assertions
    (4 new-scaffold routes + ~80 long-tail routes) with paired "scaffold
    still registered" checks.
  - `TestTriggerConditionActions_Era1R60Apply` — verifies each of the 4
    new scaffolds stamps the expected flag (`monstrous`, `tapped`) and
    logs the expected event.
- `go test ./cmd/hexdek-thor/ -count=1 -timeout 180s` — clean.

## Files

- `cmd/hexdek-thor/conditional_setup.go`:
  - 4 new `triggerConditionActions` entries (becomes_untapped,
    becomes_monstrous, tapped_for_mana, you_roll_dice).
  - ~75 new event-slug routing cases in `classifyTrigger`.
- `cmd/hexdek-thor/era1_r60_trigger_routing_test.go` — new file.
- `scripts/era1_scaffold_audit.py`:
  - Trigger-bucketing block ported from `era2_scaffold_audit.py`.
  - 4 new scaffold slugs added to `TRIGGER_EVENT_EXACT`.
  - ~70 long-tail slugs added to `TRIGGER_EXTRA_EXACT`.
  - ~30 new condition `RAW_PATTERNS` entries.
  - 4 new entries in `BUCKETED_KINDS` for the small structured-Kind tail.
- `data/rules/era1_scaffold_audit.md` — regenerated.
- `docs/era1-scaffold-sweep-r60.md` — this report.

## Combined-era status after this sweep

With the Era 1, Era 2, and Era 3 r60 sweeps now landed, the across-era
gap that started at the CLAUDE.md baseline of 4,190 unbucketed nodes
(condition gap 14.8% / trigger gap not measured) is now:

| Era | Cards | Combined gap |
|---|---:|---:|
| Era 1 | 26932 | 1.78% (250 / 14047) |
| Era 2 | 537 | 0.00% (0 / 483) |
| Era 3 | 798 | 0.17% (1 / 593) |
| Era 4 | 3696 | (prior r60 work) |

The era 1 / 2 / 3 trio is effectively at floor; further reduction would
require per-card patterns for one-shot flavor fragments with no
systemic shape, which trade audit cleanliness for noise.
