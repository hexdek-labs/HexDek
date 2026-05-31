# Engine Event Registry

This document enumerates every trigger event the HexDek engine fires via `FireCardTrigger(gs, eventName, ctx)`, the canonical Kind it lands under after `event_aliases.go` normalization, the per_card handlers registered to listen for it, and the dead surfaces (handlers on events that never fire, events that fire with zero handlers).

Source data: live audit of `internal/gameengine/**/*.go` on 2026-05-30. Audit script outline is in §4 — re-running it against future tree state regenerates the dead-surface lists.

## 1. Surface summary

| Metric | Count |
|---|---|
| Distinct `FireCardTrigger` event names (strict regex, prod only) | 79 |
| Distinct `OnTrigger(name, eventName, …)` event names (prod only) | 91 |
| Aliases declared in `event_aliases.go` | 169 entries |
| Canonical events (post-alias normalization) — fired | 79 |
| Canonical events — listened | 66 |
| Dead handlers (canonical events listened-to with **zero** fire sites) | **4** (13 handlers across 11 cards) |
| Dead events (canonical events fired with **zero** OnTrigger listeners) | 17 (most are intentional audit-event surfaces) |

The `Kind` strings on `gs.LogEvent(Event{Kind: …})` are a separate surface — not enumerated here. See `internal/hexapi/showmatch.go` and `internal/gameengine/invariants.go` for the read-side consumers of those.

## 2. Canonical events fired by the engine

Listed in order of fire frequency (per_card-surface, prod only). Each row records: the canonical event Kind, the engine site(s) firing it, the ctx-map shape (key → meaning), the count of per_card `OnTrigger` listeners post-alias normalization, and a representative listener.

### permanent_etb (fire sites: 78, listeners: 104)

ETB self-trigger for every permanent entering the battlefield.

- **Fires from**: `stack.go:resolvePermanentSpellETB` (cast-resolve path), `etb_dispatch.go:dispatchETB` (token-mint / blink / reanimate paths), `cmd/hexdek-loki` chaos-board scaffolding.
- **Ctx keys**:
  - `perm` (`*Permanent`) — the entering permanent
  - `controller_seat` (int) — its controller's seat index
- **Aliases that route here**: `creature_enters_battlefield`, `land_entered_battlefield`, `permanent_entered_battlefield`
- **Representative listener**: `OnTrigger("Soul Warden", "permanent_etb", …)` — fires lifegain on any creature entering.

### creature_attacks (fire sites: 52, listeners: 111)

Per-attacker trigger, fires once per declared attacker after combat legality passes.

- **Fires from**: `combat.go:DeclareAttackers` (after the §509.1 legality pass).
- **Ctx keys**:
  - `attacker_perm` (`*Permanent`) — the attacking creature
  - `defender_seat` (int) — the seat being attacked (or `-1` for planeswalker)
  - `defender_perm` (`*Permanent`, optional) — present when attacking a planeswalker / battle
- **Aliases that route here**: `attack`, `attacks`, `declare_attackers`, `you_attack`, `you_attack_with`, `attack_while_saddled`, `attack_declared`, `combat_attackers_declared`
- **Representative listener**: `OnTrigger("Edgar Markov", "creature_attacks", …)` — Vampire-tribal +1/+1 counter distribution.

### spell_cast (fire sites: 44, listeners: 84)

Generic spell-cast event; fires once per cast on stack push (CR §601.2).

- **Fires from**: `stack.go:PushSpell` after cost is paid and legality confirmed.
- **Ctx keys**:
  - `card` (`*Card`) — the cast card
  - `controller_seat` (int) — caster
  - `card_name`, `card_types` (convenience copies)
  - `mana_cost_paid` (int)
