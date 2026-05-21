package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNaduWingedWisdom wires Nadu, Winged Wisdom.
//
// Oracle text:
//
//	Flying
//	Creatures you control have "Whenever this creature becomes the
//	target of a spell or ability, reveal the top card of your library.
//	If it's a land card, put it onto the battlefield. Otherwise, put it
//	into your hand. This ability triggers only twice each turn."
//
// Implementation:
//   - The granted "becomes the target" trigger is the centerpiece of
//     the broken Nadu loop. We surface a per-creature target trigger via
//     a "creature_targeted" event. Engine plumbing must fire this event
//     on every targeting; per-card layer responds.
//   - On trigger: reveal top of controller's library; land → onto
//     battlefield, else → into hand. Twice-per-turn cap is tracked on
//     the entered permanent's Flags["nadu_target_triggers_this_turn"].
//   - ETB also stamps a seat flag so the granted-ability bookkeeping
//     can detect "Nadu is on the battlefield" cheaply (other handlers
//     and the engine target dispatcher key off this).
func registerNaduWingedWisdom(r *Registry) {
	r.OnETB("Nadu, Winged Wisdom", naduWingedWisdomETB)
	r.OnTrigger("Nadu, Winged Wisdom", "creature_targeted", naduWingedWisdomTargeted)
	// R51 batch I: LTB clears the nadu_grant_active seat flag so a
	// removed Nadu doesn't leave the granted-target-trigger contract
	// active for downstream consumers.
	r.OnTrigger("Nadu, Winged Wisdom", "permanent_ltb", naduLTBClearFlag)
}

func naduLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	delete(seat.Flags, "nadu_grant_active")
}

func naduWingedWisdomETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "nadu_winged_wisdom_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags["nadu_grant_active"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"granted_target_trigger_on_other_creatures_requires_engine_target_dispatcher")
}

func naduWingedWisdomTargeted(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nadu_creature_targeted_reveal"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil || !target.IsCreature() {
		return
	}
	if target.Controller != perm.Controller {
		return
	}
	// Twice per turn cap, per creature.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	if target.Flags["nadu_target_triggers_this_turn"] >= 2 {
		return
	}
	target.Flags["nadu_target_triggers_this_turn"]++

	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Library) == 0 {
		return
	}
	top := seat.Library[0]
	if top == nil {
		return
	}
	if cardHasType(top, "land") {
		gameengine.MoveCard(gs, top, perm.Controller, "library", "battlefield", "nadu_reveal_land")
		newPerm := createPermanent(gs, perm.Controller, top, false)
		if newPerm != nil {
			gameengine.RegisterReplacementsForPermanent(gs, newPerm)
			gameengine.FirePermanentETBTriggers(gs, newPerm)
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"target": target.Card.DisplayName(),
			"reveal": top.DisplayName(),
			"action": "to_battlefield",
		})
		return
	}
	gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "nadu_reveal_nonland")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.Card.DisplayName(),
		"reveal": top.DisplayName(),
		"action": "to_hand",
	})
}
