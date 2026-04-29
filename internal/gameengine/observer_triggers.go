package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// fireObserverETBTriggers scans all permanents on the battlefield for
// triggered abilities that fire when another permanent enters. Handles
// parser events like "another_typed_enters", "tribe_you_control_etb",
// "ally_etb", "creature_etb_any", etc. — all of which normalize to "etb"
// via the alias table.
//
// Per CR §603.6a, observer ETB triggers use the entering permanent's
// characteristics as it exists on the battlefield.
// FireObserverETBTriggers is the exported wrapper so per_card handlers
// (token creation, flicker, etc.) can invoke observer ETB dispatch.
func FireObserverETBTriggers(gs *GameState, entering *Permanent) {
	fireObserverETBTriggers(gs, entering)
}

func fireObserverETBTriggers(gs *GameState, entering *Permanent) {
	if gs == nil || entering == nil || entering.Card == nil {
		return
	}

	apnapOrder := APNAPOrder(gs)
	for _, seatIdx := range apnapOrder {
		seat := gs.Seats[seatIdx]
		if seat == nil {
			continue
		}
		perms := make([]*Permanent, len(seat.Battlefield))
		copy(perms, seat.Battlefield)

		for _, observer := range perms {
			if observer == nil || observer.Card == nil || observer.Card.AST == nil {
				continue
			}
			if observer == entering {
				continue
			}

			for _, ab := range observer.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}

				if !EventEquals(trig.Trigger.Event, "etb") {
					continue
				}

				if isSelfTrigger(trig) {
					continue
				}

				if !observerETBMatches(trig, observer, entering) {
					continue
				}

				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: observer.Controller,
					Source: observer.Card.DisplayName(),
					Details: map[string]interface{}{
						"event":    "observer_etb",
						"entering": entering.Card.DisplayName(),
						"rule":     "603.6a",
					},
				})

				PushTriggeredAbility(gs, observer, trig.Effect)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}
}

// observerETBMatches checks if the entering permanent matches an observer
// trigger's actor filter. Handles "a creature", "another creature you
// control", "a nontoken creature", type-based filters, etc.
func observerETBMatches(trig *gameast.Triggered, observer, entering *Permanent) bool {
	if trig.Trigger.Actor == nil {
		return false
	}

	base := strings.ToLower(trig.Trigger.Actor.Base)
	enteringCard := entering.Card

	isAnother := strings.Contains(base, "another")
	if isAnother && entering == observer {
		return false
	}

	cleanBase := strings.TrimPrefix(base, "another_")
	cleanBase = strings.TrimPrefix(cleanBase, "a_")

	switch {
	case cleanBase == "creature" || cleanBase == "nontoken_creature":
		if !entering.IsCreature() && (enteringCard == nil || !cardHasType(enteringCard, "creature")) {
			return false
		}
		if cleanBase == "nontoken_creature" && entering.IsToken() {
			return false
		}

	case cleanBase == "permanent" || cleanBase == "nontoken_permanent":
		if cleanBase == "nontoken_permanent" && entering.IsToken() {
			return false
		}

	case cleanBase == "artifact":
		if !entering.hasType("artifact") && (enteringCard == nil || !cardHasType(enteringCard, "artifact")) {
			return false
		}

	case cleanBase == "enchantment":
		if !entering.hasType("enchantment") && (enteringCard == nil || !cardHasType(enteringCard, "enchantment")) {
			return false
		}

	case cleanBase == "land":
		if !entering.hasType("land") && (enteringCard == nil || !cardHasType(enteringCard, "land")) {
			return false
		}

	case cleanBase == "planeswalker":
		if !entering.hasType("planeswalker") && (enteringCard == nil || !cardHasType(enteringCard, "planeswalker")) {
			return false
		}

	default:
		if enteringCard != nil && !cardHasType(enteringCard, cleanBase) &&
			!entering.hasType(cleanBase) && !cardHasSubtype(enteringCard, cleanBase) {
			return false
		}
	}

	ctrl := strings.ToLower(trig.Trigger.Actor.Quantifier)
	if ctrl == "" {
		ctrl = strings.ToLower(trig.Trigger.Controller)
	}
	if ctrl == "you" || ctrl == "you_control" {
		if entering.Controller != observer.Controller {
			return false
		}
	}
	if ctrl == "opponent" || ctrl == "an_opponent" {
		if entering.Controller == observer.Controller {
			return false
		}
	}

	return true
}

