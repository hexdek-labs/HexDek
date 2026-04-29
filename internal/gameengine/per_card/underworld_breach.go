package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUnderworldBreach wires up Underworld Breach.
//
// Oracle text:
//
//	Each instant and sorcery card in your graveyard has escape. The
//	escape cost is equal to the card's mana cost plus exile three
//	other cards from your graveyard.
//	At the beginning of the end step, sacrifice Underworld Breach.
//
// Breach is a zone-cast-grant. Without a proper "cast from graveyard
// with escape" primitive in the engine, we model the CORE combo loop:
// given a pool of instants/sorceries in the graveyard, the controller
// can repeatedly cast them while Breach is out, exiling 3 other cards
// each time. The typical combo line is Breach + Brain Freeze /
// Grapeshot / Lion's Eye Diamond to mill/storm to a win.
//
// Batch #1 scope (MVP):
//   - Flag the controller's graveyard as "escape_enabled" (perm.Flags).
//     Downstream cast-spell logic can consult this flag when we build
//     zone-cast support (Phase 15+). Currently a NO-OP at the cast
//     resolution site — tests assert the flag is set.
//   - Register a delayed end-step trigger to sacrifice Breach.
//   - Does NOT actually allow cards to be cast from the graveyard yet
//     (engine doesn't have zone-cast primitive). Log partial.
//
// Why ship the stub: having the flag + sacrifice trigger means decks
// using Breach won't crash, and downstream work can plug in the zone-
// cast grant without refactoring call sites.
func registerUnderworldBreach(r *Registry) {
	r.OnETB("Underworld Breach", underworldBreachETB)
}

func underworldBreachETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "underworld_breach"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Flag the controller's graveyard as escape-enabled. A future
	// zone-cast pass will consult perm.Flags / gs.Flags.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["escape_grants_to_graveyard"] = 1
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["breach_active_seat_"+intToStr(seat)] = perm.Timestamp

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"zone_cast_from_graveyard_not_implemented_in_batch1")

	// Register the end-step sacrifice trigger. Delayed triggers fire at
	// phase/step boundaries; "end_of_turn" is the canonical key.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seat,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			// Sacrifice Breach via SacrificePermanent for proper zone-change
			// handling: replacement effects, dies/LTB triggers, commander redirect.
			gameengine.SacrificePermanent(gs, perm, "underworld_breach_end_step")
			// Remove the escape-grant flag.
			delete(gs.Flags, "breach_active_seat_"+intToStr(seat))
		},
	})
}
