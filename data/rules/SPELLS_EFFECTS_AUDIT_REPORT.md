# Spells, Abilities, and Effects Audit Report (CR Section 6)

**Date:** 2026-04-16
**Auditor:** Claude Opus 4.6 (1M context)
**Engine files audited:**
- `internal/gameengine/stack.go` -- CastSpell, PushStackItem, PriorityRound, ResolveStackTop, SplitSecondActive
- `internal/gameengine/activation.go` -- ActivateAbility, IsManaAbility, StaxCheck
- `internal/gameengine/triggers.go` -- OrderTriggersAPNAP, PushSimultaneousTriggers
- `internal/gameengine/resolve.go` -- ResolveEffect + all leaf handlers
- `internal/gameengine/layers.go` -- GetEffectiveCharacteristics, ContinuousEffect, applyLayer
- `internal/gameengine/replacement.go` -- FireEvent, ReplacementEffect, ReplEvent
- `internal/gameengine/costs.go` -- AlternativeCost, AdditionalCost, CastSpellWithCosts
- `internal/gameengine/zone_cast.go` -- CastFromZone, ZoneCastPermission
- `internal/gameengine/targets.go` -- PickTarget
- `internal/gameengine/phases.go` -- FirePhaseTriggers, FireDelayedTriggers, ScanExpiredDurations
- `internal/gameengine/state.go` -- GameState, DelayedTrigger, StackItem
- `internal/gameengine/per_card/delayed_trigger_cards.go` -- delayed trigger card hooks
- `internal/gameengine/resolve_helpers.go` -- ModificationEffect handlers

**Rules reference:** `data/rules/MagicCompRules-20260227.txt`

---

## Summary

The engine implements the core casting, activation, triggering, and resolution loop with strong structural correctness for a tournament-simulation engine. The layer system covers all 7 layers plus sublayers. The replacement effect framework implements the full CR 616.1 category ordering. There are deliberate MVP simplifications (documented in code) and a handful of genuine gaps.

**Scores by section:**

| Section | Compliance | Issues |
|---------|-----------|--------|
| 601 Casting Spells | 80% | Ordering deviations, missing mode choice step, damage distribution stub |
| 602 Activating Abilities | 90% | Solid; minor gap in sorcery-speed activation timing |
| 603 Triggered Abilities | 85% | APNAP done well; reflexive triggers and state triggers not implemented |
| 608 Resolving Spells/Abilities | 75% | Missing target legality recheck at resolution (countered-on-resolution) |
| 613 Layer System | 90% | All 7 layers + sublayers; dependency system (613.8) deferred |
| 614 Replacement Effects | 85% | Strong framework; ETB replacements (614.12) partially implemented |
| 615 Prevention Effects | 30% | Minimal -- flag-based prevention only, no structured framework |
| 616 Replacement Interaction | 90% | Category ordering correct; APNAP tiebreak uses timestamp proxy |

---

## 601: Casting Spells -- Detailed Findings

### 601.2a: Move card to stack -- PASS

**Engine:** `CastSpell()` at stack.go:183 calls `removeFromHand(seat, card)` as the first step, then constructs a `StackItem` and calls `PushStackItem()`. The card leaves the hand zone and is placed on the stack. `CastFromZone()` at zone_cast.go:231 calls `removeFromZone()` for non-hand zones.

**Rules compliance:** Correct. The card is moved from its origin zone to the stack before any cost payment occurs, matching CR 601.2a ("first moves that card from where it is to the stack").

### 601.2b: Choose modes -- PARTIAL (Gap 1)

**Engine:** `CastSpell()` does not have an explicit mode-choice step. The `targets []Target` parameter is caller-supplied, and modes are handled implicitly when the resolver encounters a `Choice` effect at resolution time (resolve.go:177-193 `resolveChoice`). The `CastSpellWithCosts()` path (costs.go:701) similarly has no mode-choice step.

**Gap:** Mode choice should happen DURING casting (601.2b), not at resolution. This means:
1. Modal spells don't lock modes on the stack -- opponents can't see which mode was chosen when responding.
2. X value announcement is handled correctly via `ManaCostContainsX` + `Hat.ChooseX` (stack.go:193-211), which matches 601.2b's variable cost announcement.
3. Additional/alternative cost announcements are handled by `CastSpellWithCosts`.

**Severity:** MEDIUM. For tournament simulation accuracy this is acceptable since the Hat policy decides at cast time anyway, but mode locking is wrong per rules text.

