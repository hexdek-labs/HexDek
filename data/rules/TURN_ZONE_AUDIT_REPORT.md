# Turn Structure & Zone Rules Audit Report

**Auditor:** Claude Opus 4.6  
**Date:** 2026-04-16  
**CR Reference:** `data/rules/MagicCompRules-20260227.txt`  
**Engine Files Audited:**
- `internal/tournament/turn.go` (turn loop)
- `internal/gameengine/phases.go` (phase/step helpers)
- `internal/gameengine/combat.go` (combat phase)
- `internal/gameengine/stack.go` (stack + priority + ETB)
- `internal/gameengine/sba.go` (state-based actions)
- `internal/gameengine/state.go` (GameState, Seat, zones)
- `internal/gameengine/zone_cast.go` (zone-cast primitives)
- `internal/gameengine/resolve.go` (effect resolution)
- `internal/gameengine/replacement.go` (replacement events)
- `internal/gameengine/commander.go` (FireZoneChange)
- `internal/gameengine/triggers.go` (APNAP ordering)
- `internal/gameengine/per_card_hooks.go` (ETB/cast hooks)
- `internal/gameengine/mana.go` (DrainAllPools)
- `internal/gameengine/dfc.go` (day/night)

---

## EXECUTIVE SUMMARY

The engine's turn structure is **substantially correct** and covers all 5 phases and their constituent steps in the right order. Phase triggers, SBAs, priority rounds, and combat sub-steps are all present and fire at the right moments. The zone model correctly implements all 7 zones with appropriate properties.

**22 issues found:** 3 HIGH, 8 MEDIUM, 11 LOW.

The most critical gaps are:
1. **No "dies" / LTB / LTB-exile zone-change triggers** (HIGH) -- the engine fires ETB triggers but has no corresponding system for death triggers, LTB triggers, or "when exiled" triggers
2. **Cleanup step does not repeat when triggers fire** (HIGH) -- CR 514.3a requires another cleanup step if SBAs fire or triggers queue
3. **resolveDestroy / resolveExile / resolveBounce skip FireZoneChange** (HIGH) -- these resolve.go functions bypass the replacement-effect chain and zone-change event dispatch that the SBA path uses

---

## PART 1: TURN STRUCTURE AUDIT (CR Section 5)

### 501 Beginning Phase

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Phase exists and fires | 501.1 | PASS | `turn.go` L103: `gs.Phase, gs.Step = "beginning", "untap"` |
| Three steps in order: untap, upkeep, draw | 501.1 | PASS | Lines 103, 121, 135 set steps in correct order |

### 502 Untap Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Phasing happens first (in -> out, out -> in) | 502.1 | PASS | `UntapAll()` calls `PhaseInAll()` first (phases.go L589). Phase-out of phasing permanents handled. |
| Day/night transition | 502.2 | PASS | `EvaluateDayNightAtTurnStart()` called at L96, before untap (correct per 502.2) |
| Untap all permanents simultaneously | 502.3 | PARTIAL | Permanents untap sequentially in a loop (phases.go L611-698). CR says "simultaneously" but sequential is functionally equivalent absent untap-triggered abilities. |
| No priority during untap | 502.4 | PASS | No `PriorityRound()` call during untap step in turn.go |
| "Doesn't untap" respected | 502.2/502.3 | PASS | `DoesNotUntap` flag checked at L619; stun counters at L644-681 |
| Triggers during untap held until upkeep | 502.4 | PASS | `FireDelayedTriggers` and `ScanExpiredDurations` fire at untap (L104-105), but trigger effects queue to stack; priority not given until upkeep |
| Summoning sickness cleared | 302.1 | PASS | `p.SummoningSick = false` at L615 |
| Mana pool drained | 500.4 | PASS | `seat.ManaPool = 0` at L108 |
| Lands-played reset | -- | PASS | `clearPlayedLand(gs, active)` at L109 |
| Skip untap support | 502.1 | PASS | `seat.SkipUntapStep` check at L592 |

### 503 Upkeep Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step set correctly | 503.1 | PASS | `gs.Phase, gs.Step = "beginning", "upkeep"` at L121 |
| "At beginning of upkeep" triggers fire | 503.1a | PASS | `FirePhaseTriggers(gs, gs.Phase, gs.Step)` at L124; `triggerMatchesPhaseStep` matches "upkeep" |
| Untap-step triggers also fire here | 503.1a | PASS | Triggers queued during untap resolve when priority opens |
| Priority given | 503.1 | IMPLICIT | No explicit `PriorityRound()` call, but triggered abilities pushed to stack and SBAs run. See MEDIUM issue #1. |
| SBAs checked | 704.3 | PASS | `StateBasedActions(gs)` at L125 |
| Duration expirations scanned | 500.4 | PASS | `ScanExpiredDurations` at L122 |
| Delayed triggers checked | 603.7 | PASS | `FireDelayedTriggers` at L123 |

