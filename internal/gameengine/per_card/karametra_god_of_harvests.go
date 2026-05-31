package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Karametra, God of Harvests — {3}{G}{W} 6/7 Legendary Enchantment
// Creature — God.
//
//   Indestructible
//   As long as your devotion to green and white is less than seven,
//   Karametra isn't a creature.
//   Whenever you cast a creature spell, you may search your library for
//   a Forest or Plains card, put it onto the battlefield tapped, then
//   shuffle.
//
// Cast trigger: scoped to caster_seat == controller. "May" is greedy yes
// (every Karametra line keeps ramping). The devotion-gates-creature-status
// modal is left to the existing god-creature engine machinery (Heliod /
// Erebos / Thassa pattern); Indestructible is keyword-pipeline.

func init() {
	registerKarametraGodOfHarvests(Global())
	AddResetHook(registerKarametraGodOfHarvests)
}

func registerKarametraGodOfHarvests(r *Registry) {
	r.OnTrigger("Karametra, God of Harvests", "creature_spell_cast", karametraCreatureCast)
}

func karametraCreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "karametra_ramp_forest_or_plains"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	land := pickFirstForestOrPlains(s.Library)
	if land == nil {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "no_forest_or_plains", map[string]interface{}{"seat": seat})
		return
	}
	gameengine.MoveCard(gs, land, seat, "library", "battlefield_tapped", "karametra_ramp")
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":  []string{land.DisplayName()},
			"reason": slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
		"land": land.DisplayName(),
	})
}

func pickFirstForestOrPlains(library []*gameengine.Card) *gameengine.Card {
	for _, c := range library {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		if name == "Forest" || name == "Plains" ||
			name == "Snow-Covered Forest" || name == "Snow-Covered Plains" {
			return c
		}
		if cardHasType(c, "forest") || cardHasType(c, "plains") {
			return c
		}
	}
	return nil
}
