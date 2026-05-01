package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUmbrisFearManifest wires Umbris, Fear Manifest.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Umbris gets +1/+1 for each card your opponents own in exile.
//	Whenever Umbris or another Nightmare or Horror you control enters,
//	target opponent exiles cards from the top of their library until they
//	exile a land card.
//
// Implementation:
//   - Static P/T buff is a Layer 7 continuous effect — emit partial.
//   - OnTrigger("permanent_etb"): fires when ANY permanent enters. Gate on
//     entering perm being Umbris itself OR a Nightmare/Horror creature
//     controlled by Umbris's controller. Pick the lowest-life living
//     opponent (deterministic, mirrors aragorn target policy) and exile
//     from top of their library until a land card is exiled.
//   - "Until they exile a land card" — lands are kept in exile alongside
//     the non-lands, matching the printed effect. The trigger is one-shot
//     per qualifying ETB.
func registerUmbrisFearManifest(r *Registry) {
	r.OnTrigger("Umbris, Fear Manifest", "permanent_etb", umbrisETBTrigger)
}

func umbrisETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "umbris_fear_manifest_exile_until_land"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entryPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if entryPerm == nil || entryPerm.Card == nil {
		return
	}
	// Fires on Umbris itself OR another Nightmare/Horror creature.
	if entryPerm != perm {
		if !cardHasType(entryPerm.Card, "creature") {
			return
		}
		if !cardHasType(entryPerm.Card, "nightmare") && !cardHasType(entryPerm.Card, "horror") {
			return
		}
	}

	// Pick a target opponent — lowest-life living opponent with cards left.
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if len(s.Library) == 0 {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target_opponent", map[string]interface{}{
			"trigger_source": entryPerm.Card.DisplayName(),
		})
		return
	}

	oppSeat := gs.Seats[target]
	exiledCount := 0
	landName := ""
	for len(oppSeat.Library) > 0 {
		top := oppSeat.Library[0]
		gameengine.MoveCard(gs, top, target, "library", "exile", "umbris_fear_manifest")
		exiledCount++
		if top != nil && cardHasType(top, "land") {
			landName = top.DisplayName()
			break
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"target":         target,
		"trigger_source": entryPerm.Card.DisplayName(),
		"exiled":         exiledCount,
		"land_hit":       landName,
	})
}
