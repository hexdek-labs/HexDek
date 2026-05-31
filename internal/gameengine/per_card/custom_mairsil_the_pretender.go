package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMairsilThePretenderCustom implements Mairsil's ETB cage-counter
// exile. The auto-generated stub leaves it as no-op.
//
// Oracle text:
//
//	When Mairsil enters, you may exile an artifact or creature card
//	from your hand or graveyard and put a cage counter on it.
//	Mairsil has all activated abilities of all cards you own in exile
//	with cage counters on them. You may activate each of those
//	abilities only once each turn.
//
// Strategy: pick the highest-CMC artifact/creature from hand first
// (resource-conservation: exiling from graveyard is free, but the
// graveyard might be empty), fall back to graveyard. Tag the exiled
// card with a "cage_counter" type marker so the activation copy logic
// (engine territory) can identify it. The "Mairsil has all activated
// abilities" static is engine-side and emitted as a partial.
func registerMairsilThePretenderCustom(r *Registry) {
	r.OnETB("Mairsil, the Pretender", mairsilETB)
}

func mairsilETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mairsil_etb_cage"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	pickFromHand := func() (*gameengine.Card, int) {
		var best *gameengine.Card
		bestIdx := -1
		bestCMC := -1
		for i, c := range seat.Hand {
			if c == nil {
				continue
			}
			if !cardHasType(c, "artifact") && !cardHasType(c, "creature") {
				continue
			}
			if cmc := cardCMC(c); cmc > bestCMC {
				best = c
				bestIdx = i
				bestCMC = cmc
			}
		}
		return best, bestIdx
	}
	pickFromGraveyard := func() (*gameengine.Card, int) {
		var best *gameengine.Card
		bestIdx := -1
		bestCMC := -1
		for i, c := range seat.Graveyard {
			if c == nil {
				continue
			}
			if !cardHasType(c, "artifact") && !cardHasType(c, "creature") {
				continue
			}
			if cmc := cardCMC(c); cmc > bestCMC {
				best = c
				bestIdx = i
				bestCMC = cmc
			}
		}
		return best, bestIdx
	}
	// Prefer hand over graveyard for cage exile (graveyard is more
	// flexibly recurrable; once caged it can't be regrowed).
	pick, idx := pickFromHand()
	source := "hand"
	if pick == nil {
		pick, idx = pickFromGraveyard()
		source = "graveyard"
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"note": "no_legal_exile_target",
		})
		return
	}
	// Mark as caged so observers can identify it. Append the cage_counter
	// type tag BEFORE the zone move so card_exiled observers (Wave 2
	// MoveCard fires FireCardTrigger("card_exiled", ...)) see the caged
	// state.
	pick.Types = append(pick.Types, "cage_counter")
	_ = idx
	// Wave 2 multi-step migration: route through MoveCard so §614
	// replacements (Rest in Peace / Leyline of the Void on graveyard
	// source; redirection effects on hand source) + §903.9b commander
	// redirect + card_exiled / zone_change observers all fire. Pre-r60
	// shape direct-spliced + manual exile append, bypassing all of the
	// above.
	gameengine.MoveCard(gs, pick, perm.Controller, source, "exile", "mairsil_cage")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"caged":  pick.DisplayName(),
		"source": source,
	})
	emitPartial(gs, "mairsil_grant_activations", perm.Card.DisplayName(),
		"copying activated abilities from caged exile cards needs engine-side ability-grant hook")
}