### 504 Draw Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step set correctly | 504.1 | PASS | `gs.Phase, gs.Step = "beginning", "draw"` at L135 |
| Active player draws one card | 504.1 | PASS | `drawTop(gs, active)` at L137 |
| First player turn 1 skip | -- | PASS | `gs.Turn > 1 \|\| active != firstActive(gs)` check at L136 |
| Draw is turn-based action (not on stack) | 504.1 | PASS | `drawTop` directly moves card, no stack involvement |
| Priority after draw | 504.2 | IMPLICIT | Phase triggers fire at L139, SBAs at L140, but no explicit PriorityRound. See MEDIUM issue #1. |
| Draw triggers fire | 504.2 | PASS | `FirePhaseTriggers` at L139 checks for draw-step triggers; `FireDrawTriggerObservers` at L421 fires Bowmasters-type observers |

### 505 Main Phase (Pre-combat)

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Phase/step set correctly | 505.1 | PASS | `gs.Phase, gs.Step = "main", "precombat_main"` at L152 |
| Land drop allowed (pre-combat) | 505.6b | PASS | `tryPlayLand` called at L429 when `precombat && !playedLandThisTurn` |
| Only one land per turn (default) | 505.6b | PASS | `playedLandThisTurn` / `setPlayedLand` flag tracking |
| Casting spells allowed | 505.6a | PASS | `buildCastableList` + Hat-driven cast loop at L447-468 |
| Saga lore counter advance | 505.4 | MISSING | **LOW #1:** No explicit saga counter advance at precombat main. Saga SBA (704.5s) handles chapter triggers, but the precombat-main lore-counter placement per 505.4 is not implemented as a turn-based action. |
| Commander cast supported | 903.8 | PASS | `tryCastCommander` at L442 |
| SBAs after main phase | 704.3 | PASS | `StateBasedActions(gs)` at L154 |
| Mana tapping | -- | PASS | All untapped lands auto-tapped at L434-439 (MVP bucket mana) |

### 505 Main Phase (Post-combat)

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Phase/step set correctly | 505.1 | PASS | `gs.Phase, gs.Step = "main", "postcombat_main"` at L179 |
| Land drop NOT offered post-combat | 505.6b | PARTIAL | `runMainPhase(gs, active, false)` passes `precombat=false`, so `tryPlayLand` is skipped. **LOW #2:** CR 505.6b says land can be played during EITHER main phase. Post-combat land drop should be allowed if the player hasn't played a land yet. |
| Casting works | 505.6a | PASS | Same cast loop runs |

### 506-511 Combat Phase

#### 507 Beginning of Combat Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 507.1 | PASS | `gs.Phase, gs.Step = "combat", "beginning_of_combat"` at L241 |
| "At beginning of combat" triggers | 507.1 | PASS | `FirePhaseTriggers(gs, gs.Phase, gs.Step)` at L242 + `fireBeginningOfCombatTriggers` in combat.go L280 |
| Priority after | 507.2 | PASS | `PriorityRound(gs)` in combat.go L194 |
| Defending player chosen (multiplayer) | 507.1 | PASS | Per-attacker defender selection via `pickAttackDefender` + `setAttackerDefender` |

#### 508 Declare Attackers Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 508.1 | PASS | `gs.Step = "declare_attackers"` at L201 |
| Attackers declared | 508.1a | PASS | `DeclareAttackers()` at L202; checks untapped, creature, no summoning sickness (or haste), no defender keyword |
| Attackers tapped | 508.1f | PASS | `p.Tapped = true` at L379, with vigilance exemption |
| Attack triggers fire | 508.1m | PASS | `fireAttackTriggers()` at L416; both self and ally triggers |
| Priority after | 508.2 | PASS | `PriorityRound(gs)` at L209 |
| Skip blockers/damage if no attackers | 508.8 | PASS | `if len(attackers) == 0 { EndOfCombatStep(gs); return }` at L203-206 |
| Menace enforced on attack | 508.1c | PARTIAL | Menace checked on BLOCKING side but not on attack declaration. No attack restrictions checking. **LOW #3.** |

