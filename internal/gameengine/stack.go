package gameengine

// Phase 5 — Stack + priority system.
//
// This file wires CR §117 priority, §601 spell casting, and §608.2 spell
// resolution on top of the GameState + resolver primitives built in
// Phases 3–4. It mirrors the Python reference at scripts/playloop.py —
// specifically cast_spell, _priority_round, _get_response,
// _resolve_stack_top, _split_second_active, and
// _opp_restricts_defender_to_sorcery_speed.
//
// Scope:
//
//   - CastSpell(gs, seat, card, targets)     — CR §601.2 casting sequence
//   - PushStackItem(gs, item)                — allocate ID + append + log
//   - PushTriggeredAbility(gs, src, effect)  — CR §603.2 put trigger on stack
//   - PriorityRound(gs)                      — CR §117.3-5 APNAP polling
//   - ResolveStackTop(gs)                    — CR §608.2 pop + resolve
//   - GetResponse(gs, defenderSeat, top)     — policy hook (greedy default)
//   - SplitSecondActive(gs)                  — CR §702.61a detection
//   - OppRestrictsDefenderToSorcerySpeed     — CR §307.1 / §601.3a check
//
// Comp-rules citations throughout refer to data/rules/MagicCompRules-20260227.txt.
//
// Implementation notes:
//   - Stack is LIFO; gs.Stack[len-1] is the TOP. §608.2 pops the top.
//   - Priority round is bounded at 16 iterations to match Python — counter
//     wars should terminate long before that; the cap catches policy bugs.
//   - Greedy response policy: a seat with a CounterSpell-bearing instant in
//     hand and enough mana to pay will always counter an opponent's spell.
//     Policy hooks can be layered later (Phase 10).
//   - Post-resolution: SBAs fire (CR §704.3 / §117.5), then priority re-opens
//     if the stack is still non-empty.
//   - Triggered abilities from combat damage, attack, and ETB now flow
//     through the stack rather than resolving inline. This is the Phase 4
//     coupling flagged in the combat agent's handoff note.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// maxStackDrainIterations caps the resolve-SBA-priority loop that runs
// after a cast or activation. Each iteration resolves one stack item;
// cascading triggers can push many items per resolution, causing the loop
// to run thousands of times. 500 iterations is enough for any legal game
// state (most games peak at 20-30) while preventing the engine from
// spinning for minutes on degenerate trigger avalanches.
const maxStackDrainIterations = 500

// maxDrainRecursion caps how deep DrainStack can recurse into itself
// (via ResolveStackTop → trigger handler → CastSpell → DrainStack).
const maxDrainRecursion = 10

// maxResolveDepth caps the inline PriorityRound+ResolveStackTop recursion
// inside PushTriggeredAbility. Reanimate/sacrifice loops create a cycle:
// PushTriggeredAbility → PriorityRound → ResolveStackTop → ResolveEffect
// → zone-change triggers → PushTriggeredAbility → ... which recurses
// through Go's call stack. Beyond this depth, triggers are left on the
// stack for DrainStack's iterative loop to resolve.
const maxResolveDepth = 50

// maxTriggerFiresPerTurn caps total trigger firings within one turn to
// prevent infinite trigger loops from consuming unbounded memory.
// A typical complex turn fires 20-50 triggers; 1000 is pathological.
const maxTriggerFiresPerTurn = 1000

// triggerCapForGame returns the per-turn trigger fire limit. MCTS rollout
// clones set _rollout_trigger_cap for a tighter budget since rollouts are
// approximations and don't need full trigger fidelity.
func triggerCapForGame(gs *GameState) int {
	if gs.Flags != nil {
		if cap := gs.Flags["_rollout_trigger_cap"]; cap > 0 {
			return cap
		}
	}
	return maxTriggerFiresPerTurn
}

// DrainStack resolves items until the stack is empty, with loop detection
// (CR §727) and an iteration safety cap. Used by all cast/activation paths.
func DrainStack(gs *GameState) {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["drain_depth"]++
	defer func() { gs.Flags["drain_depth"]-- }()
	if gs.Flags["drain_depth"] > maxDrainRecursion {
		gs.Stack = gs.Stack[:0]
		return
	}

	var ld *loopDetector
	for drainIter := 0; len(gs.Stack) > 0 && drainIter < maxStackDrainIterations; drainIter++ {
		if drainIter >= loopMinReps*2 {
			if ld == nil {
				ld = newLoopDetector()
			}
			ld.record(gs, stackTopFingerprint(gs))
			if ld.projectAndApply(gs) {
				break
			}
		}
		ResolveStackTop(gs)
		StateBasedActions(gs)
		if len(gs.Stack) > 0 {
			PriorityRound(gs)
		}
	}
	// The iteration cap previously exited SILENTLY with items still on
	// the stack — invisible to liveness auditing. Emit the uniform
	// guard event so cap_contract scanning sees it.
	if len(gs.Stack) > 0 {
		LogLoopGuardFired(gs, "drain_iteration_cap", map[string]interface{}{
			"stack_remaining": len(gs.Stack), "cap": maxStackDrainIterations,
		})
	}
	StateBasedActions(gs)
}

// ---------------------------------------------------------------------------
// Stack-item construction + push.
// ---------------------------------------------------------------------------

// nextStackID returns the next monotonically increasing stack item ID.
// We reuse gs.EffectTimestamp's counter via a dedicated flag key so
// state.go stays untouched (Phase 5 contract).
func nextStackID(gs *GameState) int {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_stack_seq"]++
	return gs.Flags["_stack_seq"]
}

// PushStackItem pushes a prepared StackItem onto gs.Stack, assigns it an ID
// if it doesn't already have one, and logs a stack_push event. Shared helper
// used by CastSpell, PushTriggeredAbility, and the response-casting code
// path inside PriorityRound.
func PushStackItem(gs *GameState, item *StackItem) *StackItem {
	if gs == nil || item == nil {
		return item
	}
	if item.ID == 0 {
		item.ID = nextStackID(gs)
	}
	gs.Stack = append(gs.Stack, item)
	name := ""
	if item.Card != nil {
		name = item.Card.DisplayName()
	} else if item.Source != nil {
		name = item.Source.Card.DisplayName()
	}
	// Stack trace: log push event for CR audit.
	GlobalStackTrace.Log("push", name, item.Controller, len(gs.Stack), "spell_cast")
	gs.LogEvent(Event{
		Kind:   "stack_push",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"stack_id":   item.ID,
			"stack_size": len(gs.Stack),
			"rule":       "608.1",
		},
	})
	return item
}

// PushTriggeredAbility creates a StackItem for a triggered ability and pushes
// it. Mirrors CR §603.2: "Whenever a game event or game state matches a
// triggered ability's trigger event, the ability automatically triggers. The
// ability doesn't do anything at this point." Then per §603.3, "the next
// time a player would receive priority, each ability that has triggered but
// hasn't yet been put on the stack is put on the stack."
//
// We push immediately here (no lazy bucket) because the Phase 4 combat
// code fires triggers at well-defined rulebook moments and the engine is
// single-threaded. A future "pending triggers" queue is a Phase 7 concern.
func PushTriggeredAbility(gs *GameState, src *Permanent, effect gameast.Effect) *StackItem {
	return PushTriggeredAbilityWithIf(gs, src, effect, nil)
}

// PushTriggeredAbilityWithIf is PushTriggeredAbility carrying the
// trigger's intervening "if" clause (CR §603.4, r63 PROGRESSION
// dimension): the condition is checked HERE — when the ability would
// trigger; a false condition means it never triggers at all — and
// AGAIN at resolution (stamped onto the StackItem; ResolveStackTop
// re-evaluates and the ability does nothing if it became false).
// Pre-r63 the engine ignored InterveningIf entirely; the corpus also
// emits zero intervening_if nodes today (parser gap, reported in
// /tmp/fable-review/progression-triggers-r63.md), so this lights up as
// the parser starts carrying the clause.
func PushTriggeredAbilityWithIf(gs *GameState, src *Permanent, effect gameast.Effect, interveningIf *gameast.Condition) *StackItem {
	if gs == nil || src == nil || effect == nil {
		return nil
	}
	if interveningIf != nil && !evalCondition(gs, src, interveningIf) {
		gs.LogEvent(Event{
			Kind: "trigger_suppressed", Seat: src.Controller, Target: -1,
			Source: sourceName(src),
			Details: map[string]interface{}{
				"reason": "intervening_if_false_at_trigger",
				"rule":   "603.4",
			},
		})
		return nil
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		return nil
	}
	// CR §800.4a: an ability controlled by a player who has left the game
	// ceases to exist — it never goes on the stack. Mirrors the per-card
	// path guard in PushPerCardTrigger; covers AST-driven triggers fired
	// AFTER their controller's elimination sweep (r63 depth-frontier).
	if SeatHasLeftGame(gs, src.Controller) {
		name := ""
		if src.Card != nil {
			name = src.Card.DisplayName()
		}
		gs.LogEvent(Event{
			Kind:   "trigger_ceased",
			Seat:   src.Controller,
			Source: name,
			Details: map[string]interface{}{
				"reason": "controller_left_game",
				"rule":   "800.4a",
			},
		})
		return nil
	}
	gs.Flags["_trigger_fires_this_turn"]++
	if gs.Flags["_trigger_fires_this_turn"] > triggerCapForGame(gs) {
		capCard := ""
		if src.Card != nil {
			capCard = src.Card.DisplayName()
		}
		LogLoopGuardFired(gs, "trigger_loop_cap", map[string]interface{}{
			"fires": gs.Flags["_trigger_fires_this_turn"], "site": "ast_trigger",
			"card": capCard,
		})
		for i, s := range gs.Seats {
			if s != nil && !s.Lost && !s.Won {
				markSeatLostLoopDraw(s)
				gs.LogEvent(Event{Kind: "game_draw", Seat: i, Details: map[string]interface{}{"reason": "trigger_loop_cap"}})
			}
		}
		gs.Stack = gs.Stack[:0]
		gs.Flags["game_draw"] = 1
		gs.Flags["ended"] = 1
		return nil
	}
	item := &StackItem{
		Controller: src.Controller,
		Source:     src,
		Effect:     effect,
		Kind:       "triggered",
	}
	if interveningIf != nil {
		item.CostMeta = map[string]interface{}{"intervening_if": interveningIf}
	}
	if src.Card != nil {
		// StackItem.Card is usually for spells, not triggers, but we point it
		// at the source card so logs show the right name.
		item.Card = src.Card
	}
	// Mint an AB AbilityInstance for this triggered ability. The enabler
	// is the InstanceID of the currently-resolving frame (the spell or
	// ability whose resolution / event caused this trigger to fire) —
	// captured by walking gs.IIDEnablerStack. For self-triggers (ETB on
	// the trigger source itself), the enabler is whoever caused the ETB;
	// for observer triggers, it's the ability that produced the observed
	// event. Empty when no resolving frame exists (turn-edge triggers).
	abilityID := "trig"
	if effect != nil {
		abilityID = "trig:" + effect.Kind()
	}
	item.Ability = NewAbilityInstance(gs, src, src.Controller,
		abilityID, currentMintEnablerID(gs), nil)

	// CR §603.2: "the ability automatically triggers." Log the trigger
	// formation now — before the batch/push fork — so TriggerCompleteness
	// and downstream observers see a marker regardless of whether the
	// trigger goes through the §603.3b batch path (collectTrigger →
	// PushSimultaneousTriggers, which doesn't log per-item) or the
	// inline push+resolve path. Without this, AST-driven dies/etb triggers
	// whose Effect resolution doesn't itself emit life_change/etc. leave
	// no event-log breadcrumb that the invariant accepts.
	trigName := ""
	if src.Card != nil {
		trigName = src.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "triggered_ability",
		Seat:   src.Controller,
		Source: trigName,
		Details: map[string]interface{}{
			"rule":    "603.2",
			"batched": gs.triggerBatchDepth > 0,
		},
	})

	// CR §603.3b: if a batch is open, defer the push so siblings can be
	// ordered together at EndTriggerBatch. See trigger_batch.go.
	if gs.triggerBatchDepth > 0 {
		collectTrigger(gs, item)
		return item
	}

	// CR §608.2c: if we're inside the resolution of an outer spell or
	// ability, the trigger goes on the stack AFTER that resolution
	// completes — defer it to the pending queue. The outer ResolveStackTop's
	// outermost-frame defer drains the queue. Without this, a trigger
	// fired by an effect mid-resolution would push+resolve before the
	// outer effect finished, which §608.2c explicitly forbids.
	if gs.Flags["_resolve_frame_depth"] > 0 {
		collectTrigger(gs, item)
		return item
	}

	// Stack trace: log triggered ability push for CR audit.
	GlobalStackTrace.Log("trigger_push", trigName, src.Controller, len(gs.Stack), "triggered_ability")
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["resolve_depth"]++
	defer func() { gs.Flags["resolve_depth"]-- }()

	if gs.Flags["resolve_depth"] > maxResolveDepth {
		return item
	}

	// Per CR §117.3a priority opens on triggers — open a priority round then
	// resolve. This matches the Python _push_trigger_and_resolve pattern.
	PriorityRound(gs)
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		ResolveStackTop(gs)
	}
	return item
}

