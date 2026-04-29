package gameengine

// P0 zone-change infrastructure: dies/LTB triggers, proper destroy/exile/
// sacrifice/bounce helpers that respect indestructible, replacement effects,
// and commander redirect.
//
// Comp-rules citations (data/rules/MagicCompRules-20260227.txt):
//
//   §700.4    "Dies" = battlefield → graveyard (CR §700.4).
//   §603.6    A trigger fires whenever the specified event occurs.
//   §603.10   Zone-change triggers "look back in time" to the game state
//             just before the event to determine if the trigger fires.
//   §701.7    "Destroy" = battlefield → graveyard (indestructible stops it).
//   §701.17   "Sacrifice" = battlefield → graveyard (ignores indestructible).
//   §614      Replacement effects modify zone changes.
//   §903.9a/b Commander zone redirect.
//
// This file provides:
//
//   DestroyPermanent(gs, perm, source)   — §701.7 destroy
//   ExilePermanent(gs, perm, source)     — §406.3 exile
//   sacrificePermanentImpl(gs, perm, source, reason) — §701.17 sacrifice (internal)
//   BouncePermanent(gs, perm, source, dest) — return to hand
//   FireZoneChangeTriggers(gs, perm, card, fromZone, toZone) — observer triggers

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// DestroyPermanent — CR §701.7
// ---------------------------------------------------------------------------

// DestroyPermanent handles a destroy effect from a spell or ability (NOT an
// SBA). Per CR §701.7a: "To destroy a permanent, move it from the battlefield
// to its owner's graveyard." Per §702.12b, indestructible permanents can't be
// destroyed. Replacement effects (Rest in Peace, Anafenza, etc.) may redirect
// the destination. Commander redirect (§903.9a/b) is applied via FireZoneChange.
//
// Returns true if the permanent was actually removed from the battlefield.
func DestroyPermanent(gs *GameState, perm *Permanent, source *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}

	// §122.1b: Shield counter — "If this permanent would be destroyed or
	// dealt damage, remove a shield counter from it instead."
	if perm.Counters != nil && perm.Counters["shield"] > 0 {
		perm.Counters["shield"]--
		if perm.Counters["shield"] <= 0 {
			delete(perm.Counters, "shield")
		}
		gs.LogEvent(Event{
			Kind:   "shield_counter_consumed",
			Seat:   perm.Controller,
			Source: sourceName(source),
			Details: map[string]interface{}{
				"target_card":      perm.Card.DisplayName(),
				"shields_remaining": perm.Counters["shield"],
				"rule":             "122.1b",
			},
		})
		return false
	}

	// §702.12b: indestructible permanents can't be destroyed.
	if perm.IsIndestructible() {
		gs.LogEvent(Event{
			Kind:   "destroy_prevented",
			Seat:   perm.Controller,
			Source: sourceName(source),
			Details: map[string]interface{}{
				"target_card": perm.Card.DisplayName(),
				"reason":      "indestructible",
				"rule":        "702.12b",
			},
		})
		return false
	}

	// §614 "would die" replacement chain — same path as SBA.
	repl := FireDieEvent(gs, perm)
	if repl.Cancelled {
		return false
	}
	destZone := "graveyard"
	if z := repl.String("to_zone"); z != "" {
		destZone = z
	}

	if !gs.removePermanent(perm) {
		return false
	}

	// Unregister replacement + continuous effects for the leaving permanent.
	gs.UnregisterReplacementsForPermanent(perm)
	gs.UnregisterContinuousEffectsForPermanent(perm)

	gs.LogEvent(Event{
		Kind:   "destroy",
		Seat:   controllerSeat(source),
		Target: perm.Controller,
		Source: sourceName(source),
		Details: map[string]interface{}{
			"target_card": perm.Card.DisplayName(),
			"to_zone":     destZone,
			"rule":        "701.7",
		},
	})

	// Detach anything attached to p.
	detachAll(gs, perm)

	// Tokens cease to exist — skip zone write (§704.5d cleanup).
	if !perm.IsToken() {
		finalZone := FireZoneChange(gs, perm, perm.Card, perm.Card.Owner, "battlefield", destZone)
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", destZone)
	}

	return true
}