#### 509 Declare Blockers Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 509.1 | PASS | `gs.Step = "declare_blockers"` at L217 |
| Blockers declared | 509.1a | PASS | `DeclareBlockersMulti()` at L218 |
| Flying/reach evasion | 509.1b | PASS | `canBlock()` checks flying/reach at L750-753 |
| Protection blocks | 509.1b | PASS | `attackerHasProtectionFrom` check at L762 |
| Menace blocking (2+ required) | 509.1b | PASS | Menace check in `DeclareBlockers` at L625-668 |
| Block triggers fire | 509.1i | MISSING | **MEDIUM #2:** No "when this blocks" or "when this becomes blocked" triggers fire. |
| Priority after | 509.2 | PASS | `PriorityRound(gs)` at L220 |
| Damage ordering for multiple blockers | 510.1c | PASS | Attackers assign damage to blockers ordered by ascending toughness at L971-979 |

#### 510 Combat Damage Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Damage assigned and dealt simultaneously | 510.2 | PASS | `DealCombatDamageStep` processes all damage in one pass |
| First strike / double strike separate step | 510.4 | PASS | `hasFS` detection at L227-246; first strike step at L249, regular at L261 |
| SBAs between FS and regular damage | 510.4 | PASS | `StateBasedActions(gs)` at L253 |
| Priority after each damage step | 510.3 | PASS | `PriorityRound(gs)` at L255 and L265 |
| Trample overflow to player | 510.1c | PASS | `if remaining > 0 && atk.HasKeyword("trample")` at L994 |
| Lifelink on combat damage | 702.15 | PASS | Lifelink handled in `applyCombatDamageToPlayer` at L1089 |
| Deathtouch on combat damage | 702.2 | PASS | `lethalAmount` returns 1 for deathtouch at L1048; `deathtouch_damaged` flag set for SBA 704.5h |
| Unblocked creature damage to player | 510.1b | PASS | Unblocked attackers deal damage to defender at L954 |
| Blocked creature with removed blockers | 510.1c | PASS | Trample goes through; non-trample fizzles at L957-964 |

#### 511 End of Combat Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 511.1 | PASS | `EndOfCombatStep()` at L271; sets `gs.Phase, gs.Step = "combat", "end_of_combat"` |
| "At end of combat" triggers fire | 511.2 | PASS | Walks all permanents for end-of-combat triggers at L1216-1244 |
| Combat flags cleared | 511.3 | PASS | All attacking/blocking/declared flags cleared at L1248-1254 |
| "Until end of combat" effects expire | 511.2/500.5a | PASS | `until_end_of_combat` modifications removed at L1257-1272 |
| Priority after | 511.1 | PASS | `PriorityRound(gs)` at L275 (in turn.go) |
| SBAs fire | 704.3 | PASS | `StateBasedActions(gs)` at L273 |
| Extra combats supported | 500.8 | PASS | `runCombatWithExtras` loops on `PendingExtraCombats` at L249-257 |
| Delayed triggers fire at end-of-combat | 603.7 | PASS | `FireDelayedTriggers(gs, gs.Phase, gs.Step)` at L247 |

### 512 Ending Phase

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Phase exists | 512.1 | PASS | Two steps: end and cleanup |

### 513 End Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 513.1 | PASS | `gs.Phase, gs.Step = "ending", "end"` at L194 |
| "At beginning of end step" triggers | 513.1 | PASS | `FirePhaseTriggers(gs, gs.Phase, gs.Step)` at L196 |
| Delayed triggers fire | 603.7 | PASS | `FireDelayedTriggers(gs, gs.Phase, gs.Step)` at L195 |
| Mana pools drain | 500.5 | PASS | `DrainAllPools(gs, gs.Phase, gs.Step)` at L199 |
| SBAs fire | 704.3 | PASS | `StateBasedActions(gs)` at L200 |
| Priority given | 513.1 | IMPLICIT | Triggers push to stack + SBAs run. See MEDIUM issue #1. |
| 513.2 late-entry triggers wait for next turn | 513.2 | NOT CHECKED | Engine pushes triggers immediately; no "too late" check. **LOW #4.** |