- **Aliases that route here**: `cast`, `cast_filtered`, `cast_any`, `any_player_cast`, `cast_spell`, `cast_color_spell`, `opp_cast`, `opp_cast_spell`, `opp_cast_color_spell`, `cast_mana_value`, `cast_color_filtered`, `cast_x_spell`, `you_next_cast`, `any_cast`, `cast_self`
- **Representative listener**: `OnTrigger("Niv-Mizzet, Parun", "spell_cast", …)` — draw+damage on any instant/sorcery.

### combat_damage_player (fire sites: 44, listeners: 77)

Combat damage dealt to a player by a creature (CR §510.1).

- **Fires from**: `combat.go:DealCombatDamageStep` per source-target pair.
- **Ctx keys**:
  - `source_perm` (`*Permanent`) — the attacking/blocking creature
  - `target_seat` (int) — defending seat
  - `amount` (int) — damage dealt (post-prevention, post-redirect)
- **Aliases that route here**: `deals_combat_damage`, `combat_damage_dealt`, `creature_combat_damage_to_player`, `self_deals_damage_player`, `group_combat_damage_player`, `combat_damage`, `combat_damage_creature`, `combat_damage_player_or_pw`, `combat_damage_opponent`, `combat_damage_player_or_battle`, `combat_damage_to_player`, `self_combat_damage`, `compound_tribe_combat_damage`, `one_or_more_creatures_combat_damage`
- **Representative listener**: `OnTrigger("Yuriko, the Tiger's Shadow", "combat_damage_player", …)` — top-deck reveal + ping.

### upkeep / upkeep_controller (fire sites: 12+29, listeners: 15+97)

Beginning-of-upkeep trigger, controller-scoped (CR §503.1). Active seat only.

- **Fires from**: `phases.go:FirePhaseTriggers` on the `upkeep` step boundary. Engine emits the alias name `upkeep`; aliases route both `upkeep_controller` and `upkeep_start` to it.
- **Ctx keys**:
  - `phase` ("beginning"), `step` ("upkeep"), `active_seat` (int)
  - For card-scoped emits (`upkeep_controller`): `perm` (`*Permanent`), `controller_seat` (int)
- **Aliases that route here**: `upkeep_controller`, `upkeep_start`, `your_upkeep`, `each_upkeep`
- **Representative listener**: `OnTrigger("Erebos, Bleak-Hearted", "upkeep_controller", …)` — payment of 2 life to draw.

### end_step (fire sites: 13, listeners: 86)

End-of-turn-cleanup beat (CR §513.1).

- **Fires from**: `phases.go:FirePhaseTriggers` on the `end` step boundary.
- **Ctx keys**: `phase` ("end"), `step` ("end_step"), `active_seat` (int)
- **Aliases that route here**: `end_step_controller`, `at_end_step`, `at_beginning_of_end_step`
- **Representative listener**: `OnTrigger("Marrow-Gnawer", "end_step", …)` — tokens stay on end.

### creature_dies (fire sites: 16, listeners: 82)

Creature died, including via combat damage, SBAs, or sacrifice (canonical zone-change → `die`).

- **Fires from**: `sba.go:destroyPermSBA`, `zone_change.go:sacrificePermanentImpl`, `combat.go:applyDamageAfterPrevention`
- **Ctx keys**: `perm` (`*Permanent`, may be detached post-death), `seat` (int, controller at death), `from_combat_damage` (bool, when known)
- **Aliases that route here**: `die`, `dies`, `another_creature_dies`, `another_creature_dies_any`, `another_nontoken_creature_dies_any`, `tribe_you_control_dies`, `another_typed_dies`
- **Representative listener**: `OnTrigger("Cruel Celebrant", "creature_dies", …)`

### permanent_ltb (fire sites: 11, listeners: 87)

Leaves-the-battlefield trigger for ANY permanent (not just creatures).

- **Fires from**: All 6 LTB paths: `DestroyPermanent`, `ExilePermanent`, `sacrificePermanentImpl`, `BouncePermanent`, `destroyPermSBA`, `sacrificePermSBA`.
- **Ctx keys**: `perm` (`*Permanent`), `from_zone` ("battlefield"), `to_zone` (varies)
- **Aliases that route here**: `self_leaves_battlefield`, `type_leaves_battlefield`, `another_ally_leaves`, `enchanted_ltb`
- **Representative listener**: `OnTrigger("Lord Skitter, Sewer King", "permanent_ltb", …)`