// ---------------------------------------------------------------------------
// ExilePermanent — §406.3
// ---------------------------------------------------------------------------

// ExilePermanent removes a permanent from the battlefield and places it in
// exile. Replacement effects may redirect. Commander redirect applies.
// Unlike destroy, exile is NOT stopped by indestructible.
func ExilePermanent(gs *GameState, perm *Permanent, source *Permanent) bool {
	if gs == nil || perm == nil {
		return false
	}

	// Run the would_be_exiled replacement chain.
	repl := fireExileEvent(gs, perm, source)
	if repl.Cancelled {
		return false
	}
	destZone := "exile"
	if z := repl.String("to_zone"); z != "" {
		destZone = z
	}

	if !gs.removePermanent(perm) {
		return false
	}

	gs.UnregisterReplacementsForPermanent(perm)
	gs.UnregisterContinuousEffectsForPermanent(perm)

	gs.LogEvent(Event{
		Kind:   "exile",
		Seat:   controllerSeat(source),
		Target: perm.Controller,
		Source: sourceName(source),
		Details: map[string]interface{}{
			"target_card": perm.Card.DisplayName(),
			"to_zone":     destZone,
			"rule":        "406.3",
		},
	})

	detachAll(gs, perm)

	if !perm.IsToken() {
		finalZone := FireZoneChange(gs, perm, perm.Card, perm.Card.Owner, "battlefield", destZone)
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", destZone)
	}

	return true
}

// fireExileEvent builds and dispatches a would_be_exiled event.
func fireExileEvent(gs *GameState, perm *Permanent, src *Permanent) *ReplEvent {
	ev := NewReplEvent("would_be_exiled")
	ev.TargetPerm = perm
	ev.Source = src
	if perm != nil {
		ev.TargetSeat = perm.Controller
	}
	ev.Payload["to_zone"] = "exile"
	FireEvent(gs, ev)
	return ev
}

// ---------------------------------------------------------------------------
// sacrificePermanentImpl — CR §701.17 (internal, called by SacrificePermanent)
// ---------------------------------------------------------------------------

// sacrificePermanentImpl handles sacrifice. Per CR §701.17a: "To sacrifice a
// permanent, its controller moves it from the battlefield directly to its
// owner's graveyard." §701.17b: sacrifice ignores indestructible.
// Replacement effects (Rest in Peace, etc.) still apply.
func sacrificePermanentImpl(gs *GameState, perm *Permanent, source *Permanent, reason string) bool {
	if gs == nil || perm == nil {
		return false
	}
	// NOTE: sacrifice does NOT check indestructible (CR §701.17b).

	// §614 "would die" replacement chain — sacrifice is still a "dies" event.
	repl := FireDieEvent(gs, perm)
	if repl.Cancelled {
		return false
	}
	destZone := "graveyard"
	if z := repl.String("to_zone"); z != "" {
		destZone = z
	}

	if !gs.removePermanent(perm) {
		return false
	}

	gs.UnregisterReplacementsForPermanent(perm)
	gs.UnregisterContinuousEffectsForPermanent(perm)

	cardName := "<unknown>"
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "sacrifice",
		Seat:   perm.Controller,
		Target: perm.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"target_card": cardName,
			"to_zone":     destZone,
			"reason":      reason,
			"rule":        "701.17",
		},
	})

	detachAll(gs, perm)

	if !perm.IsToken() {
		finalZone := FireZoneChange(gs, perm, perm.Card, perm.Card.Owner, "battlefield", destZone)
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", destZone)
	}

	// Emit sacrifice-type events for per-card triggers (Crime Novelist,
	// Nuka-Cola Vending Machine, etc.). Runs AFTER zone-change triggers
	// so the permanent is fully processed before sacrifice-specific
	// triggers fire.
	if perm.Card != nil {
		tl := strings.ToLower(strings.Join(perm.Card.Types, " "))
		sacCtx := map[string]interface{}{
			"perm":            perm,
			"card":            perm.Card,
			"controller_seat": perm.Controller,
			"card_name":       cardName,
		}
		if strings.Contains(tl, "artifact") {
			FireCardTrigger(gs, "artifact_sacrificed", sacCtx)
		}
		if strings.Contains(tl, "food") {
			FireCardTrigger(gs, "food_sacrificed", sacCtx)
		}
		if strings.Contains(tl, "creature") {
			FireCardTrigger(gs, "creature_sacrificed", sacCtx)
		}
		sacCtx["to_zone"] = destZone
		FireCardTrigger(gs, "permanent_sacrificed", sacCtx)
	}

	return true
}