### 514 Cleanup Step

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Step fires | 514.1 | PASS | `gs.Phase, gs.Step = "ending", "cleanup"` at L210 |
| Discard to hand size | 514.1 | PASS | `CleanupHandSize(gs, active, 7)` at L212 |
| Default max hand size = 7 | 402.2 | PASS | phases.go L714-715 defaults to 7 |
| Damage removed from permanents | 514.2 | PASS | `p.MarkedDamage = 0` at L234 in ScanExpiredDurations |
| "Until end of turn" effects expire | 514.2 | PASS | `ScanExpiredDurations` at L214; cleans `until_end_of_turn` mods and continuous effects |
| Granted abilities cleared | 514.2 | PASS | `p.GrantedAbilities = p.GrantedAbilities[:0]` at L251 |
| No priority normally | 514.3 | PASS | No PriorityRound call during cleanup |
| **REPEAT cleanup if triggers/SBAs fire** | **514.3a** | **FAIL** | **HIGH #1:** SBAs run at L215 but if they cause triggers to fire, the engine does NOT repeat the cleanup step. CR 514.3a explicitly requires: "If so, those SBAs are performed, then those triggered abilities are put on the stack, then the active player gets priority. Once the stack is empty and all players pass, another cleanup step begins." The engine just runs SBAs once and moves on. |
| Mindslaver control released | 712.6 | PASS | `seat.ControlledBy = -1` at L229 |
| End-of-turn snapshot | -- | PASS | `gs.Snapshot()` at L234 |
| "End the turn" effect (Sundial) | 712.5 | PASS | `turnEndingNow` / `fastForwardCleanup` checked after every phase at L129, 144, 159, 172, 187, 204 |

### Turn Structure General

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| 5 phases in order | 500.1 | PASS | beginning -> main1 -> combat -> main2 -> ending |
| Storm count reset at untap | 700.4 | PASS | `gs.SpellsCastThisTurn = 0` at L116 |
| Extra turns support | 500.7 | NOT AUDITED | Turn rotation is in tournament runner, not in turn.go. |

---

## PART 2: ZONE AUDIT (CR Section 4)

### 401 Library

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Face-down pile | 401.2 | PASS | `Library []*Card` slice, not publicly visible |
| Order maintained | 401.2 | PASS | Go slice preserves order; Library[0] = top |
| Draw from top | 401.2 | PASS | `drawTop` / `drawOne` take `s.Library[0]` |
| Search + shuffle | 401.2 | PASS | Tutor effects exist; shuffle via `gs.Rng.Shuffle` |
| Put cards on top/bottom | 401.4/401.7 | PASS | `moveToZone` handles `library_top` and `library_bottom` at state.go L996-998 |
| Cards returned to library go to owner's | 400.3 | PASS | `moveToZone` uses `ownerSeat` parameter |

### 402 Hand

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Hidden zone | 402.3 | PASS | `Hand []*Card` per-seat; no cross-seat visibility API |
| Starting hand size (London mulligan) | 103.5 | PASS | `RunLondonMulligan` in turn.go L281-376 |
| Max hand size default 7 | 402.2 | PASS | `CleanupHandSize` defaults to 7 |
| Discard at cleanup | 402.2 | PASS | `CleanupHandSize` invoked at cleanup step |
| Discard to graveyard | 404.1 | PASS | Discarded cards appended to `seat.Graveyard` at phases.go L744 |

### 403 Battlefield

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Only permanents exist on battlefield | 403.3 | PASS | `Battlefield []*Permanent` type constraint |
| Tapped/untapped tracking | 403.1 | PASS | `Permanent.Tapped` field |
| Summoning sickness | 302.6 | PASS | `Permanent.SummoningSick` field; cleared at untap step |
| Controller tracking | 403.1 | PASS | `Permanent.Controller` field |
| Timestamp ordering | 403.4/613 | PASS | `Permanent.Timestamp` assigned via `gs.NextTimestamp()` |
| Control change | 403.1 | PASS | `resolveGainControl` moves permanent between controllers |
| Phased-out permanents treated as nonexistent | 702.26 | PASS | `PhasedOut` flag checked throughout; `IsEffectivelyOnBattlefield` helper |
| Instant/sorcery can't enter battlefield | 400.4a | MISSING | **LOW #5:** No explicit check prevents instant/sorcery cards from being placed on the battlefield. The stack resolution path naturally handles this (instants/sorceries go to graveyard after resolution), but a direct `moveToZone` call could bypass this. |

### 404 Graveyard

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Face-up pile | 404.2 | PASS | `Graveyard []*Card` per-seat; cards are public |
| Order maintained (bottom-to-top) | 404.2 | PASS | Go slice append maintains order; most recent on top (end of slice) |
| Cards go to OWNER's graveyard | 400.3 | PASS | `moveToZone` and `destroyPermSBA` use `p.Card.Owner` |
| Destroyed/sacrificed/countered go here | 404.1 | PASS | `resolveDestroy`, `resolveSacrifice`, `destroyPermSBA` all route to graveyard |
| Discards go here | 404.1 | PASS | `CleanupHandSize` appends to graveyard at phases.go L744 |
| Zone-change triggers on entering graveyard | 404 | FAIL | **See HIGH #2 below** |

