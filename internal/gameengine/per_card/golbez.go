package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGolbez wires Golbez, Crystal Collector.
//
// Oracle text:
//
//	Whenever you cast a spell from your graveyard or from exile, create
//	a Treasure token.
//	{2}{B}: Exile target card from a graveyard. You may cast it from
//	exile.
//
// Implementation:
//   - OnTrigger("spell_cast"): gate on caster_seat == controller and
//     ctx["cast_zone"] in {graveyard, exile}. Mint a Treasure.
//   - OnActivated(0): pick the highest-CMC nonland card from any
//     graveyard (prefer opponents' to disrupt them, fall back to own
//     graveyard for self-recursion). Move to exile and register a
//     ZoneCastPermission so the controller may cast it from exile for
//     its normal mana cost. The {2}{B} cost is paid by the engine at
//     activation time.
func registerGolbez(r *Registry) {
	r.OnTrigger("Golbez, Crystal Collector", "spell_cast", golbezSpellCast)
	r.OnActivated("Golbez, Crystal Collector", golbezActivate)
}

func golbezSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "golbez_treasure_on_recast"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	zone, _ := ctx["cast_zone"].(string)
	if zone != gameengine.ZoneGraveyard && zone != gameengine.ZoneExile {
		return
	}
	// Don't trigger off Golbez's own command-zone cast or off the cast
	// that Golbez himself enabled-but-this-is-still-the-same-event;
	// the spell is identified by ctx["card"] which is the casting spell,
	// distinct from Golbez.
	if card, _ := ctx["card"].(*gameengine.Card); card != nil && card == perm.Card {
		return
	}
	gameengine.CreateTreasureToken(gs, perm.Controller)
	spellName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		spellName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_zone":  zone,
		"spell_name": spellName,
		"token":      "Treasure Token",
	})
}

func golbezActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "golbez_exile_target_card_from_grave"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}

	target, ownerSeat := golbezPickGraveTarget(gs, src.Controller, ctx)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_grave_target", nil)
		return
	}

	gameengine.MoveCard(gs, target, ownerSeat, "graveyard", "exile", "golbez_exile_target")

	// "You may cast it from exile" — controller of Golbez may cast for
	// its normal mana cost. Lands can't be cast as spells; we still
	// exile, but skip the grant for non-castable cards.
	granted := false
	if !cardHasType(target, "land") {
		gPerm := &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneExile,
			Keyword:           "golbez_exile_cast",
			ManaCost:          -1,
			ExileOnResolve:    false,
			RequireController: src.Controller,
			SourceName:        "Golbez, Crystal Collector",
		}
		gameengine.RegisterZoneCastGrant(gs, target, gPerm)
		granted = true
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            src.Controller,
		"exiled_card":     target.DisplayName(),
		"from_owner_seat": ownerSeat,
		"cast_grant":      granted,
	})
}

// golbezPickGraveTarget chooses the best graveyard card for Golbez to
// exile. Priority:
//  1. ctx["target_card"] if supplied.
//  2. Highest-CMC nonland card across all graveyards. Prefer opponents'
//     graveyards to disrupt; tiebreak by opponent ownership over own.
//
// Returns (card, ownerSeat). Returns (nil, -1) if no graveyard card
// exists.
func golbezPickGraveTarget(gs *gameengine.GameState, controller int, ctx map[string]interface{}) (*gameengine.Card, int) {
	if v, ok := ctx["target_card"].(*gameengine.Card); ok && v != nil {
		owner := v.Owner
		if owner < 0 || owner >= len(gs.Seats) {
			owner = controller
		}
		return v, owner
	}
	var bestCard *gameengine.Card
	bestOwner := -1
	bestCMC := -1
	bestIsOpp := false
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		isOpp := i != controller
		for _, c := range s.Graveyard {
			if c == nil {
				continue
			}
			if cardHasType(c, "land") {
				continue
			}
			cmc := cardCMC(c)
			better := false
			switch {
			case bestCard == nil:
				better = true
			case isOpp && !bestIsOpp:
				better = true
			case isOpp == bestIsOpp && cmc > bestCMC:
				better = true
			}
			if better {
				bestCard = c
				bestOwner = i
				bestCMC = cmc
				bestIsOpp = isOpp
			}
		}
	}
	return bestCard, bestOwner
}