// ---------------------------------------------------------------------------
// BouncePermanent — return to hand / library
// ---------------------------------------------------------------------------

// BouncePermanent returns a permanent to its owner's hand (or library top/
// bottom if dest is specified). Not a destroy; does not check indestructible.
// Replacement effects and commander redirect apply.
func BouncePermanent(gs *GameState, perm *Permanent, source *Permanent, dest string) bool {
	if gs == nil || perm == nil {
		return false
	}
	if dest == "" {
		dest = "hand"
	}

	if !gs.removePermanent(perm) {
		return false
	}

	gs.UnregisterReplacementsForPermanent(perm)
	gs.UnregisterContinuousEffectsForPermanent(perm)

	gs.LogEvent(Event{
		Kind:   "bounce",
		Seat:   controllerSeat(source),
		Target: perm.Controller,
		Source: sourceName(source),
		Details: map[string]interface{}{
			"target_card": perm.Card.DisplayName(),
			"to":          dest,
			"rule":        "701.8",
		},
	})

	detachAll(gs, perm)

	if !perm.IsToken() {
		finalZone := FireZoneChange(gs, perm, perm.Card, perm.Card.Owner, "battlefield", dest)
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", finalZone)
	} else {
		FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", dest)
	}

	return true
}

// ---------------------------------------------------------------------------
// FireZoneChangeTriggers — observer-pattern zone-change triggers
// ---------------------------------------------------------------------------

// FireZoneChangeTriggers scans ALL permanents on the battlefield for triggered
// abilities that match the given zone change. This handles "observer" triggers
// like Blood Artist ("whenever a creature dies"), Grave Pact ("whenever a
// creature you control dies"), and Dictate of Erebos.
//
// It also fires self-triggers from the permanent that just changed zones
// (looking back in time per CR §603.10).
//
// Parameters:
//   - perm:     the permanent that changed zones (may no longer be on battlefield)
//   - card:     the card for that permanent
//   - fromZone: the source zone ("battlefield", "hand", "library", etc.)
//   - toZone:   the final destination zone ("graveyard", "exile", "hand", etc.)
func FireZoneChangeTriggers(gs *GameState, perm *Permanent, card *Card, fromZone, toZone string) {
	if gs == nil {
		return
	}

	// Determine which trigger events match this zone change.
	events := zoneChangeToTriggerEvents(fromZone, toZone)
	if len(events) == 0 {
		return
	}

	// 1. Fire self-triggers: scan the dying/leaving permanent's own abilities
	//    for matching triggers. Per CR §603.10 we look at the state just before
	//    the zone change (i.e. the permanent's abilities as they were).
	fireSelfZoneChangeTriggers(gs, perm, events)

	// 2. Fire observer triggers: scan all permanents on the battlefield for
	//    triggered abilities that watch for this zone-change event.
	fireObserverZoneChangeTriggers(gs, perm, card, events, fromZone, toZone)

	// 3. Fire the per-card hook trigger system for "creature_dies" / "permanent_ltb".
	// CR §700.4: "dies" = "is put into a graveyard from the battlefield."
	if fromZone == "battlefield" && toZone == "graveyard" {
		if perm != nil && (perm.IsCreature() || (card != nil && cardHasType(card, "creature"))) {
			FireCardTrigger(gs, "creature_dies", map[string]interface{}{
				"perm":            perm,
				"card":            card,
				"controller_seat": perm.Controller,
				"to_zone":         toZone,
			})
		}
	}
	if fromZone == "battlefield" {
		FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
			"perm":            perm,
			"card":            card,
			"controller_seat": perm.Controller,
			"to_zone":         toZone,
		})
	}
}