### 405 Stack

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| LIFO order | 405.2 | PASS | `gs.Stack[len-1]` is top; `ResolveStackTop` pops from end |
| Spells and abilities on stack | 405.1 | PASS | `StackItem` holds both spells (Card) and triggered abilities (Effect) |
| Priority after each resolution | 405.5 | PASS | `PriorityRound` called after each `ResolveStackTop` in the cast-and-resolve loop |
| All pass = step/phase ends | 405.5 | PASS | Empty stack + pass = next step |
| Mana abilities resolve immediately | 405.6c | PASS | Mana abilities bypass stack (not pushed) |
| Static abilities don't use stack | 405.6b | PASS | Static abilities in ContinuousEffects registry, not stack |
| Turn-based actions don't use stack | 405.6e | PASS | Draw, untap, etc. happen directly |
| SBAs don't use stack | 405.6f | PASS | `StateBasedActions` mutates directly |
| APNAP trigger ordering | 603.3b | PASS | `OrderTriggersAPNAP` in triggers.go implements correct ordering |

### 406 Exile

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Exile zone exists | 406.1 | PASS | `Exile []*Card` per-seat |
| Face-up default | 406.3 | PASS | Exiled cards are `*Card` (visible); FaceDown flag exists for face-down exile |
| Face-down exile support | 406.3 | PASS | `Card.FaceDown` field |
| Re-exile becomes new object | 406.7 | NOT IMPLEMENTED | **LOW #6:** No special handling for re-exiling an already-exiled card. |
| Zone-change triggers on exile | 406 | FAIL | **See HIGH #2 below** |

### 408 Command Zone

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| Command zone exists | 408.1 | PASS | `CommandZone []*Card` per-seat |
| Commander starts here | 408.3 | PASS | Commander setup populates CommandZone |
| Commander cast from command zone | 903.8 | PASS | `CastCommanderFromCommandZone` in commander.go |
| Commander tax tracking | 903.8 | PASS | `CommanderCastCounts` / `CommanderTax` maps |
| Commander redirect to command zone | 903.9b | PASS | `FireZoneChange` checks for commander and redirects to command_zone |
| Emblems | 408.2 | MISSING | **LOW #7:** No emblem model. Emblems would need a separate data structure in the command zone. |
| Companion support | 702.139 | PASS | `Seat.Companion` and `Seat.CompanionMoved` fields |
| Dungeons | 408.3 | MISSING | **LOW #8:** Dungeon tracking not implemented (SBA 704.5t is a stub). |

---

## PART 3: ZONE-CHANGE TRIGGERS AUDIT

### ETB Triggers ("Enters the battlefield")

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| ETB from stack resolution | 603.6 | PASS | `resolvePermanentSpellETB` in stack.go fires ETB triggers at L970-986 |
| ETB trigger goes on stack | 603.3 | PASS | `PushTriggeredAbility(gs, perm, trig.Effect)` at L981 |
| Per-card ETB hooks | -- | PASS | `InvokeETBHook(gs, perm)` at L992 |
| Generic "nonland permanent ETB" event | -- | PASS | `FireCardTrigger(gs, "nonland_permanent_etb", ...)` at L996 |
| ETB from non-stack sources (tokens) | 603.6 | PASS | Token creation triggers ETB via same path |
| Replacement effects on ETB | 614 | PASS | `RegisterReplacementsForPermanent` called at L956 |
| Continuous effects on ETB | 613 | PASS | `RegisterContinuousEffectsForPermanent` called at L954 |

### Dies Triggers ("Battlefield -> Graveyard, creatures only")

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| **"When this creature dies" triggers** | **603.6** | **FAIL** | **HIGH #2a:** The engine has NO death-trigger system. When `destroyPermSBA` removes a creature from the battlefield and places it in the graveyard, it fires a `FireDieEvent` (replacement chain) but does NOT scan the dying creature's abilities for "when this dies" triggered abilities, nor does it push them onto the stack. The `per_card/necrotic_ooze.go` even comments: "engine doesn't have LTB hook yet." |
| Death trigger from sacrifice | 603.6 | FAIL | `resolveSacrifice` in resolve.go moves card to graveyard but fires no dies triggers. |
| Death trigger from destroy effect | 603.6 | FAIL | `resolveDestroy` in resolve.go moves card to graveyard but fires no dies triggers. |