// ---------------------------------------------------------------------------
// CastSpell — CR §601.
// ---------------------------------------------------------------------------

// CastError is returned by CastSpell when the cast fails a legality check
// (card not in hand, insufficient mana, split-second in play, etc.).
// CR §601.2e: "If the proposed spell is illegal, the game returns to the
// moment before the casting of that spell was proposed."
type CastError struct {
	Reason string
}

func (e *CastError) Error() string { return "cast failed: " + e.Reason }

// CastSpell executes the CR §601.2 casting sequence for a single spell:
//
//  1. §601.2a  — announce the spell (remove from hand, create stack item).
//  2. §601.2b  — choose modes / targets (caller-supplied targets[]).
//  3. §601.2f  — pay costs (mana only for MVP).
//  4. Priority opens (CR §117.3c).
//  5. On all-pass, top of stack resolves (CR §117.4).
//
// Returns a CastError on any of:
//   - card not in caster's hand
//   - insufficient mana in pool
//   - split-second spell on stack forbidding non-mana casts (CR §702.61a)
//   - sorcery-speed restriction + stack non-empty (CR §307.1 / §601.3a)
//
// For MVP, mana cost is read from card.AST.ManaCost.CMC() if available,
// else the callers of CastSpell can stash a `_cost` flag via Flags. A real
// Phase 8 color/mana pool will replace this.
func CastSpell(gs *GameState, seatIdx int, card *Card, targets []Target) error {
	if gs == nil {
		return &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return &CastError{Reason: "nil card"}
	}
	seat := gs.Seats[seatIdx]

	// CR §702.61a: split-second spell on stack forbids casting non-mana
	// spells. Mana abilities are fine, but the caller doesn't pass those
	// through CastSpell.
	if SplitSecondActive(gs) {
		return &CastError{Reason: "split_second"}
	}

	// CR §307.1: sorcery-speed timing. Sorcery-type cards can only be cast
	// during the active player's main phase when the stack is empty.
	// CRITICAL (r61): the live turn runner sets gs.Phase = "main" (with
	// gs.Step = "precombat_main"/"postcombat_main") — see turn.go:275,354.
	// "precombat_main"/"postcombat_main" are STEP values, not PHASE values,
	// so the original disjunction matched NONE of the real main-phase casts
	// and silently rejected every sorcery the AI tried to cast in a real game
	// (every wrath/tutor/ramp sorcery went dead). gs.Phase == "main" is the
	// canonical main-phase value; "" is kept for fixtures and the step-named
	// values for any fixture that sets them as the phase. "beginning"
	// (upkeep/draw) is intentionally NOT a sorcery-speed window.
	if cardHasType(card, "sorcery") {
		isMainPhase := gs.Phase == "" || gs.Phase == "main" ||
			gs.Phase == "beginning" || gs.Phase == "precombat_main" ||
			gs.Phase == "postcombat_main"
		if !isMainPhase || len(gs.Stack) > 0 {
			return &CastError{Reason: "sorcery_speed_timing"}
		}
	}

	// CR §307.1 / §601.3a: a Teferi-style static that restricts the seat to
	// sorcery speed while an opponent controls it, combined with a non-empty
	// stack, forbids the cast. Active player casting sorceries on an empty
	// stack is fine.
	if len(gs.Stack) > 0 && OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return &CastError{Reason: "sorcery_speed_restriction"}
	}

	// Grand Abolisher: opponents can't cast spells during the
	// Abolisher-controller's turn.
	if seatIdx != gs.Active && grandAbolisherBlocksCast(gs, seatIdx) {
		return &CastError{Reason: "grand_abolisher"}
	}

	// CR §601.2c: targets are declared at announcement; illegal targets
	// (hexproof, shroud, protection, off-battlefield) must abort the cast
	// per §601.2e ("the game returns to the moment before the casting was
	// proposed"). PickTarget enforces this for AI-driven callers, but
	// CastSpell is the central trust boundary — validate again here so
	// fuzz-generated or fixture-built target lists can't bypass §115.2.
	if len(targets) > 0 {
		if err := ValidateTargetsAtAnnouncement(gs, seatIdx, card, targets, nil); err != nil {
			return err
		}
	}

	// Remove from hand. CR §601.2a places the card on the stack (it leaves
	// its origin zone) as the first step of casting.
	if !removeFromHand(seat, card) {
		return &CastError{Reason: "not_in_hand"}
	}
	// Cast-transit census presence (r63 CONSERVATION residual class,
	// seed-5150 game 780): between the hand-removal above and the stack
	// push below, the card exists only in this frame. A cast trigger
	// that resolves immediately (the PushPerCardTrigger bridge) can
	// eliminate a seat from inside that window; HandleSeatElimination's
	// orphan sweep then reads the in-flight card as minted-but-absent
	// and ceases the LIVING caster's card — its later graveyard arrival
	// reads as a stale-ceased fabrication for the rest of the game.
	// Track it in gs.ResolvingCards (the established mid-flight limbo
	// registry, counted as zone presence by the census and the sweep)
	// for the full CastSpell window; the deferred pop also covers the
	// cost-failure rollback paths, which return the card to hand.
	gs.ResolvingCards = append(gs.ResolvingCards, card)
	defer func() {
		gs.ResolvingCards = gs.ResolvingCards[:len(gs.ResolvingCards)-1]
	}()

	// Ride-along legality validator (legality.go): snapshot announcement-
	// time state BEFORE any cost is paid. nil-receiver no-op when off.
	legalityObs := gs.Legality.BeginCast(gs, seatIdx, card)

	// Pay mana cost per CR §601.2f. CalculateTotalCost walks the battlefield
	// for static cost modifiers (Thalia, Trinisphere, Helm of Awakening,
	// medallions, etc.) and applies increases → reductions → minimums.
	baseCost := CalculateTotalCost(gs, card, seatIdx)
	chosenX := 0

	// CR §601.2b — cast-time ALTERNATIVE costs (overload / surge / spectacle).
	// These REPLACE the printed mana cost, so the decision must happen BEFORE
	// the base mana is paid. We ask the Hat once per applicable mechanic (the
	// keyword guards skip the 99% of cards entirely) and, on a "yes", swap
	// baseCost for the alternative cost and remember which one to stamp on the
	// StackItem after it is built. Only one alternative cost can apply per
	// cast; the precedence order here (overload → surge → spectacle) is
	// arbitrary — no printed card carries two of these keywords.
	altCostMeta := map[string]interface{}{}
	if HasOverload(card) {
		oc := OverloadCost(card)
		avail := EnsureTypedPool(seat).Total()
		maxPay := 0
		// oc == 0 means the parser didn't capture the printed overload
		// cost — decline rather than overload for free (no printed
		// overload cost is {0}).
		if oc > 0 && avail >= oc {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "overload", oc, maxPay) > 0 {
			baseCost = oc
			altCostMeta["overloaded"] = true
		}
	}
	if len(altCostMeta) == 0 && HasSurge(card) && CanPaySurge(gs, seatIdx) {
		sc := SurgeCost(card)
		avail := EnsureTypedPool(seat).Total()
		maxPay := 0
		// sc == 0 means the parser didn't capture the printed surge cost
		// — decline rather than surge for free.
		if sc > 0 && avail >= sc {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "surge", sc, maxPay) > 0 {
			baseCost = sc
			altCostMeta["surge_cast"] = true
			altCostMeta["surge_cost"] = sc
		}
	}
	if len(altCostMeta) == 0 && HasSpectacle(card) && CanPaySpectacle(gs, seatIdx) {
		spc := SpectacleCost(card)
		avail := EnsureTypedPool(seat).Total()
		maxPay := 0
		// spc == 0 means the parser didn't capture the printed spectacle
		// cost — decline rather than cast for free.
		if spc > 0 && avail >= spc {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "spectacle", spc, maxPay) > 0 {
			baseCost = spc
			altCostMeta["spectacle_cast"] = true
			altCostMeta["spectacle_cost"] = spc
		}
	}

	// §107.3: if the mana cost contains X, the Hat announces X.
	if ManaCostContainsX(card) {
		xPool := EnsureTypedPool(seat)
		availableForX := xPool.Total() - baseCost
		if availableForX < 0 {
			seat.Hand = append(seat.Hand, card)
			return &CastError{Reason: "insufficient_mana"}
		}
		if seat.Hat != nil {
			chosenX = seat.Hat.ChooseX(gs, seatIdx, card, availableForX)
			if chosenX < 0 {
				chosenX = 0
			}
			if chosenX > availableForX {
				chosenX = availableForX
			}
		} else {
			// No hat — default to spending all available mana.
			chosenX = availableForX
		}
	}

	cost := baseCost + chosenX
	// Check total available mana. Use EnsureTypedPool to bridge any
	// legacy ManaPool integer into the typed pool, then read Total()
	// as the single source of truth. The previous approach of adding
	// seat.ManaPool + seat.Mana.Total() double-counted because AddMana
	// already syncs ManaPool = Mana.Total().
	pool := EnsureTypedPool(seat)
	availMana := pool.Total()
	if availMana < cost {
		seat.Hand = append(seat.Hand, card)
		return &CastError{Reason: "insufficient_mana"}
	}
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)

	// CR §702.33 — cast-time optional additional cost (kicker / multikicker).
	// AFTER the base + X cost is settled, the caster may choose to pay the
	// kicker cost (0+ times). We compute the max affordable kicks from the
	// remaining mana, ask the Hat, then charge that many × kicker cost as a
	// further mana payment. The decision is stamped onto CostMeta below
	// (one canonical key) and mirrored onto the permanent at ETB. Cards with
	// no kicker keyword skip this entirely — zero behavior change for the
	// 99% of cards. See keywords_batch.go for the helper family.
	kickCount := 0
	if kc := KickerCost(card); kc > 0 && HasKickerKeyword(card) {
		remaining := EnsureTypedPool(seat).Total()
		maxKicks := remaining / kc
		if !IsMultikicker(card) && maxKicks > 1 {
			maxKicks = 1 // single kicker: at most once (CR §702.33c)
		}
		if maxKicks > 0 {
			chosen := maxKicks
			if seat.Hat != nil {
				chosen = seat.Hat.ChooseKickCount(gs, seatIdx, card, kc, maxKicks)
			} else {
				chosen = 0 // no hat — default to NOT kicking (conservative)
			}
			if chosen < 0 {
				chosen = 0
			}
			if chosen > maxKicks {
				chosen = maxKicks
			}
			if chosen > 0 {
				kickCost := chosen * kc
				seat.ManaPool -= kickCost
				SyncManaAfterSpend(seat)
				kickCount = chosen
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seatIdx,
					Amount: kickCost,
					Source: card.DisplayName(),
					Details: map[string]interface{}{
						"reason":          "kicker",
						"rule":            "702.33",
						"multikick_count": chosen,
						"kicker_cost":     kc,
					},
				})
				gs.LogEvent(Event{
					Kind:   "kicker",
					Seat:   seatIdx,
					Source: card.DisplayName(),
					Details: map[string]interface{}{
						"rule":            "702.33",
						"multikick_count": chosen,
					},
				})
			}
		}
	}

	// CR §702.27 — Buyback. Additional mana cost paid AFTER base + X + kicker.
	// Pay it → the spell returns to its owner's hand on resolution instead of
	// the graveyard (CostMeta["bought_back"], consumed by ResolveStackTop via
	// ShouldReturnToHandOnResolve). Same shape as kicker: detect the keyword,
	// compute affordability from leftover mana, ask the Hat, charge, stamp.
	boughtBack := false
	buybackCost := 0
	if HasBuyback(card) &&
		(cardHasType(card, "instant") || cardHasType(card, "sorcery")) {
		bc := BuybackCost(card)
		remaining := EnsureTypedPool(seat).Total()
		maxPay := 0
		// bc == 0 means the parser didn't capture the printed buyback
		// cost (24 of 29 printed buyback costs are single-pip and were
		// dropped by the pre-r61.1 parser) — decline rather than grant
		// Capsize-class buyback for free.
		if bc > 0 && remaining >= bc {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "buyback", bc, maxPay) > 0 {
			seat.ManaPool -= bc
			SyncManaAfterSpend(seat)
			boughtBack = true
			buybackCost = bc
			gs.LogEvent(Event{
				Kind:   "pay_mana",
				Seat:   seatIdx,
				Amount: bc,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"reason":       "buyback",
					"rule":         "702.27a",
					"buyback_cost": bc,
				},
			})
			gs.LogEvent(Event{
				Kind:    "buyback_cast",
				Seat:    seatIdx,
				Source:  card.DisplayName(),
				Amount:  bc,
				Details: map[string]interface{}{"rule": "702.27a"},
			})
			if gs.Flags == nil {
				gs.Flags = map[string]int{}
			}
			gs.Flags["spell_bought_back_this_turn:"+itoa(seatIdx)] = 1
		}
	}

	if cost > 0 {
		details := map[string]interface{}{
			"reason": "cast",
			"rule":   "601.2f",
		}
		if chosenX > 0 {
			details["chosen_x"] = chosenX
		}
		gs.LogEvent(Event{
			Kind:    "pay_mana",
			Seat:    seatIdx,
			Amount:  cost,
			Source:  card.DisplayName(),
			Details: details,
		})
	}
	gs.LogEvent(Event{
		Kind:   "cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cost,
		Details: map[string]interface{}{
			"rule":     "601.2",
			"chosen_x": chosenX,
		},
	})

	// CR §700.4 / §702.40 cast-count bookkeeping. Increment BEFORE storm
	// + cast-trigger observers so the storm spell itself counts toward
	// its own storm tally (copies = spells_cast_this_turn - 1). Copies
	// do NOT call IncrementCastCount (§707.10).
	IncrementCastCount(gs, seatIdx)
	RecordCast(gs, seatIdx, card, 0)

	// Fire per-card triggers keyed on "spell cast" events. Rhystic Study,
	// Mystic Remora, Aetherflux Reservoir, Displacer Kitten, Hullbreaker
	// Horror all depend on these. We fire BEFORE pushing the stack item
	// so the triggered abilities go onto the stack ABOVE the spell being
	// cast — matching CR §603.3 ("the next time a player would receive
	// priority...").
	gs.Flags["_cast_chosen_x"] = chosenX
	fireCastTriggers(gs, seatIdx, card)
	delete(gs.Flags, "_cast_chosen_x")

	// Bridge fire for cast-trigger observer permanents NOT yet wired into
	// the per_card registry (Storm-Kiln Artist, Young Pyromancer, Birgi,
	// Monastery Mentor, Runaway Steam-Kin, Niv-Mizzet Parun, Third Path
	// Iconoclast). Real casts only; storm copies bypass. Long-term these
	// should migrate to the per_card pipeline.
	FireCastTriggerObservers(gs, card, seatIdx, false)

	// Build stack item. For non-permanent spells (instants/sorceries) we
	// pull the Effect off the AST's first Activated/Triggered or — more
	// commonly — the collected spell effect. MVP: pick the first Damage/
	// CounterSpell/Draw/etc. from card.AST.Abilities by scanning for
	// Activated-with-empty-cost (spell effect pattern).
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: seatIdx,
		Card:       card,
		Effect:     eff,
		Targets:    targets,
		ChosenX:    chosenX,
	}
	// CR §702.33 — stamp the kicker decision in the single canonical place
	// (CostMeta["kicked"] / CostMeta["multikick_count"]) so the ETB mirror
	// and every downstream consumer (Grunn, Zethi, Everflowing Chalice) read
	// the same value. Only the card carries a kicker keyword stamps a result.
	if HasKickerKeyword(card) {
		StampKickResult(item, kickCount)
	}
	// PR-5 cost-mechanic family — stamp the cast-time optional/alternative/
	// additional-cost decisions chosen above onto the canonical CostMeta keys
	// the resolution-time consumers read (overload fan-out via
	// beginOverloadResolution; buyback return-to-hand via
	// ShouldReturnToHandOnResolve; surge/spectacle per-card OnResolve gates).
	if len(altCostMeta) > 0 || boughtBack {
		if item.CostMeta == nil {
			item.CostMeta = map[string]interface{}{}
		}
		for k, v := range altCostMeta {
			item.CostMeta[k] = v
		}
		if boughtBack {
			item.CostMeta["bought_back"] = true
			item.CostMeta["buyback_cost"] = buybackCost
		}
	}
	PushStackItem(gs, item)

	// r62 — CR §601.2c announcement-time target selection. When the caller
	// passed no targets (the turn runner and most AI paths), pick them NOW
	// for required single-target spells: enumerate the legal set, consult
	// the seat's Hat (§608.2a ChooseTarget), stamp the result onto
	// item.Targets. This is what makes CheckWardOnTargeting /
	// FireHeroicTriggers below and the populated-path §608.2b resolution
	// gate operate on real data for AI casts, and what un-inerts the r61
	// PR-7/PR-8 hat targeting work. Runs AFTER PushStackItem so the Hat's
	// intent classifier (which inspects the caster's topmost stack item)
	// sees the spell being cast. Skipped for overloaded casts (§702.96
	// rewrites "target" to "each" — there are no targets) and when no
	// legal target exists (Targets stays empty; the r61 PR-3 lazy fizzle
	// gate applies unchanged).
	if len(item.Targets) == 0 && item.Effect != nil && altCostMeta["overloaded"] == nil {
		if f, ok := announceTargetFilter(item); ok {
			announceSrc := &Permanent{
				Card:       card,
				Controller: seatIdx,
				Owner:      card.Owner,
				Flags:      map[string]int{},
			}
			item.Targets = AnnounceTargets(gs, announceSrc, seatIdx, f)
		}
	}

	// Ride-along legality validator: the announcement is complete — the
	// spell is on the stack with its targets and CostMeta stamped, and
	// base + X + kicker + buyback have been paid (replicate/conspire/
	// casualty additional costs below are out of phase-1 scope). Runs
	// the registered checks. nil-receiver no-op when off.
	gs.Legality.FinishCast(gs, legalityObs, item)

	// CR §702.56 / §702.78 / §702.153 — cast-time ADDITIONAL costs that COPY
	// the spell (replicate / conspire / casualty). These act on the StackItem
	// AFTER it is on the stack: the helpers pay the cost (extra mana for
	// replicate; tap-two for conspire; sacrifice for casualty) and push the
	// resulting copies ABOVE the original. The keyword guards skip the 99% of
	// cards with none of these keywords.
	if HasReplicate(card) {
		rc := ReplicateCost(card)
		remaining := EnsureTypedPool(seat).Total()
		maxCopies := 0
		if rc > 0 {
			maxCopies = remaining / rc
		}
		if maxCopies > 0 && seat.Hat != nil {
			n := seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "replicate", rc, maxCopies)
			if n > maxCopies {
				n = maxCopies
			}
			if n > 0 {
				ApplyReplicate(gs, item, n)
			}
		}
	}
	if HasConspire(card) && len(card.Colors) > 0 {
		// Conspire needs two untapped creatures sharing a color with the
		// spell (ApplyConspire enforces this and taps them). max=1 only when
		// two eligible creatures are present so the Hat isn't asked to pay an
		// impossible cost.
		if conspireEligibleCount(gs, seatIdx, card) >= 2 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "conspire", 0, 1) > 0 {
			ApplyConspire(gs, seatIdx, item)
		}
	}
	if HasCasualty(card) {
		minPow := CasualtyMinPower(card)
		maxPay := 0
		// minPow == 0 means the parser didn't capture the printed
		// casualty N — decline rather than sacrifice for an unknown cost.
		if minPow > 0 && CanPayCasualty(gs, seatIdx, minPow) {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "casualty", minPow, maxPay) > 0 {
			ApplyCasualty(gs, seatIdx, item, minPow)
		}
	}
	if HasBargain(card) {
		maxPay := 0
		if CanBargain(gs, seatIdx) {
			maxPay = 1
		}
		if maxPay > 0 && seat.Hat != nil &&
			seat.Hat.ChooseOptionalCost(gs, seatIdx, card, "bargain", 0, maxPay) > 0 {
			ApplyBargain(gs, seatIdx, item)
		}
	}

	// CR §702.21 — Ward. "When this creature becomes the target of a
	// spell or ability an opponent controls, counter it unless that
	// player pays {cost}." Check each target for ward and apply.
	CheckWardOnTargeting(gs, item)

	// CR §702.123 — Heroic. "Whenever you cast a spell that targets
	// this creature, [effect]." Cast-time trigger; fires regardless of
	// whether the spell ultimately resolves. Mirror cast-time placement
	// of the ward check above so both targeting triggers converge here.
	FireHeroicTriggers(gs, item)

	// CR §702.40 — storm trigger. (Copies land ON TOP of the original
	// storm spell. LIFO resolution gives the copies priority, which is
	// gameplay-correct: triggered abilities go on the stack above the
	// spell that triggered them.)
	if HasStormKeyword(card) {
		ApplyStormCopies(gs, item, seatIdx)
	}

	// CR §702.84 — cascade trigger. Exile from library until nonland
	// with lesser CMC, may cast for free, put rest on bottom.
	if HasCascadeKeyword(card) {
		ApplyCascade(gs, seatIdx, manaCostOf(card), card.DisplayName())
	}

	// CR §701.51 — discover trigger. Like cascade but card goes to hand.
	if cardHasKeyword(card, "discover") {
		PerformDiscover(gs, seatIdx, manaCostOf(card))
	}

	// Per-card cast-time snowflake dispatch (currently unused in batch #1
	// but wired for future cards that need to mutate state at cast).
	InvokeCastHook(gs, item)

	// CR §117.3c: caster keeps priority after casting. Open a priority
	// window in which opponents can respond.
	PriorityRound(gs)

	// CR §117.4 + §608.2 + §727: resolve stack with loop shortcut detection.
	DrainStack(gs)
	return nil
}

