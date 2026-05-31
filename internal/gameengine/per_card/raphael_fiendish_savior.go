package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaphaelFiendishSavior wires Raphael, Fiendish Savior.
//
// Oracle text (Scryfall, verified 2026-05-30, {3}{B}{R}, 4/4
// Legendary Creature — Devil Noble):
//
//	Flying
//	Other Demons, Devils, Imps, and Tieflings you control get +1/+1
//	and have lifelink.
//	At the beginning of each end step, if a creature card was put
//	into your graveyard from anywhere this turn, create a 1/1 red
//	Devil creature token with "When this token dies, it deals 1
//	damage to any target."
//
// Implementation (R60 stub sweep):
//   - creature_dies trigger: when ANY creature whose owner is Raphael's
//     controller dies, stamp seat.Flags["raphael_creature_died_turn"]
//     to gs.Turn+1 (using +1 because 0 is the empty-flag sentinel).
//   - end_step trigger: if the seat flag matches the current turn,
//     mint a 1/1 red Devil creature token. Fires for "each end step"
//     — every player's end step.
//   - Static tribal +1/+1 / lifelink anthem stays AST/layers territory.
//     The token's death-ping ability ("When this token dies, it deals
//     1 damage to any target") is a printed-on-token triggered ability
//     and not wired through per_card.
func registerRaphaelFiendishSavior(r *Registry) {
	r.OnTrigger("Raphael, Fiendish Savior", "creature_dies", raphaelFiendishCreatureDies)
	r.OnTrigger("Raphael, Fiendish Savior", "end_step", raphaelFiendishEndStep)
}

func raphaelFiendishCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadCard, _ := ctx["card"].(*gameengine.Card)
	if deadCard == nil || !cardHasType(deadCard, "creature") {
		return
	}
	if deadCard.Owner != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["raphael_creature_died_turn"] = gs.Turn + 1
}

func raphaelFiendishEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "raphael_fiendish_end_step_devil_token"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil || seat.Flags["raphael_creature_died_turn"] != gs.Turn+1 {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Devil Token",
		[]string{"creature", "token", "devil", "pip:R"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Devil Token",
	})
}