### LTB Triggers ("Leaves the battlefield")

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| **"When this leaves the battlefield" triggers** | **603.6** | **FAIL** | **HIGH #2b:** No LTB trigger system exists. When a permanent is removed via `gs.removePermanent()`, the only cleanup is `UnregisterReplacementsForPermanent` and `UnregisterContinuousEffectsForPermanent`. There is no scanning for LTB triggered abilities. |
| LTB from exile | 603.6 | FAIL | `resolveExile` does not fire LTB triggers. |
| LTB from bounce | 603.6 | FAIL | `resolveBounce` does not fire LTB triggers. |

### Exile Triggers ("When exiled")

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| "When exiled" triggers | 603.6 | FAIL | **HIGH #2c:** No "when exiled" trigger scanning. |

### "Put into graveyard from anywhere" Triggers

| Check | CR Rule | Status | Notes |
|-------|---------|--------|-------|
| "When put into graveyard from anywhere" | 603.6 | FAIL | **HIGH #2d:** No "from anywhere to graveyard" trigger system. Cards like Kozilek/Ulamog with "when put into a graveyard from anywhere, shuffle into library" have no trigger path. |

---

## PART 4: DETAILED ISSUE LIST

### HIGH SEVERITY

#### HIGH #1: Cleanup step does not repeat when triggers fire (CR 514.3a)

**File:** `internal/tournament/turn.go` L209-234  
**Rule:** CR 514.3a  
**Problem:** After running `CleanupHandSize` + `ScanExpiredDurations` + `StateBasedActions`, if any triggers would fire (e.g., a madness trigger from discarding, or a "whenever a creature dies" trigger from lethal-damage SBA), the engine does NOT repeat the cleanup step. CR 514.3a explicitly states: "If so, those SBAs are performed, then those triggered abilities are put on the stack, then the active player gets priority. Players may cast spells and activate abilities. Once the stack is empty and all players pass in succession, another cleanup step begins."

**Impact:** Madness, "whenever you discard" triggers, and any SBA-triggered abilities during cleanup are lost.

**Fix:** After SBAs at L215, check if any triggers were queued. If so, give priority, resolve the stack, then loop back to the beginning of the cleanup step (discard check + expiration + SBA again).

---

#### HIGH #2: No dies/LTB/exile zone-change trigger system

**Files:** `internal/gameengine/resolve.go`, `internal/gameengine/sba.go`  
**Rules:** CR 603.6, 603.10  
**Problem:** The engine fires ETB triggers correctly but has no corresponding system for:
- "When this creature dies" (battlefield -> graveyard)
- "When this leaves the battlefield" (battlefield -> any zone)  
- "When this is exiled" (any zone -> exile)
- "When put into a graveyard from anywhere" (any zone -> graveyard)

The SBA death path (`destroyPermSBA`) calls `FireDieEvent` which is a REPLACEMENT event check (Anafenza, Rest in Peace redirect), but it does not fire TRIGGERED abilities on the dying creature.

The non-SBA paths (`resolveDestroy`, `resolveExile`, `resolveBounce`, `resolveSacrifice`) are even more bare -- they call `gs.removePermanent()` + `gs.moveToZone()` and log events, but fire zero triggers and don't even call `FireZoneChange` for replacement processing.

**Impact:** Major class of cards broken: Blood Artist, Zulaport Cutthroat, Grave Pact, Dictate of Erebos, Elenda, Kokusho, Wurmcoil Engine, Solemn Simulacrum, and all "when this dies" cards.

**Fix:** 
1. Create a `FireLTBTriggers(gs, perm, fromZone, toZone)` function that scans the leaving permanent's AST abilities for LTB/dies triggers and pushes them to the stack.
2. Call it from `destroyPermSBA`, `resolveDestroy`, `resolveExile`, `resolveBounce`, and `resolveSacrifice`.
3. Route all zone-change paths through `FireZoneChange` for replacement-effect consistency.

---

#### HIGH #3: resolveDestroy/Exile/Bounce/Sacrifice bypass FireZoneChange

