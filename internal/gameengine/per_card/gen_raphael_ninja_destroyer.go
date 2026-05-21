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
//     himself. Adds `amount` to controller's mana pool and stamps a
//     "raphael_keep_red_until_eot" flag the engine cleanup pass reads
//     to suppress phase-boundary mana drain.
func registerRaphaelNinjaDestroyer(r *Registry) {
	r.OnETB("Raphael, Ninja Destroyer", raphaelNinjaDestroyerETB)
	r.OnTrigger("Raphael, Ninja Destroyer", "damage_taken", raphaelNinjaDestroyerEnrage)
}

func raphaelNinjaDestroyerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "raphael_ninja_destroyer_etb"
	if gs == nil || perm == nil {
		return
	}
	// R51 batch H port: stamp the canonical must_be_blocked perm flag
	// (same key the AST keyword "must_be_blocked" path uses) so the
	// combat blocker-legality scanner forces an opponent to block
	// Raphael when able. The flag lives on the perm and dies with it
	// at LTB — no cleanup hook required.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["must_be_blocked"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"must_be_blocked": true,
	})
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
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["raphael_keep_red_until_eot"] += amount
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"red_added": amount,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"phase_boundary_mana_persist_relies_on_engine_until_eot_pass")
}