### 601.2c: Choose targets -- PARTIAL (Gap 2)

**Engine:** Targets are supplied by the caller as `[]Target` and stored on `StackItem.Targets`. The actual target resolution happens at resolution time via `PickTarget()` (targets.go:23), not at cast time. The `StackItem.Targets` field IS populated by the caller, but the engine's effect resolver calls `PickTarget()` independently during resolution (e.g., resolve.go:271 `resolveDamage` calls `PickTarget(gs, src, e.Target)`).

**Gap:** The engine picks targets at resolution rather than locking them at cast time. This means targets can change between casting and resolution, which is incorrect per CR 601.2c. For simulation purposes, this is mitigated by the single-threaded execution model -- the game state at cast time and resolution time is often the same for simple sequences.

**Severity:** MEDIUM. Targets should be locked on the StackItem at cast time and re-validated at resolution (see 608.2b finding below).

### 601.2d: Choose division (damage distribution) -- STUB

**Engine:** No damage/counter distribution step exists. When a spell deals damage to multiple targets, the engine uses `PickTarget` to find targets and applies the full amount to each independently.

**Severity:** LOW. Division is rare in tournament simulation and only matters for a handful of cards (e.g., Aurelia's Fury, Rolling Thunder).

### 601.2e: Choose X value -- PASS

**Engine:** `ManaCostContainsX()` (stack.go:354-377) detects X in cost. `CastSpell()` calls `Hat.ChooseX()` (stack.go:201) to determine the X value, which is capped at available mana minus base cost (stack.go:195-206). The chosen X is stored on `StackItem.ChosenX` (stack.go:279) for resolution.

**Note:** CR 601.2b says X should be announced in the mode/target step, but the engine handles it correctly in terms of sequencing relative to cost payment.

### 601.2f: Determine total cost -- PARTIAL (Gap 3)

**Engine:** `manaCostOf()` (stack.go:397-433) extracts base cost. `CastSpellWithCosts()` handles alternative costs (costs.go:736-773) and additional costs (costs.go:776-795). Cost increases and reductions are NOT implemented -- the engine computes `baseCost + chosenX` and pays that directly.

**Gap:** No cost modification framework exists. Effects like "spells cost {1} more to cast" (Thalia, Guardian of Thraben), "spells cost {1} less" (Thunderscape Familiar), or "spells you cast cost {2} less" (Jhoira) are not modeled. The "locked in" total cost rule (601.2f: "the resulting total cost becomes locked in") is also not implemented since there's no cost-modification pipeline to lock.

**Severity:** HIGH for certain matchups. Cards like Thalia, Trinisphere, and cost reducers are common in competitive play. The engine pays base CMC only.

### 601.2g: Activate mana abilities -- PASS (implicit)

**Engine:** The engine doesn't have an explicit "activate mana abilities" window during casting. However, `ActivateAbility()` in activation.go correctly handles mana abilities inline (activation.go:359-379): mana abilities resolve immediately without using the stack per CR 605.3a. The Hat/AI layer handles the sequencing of "tap lands, then cast spell" at a higher level.

**Status:** Functionally correct for simulation. The mana pool is populated before `CastSpell` is called.

### 601.2h: Pay costs -- PASS

**Engine:** `CastSpell()` pays mana at stack.go:214-235. Life cost payment is handled via `AlternativeCost`/`AdditionalCost` (costs.go). Sacrifice as additional cost is handled at costs.go:357-384. The cost payment is atomic -- if insufficient mana, the card is returned to hand (stack.go:216-218).

**Note:** The rules say non-random costs are paid first, then random costs. The engine pays in a fixed order (mana first, then additional costs), which is acceptable.

### 601.2i: Spell is "cast" -- cast triggers fire -- PASS

**Engine:** After cost payment, `IncrementCastCount()` and `fireCastTriggers()` are called (stack.go:251-266). Storm copies are also created here (stack.go:287-289). Cast-trigger observers (Rhystic Study, Mystic Remora, etc.) fire via `FireCastTriggerObservers()` (stack.go:266).

**Ordering concern:** Cast triggers fire BEFORE `PushStackItem()` at stack.go:281. The code comment says "We fire BEFORE pushing the stack item so the triggered abilities go onto the stack ABOVE the spell being cast -- matching CR 603.3." This is correct per the rules -- triggered abilities go on the stack after the spell, so they resolve before the spell.

---

## 602: Activating Abilities -- Detailed Findings

### 602.1: Activated ability goes on stack (not mana abilities) -- PASS

**Engine:** `ActivateAbility()` (activation.go:241) correctly separates mana abilities (resolve inline at line 359-379) from non-mana abilities (push to stack at line 393-402). Mana abilities bypass the stack per CR 605.3a.

### 602.2: Cost paid before ability goes on stack -- DEVIATION (Gap 4)

**Engine:** Costs are paid at activation.go:280-348 (tap + mana + life), then the ability is pushed to the stack at line 402. This matches the rules: 602.2a says "The ability is created on the stack" and 602.2b says the activation process matches 601.2b-i (costs determined then paid, then ability is "activated").

However, the engine pays costs BEFORE pushing to stack, which is the correct order per CR. The deviation is: if cost payment fails partway through (e.g., insufficient mana after tapping), the engine rolls back the tap (activation.go:307-309) but there's no general transaction/rollback mechanism for complex multi-part costs.

**Severity:** LOW. The rollback for the common case (tap + mana) is handled.

### 602.5a: Summoning sickness -- PASS

**Engine:** activation.go:287-289 checks `perm.SummoningSick && perm.IsCreature()` before allowing tap-symbol abilities. This correctly implements CR 602.5a (and by extension 302.6).

### 602.5d: Sorcery-speed activations -- PARTIAL (Gap 5)

**Engine:** `ActivateAbility()` does NOT check sorcery-speed restrictions on activated abilities. The `OppRestrictsDefenderToSorcerySpeed()` check is only applied in the casting path and priority round, not in the activation path. An activated ability with "Activate only as a sorcery" wording would not have timing enforced.

**Severity:** MEDIUM. Many activated abilities have sorcery-speed restrictions (fetchlands, Birthing Pod, etc.). The stax checks (StaxCheck) cover Null Rod / Cursed Totem / Grand Abolisher but not inherent timing restrictions.

### Stax enforcement -- PASS (Strong)

The stax enforcement in `StaxCheck()` (activation.go:182-222) is comprehensive:
- Null Rod / Collector Ouphe: artifact non-mana activations blocked (activation.go:196-199)
- Cursed Totem: ALL creature activated abilities blocked, including mana (activation.go:203-206)
- Grand Abolisher: opponents can't activate during controller's turn (activation.go:209-219)
- Split Second: non-mana activations blocked (activation.go:191-193)

Mana ability exemption is correctly applied for Null Rod but not Cursed Totem, matching the oracle text.

---

## 603: Triggered Abilities -- Detailed Findings

### 603.2: Triggers fire when condition met -- PASS

**Engine:** Triggers are identified and pushed via `PushTriggeredAbility()` (stack.go:99-121) and `FirePhaseTriggers()` (phases.go:42-99). ETB triggers fire during `resolvePermanentSpellETB()` (stack.go:970-986).

### 603.3b: APNAP ordering for simultaneous triggers -- PASS (Strong)

**Engine:** `OrderTriggersAPNAP()` in triggers.go:46-79 implements the full APNAP ordering:
1. Groups triggers by controller (line 52-58)
2. Walks seats in APNAP order via `apnapOrder()` (line 61-62)
3. Delegates intra-group ordering to each seat's Hat (line 71-73)
4. Active player's triggers pushed first (resolve last per LIFO) -- correct per CR 603.3b

`PushSimultaneousTriggers()` (triggers.go:88-112) wraps this for batch use.

**FirePhaseTriggers:** Uses a simplified stable sort by seat index (phases.go:89-93) which approximates but doesn't perfectly match APNAP. The active player should be first, not lowest seat index. This is correct when Active==0 but incorrect otherwise.

**Severity:** LOW. The APNAP function itself is correct; the phase-trigger caller has a minor sorting issue.

### 603.4: Intervening "if" clause -- PARTIAL (Gap 6)

**Engine:** phases.go:79-82 comments: "Intervening-if: evaluate the condition now, and again on resolution (both per 603.4). MVP check: defer condition until resolution (resolveConditional handles it)."

**Gap:** The "if" condition should be checked BOTH when the trigger event occurs (to decide if it triggers at all) AND at resolution (to decide if it resolves). The engine only checks at resolution via `resolveConditional()`. This means abilities with intervening "if" clauses will always trigger and only be checked at resolution.

**Example impact:** Felidar Sovereign ("At the beginning of your upkeep, if you have 40 or more life, you win the game") would trigger even at 20 life, then be checked at resolution. In fast simulation this usually doesn't matter (the trigger resolves immediately), but it could cause incorrect trigger counts and misfire storm-like interactions.

**Severity:** MEDIUM. The second check at resolution is the more important one and IS present.

### 603.7: Delayed triggered abilities -- PASS

**Engine:** `DelayedTrigger` struct in state.go:166-210 covers all standard trigger-at boundaries: end_of_turn, next_end_step, your_next_end_step, next_upkeep, end_of_combat, on_event. `FireDelayedTriggers()` in phases.go:343-400 fires them at the correct phase/step boundaries.

`RegisterDelayedTrigger()` (state.go:281-300) assigns timestamps and records created-turn. OneShot semantics (603.7b) are handled via the `OneShot` field + `Consumed` auto-set. Event-based delayed triggers use `ConditionFn` with `FireEventDelayedTriggers()` (state.go:216-277).

### 603.8: State-triggered abilities -- NOT IMPLEMENTED (Gap 7)

**Engine:** No mechanism exists for state triggers ("whenever [condition is true]"). State triggers should fire once when the condition becomes true, then not again until the ability has left the stack and the condition is still true. Examples: Serra Ascendant's conditional buff, Azami's "whenever you have no cards in hand."

**Severity:** MEDIUM. State triggers are uncommon in competitive play but do exist on some relevant cards.

### 603.12: Reflexive triggers ("when you do") -- NOT IMPLEMENTED (Gap 8)

**Engine:** No explicit support for reflexive triggered abilities. Reflexive triggers are a relatively modern rules concept where a spell or ability creates a trigger during its resolution that fires immediately. Example: "Choose target creature. When you do, [effect]."

**Severity:** LOW. Most reflexive triggers are on newer cards and can be handled by per-card hooks.

---

## 608: Resolving Spells and Abilities -- Detailed Findings

### 608.1: Top of stack resolves when all pass -- PASS

**Engine:** `ResolveStackTop()` (stack.go:700-880) pops the top of `gs.Stack` (LIFO at line 704-705) and resolves it. The CastSpell loop (stack.go:308-317) calls `PriorityRound()` then `ResolveStackTop()` then `StateBasedActions()` in a loop until the stack is empty, matching CR 117.4 + 608.1.

### 608.2b: Check if targets still legal at resolution -- MISSING (Gap 9 - CRITICAL)

**Engine:** `ResolveStackTop()` does NOT check target legality at resolution. When a spell resolves:
- It pops the stack item (stack.go:704)
- Checks if countered (stack.go:727)
- Resolves the effect directly (stack.go:769-788)

CR 608.2b states: "If the spell or ability specifies targets, it checks whether the targets are still legal. A target that's moved to another zone [...] is illegal. [...] If all its targets [...] are now illegal, the spell or ability doesn't resolve."

**Impact:** Spells targeting creatures that have been destroyed or bounced in response will still attempt to resolve their effects. The `PickTarget()` function in targets.go re-picks targets at resolution time rather than using the locked targets, which partially masks this bug (a new valid target may be found), but this is incorrect behavior.

**Severity:** CRITICAL. This is the most significant rules gap in the engine. A Lightning Bolt targeting a creature that gets bounced should fizzle (be countered on resolution), but instead the engine will pick a new target or apply damage to whatever `PickTarget` finds.

### 608.2c-m: Effects happen in order printed -- PASS

**Engine:** `resolveSequence()` at resolve.go:171-175 processes effects in order: `for _, item := range e.Items { ResolveEffect(gs, src, item) }`. This matches CR 608.2c ("the instructions are followed in the order they're written").

### 608.2g: Non-permanent spells go to graveyard -- PASS

**Engine:** stack.go:866-878 moves non-permanent spells to the graveyard after resolution. Copies cease to exist (stack.go:834-848 per CR 706.10). Flashback/escape spells are exiled instead (stack.go:848-865 per CR 702.33).

### 608.3: Permanent spells enter the battlefield -- PASS

**Engine:** `resolvePermanentSpellETB()` (stack.go:898-1010) handles permanent spell resolution:
1. Creates a `Permanent` with summoning sickness check (stack.go:914-919)
2. Assigns timestamp via `NextTimestamp()` (stack.go:925)
3. Initializes planeswalker loyalty (stack.go:935-941)
4. Appends to battlefield (stack.go:951)
5. Registers continuous + replacement effects (stack.go:953-956)
6. Fires ETB triggers (stack.go:970-986)

This closely mirrors CR 608.3a-3c.

---

## 613: Layer System -- Detailed Findings

### All 7 layers + sublayers -- PASS (Strong)

**Engine:** `GetEffectiveCharacteristics()` at layers.go:406-447 applies layers in strict order:

```
Layer 1:  applyLayer(gs, perm, chars, 1, "")   -- copy effects (613.1a)
Layer 1a: applyLayer(gs, perm, chars, 1, "a")  -- copiable effects (613.2a)
Layer 1b: applyLayer(gs, perm, chars, 1, "b")  -- face-down (613.2b)
Layer 2:  applyLayer(gs, perm, chars, 2, "")   -- control-changing (613.1b)
Layer 3:  applyLayer(gs, perm, chars, 3, "")   -- text-changing (613.1c)
Layer 4:  applyLayer(gs, perm, chars, 4, "")   -- type-changing (613.1d)
Layer 5:  applyLayer(gs, perm, chars, 5, "")   -- color-changing (613.1e)
Layer 6:  applyLayer(gs, perm, chars, 6, "")   -- ability add/remove (613.1f)
Layer 7a: applyLayer(gs, perm, chars, 7, "a")  -- CDA P/T (613.4a)
Layer 7b: applyLayer(gs, perm, chars, 7, "b")  -- set P/T (613.4b)
Layer 7c: applyLayer(gs, perm, chars, 7, "c")  -- modify P/T (613.4c)
Layer 7d: applyLayer(gs, perm, chars, 7, "d")  -- switch P/T (613.4d)
Layer 7e: applyLayer(gs, perm, chars, 7, "e")  -- reserved
```

Constants defined at layers.go:175-183 match the CR numbering.

### 613.4c: Counter application -- PASS

**Engine:** `applyCountersAndMods()` at layers.go:514-537 applies +1/+1 and -1/-1 counter bonuses plus until-EOT modifications after all layer effects. This is applied as a post-layer pass, which the code comments explain is because per-counter timestamps aren't tracked.

**Note:** Counters should technically be in layer 7c (613.4c), but applying them post-pass produces the same results for all practical cases except Humility-style interactions where layer 7b sets P/T and counters should apply on top in 7c. The engine handles this correctly -- the comment at layers.go:437-439 explicitly notes the Humility case.

### 613.7: Timestamp ordering -- PASS

**Engine:** `applyLayer()` at layers.go:451-506 sorts candidates by (APNAP tiebreak, timestamp ascending). Timestamps are assigned via `NextTimestamp()` which is a monotonically increasing counter on GameState.

### 613.8: Dependency ordering -- NOT IMPLEMENTED (Documented Gap)

**Engine:** layers.go:33-38 explicitly states: "613.8 dependency ordering: NOT IMPLEMENTED. This matches the Python reference which also skips dependency detection and relies on timestamp order."

The `DependsOn` field exists on `ContinuousEffect` (layers.go:171) for forward compatibility but is never read.

**Impact:** The Humility + Opalescence canonical paradox will not be resolved correctly. The code states Humility is handled via per-oracle self-exclusion.

**Severity:** LOW for tournament simulation. Dependency is only relevant for a small number of card interactions.

### Caching -- PASS

Characteristics are cached per-permanent with epoch invalidation (layers.go:95-98, 210-215). `InvalidateCharacteristicsCache()` is called on register/unregister, counter changes, and modification updates.

---

## 614: Replacement Effects -- Detailed Findings

### 614.1: "Instead" effects -- PASS

**Engine:** `ReplacementEffect` at replacement.go:176-208 has Applies/ApplyFn predicates. The ApplyFn mutates a `ReplEvent` in place (the "modified event" of 614.6).

### 614.5: Applied-once tracking -- PASS

**Engine:** `ReplEvent.AppliedIDs` (replacement.go:100) tracks which handlers have already been applied. `FireEvent()` at replacement.go:294 adds the HandlerID to AppliedIDs BEFORE calling ApplyFn, preventing re-application.

### 614.6: Modified event never happens -- PASS

**Engine:** `ReplEvent.Cancelled` (replacement.go:95) allows handlers to cancel the underlying event entirely. Wire-in helpers like `FireDrawEvent` (replacement.go:363) check Cancelled before proceeding.

### 614.7: Replaced events that never happen -- PASS

**Engine:** The Applies predicate (replacement.go:327) is checked before each application, so if the event condition is no longer true (e.g., 0 damage), the handler is skipped.

### 614.12: Copy-as-enters replacements -- PARTIAL (Gap 10)

**Engine:** The layer system handles copy effects via Layer 1 (613.2a), and `CopyPermanentLayered()` is available for runtime copy effects (used by Mirage Mirror, delayed_trigger_cards.go:79). However, "as [this] enters the battlefield" replacement effects for ETB choices (e.g., Clone choosing what to copy) are not fully structured in the replacement framework. They're handled via per-card hooks.

**Severity:** LOW. Per-card hooks are the correct approach for these cards in the current architecture.

### Wire-in coverage -- PASS (Comprehensive)

The replacement framework is wired into:
- Damage: `FireDamageEvent` (replacement.go:395-403)
- Draw: `FireDrawEvent` (replacement.go:363-369)
- Gain life: `FireGainLifeEvent` (replacement.go:374-381)
- Lose life: `FireLoseLifeEvent` (replacement.go:383-391)
- Put counter: `FirePutCounterEvent` (replacement.go:406-415)
- Create token: `FireCreateTokenEvent` (replacement.go:418-426)
- ETB trigger: `FireETBTriggerEvent` (replacement.go:429-437)
- Die: `FireDieEvent` (replacement.go:442-449)

12 canonical card handlers are implemented (Laboratory Maniac, Jace Wielder, Alhammarret's Archive, Boon Reflection, Rhox Faithmender, Rest in Peace, Leyline of the Void, Anafenza, Doubling Season, Hardened Scales, Panharmonicon, Platinum Angel).

---

## 615: Prevention Effects -- Detailed Findings

### Overall assessment -- MINIMAL (Gap 11 - HIGH)

**Engine:** Prevention is handled via two mechanisms:
1. A `prevent_damage` flag on permanents (resolve.go:138-145): when a `Prevent` effect is encountered, it sets `src.Flags["prevent_damage"] += amt`. This flag is not consumed by the damage resolution path.
2. A `suppress_prevention` global flag (resolve_helpers.go:163-169) for "Damage can't be prevented."
3. Protection-based prevention in combat.go:1061-1066 (protection from a quality prevents damage from sources of that quality).

**Missing:**
- No "Prevent the next N damage" shield system
- No "Prevent all damage" framework
- No prevention-as-replacement-effect integration (615.1 says prevention effects are a special case of replacement effects)
- The `prevent_damage` flag is set but never read by `applyDamage()` or `FireDamageEvent()`
- Protection prevents damage in combat but not from spells

**Severity:** HIGH. Prevention is a core rules concept. Cards like Fog, Teferi's Protection, and protection-based damage prevention are common.

---

## 616: Interaction of Replacement and Prevention Effects -- Detailed Findings

### 616.1: Category ordering -- PASS (Strong)

**Engine:** `replCategoryOrder()` at replacement.go:158-171 maps categories to numeric ranks:
- 0: Self-replacement (616.1a)
- 1: Control ETB (616.1b)
- 2: Copy ETB (616.1c)
- 3: Back face up (616.1d)
- 4: Other (616.1e)

`pickReplacement()` at replacement.go:311-354 sorts by (category rank ascending, APNAP tiebreak, timestamp ascending) and picks the first applicable effect.

### 616.1f: Iterate until no more applicable -- PASS

**Engine:** `FireEvent()` at replacement.go:284-307 loops up to `maxReplacementIterations` (64), checking for applicable replacements each iteration. This correctly implements 616.1f ("this process is repeated until there are no more left to apply").

### APNAP tiebreak -- PASS (with MVP simplification)

**Engine:** replacement.go:337-351 sorts by active player first within a category, then by timestamp. The code comments note this is "MVP deterministic by timestamp" rather than full player-choice, which is acceptable for simulation.

---

## Critical Gaps Summary

### Priority 1 (Should fix)

| # | Gap | Section | Severity | Description |
|---|-----|---------|----------|-------------|
| 9 | Target legality at resolution | 608.2b | CRITICAL | Spells don't check if targets are still legal at resolution. Should fizzle (be countered on resolution) if all targets are illegal. |
| 3 | Cost modification framework | 601.2f | HIGH | No cost increases/reductions (Thalia, Trinisphere, cost reducers). |
| 11 | Prevention effects | 615 | HIGH | Minimal implementation -- flag-based only, no shield system, prevent_damage flag unused. |

### Priority 2 (Should track)

| # | Gap | Section | Severity | Description |
|---|-----|---------|----------|-------------|
| 2 | Target locking at cast time | 601.2c | MEDIUM | Targets re-picked at resolution instead of locked at cast. |
| 1 | Mode choice step | 601.2b | MEDIUM | Modes chosen at resolution rather than at cast time. |
| 5 | Sorcery-speed activations | 602.5d | MEDIUM | "Activate only as a sorcery" timing not enforced. |
| 6 | Intervening "if" clause | 603.4 | MEDIUM | First check (at trigger time) skipped; only second check (at resolution) performed. |
| 7 | State triggers | 603.8 | MEDIUM | No mechanism for "whenever [condition is true]" state-triggered abilities. |

### Priority 3 (Acceptable for MVP)

| # | Gap | Section | Severity | Description |
|---|-----|---------|----------|-------------|
| 4 | Multi-part cost rollback | 602.2 | LOW | Partial rollback for complex costs. |
| 8 | Reflexive triggers | 603.12 | LOW | No "when you do" trigger support. |
| 10 | Copy-as-enters replacements | 614.12 | LOW | Handled via per-card hooks rather than framework. |
| -- | Dependency ordering | 613.8 | LOW | Documented as deferred; timestamp order used instead. |
| -- | Damage distribution | 601.2d | LOW | Division of damage/counters among targets not supported. |

---

## Strengths

1. **Layer system:** All 7 layers + sublayers correctly ordered, with caching and epoch invalidation. This is production-quality.
2. **Replacement effect framework:** Full CR 616.1 category ordering with applied-once tracking and iterate-until-done loop. Well-wired into 8 event types.
3. **APNAP trigger ordering:** `OrderTriggersAPNAP()` is a textbook implementation of CR 603.3b.
4. **Stax enforcement:** Null Rod, Cursed Totem, Grand Abolisher, and Split Second correctly integrated into the activation and casting paths.
5. **Alternative/additional costs:** Pitch (Force of Will), evoke, commander-free, spell-count-threshold, sacrifice, exile, and pay-life costs all implemented.
6. **Zone casting:** Flashback, escape, exile-cast, and library-cast all wired with correct post-resolution routing.
7. **Delayed triggers:** Full lifecycle with registration, phase-boundary firing, event-based firing, and oneshot semantics.
8. **Code quality:** Every function cites the specific CR section it implements. The code is heavily commented with rules citations throughout.

---

## Test Coverage

The engine has 12,830 lines of test code across 18 test files:
- `stack_test.go` (930 lines) -- casting, priority, split-second
- `layers_test.go` (901 lines) -- all 7 layers
- `combat_test.go` (848 lines) -- includes protection prevention
- `resolve_test.go` (823 lines) -- effect resolution
- `replacement_test.go` (546 lines) -- replacement effects
- `activation_test.go` (565 lines) -- activated abilities, stax
- `costs_test.go` (619 lines) -- alternative/additional costs
- `zone_cast_test.go` (495 lines) -- flashback, escape, exile-cast
- `triggers_test.go` -- APNAP ordering
- `phases_test.go` -- phase triggers, delayed triggers

---

## Recommendations

1. **608.2b (Critical):** Add a `checkTargetLegality()` call at the top of `ResolveStackTop()` that re-validates `item.Targets` against current game state. If all targets are illegal, set `item.Countered = true` and log a `"countered_on_resolution"` event with rule `"608.2b"`.

2. **601.2f (High):** Implement a `CostModification` registry on GameState (similar to ContinuousEffects) that accumulates cost increases and reductions. Apply them in `manaCostOf()` or a new `totalCostOf()` function.

3. **615 (High):** Extend the replacement framework to handle prevention as a special category. Add a `"would_be_dealt_damage"` replacement that checks for prevention shields before the existing damage doublers/redirectors.

4. **601.2b/c (Medium):** Lock modes and targets on the StackItem at cast time. Change `PickTarget` calls in resolve.go to read from `item.Targets` when available.

5. **602.5d (Medium):** Add a `SorcerySpeedActivation` flag on `gameast.Activated` and check it in `ActivateAbility()` against the current phase/stack state.