**File:** `internal/gameengine/resolve.go` L570-738  
**Rule:** CR 614 (replacement effects on zone changes)  
**Problem:** The SBA death path correctly routes through `FireDieEvent` (replacement chain), but the effect-resolution paths do not:
- `resolveDestroy` (L570): directly calls `gs.removePermanent(p)` then `gs.moveToZone(p.Card.Owner, p.Card, "graveyard")` -- no replacement check, no FireZoneChange.
- `resolveExile` (L592): same pattern, no replacement check.
- `resolveBounce` (L614): same pattern, no replacement check.
- `resolveSacrifice` (L696): same pattern, no replacement check.

This means:
- Rest in Peace ("if a card would be put into a graveyard from anywhere, exile it instead") does NOT work for Destroy/Sacrifice effects -- only for SBA deaths.
- Commander redirect (903.9b) does NOT work for spell-based destroy/exile/bounce.
- Indestructible is NOT checked by `resolveDestroy` (it directly removes the permanent).

**Impact:** Rest in Peace, Leyline of the Void, Anafenza, commander redirect, and indestructible are all bypassed by spell effects.

**Fix:** Route all zone-change paths through `FireZoneChange` and add an indestructible check to `resolveDestroy`.

---

### MEDIUM SEVERITY

#### MEDIUM #1: No explicit PriorityRound at upkeep, draw, and end steps

**File:** `internal/tournament/turn.go`  
**Rules:** CR 503.1, 504.2, 513.1  
**Problem:** The upkeep, draw, and end steps do not call `PriorityRound(gs)` explicitly. Triggers fire and SBAs run, but there is no point where players can cast instants or activate abilities in response to triggers during these steps (the Hat's respond-to-triggers mechanism goes through the stack, but there's no explicit "all players pass" check).

**Note:** The current engine is a "goldfish" (no interaction) MVP where the Hat auto-resolves, so this may be functionally equivalent. But for true interactive play, explicit priority rounds are needed at:
- L125 (after upkeep triggers + SBAs)
- L140 (after draw triggers + SBAs)
- L200 (after end-step triggers + SBAs)

---

#### MEDIUM #2: No "when this blocks" / "when this becomes blocked" triggers

**File:** `internal/gameengine/combat.go`  
**Rules:** CR 509.1i, 509.2a, 509.3a-g  
**Problem:** After blockers are declared, no block-related triggers fire. Cards like Ohran Frostfang ("whenever a creature you control becomes blocked"), or any "when this blocks" creature, have no trigger path.

---

#### MEDIUM #3: Post-combat land drop not allowed

**File:** `internal/tournament/turn.go` L426-431  
**Rule:** CR 505.6b  
**Problem:** `runMainPhase(gs, active, false)` skips `tryPlayLand` when `precombat=false`. But CR 505.6b says: "During either main phase, the active player may play one land card from their hand." A player who hasn't played a land pre-combat should be able to play one post-combat.

---

#### MEDIUM #4: resolveDestroy does not check indestructible

**File:** `internal/gameengine/resolve.go` L570-590  
**Rule:** CR 702.12  
**Problem:** `resolveDestroy` directly removes the permanent from the battlefield without checking `p.IsIndestructible()`. A creature with indestructible hit by a Destroy effect should survive. Only the SBA path (704.5g) respects indestructible.

---

#### MEDIUM #5: No "until end of combat" continuous effects expiry on combat phase end

**File:** `internal/gameengine/combat.go` L1257-1272  
**Rule:** CR 500.5a  
**Problem:** `EndOfCombatStep` only cleans `Permanent.Modifications` with `duration == "until_end_of_combat"`. It does NOT clean `gs.ContinuousEffects` entries with equivalent duration. If a continuous effect was registered with an "until end of combat" duration via the layer system, it would persist.

---

#### MEDIUM #6: Phasing: phase-out of phasing permanents not implemented

**File:** `internal/gameengine/phases.go` L526-547  
**Rule:** CR 502.1  
**Problem:** `PhaseInAll` handles phasing IN, but `UntapAll` does not explicitly phase OUT permanents that have the phasing keyword. CR 502.1 says: "all phased-in permanents with phasing that the active player controls phase out, and all phased-out permanents that the active player controlled when they phased out phase in." The phase-out half (for permanents that HAVE the phasing keyword) is missing.

---

#### MEDIUM #7: Mana pool not drained at each phase/step transition

**File:** `internal/tournament/turn.go`  
**Rule:** CR 500.5  
**Problem:** `DrainAllPools` is only called at the end step (L199). CR 500.5 says mana empties at the end of EACH step and phase. Currently, mana accumulated during the main phase persists into combat. This is partially correct for the MVP (the generic ManaPool counter is auto-tapped at main phase start), but typed mana from Mana artifacts could incorrectly persist across phase boundaries.

---

