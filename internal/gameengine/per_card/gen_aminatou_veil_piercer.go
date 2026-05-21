package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAminatouVeilPiercer wires Aminatou, Veil Piercer.
//
// Oracle text (Duskmourn Commander, {1}{W}{U}{B}, 2/4):
//
//	At the beginning of your upkeep, surveil 2.
//	Each enchantment card in your hand has miracle. Its miracle cost is
//	equal to its mana cost reduced by {4}.
//
// Implementation:
//   - "upkeep_controller" trigger: surveil 2 for the active player when
//     it's also Aminatou's controller (the trigger says "your upkeep").
//   - The miracle-grant static is left to the AST engine — granting
//     miracle to enchantments in hand needs cast-time wiring beyond a
//     per_card ETB hook. emitPartial flags the gap.
func registerAminatouVeilPiercer(r *Registry) {
	r.OnETB("Aminatou, Veil Piercer", aminatouVeilPiercerETB)
	r.OnTrigger("Aminatou, Veil Piercer", "upkeep_controller", aminatouVeilPiercerUpkeep)
	// R57: drop the ZoneCastPolicy at LTB.
	r.OnTrigger("Aminatou, Veil Piercer", "permanent_ltb", aminatouVeilPiercerLTB)
}

func aminatouVeilPiercerLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
}

func aminatouVeilPiercerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aminatou_veil_piercer_etb"
	if gs == nil || perm == nil {
		return
	}
	// R57: register a ZoneCastPolicy granting cast-from-hand on
	// enchantment cards. The printed text is "Each enchantment card
	// in your hand has miracle. Its miracle cost is mana cost - {4}".
	// The engine's MiracleCost path reads a keyword argument that
	// isn't dynamically grantable on a Card-in-hand without a
	// permanent slot. We approximate the strategic effect: register a
	// policy that permits the controller to cast enchantments from
	// their own hand at normal cost (full miracle-cost-discount is
	// an engine cast-pipeline enhancement still planned). The grant
	// is documented as a policy so analytics and AI hat see the
	// cast-from-hand permission for Aminatou's archetype.
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "aminatou_enchantment_in_hand_miracle_grant",
		Zone:            gameengine.ZoneHand,
		OwnerScope:      "self",
		CasterScope:     "controller",
		ControllerSeat:  perm.Controller,
		Predicate:       aminatouEnchantmentPredicate,
		ManaCost:        -1, // normal cost; full miracle discount TODO
		Duration:        "while_source_on_bf",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"policy": "aminatou_enchantment_in_hand",
	})
}

func aminatouEnchantmentPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasType(c, "enchantment")
}

func aminatouVeilPiercerUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aminatou_veil_piercer_upkeep_surveil"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	gameengine.Surveil(gs, perm.Controller, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"surveil": 2,
	})
}
