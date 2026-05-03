package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagusLuceaKane wires Magus Lucea Kane.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	At the beginning of your upkeep, scry 1.
//	{T}: Add {C}{C}. If this mana is spent on a spell with {X} in its
//	mana cost, copy that spell and you may choose new targets for the copy.
//
// Implementation:
//   - OnTrigger("upkeep_controller"): scry 1. Heuristic for ChooseScry:
//     delegate to the Hat via gameengine.Scry so the seat's Hat makes the
//     keep/bottom decision per §701.18. If the seat has no Hat, Scry's
//     default (all on top) is correct.
//   - OnActivated(abilityIdx 0): {T}: tap Magus Lucea Kane (paying the
//     cost), then add {C}{C} via AddManaFromPermanent so Kinnan and other
//     mana-augmenters fire correctly. The "spend only on X-spell" tracking
//     would require a per-mana-spend callback the engine does not have
//     today — we tag the seat flag "mlk_cc_floating" to signal that two
//     colorless mana from this source are available, but the engine cannot
//     enforce the restriction. emitPartial flags the gap.
//   - X-spell copy effect: copying the spell on the stack when the
//     restricted mana is actually spent requires a spell-cast hook with
//     X-cost detection and mana-source attribution — neither surface
//     exists today. emitPartial records the gap for Muninn tracking.
func registerMagusLuceaKane(r *Registry) {
	r.OnTrigger("Magus Lucea Kane", "upkeep_controller", magusLuceaKaneUpkeep)
	r.OnActivated("Magus Lucea Kane", magusLuceaKaneActivate)
}

// magusLuceaKaneUpkeep fires at the beginning of the controller's upkeep
// and performs scry 1 (§701.18).
func magusLuceaKaneUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "magus_lucea_kane_scry"
	if gs == nil || perm == nil || ctx == nil {
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
	if len(seat.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "library_empty", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	// Delegate scry 1 to the engine's Scry surface, which consults the
	// seat's Hat (ChooseScry) and reorders the library in place.
	gameengine.Scry(gs, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"count": 1,
	})
}

// magusLuceaKaneActivate handles the {T}: Add {C}{C} activated ability
// (abilityIdx 0). The X-spell copy rider is partially implemented.
func magusLuceaKaneActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	const slug = "magus_lucea_kane_tap_add_cc"

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Cost: tap Magus Lucea Kane. Skip if already tapped or summoning sick
	// (§602.1 — activated abilities with {T} in the cost require the
	// permanent to be untapped and, for creatures, not summoning sick
	// unless it has haste).
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	if src.SummoningSick {
		emitFail(gs, slug, src.Card.DisplayName(), "summoning_sick", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	src.Tapped = true

	// Effect: Add {C}{C}. Use AddManaFromPermanent so Kinnan and other
	// "whenever you tap a nonland permanent for mana" effects trigger.
	gameengine.AddManaFromPermanent(gs, s, src, "C", 2)

	// Tag the seat so downstream observers know two colorless were produced
	// from Magus Lucea Kane's ability. The copy-on-X-spend enforcement is
	// not implemented — see emitPartial below.
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["mlk_cc_floating"] += 2

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"added":    2,
		"color":    "C",
		"new_pool": s.ManaPool,
	})

	// The copy-a-spell-with-X-cost clause requires tracking which mana in
	// the pool came from this activation and hooking the spend path — the
	// engine has no per-mana-source attribution surface today.
	emitPartial(gs, slug, src.Card.DisplayName(),
		"x_spell_copy_on_mlk_mana_spend_not_implemented")
}