#### MEDIUM #8: Creature token deaths skip zone-change bookkeeping

**File:** `internal/gameengine/sba.go` L1312-1313  
**Rule:** CR 704.5d  
**Problem:** `destroyPermSBA` skips `moveToZone` for tokens (`if !p.IsToken()`), which is correct (tokens cease to exist in other zones). However, this also means token deaths don't fire "dies" triggers. A creature token dying should still fire death triggers (it does momentarily exist in the graveyard before 704.5d removes it). Since the engine has no dies triggers at all (HIGH #2), this is grouped under MEDIUM as a follow-on concern.

---

### LOW SEVERITY

1. **LOW #1:** Saga lore counter advance at precombat main (505.4) not implemented as a turn-based action. Saga SBA handles chapters, but the explicit lore-counter addition per 505.4 is missing.

2. **LOW #2:** Post-combat land drop skipped (see MEDIUM #3 for the actionable version; listed separately as a design note).

3. **LOW #3:** Attack restrictions/requirements checking (508.1c/508.1d) not fully implemented. The engine uses a greedy "everything attacks" policy without checking "can't attack" or "must attack" effects.

4. **LOW #4:** CR 513.2 "late entry" check not implemented. A permanent entering during the end step with an "at beginning of end step" trigger should wait until the next turn.

5. **LOW #5:** No guard preventing instant/sorcery cards from being placed on the battlefield via direct zone manipulation (400.4a).

6. **LOW #6:** Re-exile does not create a "new object" (406.7 / 400.8).

7. **LOW #7:** No emblem model for the command zone (408.2).

8. **LOW #8:** No dungeon tracking (408.3 / 704.5t stub).

9. **LOW #9:** `canAttack` does not check for 0-power creatures that have an attack requirement. A 0/1 Wall with "attacks each combat if able" would incorrectly be filtered out by the `p.Power() <= 0` check.

10. **LOW #10:** `resolveDestroy` does not check for protection from the source. CR 702.16b says protection prevents being destroyed by sources of the protected quality.

11. **LOW #11:** `CleanupHandSize` clears ALL granted abilities at cleanup (L251), which is over-aggressive. Only abilities with "until end of turn" / "this turn" duration should be cleared. Permanent grants (from Equipment, Auras) should persist.

---

## PART 5: PRIORITY MAP

Summary of priority-round presence at each step:

| Step | Priority Round Present? | Notes |
|------|------------------------|-------|
| Untap | No (correct) | CR 502.4: no priority |
| Upkeep | No (should be yes) | MEDIUM #1 |
| Draw | No (should be yes) | MEDIUM #1 |
| Main 1 | Implicit (cast loop) | Hat-driven casting acts as priority |
| Beginning of combat | Yes | combat.go L194 |
| Declare attackers | Yes | combat.go L209 |
| Declare blockers | Yes | combat.go L220 |
| First strike damage | Yes | combat.go L255 |
| Combat damage | Yes | combat.go L265 |
| End of combat | Yes | turn.go L275 |
| Main 2 | Implicit (cast loop) | Hat-driven casting acts as priority |
| End step | No (should be yes) | MEDIUM #1 |
| Cleanup | No (correct normally) | But needs conditional priority per 514.3a |

---

## PART 6: RECOMMENDATIONS (Priority Order)

1. **HIGH #2 + #3:** Implement zone-change trigger system. Create `FireZoneChangeTriggers(gs, perm, card, fromZone, toZone)` that scans for dies/LTB/exile triggers and pushes them to stack. Route ALL zone-change paths (resolve effects AND SBAs) through a unified `PerformZoneChange()` that calls `FireZoneChange` (replacement) + `FireZoneChangeTriggers` (triggers). This is the single highest-impact fix.

2. **HIGH #1:** Add cleanup step looping per 514.3a. After SBAs at L215, if any triggers were queued or SBAs fired, loop back.

3. **MEDIUM #4:** Add `p.IsIndestructible()` check to `resolveDestroy`.

4. **MEDIUM #3:** Allow post-combat land drop by passing land-drop eligibility into `runMainPhase` regardless of precombat flag.

5. **MEDIUM #1:** Add explicit `PriorityRound` calls at upkeep, draw, and end steps.

6. **MEDIUM #6:** Implement phase-out for permanents with the phasing keyword at the start of `UntapAll`.

7. **MEDIUM #2:** Add block trigger firing after `DeclareBlockers`.

8. Remaining MEDIUMs and LOWs can be addressed incrementally.

---

*End of audit.*