### instant_or_sorcery_cast (fire sites: 16, listeners: 28)

Cast of an instant or sorcery, fires after the §601.2 spell-cast pass.

- **Fires from**: `stack.go:PushSpell` filtered for `IsInstant() || IsSorcery()`.
- **Ctx keys**: `card` (`*Card`), `controller_seat` (int)
- **Representative listener**: `OnTrigger("Storm-Kiln Artist", "instant_or_sorcery_cast", …)`

### tribute_resolved (fire sites: 15, listeners: 6) — fully wired (PR #710)

Per_card consumer for tribute-keyword cards. Fires once per tribute-bearing creature on ETB after tribute decision is made.

### class_level_up (fire sites: 15, listeners: 6) — class-level pump trigger

### card_cycled (fire sites: 15, listeners: 6) — cycling cost paid trigger

…full set continues below in §4 audit data. Most events follow the same shape: engine site → ctx map → per_card listeners with alias normalization.

## 3. Dead surfaces

### 3.1 Dead handlers (events that never fire — handlers silently never run)

**4 canonical events, 13 handler registrations across 11 distinct cards.** Each is a real engine-level bug: the per_card author registered a handler expecting a phase-step event the engine does not emit.

| Canonical event | Listener count | Cards affected | Fix surface |
|---|---|---|---|
| `untap_step` | 2 | Seedborn Muse, Rasputin Dreamweaver | **Closed in this PR**: added `"untap_step": {"untap"}` to `event_aliases.go`. Engine fires `untap`; aliasing routes the handlers. |
| `draw_step_controller` | 2 | Sylvan Library, Nekusar the Mindrazer | **Closed in this PR**: added `"draw_step_controller": {"draw_step"}` to `event_aliases.go`. Engine fires `draw_step`; aliasing routes the handlers. |
| `postcombat_main_controller` | 7 | Tymna the Weaver, Kona (Rescue Beastie), Megatron (Tyrant), Sorin of House Markov, Kirri (Talented Sprout) | **Open**: engine `FirePhaseTriggers` only switches on `upkeep` / `draw` / `end` / `untap`. There is no postcombat-main fire surface at all. Needs a new `case "main_postcombat"` branch and a phase-loop emit. Aliasing alone won't fix this — engine has to emit the event. Tracked as a TODO in the engine open-issues queue. **Pre-this-PR Tymna silently drew zero cards from combat damage**; same for the other 4 cards. |
| `upkeep_opponent` | 2 | Slicer (Hired Muscle) | **Open**: Slicer's "at the beginning of each opponent's upkeep, this creature deals 1 damage to you" needs an opp-seat fire of upkeep — the engine currently only fires `upkeep` on the active seat. Needs an `upkeep_opponent` fire from `phases.go` for each non-active seat. Aliasing to `upkeep` would not work (would fire on Slicer's controller, not on opponents). Tracked as a TODO. |

The 2 alias fixes in this PR close 4 of the 11 affected cards. The 9 cards behind the 2 Open surfaces (Tymna, Kona, Megatron, Sorin, Kirri, Slicer) need a follow-up engine PR. Heimdall post-game analytics will surface these as "no trigger observed" cells in the trigger-completeness audit until that PR lands.

### 3.2 Dead events (events that fire with zero `OnTrigger` listeners)

**17 events.** Audit: each was checked for `gs.LogEvent(Event{Kind:…})` consumers (showmatch / invariants / tests). The vast majority are intentional audit-event surfaces, not bugs. Listed here for visibility.