// removeFromHand removes the card (by pointer identity) from seat.Hand.
// Returns true iff removed.
func removeFromHand(seat *Seat, card *Card) bool {
	for i, c := range seat.Hand {
		if c == card {
			seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
			return true
		}
	}
	return false
}

// ManaCostOf is the exported version of manaCostOf for consumers
// outside gameengine (Phase 10 Hat implementations in internal/hat).
func ManaCostOf(card *Card) int { return manaCostOf(card) }

// CounterSpellEffectOf exposes counterSpellEffect for Hat implementations
// that need to detect counter-capable cards in hand.
func CounterSpellEffectOf(card *Card) gameast.Effect { return counterSpellEffect(card) }

// CollectSpellEffectOf exposes collectSpellEffect to Hat implementations
// that need to enumerate the spell-side body effects for card
// classification (ramp / draw / tutor / recursion detection).
func CollectSpellEffectOf(card *Card) gameast.Effect { return collectSpellEffect(card) }

// CardHasCounterSpell returns true if the card's AST contains a
// CounterSpell effect anywhere in the spell-side body. Mirrors Python
// `_card_has_counterspell`.
func CardHasCounterSpell(card *Card) bool { return counterSpellEffect(card) != nil }

// ManaCostContainsX returns true if the card's mana cost includes at least
// one X symbol (CR §107.3). Checks both the AST ManaCost and the Card.Types
// "cost:X" test convention.
func ManaCostContainsX(card *Card) bool {
	if card == nil {
		return false
	}
	// Check Types for test convention "cost:X" or "x_cost".
	for _, t := range card.Types {
		if t == "x_cost" || t == "cost:X" {
			return true
		}
	}
	// Check AST ManaCost symbols for IsX.
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Cost.Mana != nil {
				for _, sym := range a.Cost.Mana.Symbols {
					if sym.IsX {
						return true
					}
				}
			}
		}
	}
	return false
}

// manaCostOf extracts the generic mana cost for MVP. CardAST doesn't
// currently carry a ManaCost field (the parser stores cost on Activated
// abilities via Cost.Mana). For spell-cost, tests encode the cost as a
// "cost:N" token in Card.Types — this avoids rewriting state.go, which
// is out of scope for Phase 5. A proper typed mana pool + per-card CMC
// is Phase 8 territory.
func manaCostOf(card *Card) int {
	if card == nil {
		return 0
	}
	for _, t := range card.Types {
		if strings.HasPrefix(t, "cost:") {
			n := 0
			for _, ch := range t[5:] {
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	// Fallback: if the card has a top-level Activated with a non-zero mana
	// cost, use that. This lets AST-constructed test cards work without
	// the cost:N hack.
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			if a, ok := ab.(*gameast.Activated); ok && a.Cost.Mana != nil {
				cmc := a.Cost.Mana.CMC()
				if cmc > 0 {
					return cmc
				}
			}
		}
	}
	// Second-tier fallback: Card.CMC was populated by the deckparser's
	// buildCard from the Scryfall metadata. This is the canonical source
	// of truth for real corpus cards; the cost:N hack above is test-only.
	if card.CMC > 0 {
		return card.CMC
	}
	return 0
}

// collectSpellEffect returns the first "spell effect" on a card's AST.
// For instants/sorceries the parser emits these as Activated abilities
// whose cost is empty; the effect is the body. Falls back to the first
// Triggered's effect, and ultimately nil (permanent spells have no
// intrinsic on-resolution effect — ETB abilities fire from resolve.go).
func collectSpellEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	// CR §112.1 — permanent spells (creature/artifact/enchantment/
	// planeswalker/battle) have no spell-resolution effect. The card simply
	// becomes a permanent (CR §608.3); printed activated/triggered abilities
	// only function on the battlefield (CR §112.6, §603.5). Returning a
	// non-nil Effect here causes the first Activated ability to fire at cast
	// time, which is wrong — Cerulean Sphinx's "{U}: Its owner shuffles it
	// into their library" was running before ETB, putting the card into
	// owner's library AND the controller's battlefield simultaneously.
	// See docs/loki-r41-report.md / Loki r41 follow-up.
	if isPermanentSpell(card) {
		return nil
	}
	// Instants/sorceries: the spell body sometimes comes through as an
	// Activated AST node with an empty cost (parser artifact for cards like
	// Summon the School, Divergent Growth, Eldrazi Confluence). Only return
	// Activated effects that have no real activation cost — a non-empty
	// Mana/Tap/Untap/Sacrifice/etc. means this is a genuine activated
	// ability that should only function on the battlefield, not the
	// spell body of an instant/sorcery.
	for _, ab := range card.AST.Abilities {
		if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
			if isEmptyActivationCost(a.Cost) {
				return a.Effect
			}
		}
	}
	return nil
}

