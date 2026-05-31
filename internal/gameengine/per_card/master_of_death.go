package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMasterOfDeath wires Master of Death (Muninn snowflake first
// seen 2026-05-15).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Master of Death — {2}{B} Creature — Dauthi Horror 3/2
//	When this creature enters, surveil 2. (Look at the top two cards of
//	your library, then put any number of them into your graveyard and
//	the rest on top of your library in any order.)
//	At the beginning of your upkeep, if this card is in your graveyard,
//	you may pay 1 life. If you do, return it to your hand.
//
// Implementation:
//   - OnETB: surveil 2 (controller's library top 2 → graveyard by
//     default; engine has no per-card AI chooser for "keep on top vs.
//     mill". We mill both by default and emit partial for the
//     keep-on-top branch).
//   - Graveyard-side upkeep return uses the same private-zone phase
//     trigger gap noted on Ichorid / Sproutback Trudge. emitPartial.
func registerMasterOfDeath(r *Registry) {
	r.OnETB("Master of Death", masterOfDeathETB)
}

func masterOfDeathETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "master_of_death_etb_surveil_2"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	milled := 0
	for i := 0; i < 2; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "master_of_death_surveil")
		seat.Turn.Milled++
		milled++
	}
	emit(gs, slug, "Master of Death", map[string]interface{}{
		"seat":   perm.Controller,
		"milled": milled,
	})
	if milled > 0 {
		emitPartial(gs, slug, "Master of Death",
			"surveil_2_keep_on_top_branch_requires_engine_surveil_decision_hook")
	}
	emitPartial(gs, "master_of_death", "Master of Death",
		"graveyard_upkeep_pay_1_life_return_to_hand_requires_graveyard_phase_trigger_dispatch")
}
