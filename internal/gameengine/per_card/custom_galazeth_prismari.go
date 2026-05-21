package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGalazethPrismariCustom wires the ETB Treasure for Galazeth
// Prismari. The auto-generated stub registerGalazethPrismari in
// gen_galazeth_prismari.go remains an inert breadcrumb.
//
// Oracle text (Strixhaven, {2}{U}{R}):
//
//	Flying
//	When Galazeth Prismari enters, create a Treasure token.
//	{T}: Add one mana of any color. Spend this mana only to cast an
//	instant or sorcery spell.
//
// Implementation:
//   - OnETB: create one Treasure token for the controller.
//   - {T}: Add one mana of any color (abilityIdx 0). Default to {U} —
//     Galazeth is a Prismari (UR) commander and U is the most common
//     instant-cast color. Tap Galazeth, then route mana through
//     AddManaFromPermanent so Kinnan / mana-doubler triggers fire.
//     Stamp seat.Flags["galazeth_instant_sorcery_mana"]++ as an
//     attribution breadcrumb for the typed-mana scanner that the
//     untyped ManaPool can't fully model.
//   - Flying — AST keyword pipeline.
func registerGalazethPrismariCustom(r *Registry) {
	r.OnETB("Galazeth Prismari", galazethPrismariCustomETB)
	r.OnActivated("Galazeth Prismari", galazethPrismariActivate)
}

func galazethPrismariCustomETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "galazeth_prismari_etb_treasure"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	gameengine.CreateTreasureToken(gs, seatIdx)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"tokens": 1,
	})
}

func galazethPrismariActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "galazeth_prismari_tap_any_color"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	if src.SummoningSick {
		emitFail(gs, slug, src.Card.DisplayName(), "summoning_sick", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	src.Tapped = true
	// Default produced color: U (Prismari blue half — most common for
	// instant/sorcery casting). Caller can override via ctx["color"].
	color := "U"
	if ctx != nil {
		if c, ok := ctx["color"].(string); ok && c != "" {
			color = c
		}
	}
	gameengine.AddManaFromPermanent(gs, seat, src, color, 1)
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["galazeth_instant_sorcery_mana"]++
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  src.Controller,
		"color": color,
		"added": 1,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"instant_or_sorcery_only_restriction_tracked_via_seat_flag_untyped_pool")
}