// isEmptyActivationCost reports whether c carries no real activation cost.
// Spell-body AST nodes for instants/sorceries occasionally come through as
// Activated with an all-nil Cost (the parser couldn't unify them with a
// Static spell_effect shape). Genuine activated abilities always carry at
// least one of: mana, tap, untap, sacrifice, discard, pay-life, exile-self,
// return-self-to-hand, or counter removal.
func isEmptyActivationCost(c gameast.Cost) bool {
	return c.Mana == nil && !c.Tap && !c.Untap && c.Sacrifice == nil &&
		c.Discard == nil && c.PayLife == nil && !c.ExileSelf &&
		!c.ReturnSelfToHand && c.RemoveCountersN == nil
}

// ---------------------------------------------------------------------------
// Priority round — CR §117.3–§117.5.
// ---------------------------------------------------------------------------

// apnapOrder returns the list of seats in Active-Player-Next-Active-Player
// order starting from gs.Active. CR §101.4a: "starting with the active
// player and proceeding in turn order."
func apnapOrder(gs *GameState) []int {
	n := len(gs.Seats)
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, (gs.Active+i)%n)
	}
	return out
}

// PriorityRound polls seats in APNAP order for responses. When any seat
// responds (casts an instant or activates an ability), a new StackItem is
// pushed and the round restarts at the new top. When all seats pass in
// succession, the round ends — the caller (CastSpell or ResolveStackTop)
// is responsible for resolving the top of the stack per CR §117.4.
//
// Depth-capped at 8; realistic counter-wars rarely exceed 4-5.
func PriorityRound(gs *GameState) {
	if gs == nil {
		return
	}
	const maxDepth = 8
	for depth := 0; depth < maxDepth; depth++ {
		if len(gs.Stack) == 0 {
			return
		}
		top := gs.Stack[len(gs.Stack)-1]
		// CR §702.61a: with a split-second spell in play, nobody can cast
		// non-mana spells — so nobody can respond. Short-circuit the round.
		if SplitSecondActive(gs) {
			gs.LogEvent(Event{
				Kind: "priority_skipped",
				Details: map[string]interface{}{
					"reason": "split_second",
					"rule":   "702.61a",
				},
			})
			return
		}
		responded := false
		for _, seat := range apnapOrder(gs) {
			if seat == top.Controller {
				// MVP: caster passes after casting (CR §117.3c says they keep
				// priority but the greedy policy always passes).
				continue
			}
			s := gs.Seats[seat]
			if s == nil || s.Lost {
				continue
			}
			// CR §307.1 / §601.3a: a Teferi-style static on an opponent's
			// battlefield restricts this seat to sorcery speed. Stack is
			// non-empty by definition here, so they can't respond.
			if OppRestrictsDefenderToSorcerySpeed(gs, seat) {
				gs.LogEvent(Event{
					Kind: "priority_skipped",
					Seat: seat,
					Details: map[string]interface{}{
						"reason": "sorcery_speed",
						"rule":   "307.1",
					},
				})
				continue
			}
			resp := GetResponse(gs, seat, top)
			if resp == nil {
				gs.LogEvent(Event{
					Kind: "priority_pass", Seat: seat,
					Details: map[string]interface{}{"rule": "117.3d"},
				})
				continue
			}
			// ---------------------------------------------------------
			// r62 response-cast legality (validator follow-up #4). This
			// path bypasses CastSpell entirely, so until r62 it applied
			// NO cost modifiers (bare manaCostOf — Thalia/Sphere/
			// medallions ignored), no timing gate on what the policy
			// returned, no announced-target validation, and none of the
			// legality-validator hooks. The gates below are the bounded
			// convergence; the full route-through-CastSpell plan is in
			// /tmp/fable-review/plan-priorityround-convergence.md.
			// ---------------------------------------------------------
			// CR §117.1a / §307.1 — a response is cast mid-stack at
			// instant speed: sorceries and non-flash permanent spells
			// are illegal responses no matter what the policy returned.
			// (Bare test fixtures with neither type pass unchanged.)
			if resp.Card != nil &&
				(cardHasType(resp.Card, "sorcery") ||
					(isPermanentSpell(resp.Card) && !legalityCardIsInstantSpeed(resp.Card))) {
				gs.LogEvent(Event{
					Kind: "response_rejected", Seat: seat,
					Source: resp.Card.DisplayName(),
					Details: map[string]interface{}{
						"reason": "sorcery_speed_response",
						"rule":   "117.1a",
					},
				})
				continue
			}
			// CR §601.2c / §608.2b — counter-shaped responses must
			// legally target the incoming item: the counter's printed
			// filter has to accept it ("counter target creature spell"
			// cannot be announced at a noncreature spell — the hat path
			// only checked counter-capability, not the filter), and the
			// announced target is stamped onto the item so the legality
			// validator and the event stream can see it. Resolution
			// re-derives its target (findGenericCounterTarget), so the
			// stamp is announcement metadata, not a behavior change.
			respEffect := resp.Effect
			if respEffect == nil {
				respEffect = counterSpellEffect(resp.Card)
			}
			if ExtractCounterSpellNode(respEffect) != nil {
				if top.Countered || !CounterCanTarget(respEffect, top) {
					gs.LogEvent(Event{
						Kind: "response_rejected", Seat: seat,
						Source: resp.Card.DisplayName(),
						Details: map[string]interface{}{
							"reason": "counter_filter_mismatch",
							"rule":   "601.2c",
						},
					})
					continue
				}
				if len(resp.Targets) == 0 {
					resp.Targets = []Target{{Kind: TargetKindStackItem, Stack: top}}
				}
			}
			// CR §608.2b defense-in-depth: any announced targets on the
			// response must be legal at announcement, same as CastSpell.
			if len(resp.Targets) > 0 {
				if err := ValidateTargetsAtAnnouncement(gs, seat, resp.Card, resp.Targets, resp); err != nil {
					gs.LogEvent(Event{
						Kind: "response_rejected", Seat: seat,
						Source: resp.Card.DisplayName(),
						Details: map[string]interface{}{
							"reason": "target_illegal",
							"rule":   "608.2b",
						},
					})
					continue
				}
			}
			// Pay its cost; if broke, skip. CR §601.2f — cost statics
			// apply to response casts exactly like main-phase casts.
			cost := CalculateTotalCost(gs, resp.Card, seat)
			if s.ManaPool < cost {
				continue
			}
			// Ride-along legality validator bracket (nil-receiver no-op
			// when off). Begin BEFORE payment so PoolBefore and the
			// announced base cost snapshot the pre-payment state.
			legalityObs := gs.Legality.BeginCast(gs, seat, resp.Card)
			// CR §601.2a — the response spell leaves its origin zone (hand)
			// when it's announced. Hat policies (GreedyHat / Yggdrasil /
			// Poker) return read-only advice — their ChooseResponse leaves
			// the card in hand for engine inspection. The legacy fallback
			// branch of GetResponse pre-removed it; here we remove it
			// centrally so EVERY response path (Hat or fallback) sees the
			// card leave hand before stack push. Without this the card
			// pointer ends up in BOTH hand and stack — and on resolution,
			// hand + battlefield — which is the Adric / Doctor's Companion
			// pattern surfaced in Loki r43 (562 of 564 CardIdentity hits,
			// game 170 seed 1700042, see docs/loki-r43-postfix.md).
			//
			// removeFromHand is a no-op when the card isn't there (the
			// legacy fallback already pulled it), so this is idempotent.
			if resp.Card != nil {
				removeFromHand(s, resp.Card)
			}
			s.ManaPool -= cost
			SyncManaAfterSpend(s)
			if cost > 0 {
				gs.LogEvent(Event{
					Kind:   "pay_mana",
					Seat:   seat,
					Amount: cost,
					Source: resp.Card.DisplayName(),
					Details: map[string]interface{}{
						"reason": "response",
						"rule":   "601.2f",
					},
				})
			}
			gs.LogEvent(Event{
				Kind:   "cast",
				Seat:   seat,
				Source: resp.Card.DisplayName(),
				Amount: cost,
				Details: map[string]interface{}{
					"in_response_to": top.Card.DisplayName(),
					"rule":           "117.7",
				},
			})
			// CR §700.4 / §702.40 — response casts (counterspells) are
			// casts per §601 and MUST increment the cast counters + fire
			// reactive observers. Without this, Rhystic Study / Mystic
			// Remora / Esper Sentinel miss every counterspell cast —
			// the "bless you" gap Wave 1b closes.
			IncrementCastCount(gs, seat)
			RecordCast(gs, seat, resp.Card, 0)
			fireCastTriggers(gs, seat, resp.Card)
			FireCastTriggerObservers(gs, resp.Card, seat, false)
			PushStackItem(gs, resp)
			gs.Legality.FinishCast(gs, legalityObs, resp)
			responded = true
			break // Restart priority at new top.
		}
		if !responded {
			// Stack trace: all players passed priority (CR §117.4).
			GlobalStackTrace.Log("priority_pass", "", gs.Active, len(gs.Stack), "all_players_pass")
			return
		}
	}
}

// GetResponse is the defender-side policy hook. Returns a *StackItem to push
// on the stack (typically a counter-spell or instant-speed removal) or nil
// to pass. The greedy MVP implementation:
//
//   - If stack is currently topped by a spell controlled by an opponent of
//     `defenderSeat`, scan defender's hand for a card whose AST carries
//     a CounterSpell effect. If affordable, return a StackItem wrapping it.
//   - Respect CR §702.61a (split-second) and CR §307.1 (sorcery speed) —
//     already screened by PriorityRound, but we defend-in-depth here.
//
// Note: this intentionally does NOT mutate the seat's hand. The caller
// (PriorityRound) is responsible for the hand-removal side-effect after
// confirming the cost was paid — this keeps the policy free of side effects
// and simplifies policy-swap experiments.
func GetResponse(gs *GameState, defenderSeat int, incoming *StackItem) *StackItem {
	if gs == nil || incoming == nil {
		return nil
	}
	if SplitSecondActive(gs) {
		return nil
	}
	if OppRestrictsDefenderToSorcerySpeed(gs, defenderSeat) {
		return nil
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[defenderSeat]
	// Only counter opponents' spells.
	if incoming.Controller == defenderSeat {
		return nil
	}
	if incoming.Countered {
		return nil
	}

	// Delegate to Hat if available (§117.3).
	if s.Hat != nil {
		return s.Hat.ChooseResponse(gs, defenderSeat, incoming)
	}

	// Fallback: hardcoded counter-scan policy.
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		ceff := counterSpellEffect(c)
		if ceff == nil {
			continue
		}
		cost := manaCostOf(c)
		if cost > s.ManaPool {
			continue
		}
		// Check that the counterspell's filter matches the incoming spell.
		if !CounterCanTarget(ceff, incoming) {
			continue
		}
		// Return advice — PriorityRound centralizes the hand-removal
		// side-effect so both Hat and fallback response paths converge on
		// the same place. Without that single source of truth, the Hat
		// path left the card in hand AND on the stack, tripping
		// CardIdentity on resolution (the Adric / Doctor's Companion
		// pattern from Loki r43 game 170).
		_ = i
		return &StackItem{
			Controller: defenderSeat,
			Card:       c,
			Effect:     ceff,
		}
	}
	return nil
}

// counterSpellEffect returns the CounterSpell effect from a card's AST if
// one exists, else nil. Used by GetResponse to identify counter-capable
// cards in hand.
//
// The AST stores instant/sorcery spell bodies in two layouts:
//  1. Activated ability with empty cost (legacy / test cards)
//  2. Static ability with Modification.ModKind == "spell_effect" and
//     Modification.Args[0] being the CounterSpell effect (real corpus)
//
// This function scans both.
func counterSpellEffect(c *Card) gameast.Effect {
	if c == nil || c.AST == nil {
		return nil
	}
	// CR §112.6 / §603.5: a permanent spell (creature, artifact,
	// enchantment, planeswalker, battle) becomes a permanent on
	// resolution — it does NOT cast as a counterspell, even if one of
	// its printed activated abilities counters spells (Adric,
	// Mathematical Genius's Ultimate Sacrifice; Mindcrank-style
	// triggered counter abilities; Disruptive Pitmage's morph-era
	// `{T}: Counter target spell`). Treating the card itself as a
	// counter-response selector pulls the permanent's *Card out of
	// hand and pushes it onto the stack with an instant-speed effect,
	// which then resolves to the battlefield while the original *Card
	// reference remains in hand (Adric leak in Loki r43 game 170;
	// Pitmage leak in r44 game 404). Only Layout-1 (Activated-with-
	// empty-cost) and Layout-2 (Static spell_effect) shapes belong to
	// instants/sorceries — neither is a permanent spell, so the
	// early-return is a strict refinement.
	if isPermanentSpell(c) {
		return nil
	}
	for _, ab := range c.AST.Abilities {
		// Layout 1: Activated ability with EMPTY cost (test cards / legacy
		// spell-body shape — Summon the School-style parser artifacts). A
		// non-empty cost means this is a genuine activated ability that
		// only functions on the battlefield, even on an instant/sorcery —
		// it must not register as a hand-castable counterspell. Mirrors
		// collectSpellEffect's r41 follow-up gating.
		if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
			if !isEmptyActivationCost(a.Cost) {
				continue
			}
			if isCounterSpellEffect(a.Effect) {
				return a.Effect
			}
		}
		// Layout 2: Static with Modification.kind == "spell_effect"
		// whose first arg is a CounterSpell effect (real AST corpus).
		if s, ok := ab.(*gameast.Static); ok && s.Modification != nil &&
			s.Modification.ModKind == "spell_effect" && len(s.Modification.Args) > 0 {
			if eff, ok := s.Modification.Args[0].(gameast.Effect); ok {
				if isCounterSpellEffect(eff) {
					return eff
				}
			}
		}
	}
	return nil
}

