package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaphaelNinjaDestroyer wires Raphael, Ninja Destroyer.
//
// Oracle text:
//
//	Raphael must be blocked if able.
//	Enrage — Whenever Raphael is dealt damage, add that much {R}. Until
//	end of turn, you don't lose this mana as steps and phases end.
//
// Implementation:
//   - Must-be-blocked is a combat-legality concern (engine-side).
//     emitPartial flags the boundary.
//   - "damage_dealt_to_perm" trigger gated on the target being Raphael
//     himself. Adds `amount` to controller's mana pool. R58: register
//     a ManaPoolExemption({R}) so the enrage mana survives phase
//     boundaries; a delayed end_of_turn trigger unregisters it so the
//     "until end of turn" duration is honored.
func registerRaphaelNinjaDestroyer(r *Registry) {
	r.OnETB("Raphael, Ninja Destroyer", raphaelNinjaDestroyerETB)
	r.OnTrigger("Raphael, Ninja Destroyer", "damage_taken", raphaelNinjaDestroyerEnrage)
	r.OnTrigger("Raphael, Ninja Destroyer", "permanent_ltb", raphaelNinjaDestroyerLTB)
}

func raphaelNinjaDestroyerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "raphael_ninja_destroyer_etb"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["must_be_blocked"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"must_be_blocked": 1,
	})
}

func raphaelNinjaDestroyerLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gameengine.UnregisterManaPoolExemptionForPerm(gs, perm)
}

func raphaelNinjaDestroyerEnrage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "raphael_enrage_red_mana"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target != perm {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	seat.ManaPool += amount
	// R58: register a ManaPoolExemption({R}) until end of turn so the
	// enrage mana persists across phase boundaries. The seat flag tally
	// is retained for downstream consumers that read it.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["raphael_keep_red_until_eot"] += amount
	gameengine.RegisterManaPoolExemption(gs, perm, perm.Controller, []string{"R"})
	captured := perm
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.UnregisterManaPoolExemptionForPerm(gs, captured)
			s := gs.Seats[captured.Controller]
			if s != nil && s.Flags != nil {
				delete(s.Flags, "raphael_keep_red_until_eot")
			}
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"red_added": amount,
		"exemption": "R_until_eot",
	})
}