// zoneChangeToTriggerEvents maps a (fromZone, toZone) pair to the set of
// trigger event strings that should fire.
func zoneChangeToTriggerEvents(fromZone, toZone string) []string {
	var events []string

	// "dies" = battlefield → graveyard (CR §700.4)
	if fromZone == "battlefield" && toZone == "graveyard" {
		events = append(events, "die", "dies")
	}

	// "leaves the battlefield" = battlefield → any zone
	if fromZone == "battlefield" {
		events = append(events, "ltb", "leaves_battlefield", "leave_battlefield")
	}

	// "put into graveyard from anywhere"
	if toZone == "graveyard" {
		events = append(events, "put_into_graveyard")
	}

	// "exiled"
	if toZone == "exile" {
		events = append(events, "exiled")
	}

	return events
}

// fireSelfZoneChangeTriggers fires triggers on the permanent itself (looking
// back in time per CR §603.10). E.g. "When Kokusho dies, each opponent
// loses 5 life and you gain life equal to the life lost this way."
func fireSelfZoneChangeTriggers(gs *GameState, perm *Permanent, events []string) {
	if perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}

	for _, ab := range perm.Card.AST.Abilities {
		trig, ok := ab.(*gameast.Triggered)
		if !ok || trig.Effect == nil {
			continue
		}

		trigEvent := strings.ToLower(strings.TrimSpace(trig.Trigger.Event))
		if !EventMatchesAny(trigEvent, events) {
			continue
		}

		// Check if the trigger is self-referencing ("this creature",
		// "this permanent", or has no actor filter).
		if !isSelfTrigger(trig) {
			continue
		}

		// Push the trigger onto the stack. We create a phantom permanent
		// since the original is no longer on the battlefield but we need
		// a source for controller + card reference on the stack item.
		PushTriggeredAbility(gs, &Permanent{
			Card:       perm.Card,
			Controller: perm.Controller,
			Owner:      perm.Owner,
			Timestamp:  perm.Timestamp,
			Flags:      map[string]int{},
			Counters:   map[string]int{},
		}, trig.Effect)
		if gs.CheckEnd() {
			return
		}
	}
}

// fireObserverZoneChangeTriggers scans all permanents on the battlefield for
// triggered abilities that match the zone-change event. E.g. Blood Artist
// ("whenever a creature dies") or Grave Pact ("whenever a creature you
// control dies").
//
// Per CR §101.4 (APNAP ordering), observer triggers are processed in
// APNAP order: active player's triggers go on the stack first (resolve
// last due to LIFO), then each other player in turn order.
func fireObserverZoneChangeTriggers(gs *GameState, dyingPerm *Permanent, dyingCard *Card, events []string, fromZone, toZone string) {
	if gs == nil {
		return
	}

	apnapOrder := APNAPOrder(gs)
	for _, seatIdx := range apnapOrder {
		seat := gs.Seats[seatIdx]
		if seat == nil {
			continue
		}
		// Snapshot because triggers may modify the battlefield.
		perms := make([]*Permanent, len(seat.Battlefield))
		copy(perms, seat.Battlefield)

		for _, observer := range perms {
			if observer == nil || observer.Card == nil || observer.Card.AST == nil {
				continue
			}
			// Skip the dying permanent itself — self-triggers already handled.
			if observer == dyingPerm {
				continue
			}

			for _, ab := range observer.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}

				trigEvent := strings.ToLower(strings.TrimSpace(trig.Trigger.Event))
				if !EventMatchesAny(trigEvent, events) {
					continue
				}

				// Must NOT be a self-trigger (those were handled above).
				if isSelfTrigger(trig) {
					continue
				}

				// Check if the dying permanent matches the trigger's actor filter.
				if !observerTriggerMatches(trig, observer, dyingPerm, dyingCard) {
					continue
				}

				PushTriggeredAbility(gs, observer, trig.Effect)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}
}

// isSelfTrigger returns true if the triggered ability is self-referencing
// (e.g. "When THIS creature dies" or has no explicit actor, or uses "self"
// / "this" reference).
func isSelfTrigger(trig *gameast.Triggered) bool {
	if trig.Trigger.Actor == nil {
		// No actor filter → self-trigger by convention (e.g. "When this
		// creature dies").
		return true
	}
	base := strings.ToLower(trig.Trigger.Actor.Base)
	return base == "self" || base == "this" || base == "this_creature" ||
		base == "this_permanent" || base == "it"
}