// fireObserverCastTriggers scans all permanents for triggered abilities that
// fire when a spell is cast. Handles parser events like "cast_filtered",
// "cast_any", "opp_cast", etc. — all normalize to "cast" via the alias table.
//
// Called from fireCastTriggers in stack.go.
func fireObserverCastTriggers(gs *GameState, casterSeat int, card *Card) {
	if gs == nil || card == nil {
		return
	}

	for _, seatIdx := range APNAPOrder(gs) {
		seat := gs.Seats[seatIdx]
		if seat == nil {
			continue
		}
		perms := make([]*Permanent, len(seat.Battlefield))
		copy(perms, seat.Battlefield)

		for _, observer := range perms {
			if observer == nil || observer.Card == nil || observer.Card.AST == nil {
				continue
			}

			for _, ab := range observer.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}

				if !EventEquals(trig.Trigger.Event, "cast") {
					continue
				}

				if isSelfTrigger(trig) {
					continue
				}

				origEvent := strings.ToLower(strings.TrimSpace(trig.Trigger.Event))

				if strings.Contains(origEvent, "opp") {
					if casterSeat == observer.Controller {
						continue
					}
				}

				if trig.Trigger.Actor != nil {
					base := strings.ToLower(trig.Trigger.Actor.Base)
					if base != "" && base != "spell" && base != "a_spell" {
						if card != nil && !matchesActorFilter(card, base) {
							continue
						}
					}
				}

				gs.LogEvent(Event{
					Kind: "trigger_fires", Seat: observer.Controller,
					Source: observer.Card.DisplayName(),
					Details: map[string]interface{}{
						"event":      "observer_cast",
						"spell_name": card.DisplayName(),
						"caster":     casterSeat,
						"rule":       "603.3a",
					},
				})

				PushTriggeredAbility(gs, observer, trig.Effect)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}
}

// fireBlockTriggers fires AST-driven block triggers after blockers are declared.
// Handles: "whenever ~ blocks" (self), "whenever ~ becomes blocked" (self on
// attacker), and observer block triggers on other permanents.
func fireBlockTriggers(gs *GameState, attackers []*Permanent, blockerMap map[*Permanent][]*Permanent) {
	if gs == nil {
		return
	}

	// (1) Self-block triggers on blockers.
	for _, blockers := range blockerMap {
		for _, blocker := range blockers {
			if blocker == nil || blocker.Card == nil || blocker.Card.AST == nil {
				continue
			}
			for _, ab := range blocker.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !EventEquals(trig.Trigger.Event, "block") {
					continue
				}
				if !isSelfTrigger(trig) {
					continue
				}
				PushTriggeredAbility(gs, blocker, trig.Effect)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}

	// (2) "Becomes blocked" triggers on attackers.
	for atk, blockers := range blockerMap {
		if len(blockers) == 0 || atk == nil || atk.Card == nil || atk.Card.AST == nil {
			continue
		}
		for _, ab := range atk.Card.AST.Abilities {
			trig, ok := ab.(*gameast.Triggered)
			if !ok || trig.Effect == nil {
				continue
			}
			ev := strings.ToLower(strings.TrimSpace(trig.Trigger.Event))
			if !EventEquals(ev, "blocked") && ev != "becomes_blocked" && ev != "becomes_blocked_by" {
				continue
			}
			PushTriggeredAbility(gs, atk, trig.Effect)
			if gs.CheckEnd() {
				return
			}
		}
	}
}

// matchesActorFilter handles compound AST actor filter bases like
// "instant_or_sorcery_spell", "noncreature_spell", etc.
func matchesActorFilter(card *Card, base string) bool {
	clean := strings.TrimSuffix(base, "_spell")
	clean = strings.TrimSuffix(clean, " spell")

	if strings.HasPrefix(clean, "non") {
		negType := strings.TrimPrefix(clean, "non")
		return !cardHasType(card, negType)
	}

	if strings.Contains(clean, "_or_") {
		parts := strings.Split(clean, "_or_")
		for _, p := range parts {
			if cardHasType(card, strings.TrimSpace(p)) {
				return true
			}
		}
		return false
	}

	return cardHasType(card, clean)
}
