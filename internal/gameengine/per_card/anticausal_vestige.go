package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnticausalVestige wires Anticausal Vestige (Muninn parser-gap
// rank ~144, Edge of Eternities warp Eldrazi).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{6}
//	Creature — Eldrazi
//	When this creature leaves the battlefield, draw a card, then you
//	may put a permanent card with mana value less than or equal to the
//	number of lands you control from your hand onto the battlefield
//	tapped.
//	Warp {4} (You may cast this card from your hand for its warp cost.
//	Exile this creature at the beginning of the next end step, then you
//	may cast it from exile on a later turn.)
//
// Implementation:
//   - OnTrigger("permanent_ltb"): when Anticausal Vestige itself leaves
//     the battlefield, draw a card, then drop the best hand permanent
//     (highest CMC ≤ landCount) onto the battlefield tapped via the
//     full ETB cascade.
//   - Warp (cast from hand at alt-cost, then exile at end step, then
//     recast from exile later) is a Foundations/Edge mechanic the
//     engine has not yet wired into the cast / cost / exile pipeline.
//     emitPartial documents that — the from-graveyard ETB hook still
//     gives us the value-side of the card in real games.
func registerAnticausalVestige(r *Registry) {
	r.OnTrigger("Anticausal Vestige", "permanent_ltb", anticausalVestigeLTB)
	r.OnETB("Anticausal Vestige", anticausalVestigeETB)
}

func anticausalVestigeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "anticausal_vestige_warp_partial"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"warp_alt_cost_and_eot_exile_pipeline_unwired_pending_warp_mechanic_support")
}

func anticausalVestigeLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anticausal_vestige_ltb_draw_cheat"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Step 1: draw a card.
	drew := false
	if len(seat.Library) > 0 {
		top := seat.Library[0]
		if top != nil {
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "anticausal_vestige_draw")
			drew = true
		}
	}
	// Step 2: count lands controlled.
	lands := 0
	for _, p := range seat.Battlefield {
		if p == nil {
			continue
		}
		if p.IsLand() {
			lands++
		}
	}
	// Step 3: pick best permanent card from hand with CMC ≤ lands.
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if cardHasType(c, "instant") || cardHasType(c, "sorcery") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > lands {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	cheated := ""
	if bestIdx >= 0 {
		card := seat.Hand[bestIdx]
		// createPermanent (called via enterBattlefieldWithETB) sweeps the
		// source from every private zone, so the manual hand splice is
		// redundant (Wave 2 final-audit cleanup).
		enterBattlefieldWithETB(gs, perm.Controller, card, true)
		cheated = card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"drew":    drew,
		"lands":   lands,
		"cheated": cheated,
		"cmc":     bestCMC,
	})
}