// ExtractCounterSpellNode walks an effect tree and returns the first
// *gameast.CounterSpell node found, or nil. Used by the generic resolver
// to extract the structured counter data from a potentially wrapped effect
// (e.g. inside a Sequence with side-effects).
func ExtractCounterSpellNode(e gameast.Effect) *gameast.CounterSpell {
	if e == nil {
		return nil
	}
	if cs, ok := e.(*gameast.CounterSpell); ok {
		return cs
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, sub := range seq.Items {
			if cs := ExtractCounterSpellNode(sub); cs != nil {
				return cs
			}
		}
	}
	return nil
}

// isCounterSpellEffect returns true if e (or any effect nested within a
// Sequence) is a CounterSpell. Shallow walk is enough for Phase 5.
func isCounterSpellEffect(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	if _, ok := e.(*gameast.CounterSpell); ok {
		return true
	}
	if seq, ok := e.(*gameast.Sequence); ok {
		for _, sub := range seq.Items {
			if isCounterSpellEffect(sub) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ResolveStackTop — CR §608.2.
// ---------------------------------------------------------------------------

// ResolveStackTop pops the top of gs.Stack and resolves it, handling three
// cases per CR §608.2:
//
//   - Countered item (StackItem.Countered == true): the spell is put into
//     its owner's graveyard; its effect does not happen.
//   - Non-countered spell: ResolveEffect is called; for non-permanent
//     spells the card then goes to the graveyard. For permanent spells
//     (creature/artifact/enchantment/planeswalker) the card enters the
//     battlefield as a Permanent.
//   - Triggered ability: ResolveEffect is called; there's no card-to-zone
//     move because the ability's source stays where it is.
//
// After resolution, StateBasedActions fires (CR §117.5 / §704.3). If the
// stack is still non-empty, the caller is expected to open another priority
// round — CastSpell loops until empty.

// postStackZoneForOffboard picks the destination zone for a spell card
// leaving the stack via a non-resolution path (counter, fizzle on all
// illegal targets). The successful-resolve path in ResolveStackTop calls
// ShouldExileOnResolve / ShouldReturnToHandOnResolve directly; this
// helper centralizes the same precedence for the failure paths so a
// countered or fizzled flashback spell still honors CR §702.33a's
// "exile this card instead of putting it anywhere else any time it
// would leave the stack" carveout (same for escape per §702.143b).
//
// Buyback is intentionally NOT honored here: CR §702.27b applies only
// "as it resolves" — a countered or fizzled buyback spell goes to the
// graveyard as normal.
//
// Returns (zone, rule_tag) where rule_tag is the CR clause that pinned
// the destination, included in the resolve event's "rule" detail.
func postStackZoneForOffboard(item *StackItem, defaultZone, defaultRule string) (string, string) {
	if ShouldExileOnResolve(item) {
		// flashback / escape — exile instead of any other zone.
		return "exile", "702.33a"
	}
	return defaultZone, defaultRule
}

// isRequiredSingleTargetFilter reports whether f is a genuinely-REQUIRED
// "target X" filter — i.e. the §608.2b fizzle check applies when no legal
// target exists for it. CONSERVATIVE by design: returns false for any
// shape where over-fizzling would be a false positive.
//
//   - Must be Targeted == true ("target X", not "a/an X" / "each X").
//   - Quantifier must be single-or-fixed-multi target: "" / "one" / "n".
//     Explicitly EXCLUDES "up_to_n" (CR §115.6 may-target-none), "any"
//     (any number of targets — also may-target-none), "each" / "each_player"
//     (untargeted fan-out). Those legally resolve with zero targets.
func isRequiredSingleTargetFilter(f gameast.Filter) bool {
	if !f.Targeted {
		return false
	}
	// Zone-selector bases are NOT targets even when the parser stamps
	// Targeted=true: "exile the top card of your library" (Abbot of
	// Keral Keep impulse family, 84 corpus shapes) targets nothing per
	// CR — the resolver special-cases these bases internally, and the
	// §608.2b gate fizzling on them silenced every impulse-exile
	// trigger before its library_top arm could run (r63 PROGRESSION
	// phase-3b finding: ~20 divergences across etb/attack/die/upkeep/
	// combat-damage families, all this one root cause).
	switch normalizeBase(f.Base) {
	case "library_top", "library_bottom", "top_of_library", "bottom_of_library":
		return false
	}
	switch f.Quantifier {
	case "", "one", "n":
		return true
	}
	return false
}

// requiredTargetFilter extracts the single top-level REQUIRED target filter
// from a stack item's effect, for the resolution-time §608.2b fizzle gate
// used on the lazy-pick path (item.Targets empty). Returns (filter, true)
// ONLY when the effect is a recognized single-leaf targeted effect whose
// top-level Target/Query/A filter is a required single-or-fixed-multi target.
//
// CONSERVATIVE: returns (_, false) for every wrapper node (Sequence, Choice,
// Optional, Conditional) and every effect kind not in the curated leaf set,
// so the fizzle gate never over-fires on modal spells, optional effects,
// multi-effect spells, or untargeted/fan-out effects. A false-negative here
// is safe (the effect resolves as it did before); a false-positive would
// over-fizzle a real card, so we err toward not fizzling.
func requiredTargetFilter(item *StackItem) (gameast.Filter, bool) {
	if item == nil || item.Effect == nil {
		return gameast.Filter{}, false
	}
	// NOTE: the engine's AST carries effects as POINTER types (ResolveEffect
	// dispatches on *gameast.X), so the type switch matches pointers. A value
	// effect (built by a non-AST caller) simply falls through to the
	// no-required-target default — which is the safe, no-fizzle outcome.
	switch e := item.Effect.(type) {
	case *gameast.Damage:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.LoseLife:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.GainLife:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.SetLife:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.Destroy:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.Exile:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.Bounce:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.TapEffect:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.UntapEffect:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.CounterMod:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.GainControl:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	// NOTE: CounterSpell is intentionally EXCLUDED. Its target is a spell on
	// the stack (TargetKindStackItem), which PickTarget does not resolve —
	// PickTarget would return empty for every counterspell and over-fizzle
	// it. Counterspell legality is handled by the existing counter-resolution
	// path, not this gate.
	case *gameast.Discard:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	case *gameast.Mill:
		if isRequiredSingleTargetFilter(e.Target) {
			return e.Target, true
		}
	}
	// Every other effect kind (wrappers, untargeted, query-based search,
	// buffs/grants that self-target on empty, etc.) is intentionally NOT
	// fizzled here — see the conservatism note above.
	return gameast.Filter{}, false
}

// cardPresentOffStack reports whether `card` already occupies a real
// zone (any seat's hand / library / graveyard / exile / command zone /
// foretell-exile / battlefield). The §608.2g graveyard routing uses it
// to avoid double-zoning a spell whose own resolution already relocated
// its card off the stack (Green Sun's Zenith self-shuffle, self-exiling
// or self-bouncing sorceries) — see the guard at the §608.2g branch in
// ResolveStackTop.
func cardPresentOffStack(gs *GameState, card *Card) bool {
	if gs == nil || card == nil {
		return false
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, z := range [][]*Card{s.Hand, s.Library, s.Graveyard, s.Exile, s.CommandZone, s.ForetellExile} {
			for _, c := range z {
				if c == card {
					return true
				}
			}
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == card {
				return true
			}
		}
	}
	return false
}

func ResolveStackTop(gs *GameState) {
	if gs == nil || len(gs.Stack) == 0 {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		return
	}
	// CR §608.2c: track resolution-frame nesting so triggered abilities that
	// fire during this resolution wait in gs.pendingTriggers (see
	// trigger_batch.go) and only go on the stack after the outermost
	// resolution finishes.
	gs.Flags["_resolve_frame_depth"]++
	defer func() {
		gs.Flags["_resolve_frame_depth"]--
		// Outermost frame closed: any triggers that accumulated during this
		// resolution now go on the stack and resolve.
		if gs.Flags["_resolve_frame_depth"] == 0 && len(gs.pendingTriggers) > 0 && gs.Flags["ended"] != 1 {
			drainPendingTriggers(gs)
		}
	}()
	item := gs.Stack[len(gs.Stack)-1]
	gs.Stack = gs.Stack[:len(gs.Stack)-1]

	// Census limbo-window guard (r63, seed-7777 game-76 Biorhythm): from
	// this pop until the final zone routing below, item.Card is absent
	// from every census-walked zone. If THIS resolution eliminates a seat
	// or ends the game (CheckEnd → HandleSeatElimination →
	// SweepOrphanedInstanceIDs runs mid-resolution), the orphan sweep
	// would cease the in-flight card's ID and the eventual graveyard
	// routing becomes a permanent fabrication violation. Track the card
	// as present-while-resolving; LIFO so nested resolutions (DrainStack
	// inside a handler) stay balanced.
	if item.Card != nil {
		gs.ResolvingCards = append(gs.ResolvingCards, item.Card)
		defer func() {
			gs.ResolvingCards = gs.ResolvingCards[:len(gs.ResolvingCards)-1]
		}()
	}

	// InstanceID enabler context: child objects (token mints, copy mints)
	// created during this resolution stamp the resolving frame's
	// AbilityInstance ID as their EnablerInstanceID per
	// docs/instanceid-system-v2-r60.md §4 lineage. For spell items
	// (no Ability) push an empty string so the pop stays balanced.
	enablerID := ""
	if item.Ability != nil {
		enablerID = item.Ability.InstanceID
	}
	pushIIDEnabler(gs, enablerID)
	defer popIIDEnabler(gs)

	// Log the resolution regardless of outcome so counterspell test fixtures
	// can observe ordering.
	name := ""
	isSpell := item.Card != nil && item.Source == nil
	if item.Card != nil {
		name = item.Card.DisplayName()
	} else if item.Source != nil {
		name = item.Source.Card.DisplayName()
	}
	// Stack trace: log resolution for CR audit. CR §608.2 covers both
	// spells and triggered abilities, but the audit tool documents them
	// as distinct event kinds ("resolve" vs "trigger_resolve") because
	// downstream consumers need to count them separately (e.g.,
	// "how many triggered abilities resolved this turn"). A triggered
	// ability is identified by item.Kind=="triggered" or by carrying a
	// Source permanent without a Card (legacy zero-Kind path).
	resolveKind := "resolve"
	if item.Kind == "triggered" || (item.Source != nil && !isSpell) {
		resolveKind = "trigger_resolve"
		// CR §603.4 second check: the intervening "if" is re-evaluated
		// on resolution; if it is no longer true the ability does
		// nothing (r63 PROGRESSION dimension).
		if item.CostMeta != nil {
			if ii, ok := item.CostMeta["intervening_if"].(*gameast.Condition); ok && ii != nil {
				if !evalCondition(gs, item.Source, ii) {
					gs.LogEvent(Event{
						Kind: "trigger_fizzled", Seat: item.Controller, Target: -1,
						Source: name,
						Details: map[string]interface{}{
							"reason": "intervening_if_false_at_resolution",
							"rule":   "603.4",
						},
					})
					return
				}
			}
		}
	}
	GlobalStackTrace.Log(resolveKind, name, item.Controller, len(gs.Stack), "resolving")
	gs.LogEvent(Event{
		Kind:   "stack_resolve",
		Seat:   item.Controller,
		Source: name,
		Details: map[string]interface{}{
			"countered":  item.Countered,
			"stack_size": len(gs.Stack),
			"rule":       "608.2",
		},
	})

	if item.Countered {
		// CR §701.5a: a countered spell is put into its owner's graveyard.
		// Abilities aren't cards so the "graveyard move" only applies to
		// spells (items with Card set and Source unset).
		if isSpell && item.Card != nil {
			// CR §702.33a (flashback) / §702.143b (escape): "exile this
			// card instead of putting it anywhere else any time it would
			// leave the stack." A countered flashback/escape spell still
			// triggers the exile-instead self-replacement — only the
			// successful-resolve path was honoring it before.
			toZone, ruleTag := postStackZoneForOffboard(item, "graveyard", "701.5a")
			MoveCard(gs, item.Card, item.Controller, "stack", toZone, "countered")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":        toZone,
					"countered": true,
					"rule":      ruleTag,
				},
			})
		}
		return
	}

	// CR §608.2b: target legality check at resolution. If ALL targets are
	// illegal, the spell/ability is countered on resolution ("fizzles").
	// If SOME targets are illegal but at least one is legal, resolve with
	// the legal targets only.
	if len(item.Targets) > 0 {
		allIllegal, legalTargets := CheckTargetLegality(gs, item)
		if allIllegal {
			gs.LogEvent(Event{
				Kind:   "fizzle",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"rule":   "608.2b",
					"reason": "all_targets_illegal",
				},
			})
			// Countered on resolution — spell leaves the stack. Same
			// CR §702.33a / §702.143b carveout as the explicit-counter
			// branch above: flashback/escape send to exile, not graveyard.
			if isSpell && item.Card != nil {
				toZone, _ := postStackZoneForOffboard(item, "graveyard", "608.2b")
				MoveCard(gs, item.Card, item.Controller, "stack", toZone, "fizzle")
			}
			return
		}
		// Update the stack item's targets to only the legal subset.
		item.Targets = legalTargets
	} else {
		// Lazy-pick path: targets were not announced onto item.Targets (the
		// engine picks them inside effect handlers via PickTarget at
		// resolution). The §608.2b legal-target check above is gated on
		// item.Targets being populated, so it never ran for this item. If the
		// effect has a genuinely-REQUIRED top-level target filter and no legal
		// target exists for it right now, the spell/ability is countered on
		// resolution ("fizzles") per CR §608.2b — it must NOT manufacture a
		// target inside the handler. CONSERVATIVE: requiredTargetFilter only
		// returns a filter for the curated single-leaf targeted effect set;
		// everything else falls through and resolves as before.
		if filter, ok := requiredTargetFilter(item); ok {
			var fizzleSrc *Permanent
			if item.Source != nil {
				fizzleSrc = item.Source
			} else if item.Card != nil {
				fizzleSrc = &Permanent{
					Card:       item.Card,
					Controller: item.Controller,
					Owner:      item.Card.Owner,
					Flags:      map[string]int{},
				}
			}
			if len(PickTarget(gs, fizzleSrc, filter)) == 0 {
				gs.LogEvent(Event{
					Kind:   "fizzle",
					Seat:   item.Controller,
					Source: name,
					Details: map[string]interface{}{
						"rule":   "608.2b",
						"reason": "no_legal_target",
					},
				})
				// Countered on resolution — spell leaves the stack. Same
				// flashback/escape exile carveout as the explicit-counter and
				// all-targets-illegal branches above.
				if isSpell && item.Card != nil {
					toZone, _ := postStackZoneForOffboard(item, "graveyard", "608.2b")
					MoveCard(gs, item.Card, item.Controller, "stack", toZone, "fizzle")
				}
				return
			}
		}
	}

	// r62 — expose the item's announcement-time targets (now trimmed to the
	// §608.2b-legal subset by the gate above) to PickTarget for the rest of
	// this resolution frame, so effect handlers honor the targets chosen —
	// hat-consulted, validated, and warded — at announcement instead of
	// re-running the engine policy pick. Save/restore rather than clear:
	// resolution frames nest (per-card handlers resolve sub-effects), and
	// an inner frame must never leak the outer frame's targets — or wipe
	// them — across the boundary. Always assign (nil when empty) so an
	// item WITHOUT announced targets resolves fully lazily even when an
	// outer frame had targets.
	prevAnnounced := gs.announcedTargets
	if len(item.Targets) > 0 {
		gs.announcedTargets = item.Targets
	} else {
		gs.announcedTargets = nil
	}
	defer func() { gs.announcedTargets = prevAnnounced }()

	// First-play instrumentation. Record only for true spell stack items
	// (item.Card set, no Source permanent) — triggered/activated abilities
	// have a Source and don't count as "played". Only the first resolution
	// per card name is kept; storm copies and recursion don't overwrite.
	if isSpell && item.Card != nil {
		if gs.CardFirstPlayed == nil {
			gs.CardFirstPlayed = map[string]int{}
		}
		if _, ok := gs.CardFirstPlayed[name]; !ok {
			gs.CardFirstPlayed[name] = gs.Turn
		}
	}

	// Wave 3a: activated-ability stack items resolve through their own
	// dispatch path. CR §602.2: "The controller of an activated ability
	// on the stack is the player who activated it."
	if item.Kind == "activated" {
		resolveActivatedAbility(gs, item)
		return
	}

	// Per-card trigger handler: if this stack item was pushed by
	// PushPerCardTrigger, resolve it by calling the wrapped Go function.
	// This is the CR §603.3 bridge: the trigger was placed on the stack,
	// priority passed, and now the handler executes on resolution.
	if item.CostMeta != nil {
		if trigData, ok := item.CostMeta["trigger_handler"]; ok {
			if th, ok := trigData.(*TriggerHandlerStackItem); ok {
				th.HandlerFunc(gs, th.SourcePerm, th.Ctx)
				// Run SBAs after trigger resolution per CR §704.3.
				StateBasedActions(gs)
				return
			}
		}
	}

	// Distinguish permanent spells from instant/sorcery spells. CR §608.3:
	// a permanent spell becomes a permanent on the battlefield when it
	// resolves. Non-permanent (instant/sorcery) spells resolve their effect
	// and go to graveyard per §608.2g.
	isPermanent := isSpell && item.Card != nil && isPermanentSpell(item.Card)

	// CR §702.96 — Overload: while this stack item resolves, every
	// "target X" in its text reads as "each X". The side-channel flag
	// is set here so PickTarget (called by both the snowflake hook and
	// stock Effect dispatch below) fans out single-target filters.
	endOverload := beginOverloadResolution(gs, item)
	defer endOverload()

	// Per-card resolve-time snowflake dispatch. Fired BEFORE stock Effect
	// dispatch; when a handler is registered (fired > 0), we SKIP the
	// stock dispatch — the handler is the authoritative spell body.
	// Used by Doomsday / Demonic Consultation / Tainted Pact, whose
	// oracle text doesn't fit the general AST.
	snowflakeFired := InvokeResolveHook(gs, item)

	// Resolve the effect if present. CR §608.2c.
	if item.Effect != nil && snowflakeFired == 0 {
		// Stash targets on gs.Flags so resolver helpers that support it
		// can read them. For now resolve.go uses its own PickTarget —
		// StackItem.Targets is reserved for Phase 6 retarget/fizzle logic.
		_ = item.Targets
		var src *Permanent
		if item.Source != nil {
			src = item.Source
		} else if item.Card != nil {
			// For spell resolution we synthesize a transient Permanent as
			// the source so existing resolve.go handlers (which all take
			// *Permanent) have a controller + name to reference. Owner
			// must mirror the Card's Owner — handlers that key off
			// src.Owner (e.g. shuffle_into_owner_library) otherwise read
			// the zero-value seat 0 and route effects to the wrong player
			// for any spell not cast by seat 0.
			src = &Permanent{
				Card:       item.Card,
				Controller: item.Controller,
				Owner:      item.Card.Owner,
				Flags:      map[string]int{},
			}
		}
		ResolveEffect(gs, src, item.Effect)
	}

	if isSpell && item.Card != nil {
		if isPermanent {
			// CR §608.3a: the permanent spell becomes a permanent under
			// its controller's control on the battlefield. Mirrors Python
			// _resolve_stack_top's is_permanent_spell branch. If this was
			// a COPY (§707.10f), the resolving permanent is a TOKEN copy.
			etbPerm := resolvePermanentSpellETB(gs, item)

			// §702.185 — warp: if the spell was cast for its warp cost,
			// register the delayed "exile at next end step" trigger and
			// the owner's cast-from-exile permission attaches when the
			// trigger fires. CR §702.185a.
			if etbPerm != nil && item.CostMeta != nil {
				if v, ok := item.CostMeta["warped"]; ok {
					if b, ok := v.(bool); ok && b {
						RegisterWarpExileTrigger(gs, etbPerm)
					}
				}
			}

			// Wave 2: evoke — if the spell was cast with evoke, register
			// a sacrifice trigger on ETB per CR §702.73.
			if etbPerm != nil && item.CostMeta != nil {
				if v, ok := item.CostMeta["evoke"]; ok {
					if b, ok := v.(bool); ok && b {
						// §702.73: "When this permanent enters the battlefield,
						// sacrifice it." Register as an immediate ETB trigger.
						gs.LogEvent(Event{
							Kind:   "evoke_sacrifice_trigger",
							Seat:   item.Controller,
							Source: name,
							Details: map[string]interface{}{
								"rule": "702.73",
							},
						})
						// Sacrifice the permanent immediately (after ETB
						// triggers have already fired via resolvePermanentSpellETB).
						// Route through SacrificePermanent for proper §614
						// replacement effects, dies/LTB triggers, and
						// commander redirect.
						SacrificePermanent(gs, etbPerm, "evoke")
					}
				}
			}
		} else if item.IsCopy {
			// CR §707.10 — a copy of a non-permanent spell ceases to
			// exist on resolution. Do NOT route to graveyard: the copy
			// is a transient game object, not a card in any deck, and
			// appending it to a zone would violate zone conservation.
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":   "ceases_to_exist",
					"rule": "706.10",
				},
			})
			// Phase 4 census drop: §707.10 spell-copy InstanceID exits
			// the (Minted - Ceased) expected set.
			if item.Card != nil {
				MarkInstanceIDCeased(gs, item.Card.InstanceID)
			}
		} else if ShouldExileOnResolve(item) {
			// Wave 2: flashback / escape — exile instead of graveyard.
			// CR §702.33: "If the flashback cost was paid, exile this
			// card instead of putting it anywhere else any time it would
			// leave the stack."
			// CR §400.7c: the card moves to the OWNER's exile, not the
			// caster's. Pass item.Card.Owner explicitly so the call is
			// structurally correct and doesn't rely on the moveToZone
			// owner-redirect backstop (state.go:1614-1645).
			MoveCard(gs, item.Card, item.Card.Owner, "stack", "exile", "flashback-exile")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":        "exile",
					"reason":    "zone_cast_exile_on_resolve",
					"cast_zone": item.CastZone,
					"rule":      "702.33",
				},
			})
		} else if ShouldReturnToHandOnResolve(item) {
			// CR §702.27b: if the buyback cost was paid, the spell
			// returns to its OWNER's hand instead of the graveyard.
			// Pass item.Card.Owner explicitly — the §702.27b wording
			// is explicit ("its owner's hand") and structurally
			// correct without relying on the moveToZone backstop.
			MoveCard(gs, item.Card, item.Card.Owner, "stack", "hand", "buyback")
			gs.LogEvent(Event{
				Kind:   "resolve",
				Seat:   item.Controller,
				Source: name,
				Details: map[string]interface{}{
					"to":     "hand",
					"reason": "buyback",
					"rule":   "702.27b",
				},
			})
		} else {
			// CR §608.2g: non-permanent spells go to the OWNER's
			// graveyard on resolution ("the graveyard of its owner",
			// explicit in the rule). Pass item.Card.Owner instead of
			// item.Controller so cross-control casts (Bribery /
			// Hostage Taker / Possibility Storm / Knowledge Pool /
			// Etali / Praetor's Grasp / Dauthi Voidwalker / Release
			// to the Wind / Mind's Desire / chaos cascade family)
			// route correctly without relying on the moveToZone
			// owner-redirect backstop (state.go:1614-1645). The
			// pre-fix call passed item.Controller and depended on
			// the backstop to silently re-route — structurally
			// wrong and a sibling of the Etali §400.7c cluster
			// closed in PR #685.
			//
			// r63 §608.2g guard (eliminated-seat-frontier hunt,
			// CardIdentity double-zone class): if the spell's OWN
			// resolution already moved its card to another zone — Green
			// Sun's Zenith "Shuffle ~ into its owner's library", a
			// self-exiling sorcery, a self-bounce — then the card has
			// left the stack and §608.2g does NOT also drop it into the
			// graveyard (the rule routes it to the graveyard only "as the
			// final part of its resolution" while it is still the spell on
			// the stack). Without this guard the card double-zones
			// (library + graveyard): seed-77 game 939 GSZ produced 254
			// CardIdentity hits from this exact path.
			if cardPresentOffStack(gs, item.Card) {
				gs.LogEvent(Event{
					Kind:   "resolve",
					Seat:   item.Controller,
					Source: name,
					Details: map[string]interface{}{
						"to":     "already_relocated",
						"rule":   "608.2g",
						"reason": "spell_effect_moved_card_off_stack",
					},
				})
			} else {
				MoveCard(gs, item.Card, item.Card.Owner, "stack", "graveyard", "resolve")
				gs.LogEvent(Event{
					Kind:   "resolve",
					Seat:   item.Controller,
					Source: name,
					Details: map[string]interface{}{
						"to":   "graveyard",
						"rule": "608.2g",
					},
				})
			}
		}
	}
}

