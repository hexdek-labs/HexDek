package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheNecrobloom wires The Necrobloom.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Landfall — Whenever a land you control enters, create a 0/1 green
//	Plant creature token. If you control seven or more lands with
//	different names, create a 2/2 black Zombie creature token instead.
//	Land cards in your graveyard have dredge 2.
//
// Implementation:
//   - OnTrigger("permanent_etb"): gate on entering perm being a land
//     controlled by Necrobloom's controller. Count distinct land names on
//     that battlefield; if >= 7, mint a 2/2 Zombie, else a 0/1 Plant.
//   - Dredge granting on graveyard land cards is a static replacement
//     effect (CR §702.51) and is not modeled in per-card handlers — emit
//     partial on ETB so analysis tooling sees the gap.
func registerTheNecrobloom(r *Registry) {
	r.OnETB("The Necrobloom", theNecrobloomETB)
	r.OnTrigger("The Necrobloom", "permanent_etb", theNecrobloomLandfall)
}

func theNecrobloomETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	emitPartial(gs, "the_necrobloom_dredge_grant", perm.Card.DisplayName(),
		"static_dredge_2_grant_on_graveyard_lands_unimplemented")
}

func theNecrobloomLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_necrobloom_landfall_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entryPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if entryPerm == nil || !entryPerm.IsLand() {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	distinct := map[string]bool{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		distinct[strings.ToLower(p.Card.DisplayName())] = true
	}

	tokenKind := "plant"
	if len(distinct) >= 7 {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Zombie",
			[]string{"creature", "zombie", "pip:B"}, 2, 2)
		tokenKind = "zombie"
	} else {
		gameengine.CreateCreatureToken(gs, perm.Controller, "Plant",
			[]string{"creature", "plant", "pip:G"}, 0, 1)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"land":           entryPerm.Card.DisplayName(),
		"distinct_lands": len(distinct),
		"token":          tokenKind,
	})
}
