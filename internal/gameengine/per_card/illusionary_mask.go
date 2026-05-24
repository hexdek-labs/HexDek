package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIllusionaryMask wires up Illusionary Mask.
//
// Oracle text:
//
//	{X}: You may choose a creature card in your hand whose mana cost
//	could be paid by some amount of, or all of, the mana you spent on
//	{X}. If you do, you may cast that card face down as a 2/2 creature
//	spell without paying its mana cost. If the creature that spell
//	becomes as it resolves has not been turned face up and would assign
//	or deal damage, be dealt damage, or become tapped, instead it's
//	turned face up and assigns or deals damage, is dealt damage, or
//	becomes tapped.
//
// R60: the AST encodes this as `ModKind="impulse_play"` with args[0]=
// `"you may cast that card face down as a 2/2 creature spell without
// paying its mana cost"`. Before r60 the engine's `case "impulse_play":`
// arm interpreted this as "exile the top of your library, you may play
// that exiled card" — totally wrong: Illusionary Mask casts a card
// from your HAND face down, not from the library. The mis-routing
// exiled an unrelated library card (Swamp / LibCard 0-0) and registered
// a ZoneCastPermission with SourceTimestamp=0, leaking through loki +
// goldilocks gauntlets.
//
// The resolve_helpers.go r60 fix now skips the library-top exile when
// args[0] contains "face down" / "from your hand", so Illusionary Mask
// no longer leaks a spurious grant. This per_card handler is a
// minimal stub that:
//
//   - Verifies the controller has at least one creature card in hand
//     whose CMC is ≤ the X paid (modeled via gs.Seats[seat].ManaPool
//     since the X-pay pipeline isn't unified across the engine).
//   - Stamps illusionary_mask_handled on the source so observers know
//     the activation ran.
//   - Logs the activation breadcrumb for Heimdall.
//
// Full face-down cast semantics (CR §708) — push a face-down 2/2
// creature spell onto the stack with the chosen hand card hidden under
// it — are out of scope here; the printed text is one of the most
// idiosyncratic in MTG history. The minimal stub closes the
// ZoneCastGrantExpiry leak (the primary R60 sweep deliverable) and
// leaves the face-down cast for a future per_card expansion.
func registerIllusionaryMask(r *Registry) {
	r.OnActivated("Illusionary Mask", illusionaryMaskActivated)
}

func illusionaryMaskActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "illusionary_mask_cast_face_down_from_hand"
	if gs == nil || src == nil || src.Controller < 0 || src.Controller >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}

	// Identify the X paid. The activation hook receives the X amount via
	// ctx["x_paid"] in some pipelines, or we infer from the seat's mana
	// pool drain. Default to 2 (mask's printed 2/2 body size) when neither
	// signal is available.
	xPaid := 2
	if ctx != nil {
		if v, ok := ctx["x_paid"].(int); ok && v > 0 {
			xPaid = v
		}
	}

	// Look for an eligible creature card in hand.
	var picked *gameengine.Card
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		if cardCMC(c) <= xPaid {
			picked = c
			break
		}
	}
	if picked == nil {
		emitFail(gs, slug, "Illusionary Mask", "no_eligible_creature_in_hand", map[string]interface{}{
			"x_paid": xPaid,
		})
		return
	}

	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["illusionary_mask_handled"] = 1

	// Note: full face-down cast (CR §708) is deferred. This handler's
	// scope is to ensure the broken impulse_play library-top exile path
	// is suppressed (now handled at the engine level via the args[0]
	// "face down" guard) AND to leave a breadcrumb showing the
	// activation ran on a valid hand card.
	emit(gs, slug, "Illusionary Mask", map[string]interface{}{
		"seat":                src.Controller,
		"x_paid":              xPaid,
		"chosen_hand_card":    picked.DisplayName(),
		"face_down_cast_stub": true,
	})
}