// resolvePermanentSpellETB is the ETB path for a resolving permanent
// spell. Mirrors the Python `_resolve_stack_top` permanent branch +
// `_etb_initialize`:
//
//  1. Allocate a new Permanent with summoning_sick = !has_keyword("haste")
//     (creatures only; non-creatures ignore summoning sickness per §302.1).
//  2. Assign §613.7 timestamp via NextTimestamp().
//  3. Initialize planeswalker loyalty counters (§306.5b) / battle defense
//     counters (§310.3) if the metadata hints carry a starting value
//     (BasePower / BaseToughness is the nearest runtime approximation —
//     the engine doesn't carry explicit starting_loyalty today).
//  4. Append to controller's battlefield.
//  5. Register §613 continuous effects from Static abilities.
//  6. Register §614 replacement effects.
//  7. Fire ETB triggered abilities through the stack.
//  8. Emit an enter_battlefield event.
func resolvePermanentSpellETB(gs *GameState, item *StackItem) *Permanent {
	if gs == nil || item == nil || item.Card == nil {
		return nil
	}
	card := item.Card
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	// MDFC back-face entry: swap type identity so the permanent enters as
	// the back face (e.g., Bridge = enchantment, not Esika = creature).
	// Reverse MDFCs (front=land, back=instant/sorcery) skip the swap —
	// the spell-typed back face isn't a permanent and would trip §205.2.
	// In practice a reverse MDFC's back face resolves into the graveyard,
	// not the battlefield, so we shouldn't reach this code with
	// CastingBackFace=true on one; the guard is defensive depth.
	if card.CastingBackFace && card.BackFaceName != "" && !IsReverseMDFC(card) {
		card.Name = card.BackFaceName
		if len(card.BackFaceTypes) > 0 {
			card.Types = card.BackFaceTypes
		}
		if card.BackFaceTypeLine != "" {
			card.TypeLine = card.BackFaceTypeLine
		}
		card.CastingBackFace = false
	} else if card.CastingBackFace {
		// Clear the transient flag even on the reverse-MDFC bail so it
		// doesn't leak into downstream consumers.
		card.CastingBackFace = false
	}
	// Defense in depth: any MDFC reaching the resolve path with a
	// land back face still needs the swap, in case CastingBackFace
	// wasn't set on the cast machinery (e.g. a permanent spell that
	// cascaded into another MDFC, where the cascade put-onto-stack
	// path didn't flip the flag).
	EnsureBattlefieldFrontFace(card)
	// CR 304.4 / 307.1 — instants and sorceries should never reach the
	// permanent-spell ETB path, but guard anyway: if one slips through
	// (corpus mistype, malformed StackItem) send it to the graveyard
	// instead of wrapping it in a Permanent.
	if !CardCanEnterBattlefield(card) {
		gs.moveToZone(seatIdx, card, "graveyard")
		return nil
	}

	// r60 / Naru Meha + Panharmonicon copy-cascade fix
	// (post-#692 verification residual): CR §707.10f — "If a copy of a
	// permanent spell resolves, it becomes a token; it's still a copy
	// of the spell it was a copy of." Stamp "token" onto the resolving
	// Card.Types so (a) Permanent.IsToken() / cardIsTokenForInv return
	// true (zone-conservation invariant stops counting cascade copies
	// as real cards), (b) the existing token-cleanup SBA correctly
	// makes the copy cease to exist when it leaves the battlefield.
	//
	// Done HERE (resolve-time) rather than at copy_spell creation in
	// resolve.go because copies-of-permanent-spells aren't tokens
	// WHILE ON THE STACK — only after resolution per §707.10f. The
	// stack item still answers !IsToken() until this point, which
	// matters for stack-targeting filters and the magecraft trigger
	// fan-out in resolveCopySpell.
	if item.IsCopy && !cardHasType(card, "token") {
		card.Types = append(card.Types, "token")
	}

	// Summoning sickness: only creatures care (§302.1 / §212.3f). A creature
	// with haste ignores it.
	isCreature := cardHasType(card, "creature")
	sick := false
	if isCreature {
		sick = !cardHasKeyword(card, "haste")
	}
	perm := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		Tapped:        false,
		SummoningSick: sick,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	if perm.Owner < 0 {
		perm.Owner = seatIdx
	}
	// Cast-tracking for "if you cast it" / "if it was cast" intervening-if
	// conditions (CR §603.6c). A permanent reaching this stack-resolution
	// path was cast unless it's a copy (§707.10f token copies). CastZone
	// captures the origin zone for "if you cast it from your hand" gates
	// (Cyclone Summoner, Breaching Leviathan, Wild Pair).
	if !item.IsCopy {
		perm.Flags["was_cast"] = 1
		castZone := item.CastZone
		if castZone == "" {
			castZone = "hand"
		}
		if castZone == "hand" {
			perm.Flags["cast_from_hand"] = 1
		}
	}

	// CR §702.33 — mirror the cast-time kicker decision from CostMeta onto
	// the permanent's Flags BEFORE ApplyStaticETBCounters runs, so "if
	// kicked → enters with N counters" statics (Grunn) and the "for each
	// time kicked" variable counter reader (Everflowing Chalice) see the
	// kick count. Mirrors the ChosenX → gs.Flags pattern. No-op for copies
	// (no CostMeta) and unkicked cards (flags left unset).
	MirrorKickFlagsToPermanent(item, perm)
	// §306.5b planeswalker loyalty counter initialization. We don't carry
	// starting_loyalty on Card today — fall back to CMC-ish heuristic so
	// planeswalkers at least start with a positive counter.
	if cardHasType(card, "planeswalker") {
		n := card.BaseToughness
		if n <= 0 {
			n = 3
		}
		perm.Counters["loyalty"] = n
	}
	// §310.3 battle defense counter initialization.
	if cardHasType(card, "battle") {
		n := card.BaseToughness
		if n <= 0 {
			n = 1
		}
		perm.Counters["defense"] = n
	}

	// InstanceID gap-walk: backstop for paradigm copies / cascade copies
	// / Riku-style stack copies whose *Card still carries the original
	// spell's ID when it lands on the battlefield. See
	// instanceid_gap_walk.go.
	EnforceBattlefieldUniqueInstanceID(gs, perm.Card, seatIdx)
	seat.Battlefield = append(seat.Battlefield, perm)

	// §702.146b — Disturb cast: a card cast for its disturb cost enters
	// transformed (back face up) with the §702.146c dies→exile
	// replacement registered. The transform happens BEFORE aura
	// attachment + RegisterContinuousEffectsForPermanent so the back
	// face's printed type / abilities are what the layers see. If
	// the casting helper forwarded a back-face AST hint (via
	// CostMeta["disturb_back_face_ast"]; tests + the corpus loader
	// stash this), wire it onto the Permanent's face cache so
	// ApplyDisturbETB's transform branch actually has something to
	// flip to.
	if item.CostMeta != nil {
		if v, ok := item.CostMeta["disturb_cast"]; ok {
			if b, _ := v.(bool); b {
				if v, ok := item.CostMeta["disturb_back_face_ast"]; ok && v != nil {
					if backAST, ok := v.(*gameast.CardAST); ok && backAST != nil {
						perm.FrontFaceAST = perm.Card.AST
						perm.BackFaceAST = backAST
						perm.FrontFaceName = perm.Card.Name
						perm.BackFaceName = perm.Card.BackFaceName
					}
				}
				ApplyDisturbETB(gs, perm)
			}
		}
	}

	// §303.4f — Aura attachment on ETB. When an Aura enters the battlefield
	// as a permanent spell, it must be attached to a legal object. Infer the
	// target type from the card's oracle text / TypeLine and attach to a
	// valid own permanent. Without this, SBA §704.5m destroys unattached auras.
	if perm.IsAura() {
		attachAuraOnETB(gs, perm)
	}

	// §702.136 — Riot: as this enters, choose +1/+1 counter or haste.
	ApplyRiot(gs, perm)

	// CR §122.1g / §614.1d — generic "enters with N counters" Static
	// self-replacement. Covers AST shapes with no per-card OnETB handler
	// (e.g. District Mascot, whose printed P/T is 0/0 and depends on the
	// ETB +1/+1 counter to be playable). Must run BEFORE the ETB trigger
	// fan-out so observer triggers (Mentor of the Meek, Hardened Scales,
	// etc.) see the entering perm in its final counter state.
	ApplyStaticETBCounters(gs, perm)

	// Register §613 continuous effects (layers 1-7).
	RegisterContinuousEffectsForPermanent(gs, perm)
	// Register §614 replacement effects.
	RegisterReplacementsForPermanent(gs, perm)

	gs.LogEvent(Event{
		Kind:   "enter_battlefield",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"summoning_sick": sick,
			"rule":           "608.3a",
		},
	})

	// CR §603.3b: the ETB cascade fans out to (a) the entering card's own
	// AST ETB triggers, (b) the per-card snowflake handler, (c) per-card
	// "nonland_permanent_etb" / "permanent_etb" event listeners, and
	// (d) observer ETB triggers from every other permanent on the battlefield.
	// All fire from the same single game event (this ETB) and must be batched,
	// ordered APNAP + controller-choice, then drained. Closure scope so an
	// early end-of-game return still runs the deferred End.
	func() {
		opened := BeginTriggerBatch(gs)
		defer EndTriggerBatch(gs, opened)

		// Fire ETB triggered abilities (§603.6).
		if card.AST != nil {
			for _, ab := range card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !EventEquals(trig.Trigger.Event, "etb") {
					continue
				}
				// CR §614 — consult would_fire_etb_trigger so Panharmonicon
				// and friends can add additional firings. See parallel fix
				// in etb_dispatch.go::FirePermanentETBTriggers.
				n, cancelled := FireETBTriggerEvent(gs, perm)
				if cancelled {
					continue
				}
				for i := 0; i < n; i++ {
					PushTriggeredAbilityWithIf(gs, perm, trig.Effect, trig.InterveningIf)
					if gs.CheckEnd() {
						return
					}
				}
			}
		}

		// Per-card ETB snowflake dispatch. Fires AFTER stock AST ETB triggers
		// so the snowflake runs as the "bottom" of the cascade — order matters
		// for Thassa's Oracle which reads the library AFTER any ETB scrys/
		// tutors resolve. The hook itself isn't a trigger but its effects may
		// FireCardTrigger, which appends into this same batch.
		InvokeETBHook(gs, perm)

		// §702.131 Ascend.
		CheckAscend(gs, perm.Controller)

		if !cardHasType(card, "land") {
			FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
				"perm":            perm,
				"controller_seat": perm.Controller,
				"card":            card,
			})
		}
		FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
			"perm":            perm,
			"controller_seat": perm.Controller,
			"card":            card,
		})

		// Observer ETB triggers — scan all OTHER permanents for triggered
		// abilities that fire when a permanent enters. Mirrors
		// fireObserverZoneChangeTriggers but for ETB events.
		fireObserverETBTriggers(gs, perm)
	}()

	// Ride-along legality validator (phase 3): cast-path ETB cascade has
	// settled — self-replacement counters are final. Mirrors the hook at
	// the end of FirePermanentETBTriggers. nil-receiver no-op when off.
	gs.Legality.ObserveETB(gs, perm)

	return perm
}

