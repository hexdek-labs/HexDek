package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJodahTheUnifier wires Jodah, the Unifier.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary creatures you control get +X/+X, where X is the number of
//	legendary creatures you control.
//	Whenever you cast a legendary spell from your hand, exile cards from
//	the top of your library until you exile a legendary nonland card with
//	lesser mana value. You may cast that card without paying its mana
//	cost. Put the rest on the bottom of your library in a random order.
//
// Implementation:
//   - Static P/T buff is a Layer 7 continuous effect — not modeled in the
//     per-card handler. Emitted as partial on ETB so analysis tooling sees
//     the gap.
//   - Cascade-like trigger on "spell_cast" gated on caster_seat ==
//     controller, cast_zone == "hand", and the cast card being legendary.
//     Mirrors gameengine/cascade.go's procedure but adds the "legendary
//     nonland" filter on the exiled-to-cast card.
//   - For permanents found, route through enterBattlefieldWithETB so the
//     full ETB cascade fires (parity with Etali's free-cast shortcut).
//     Instants/sorceries fall back to emitPartial — no free-cast resolver
//     for non-permanents in this engine.
func registerJodahTheUnifier(r *Registry) {
	r.OnETB("Jodah, the Unifier", jodahETB)
	r.OnTrigger("Jodah, the Unifier", "spell_cast", jodahLegendaryCast)
}

func jodahETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	emitPartial(gs, "jodah_legendary_anthem", perm.Card.DisplayName(),
		"static_layer_7_pt_buff_unimplemented")
}

func jodahLegendaryCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jodah_the_unifier_legendary_cascade"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	castZone, _ := ctx["cast_zone"].(string)
	if castZone != "" && castZone != "hand" {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "legendary") {
		return
	}
	// Jodah herself doesn't trigger her own ability (the trigger fires from
	// the battlefield, but Jodah was on the stack at cast time — defensive).
	if card == perm.Card {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	spellCMC := card.CMC

	var exiled []*gameengine.Card
	var found *gameengine.Card

	for len(seat.Library) > 0 {
		top := seat.Library[0]
		gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "jodah_reveal")
		exiled = append(exiled, top)
		if top == nil {
			continue
		}
		if cardHasType(top, "land") {
			continue
		}
		if !cardHasType(top, "legendary") {
			continue
		}
		if top.CMC >= spellCMC {
			continue
		}
		found = top
		break
	}

	castName := ""
	castedPartial := false
	if found != nil {
		// Remove from the exile pile — it goes to the battlefield (or as
		// partial: stays in exile if we can't cast it).
		for i, c := range exiled {
			if c == found {
				exiled = append(exiled[:i], exiled[i+1:]...)
				break
			}
		}
		castName = found.DisplayName()
		switch {
		case cardHasType(found, "creature"),
			cardHasType(found, "artifact"),
			cardHasType(found, "enchantment"),
			cardHasType(found, "planeswalker"),
			cardHasType(found, "battle"):
			gameengine.MoveCard(gs, found, perm.Controller, "exile", "battlefield", "jodah_free_cast")
			enterBattlefieldWithETB(gs, perm.Controller, found, false)
		case cardHasType(found, "instant"), cardHasType(found, "sorcery"):
			// Free-cast resolution shortcut — leave in exile and flag.
			castedPartial = true
		default:
			castedPartial = true
		}
	}

	// Shuffle remaining exiled cards onto bottom of library in random order.
	if len(exiled) > 1 && gs.Rng != nil {
		gs.Rng.Shuffle(len(exiled), func(i, j int) {
			exiled[i], exiled[j] = exiled[j], exiled[i]
		})
	}
	for _, c := range exiled {
		gameengine.MoveCard(gs, c, perm.Controller, "exile", "library_bottom", "jodah_miss_return")
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"cast_spell":   card.DisplayName(),
		"spell_cmc":    spellCMC,
		"hit":          castName,
		"exiled_count": len(exiled) + boolToInt(found != nil),
	})
	if castedPartial {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"instant_or_sorcery_free_cast_resolution_shortcut_unimplemented")
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