| Canonical event | Fire site | Status |
|---|---|---|
| `bargain_paid` | `keywords_bargain.go:224` (`FireCardTrigger`) + `costs.go:377` (`LogEvent`) | **Intentional audit event.** No per_card consumer — bargain riders read `StackItem.CostMeta["bargained"]` directly via `OnResolve` / `OnETB` hooks (see `per_card/bargain_consumers_r60.go:1`). The fire is future-proofing for handlers that may want to subscribe + load-bearing for test invariants. |
| `battalion_triggered` | `combat.go` (mechanism-specific) | **Audit event** for the §70x.x Boros battalion ability word. Mechanism handled inline in combat resolution. |
| `became_solved` / `becomes_renowned` / `becomes_defeated` | per-card emits | **Audit events** for state-change observability; per_card consumers use OnCounterAdded / OnDamage hooks instead. |
| `beheld` | dragon-beheld mechanic | **Audit event**; consumers read `Counters["beheld"]` directly. |
| `channel_activated` / `manifest_flipped` | activation paths | **Audit events** for replay; per_card consumers hook the underlying activation flow. |
| `draw_step` | `phases.go:156` | **Now reachable** via the `draw_step_controller` alias added in this PR. Pre-alias was technically reachable through direct `OnTrigger("X", "draw_step", …)` but no card registered that exact name. |
| `first_crime` | crime keyword | **Audit event** — `commit_crime` is the live trigger. |
| `gift_paid` / `gift_promised` / `gift_delivered` | gift mechanic | **Audit events**; gift handlers consume `Promised` flag directly. |
| `pack_tactics_triggered` | combat.go | **Audit event**; effect resolved inline. |
| `room_unlocked` | `unlock_room` is the live alias | **Intentional** — `unlock_room` is the canonical, `room_unlocked` was the legacy spelling; per_card handlers all migrated. |
| `suspected` / `unsuspected` | suspect mechanic | **Audit events** for the WOE suspect keyword. |
| `untap` | `phases.go:160` | **Now reachable** via the `untap_step` alias added in this PR. |
| `visited` | dungeon mechanic | **Audit event**; dungeon room visits resolve inline. |

No code change for any item in §3.2 — these are documented for visibility, not removal targets.

## 4. Audit script

The lists in §3 were generated by:

```bash
# 1. listened events (per_card OnTrigger registrations)
grep -rhE 'OnTrigger\("[^"]+",\s*"[^"]+"' internal/gameengine/per_card/ --include="*.go" \
  | grep -v _test.go \
  | grep -oE 'OnTrigger\("[^"]+",\s*"[a-z_0-9]+"' \
  | grep -oE '"[a-z_0-9]+"$' \
  | sort | uniq -c | sort -rn > /tmp/listened.txt

# 2. fired events (FireCardTrigger call sites — strict 3-arg regex)
grep -rnE 'FireCardTrigger\(gs,\s*"[a-z_0-9]+",' internal/gameengine/ --include="*.go" \
  | grep -v _test.go \
  | grep -oE 'FireCardTrigger\(gs,\s*"[a-z_0-9]+",' \
  | grep -oE '"[a-z_0-9]+"' \
  | sort -u > /tmp/fired_strict.txt

# 3. cross-ref via event_aliases.go (Python — see /tmp/event_xref2.py)
```

Re-running this against future trees regenerates dead-surface lists. New cards that register on events the engine doesn't fire will appear as new entries in §3.1; new engine-emitted events that no card listens to will appear in §3.2.

## 5. Related docs

- `event_aliases.go` — the canonical alias table (169 entries). New parser-emitted event names should be added here, not as parallel `FireCardTrigger` calls.
- `internal/gameengine/observer_triggers.go` — observer-pattern ETB/cast event dispatching.
- `internal/gameengine/reflexive_triggers.go` — reflexive trigger registration for "when you do" cascading paths.
- `internal/hexapi/showmatch.go:4060+` — the canonical consumer of `LogEvent` Kinds for the live spectator log (separate surface from per_card triggers).
- PR #830 / #849 / #870 — event-Kind normalization waves 1 / 2 / 3 (lose_game / untap / discard).