// attachAuraOnETB finds a valid target for an aura permanent entering the
// battlefield and sets AttachedTo. Uses the card's TypeLine to infer the
// enchant target ("Enchantment — Aura" with oracle "Enchant land/creature/...").
// Falls back to: own land → own creature → any own permanent (excluding self).
func attachAuraOnETB(gs *GameState, perm *Permanent) {
	if perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	tl := strings.ToLower(perm.Card.TypeLine)

	wantLand := strings.Contains(tl, "enchant land")
	wantCreature := strings.Contains(tl, "enchant creature")
	wantArtifact := strings.Contains(tl, "enchant artifact")

	// If TypeLine doesn't specify, check card name heuristics or default
	// to creature (most common aura target).
	if !wantLand && !wantCreature && !wantArtifact {
		wantCreature = true
	}

	// Search own battlefield for a valid target.
	for _, p := range seat.Battlefield {
		if p == perm || p == nil {
			continue
		}
		if wantLand && p.IsLand() {
			perm.AttachedTo = p
			return
		}
		if wantCreature && p.IsCreature() {
			perm.AttachedTo = p
			return
		}
		if wantArtifact && p.IsArtifact() {
			perm.AttachedTo = p
			return
		}
	}

	// Fallback: attach to any own permanent (excluding self) to prevent
	// immediate SBA destruction. This is a lossy heuristic but better than
	// the aura self-destructing.
	for _, p := range seat.Battlefield {
		if p != perm && p != nil {
			perm.AttachedTo = p
			return
		}
	}
}

// CardHasKeyword returns true if the card's AST contains a Keyword ability
// with the given name (case-insensitive). We check AST only — runtime
// grants are per-permanent, not per-card, so they're not relevant to the
// ETB initial state. Exported (R60 Phase 2C consolidation) so the
// per_card subpackage can share this rather than carry its own copy.
func CardHasKeyword(c *Card, name string) bool {
	if c == nil || c.AST == nil {
		return false
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, ab := range c.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == want {
			return true
		}
	}
	return false
}

// cardHasKeyword retains the lowercase alias for in-package callers
// that were here before the export. Cheap forwarder; saves churn on
// the many engine call sites.
func cardHasKeyword(c *Card, name string) bool {
	return CardHasKeyword(c, name)
}

// CardHasTypeExact returns true iff `t` (case-insensitive) appears as
// an exact element of card.Types. Distinct from the engine-internal
// cardHasType in cost_modifiers.go which ALSO does a TypeLine
// substring match — this strict version mirrors what the per_card
// and hat packages need (primary-type and subtype membership without
// TypeLine substring false positives). Exported R60 Phase 2C so those
// two callers can share rather than copy-paste.
func CardHasTypeExact(c *Card, t string) bool {
	if c == nil {
		return false
	}
	want := strings.ToLower(t)
	for _, got := range c.Types {
		if strings.ToLower(got) == want {
			return true
		}
	}
	return false
}

