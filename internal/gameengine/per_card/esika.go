package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEsika wires Esika, God of the Tree // The Prismatic Bridge (DFC).
//
// Front face — Esika, God of the Tree (Legendary Creature — God):
//
//	{T}: Add one mana of any color.
//	Other legendary creatures you control have "{T}: Add one mana of
//	any color."
//
// Back face — The Prismatic Bridge (Legendary Enchantment):
//
//	At the beginning of your upkeep, reveal cards from the top of your
//	library until you reveal a creature or planeswalker card. Put that
//	card onto the battlefield and shuffle the rest into your library.
//
// Implementation:
//   - Front face static abilities (own tap-for-any + grant tap-for-any
//     to other legendary creatures): emitPartial. The mana production
//     is normally surfaced via AST mana grants; granting an activated
//     ability to other permanents requires a continuous-effect
//     registration that the engine doesn't model for arbitrary
//     handlers.
//   - Back face upkeep: when the active player is the controller AND
//     the permanent is transformed (Bridge face), reveal from top of
//     library until creature/planeswalker, ETB it on the controller's
//     battlefield, then shuffle the non-creature/PW reveals back in.
//
// DFC dispatch: register all three name forms (full DFC, front, back)
// since perm.Card.Name swaps to the active face after TransformPermanent
// (CR §712.3) and the registry's " // " split fallback only catches
// pre-transform DisplayNames.
func registerEsika(r *Registry) {
	r.OnETB("Esika, God of the Tree // The Prismatic Bridge", esikaETB)
	r.OnETB("Esika, God of the Tree", esikaETB)
	r.OnETB("The Prismatic Bridge", esikaETB)
	r.OnTrigger("Esika, God of the Tree // The Prismatic Bridge", "upkeep_controller", esikaBridgeUpkeep)
	r.OnTrigger("Esika, God of the Tree", "upkeep_controller", esikaBridgeUpkeep)
	r.OnTrigger("The Prismatic Bridge", "upkeep_controller", esikaBridgeUpkeep)
}

func esikaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "esika_god_of_the_tree_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"front_face_other_legendaries_tap_for_any_color_grant_unimplemented")
}

func esikaBridgeUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "prismatic_bridge_upkeep_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Back-face only — the front face (god creature) has no upkeep
	// trigger. Esika is a modal DFC, so the back face may be active
	// either via TransformPermanent (Transformed=true) or by being cast
	// as the back face directly. Gating on the active face's type is
	// robust against both paths.
	if perm.IsCreature() || !perm.IsEnchantment() {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	var revealed []*gameengine.Card
	var hit *gameengine.Card
	for len(seat.Library) > 0 {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		if cardHasType(top, "creature") || cardHasType(top, "planeswalker") {
			seat.Library = seat.Library[1:]
			hit = top
			break
		}
		seat.Library = seat.Library[1:]
		revealed = append(revealed, top)
	}

	entered := ""
	if hit != nil {
		// Bypass library/zone bookkeeping (we already detached above) and
		// drop the card on the battlefield with full ETB semantics.
		hit.Owner = perm.Controller
		ent := enterBattlefieldWithETB(gs, perm.Controller, hit, false)
		if ent != nil {
			entered = hit.DisplayName()
		}
	}

	// Shuffle the non-hits back into library.
	if len(revealed) > 0 {
		seat.Library = append(seat.Library, revealed...)
		shuffleLibraryPerCard(gs, perm.Controller)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"revealed":          len(revealed),
		"entered":           entered,
		"library_remaining": len(seat.Library),
	})
}