// observerTriggerMatches checks if the dying permanent matches an observer
// trigger's actor filter. E.g. Blood Artist's filter is "a creature" — so
// any creature dying matches. Grave Pact's filter might be "a creature you
// control" — which only matches creatures the observer's controller controls.
func observerTriggerMatches(trig *gameast.Triggered, observer *Permanent, dying *Permanent, dyingCard *Card) bool {
	if trig.Trigger.Actor == nil {
		return false // no actor = self-trigger, handled elsewhere
	}

	base := strings.ToLower(trig.Trigger.Actor.Base)

	// "a creature" / "creature" / "another creature"
	if base == "creature" || base == "a_creature" || base == "another_creature" {
		if dying == nil || dyingCard == nil {
			return false
		}
		if !dying.IsCreature() && !cardHasType(dyingCard, "creature") {
			return false
		}
		// "another creature" — must be different from observer
		if base == "another_creature" && dying == observer {
			return false
		}
		// Check controller constraint if present.
		ctrl := strings.ToLower(trig.Trigger.Actor.Quantifier)
		if ctrl == "" {
			ctrl = strings.ToLower(trig.Trigger.Controller)
		}
		if ctrl == "you" || ctrl == "you_control" {
			if dying.Controller != observer.Controller {
				return false
			}
		}
		return true
	}

	// "a permanent" / "permanent"
	if base == "permanent" || base == "a_permanent" || base == "another_permanent" {
		if base == "another_permanent" && dying == observer {
			return false
		}
		ctrl := strings.ToLower(trig.Trigger.Controller)
		if ctrl == "you" || ctrl == "you_control" {
			if dying != nil && dying.Controller != observer.Controller {
				return false
			}
		}
		return true
	}

	// "a nontoken creature" — common for aristocrats
	if base == "nontoken_creature" || base == "a_nontoken_creature" {
		if dying == nil || dyingCard == nil {
			return false
		}
		if dying.IsToken() {
			return false
		}
		if !dying.IsCreature() && !cardHasType(dyingCard, "creature") {
			return false
		}
		return true
	}

	// Generic fallback: if the filter base matches a type on the dying card,
	// accept it. This catches "artifact", "enchantment", etc.
	if dying != nil && dying.hasType(base) {
		return true
	}
	if dyingCard != nil && cardHasType(dyingCard, base) {
		return true
	}

	return false
}

// ---------------------------------------------------------------------------
// Target legality check helpers (for P0 #2)
// ---------------------------------------------------------------------------

// CheckTargetLegality verifies that the targets on a stack item are still
// legal per CR §608.2b. Returns:
//   - allIllegal: true if ALL targets are illegal (spell fizzles)
//   - legalTargets: the subset of targets that are still legal
func CheckTargetLegality(gs *GameState, item *StackItem) (allIllegal bool, legalTargets []Target) {
	if gs == nil || item == nil || len(item.Targets) == 0 {
		return false, item.Targets
	}

	anyLegal := false
	for _, t := range item.Targets {
		if isTargetStillLegal(gs, t) {
			legalTargets = append(legalTargets, t)
			anyLegal = true
		}
	}

	if !anyLegal {
		return true, nil
	}
	return false, legalTargets
}

// isTargetStillLegal checks if a single target is still valid at resolution.
func isTargetStillLegal(gs *GameState, t Target) bool {
	switch t.Kind {
	case TargetKindSeat:
		// Player targets: valid if the player is still alive.
		if t.Seat < 0 || t.Seat >= len(gs.Seats) {
			return false
		}
		s := gs.Seats[t.Seat]
		return s != nil && !s.Lost && !s.LeftGame
	case TargetKindPermanent:
		// Permanent targets: valid if still on the battlefield.
		if t.Permanent == nil {
			return false
		}
		return permanentOnBattlefield(gs, t.Permanent)
	case TargetKindStackItem:
		// Stack targets: valid if the item is still on the stack.
		if t.Stack == nil {
			return false
		}
		for _, si := range gs.Stack {
			if si == t.Stack {
				return true
			}
		}
		return false
	default:
		// Unknown target kind — conservatively treat as legal.
		return true
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func stringSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