// isPermanentSpell returns true if the card's type line designates a
// permanent (creature/artifact/enchantment/planeswalker/land/battle). For
// these, resolution puts the card ON the battlefield, not in the graveyard.
// MVP reads from Card.Types.
func isPermanentSpell(c *Card) bool {
	if c == nil {
		return false
	}
	// MDFC back face: use back-face types to determine if the spell is
	// a permanent (e.g., Jadzi back face = sorcery, not permanent).
	types := c.Types
	if c.CastingBackFace && len(c.BackFaceTypes) > 0 {
		types = c.BackFaceTypes
	}
	for _, t := range types {
		switch strings.ToLower(t) {
		case "creature", "artifact", "enchantment", "planeswalker",
			"land", "battle":
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Timing restrictions.
// ---------------------------------------------------------------------------

// SplitSecondActive reports whether any item on the stack carries the
// split-second keyword. CR §702.61a: "Split second is a static ability that
// functions only while the spell with split second is on the stack. 'Split
// second' means 'As long as this spell is on the stack, players can't cast
// other spells or activate abilities that aren't mana abilities.'"
//
// We scan every non-countered stack item for a Keyword AST node whose name
// contains "split" (canonical parser name: "split_second"; tolerate "split"
// alone for extension parity). Flags-based tokens also trip the check
// ("kw:split_second").
func SplitSecondActive(gs *GameState) bool {
	if gs == nil {
		return false
	}
	for _, item := range gs.Stack {
		if item == nil || item.Countered {
			continue
		}
		if item.Source != nil && permHasSplitSecond(item.Source) {
			return true
		}
		if item.Card != nil && cardHasSplitSecond(item.Card) {
			return true
		}
	}
	return false
}

// permHasSplitSecond delegates to HasKeyword (which already checks AST +
// GrantedAbilities + Flags).
func permHasSplitSecond(p *Permanent) bool {
	if p == nil {
		return false
	}
	if p.HasKeyword("split_second") || p.HasKeyword("split second") || p.HasKeyword("split") {
		return true
	}
	return false
}

// cardHasSplitSecond scans a Card's AST for a Keyword ability whose name
// contains "split". Called for stack items that represent spells (no
// associated Permanent yet).
func cardHasSplitSecond(c *Card) bool {
	if c == nil || c.AST == nil {
		return false
	}
	for _, ab := range c.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		if strings.Contains(name, "split") {
			return true
		}
	}
	return false
}

// OppRestrictsDefenderToSorcerySpeed reports whether any opponent of
// `defenderSeat` controls a static ability that restricts `defenderSeat`
// to sorcery speed (CR §307.1 / §601.3a).
//
// This models Teferi, Time Raveler — "Each opponent can cast spells only
// any time they could cast a sorcery." Since sorceries can only be cast
// on an empty stack during that player's main phase, and the defender is
// being asked to respond to a spell (stack non-empty by definition, not
// their main phase), the defender can't legally cast a response.
//
// Scans every seat's battlefield (except defenderSeat's own — a player
// isn't restricted by their own Teferi) for Static abilities whose
// Modification.ModKind is "opp_sorcery_speed_only" (or its known variants),
// or whose raw text contains the canonical Teferi phrase.
func OppRestrictsDefenderToSorcerySpeed(gs *GameState, defenderSeat int) bool {
	if gs == nil {
		return false
	}
	for i, seat := range gs.Seats {
		if i == defenderSeat || seat == nil || seat.Lost {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil || perm.Card.AST == nil {
				continue
			}
			for _, ab := range perm.Card.AST.Abilities {
				st, ok := ab.(*gameast.Static)
				if !ok {
					continue
				}
				if st.Modification != nil {
					switch st.Modification.ModKind {
					case "opp_sorcery_speed_only",
						"cast_timing_opp_sorcery",
						"opp_only_sorcery_speed":
						return true
					}
				}
				// Raw-text fallback for parser variants.
				// Raw is pre-lowercased at AST load time.
				if strings.Contains(st.Raw,
					"each opponent can cast spells only any time they could cast a sorcery") {
					return true
				}
			}
			// Runtime-flag fallback — tests set perm.Flags["opp_sorcery_speed"]=1.
			if perm.Flags != nil && perm.Flags["opp_sorcery_speed"] != 0 {
				return true
			}
		}
	}
	return false
}

// grandAbolisherBlocksCast returns true if the active player controls
// a Grand Abolisher, preventing castingSeat (an opponent) from casting
// spells during the active player's turn. Checks
// gs.Flags["grand_abolisher_active_seat_N"] set by the per_card ETB handler.
func grandAbolisherBlocksCast(gs *GameState, castingSeat int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["grand_abolisher_active_seat_"+itoa(gs.Active)] > 0
}

// fireCastTriggers emits the family of "spell was cast" per-card
// triggers. Scopes the event so per-card handlers can tell whether the
// cast was by the listener's controller, an opponent, instant/sorcery,
// creature, etc.
//
// Emitted events (each handler decides which one to listen on):
//   - "spell_cast"                — always
//   - "spell_cast_by_opponent"    — caster != listener's controller
//   - "noncreature_spell_cast"    — spell is not a creature
//   - "creature_spell_cast"       — spell is a creature
//   - "instant_or_sorcery_cast"   — spell is instant/sorcery
func fireCastTriggers(gs *GameState, casterSeat int, card *Card) {
	fireCastTriggersFromZone(gs, casterSeat, card, ZoneHand)
}

// fireCastTriggersFromZone is the zone-aware variant. The cast_zone is
// threaded into the trigger ctx so per-card handlers (Golbez, future
// graveyard/exile-cast payoffs) can gate on the spell's source zone.
func fireCastTriggersFromZone(gs *GameState, casterSeat int, card *Card, fromZone string) {
	if gs == nil || card == nil {
		return
	}
	if fromZone == "" {
		fromZone = ZoneHand
	}
	ctx := map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  card.DisplayName(),
		"card":        card,
		"is_creature": cardHasType(card, "creature"),
		"cast_zone":   fromZone,
	}
	// CR §603.3b: all of the "spell was cast" family events fan out from a
	// single cast — wrap them in one batch so per-card triggers from spell_cast
	// don't resolve before noncreature_spell_cast / creature_spell_cast /
	// instant_or_sorcery_cast even fire. Without this, Rhystic Study would
	// resolve and draw before Mystic Remora's noncreature_spell_cast got the
	// chance to be batched with it, violating §603.3b ordering.
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	FireCardTrigger(gs, "spell_cast", ctx)
	// Opponent scoping — fire this event unconditionally; handlers check
	// ctx["caster_seat"] against their own controller to decide.
	FireCardTrigger(gs, "spell_cast_by_opponent", ctx)
	if cardHasType(card, "creature") {
		FireCardTrigger(gs, "creature_spell_cast", ctx)
	} else {
		FireCardTrigger(gs, "noncreature_spell_cast", ctx)
	}
	if cardHasType(card, "instant") || cardHasType(card, "sorcery") {
		FireCardTrigger(gs, "instant_or_sorcery_cast", ctx)
		// CR §702.137a — magecraft triggers on cast of an instant/sorcery
		// spell. The copy branch is fired from resolveCopySpell.
		FireMagecraftTriggers(gs, casterSeat, card, false)
	}

	// Observer cast triggers — scan all permanents for AST-driven "whenever
	// a player/opponent casts a spell" triggers (cast_filtered, cast_any, etc.)
	fireObserverCastTriggers(gs, casterSeat, card)
}

// FireCastTriggers is the exported wrapper around fireCastTriggers. It
// exists so tests + alternative cast paths (commander cast, response
// cast, future alt-cost cast) can emit the same cast-trigger fan-out
// without duplicating the cardHasType shuffle.
func FireCastTriggers(gs *GameState, casterSeat int, card *Card) {
	fireCastTriggers(gs, casterSeat, card)
}

// ---------------------------------------------------------------------------
// Ward — CR §702.21
// ---------------------------------------------------------------------------

// CheckWardOnTargeting implements the ward triggered ability. When a
// spell or ability targets a permanent with ward, and the source is
// controlled by an opponent, the controller of the spell must pay the
// ward cost or the spell is countered.
//
// Ward cost is extracted from:
//  1. Permanent.Flags["ward_cost"] — generic mana cost (int)
//  2. Keyword "ward" with no cost defaults to ward {1}
//
// Per CR §702.21c: "If a player doesn't pay the ward cost for a
// spell they control, the spell is countered."
func CheckWardOnTargeting(gs *GameState, item *StackItem) {
	if gs == nil || item == nil {
		return
	}
	for _, tgt := range item.Targets {
		if tgt.Kind != TargetKindPermanent || tgt.Permanent == nil {
			continue
		}
		perm := tgt.Permanent
		// Ward only triggers when an OPPONENT targets.
		if perm.Controller == item.Controller {
			continue
		}

		// Permanent-scope ward (printed on the target itself). Only
		// fires when the target literally has the "ward" keyword.
		if perm.HasKeyword("ward") {
			payPermanentWard(gs, item, perm)
			if item.Countered {
				return
			}
		}

		// r60 — seat-scope wards. Per CR §702.21e each ward instance is
		// a separate triggered ability, so anthem-granted wards fire
		// IN ADDITION to any printed ward on the target. Each entry is
		// dispatched as its own payment; if any one can't be paid, the
		// spell is countered.
		for _, entry := range SeatWardCostsFor(gs, perm) {
			paySeatScopeWard(gs, item, perm, entry)
			if item.Countered {
				return
			}
		}
	}
}

// payPermanentWard dispatches a single per-target ward payment for a
// permanent-scope ward (printed on the target). Extracted from the old
// monolithic CheckWardOnTargeting so seat-scope wards can reuse the
// same alt-payment vs mana-cost branching via paySeatScopeWard.
func payPermanentWard(gs *GameState, item *StackItem, perm *Permanent) {
	// Alternative-payment ward (CR §702.21d) — Sauron, Saruman,
	// Auntie Ool, Charging War Boar etc. routed through the unified
	// WardCost dispatch.
	if perm.Flags != nil && perm.Flags["ward_alt_kind"] != 0 {
		tryPayAltWardCost(gs, item, perm)
		return
	}
	// Mana ward.
	wardCost := 1
	if perm.Flags != nil {
		if v, ok := perm.Flags["ward_cost"]; ok && v > 0 {
			wardCost = v
		}
	}
	payManaWard(gs, item, perm, wardCost)
}

// paySeatScopeWard dispatches a single seat-scope ward payment.
// Bridges the SeatWardEntry struct to the same mana / alt-payment
// resolution paths used for permanent-scope ward, so the dispatcher
// is unified (the user's 2026-05-27 architecture lock requirement).
func paySeatScopeWard(gs *GameState, item *StackItem, target *Permanent, entry SeatWardEntry) {
	cost := entry.Cost
	if cost.Type == WardCostNone {
		// Empty cost — treat as mana 1 default.
		cost.Type = WardCostMana
		if cost.Amount <= 0 {
			cost.Amount = 1
		}
	}
	if cost.Type == WardCostMana {
		n := cost.Amount
		if n <= 0 {
			n = 1
		}
		payManaWard(gs, item, target, n)
		return
	}
	// Alt-payment path — synthesize a temporary stamping of the cost
	// on a dummy permanent so tryPayAltWardCost can read the existing
	// Flags-based ward kind. Cleaner than threading WardCost through
	// the legacy alt-payment signature.
	tmp := &Permanent{Card: target.Card, Controller: target.Controller}
	SetWardCost(tmp, cost)
	tryPayAltWardCost(gs, item, tmp)
}

// payManaWard — historical mana-ward branch extracted so both per-perm
// and seat-scope dispatchers share the WardPayer hat hook + the
// pay/decline → counter logic.
func payManaWard(gs *GameState, item *StackItem, perm *Permanent, wardCost int) {
	casterSeat := gs.Seats[item.Controller]
	if casterSeat == nil {
		return
	}
	willPay := casterSeat.ManaPool >= wardCost
	if willPay && casterSeat.Hat != nil {
		if wp, ok := casterSeat.Hat.(WardPayer); ok {
			willPay = wp.ShouldPayWard(gs, item.Controller, item, perm, wardCost)
		}
	}
	if willPay {
		casterSeat.ManaPool -= wardCost
		SyncManaAfterSpend(casterSeat)
		gs.Legality.NoteManaSpend(item.Controller, wardCost) // aux payment, not spell cost
		gs.LogEvent(Event{
			Kind:   "ward_paid",
			Seat:   item.Controller,
			Source: perm.Card.DisplayName(),
			Amount: wardCost,
			Details: map[string]interface{}{
				"rule":        "702.21",
				"ward_target": perm.Card.DisplayName(),
				"spell":       itemName(item),
			},
		})
		return
	}
	item.Countered = true
	gs.LogEvent(Event{
		Kind:   "ward_counter",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: wardCost,
		Details: map[string]interface{}{
			"rule":        "702.21c",
			"ward_target": perm.Card.DisplayName(),
			"spell":       itemName(item),
			"caster_seat": item.Controller,
		},
	})
}

// itemName returns the display name of a stack item for logging.
func itemName(item *StackItem) string {
	if item == nil {
		return "<nil>"
	}
	if item.Card != nil {
		return item.Card.DisplayName()
	}
	if item.Source != nil && item.Source.Card != nil {
		return item.Source.Card.DisplayName()
	}
	return "<unknown>"
}
