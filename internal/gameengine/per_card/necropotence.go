package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNecropotence wires up Necropotence.
//
// Oracle text:
//
//	Skip your draw step.
//	Whenever you discard a card, exile that card instead of putting it
//	into your graveyard.
//	Pay 1 life: Exile the top card of your library face down. Put that
//	card into your hand at the beginning of your next end step.
//
// THE most powerful card-advantage engine ever printed. cEDH decks
// that run Necro (mono-B and B-X builds) often rely on it to draw
// 10+ cards in a turn for 10+ life and then storm off via Ad Nauseam-
// like lines.
//
// Batch #3 scope:
//   - OnETB: stamp gs.Flags["necropotence_seat_N"] = 1 so the engine's
//     draw-step skip logic (phases.go / untap→upkeep→draw) can consult
//     it. Our current phase loop doesn't yet enforce this; the flag is
//     latent for downstream wiring.
//   - OnActivated(0, ...): pay 1 life, exile top card face down,
//     register delayed trigger to move it to hand at end step. We
//     physically move the card from Library → Exile and record
//     (owner, card) pairs to restore at end step. The "face down"
//     aspect is ignored in MVP (we have no hidden-info model).
//   - OnActivated(1, ...): discard-to-exile replacement is handled
//     by a replacement-effect-style wrapper. MVP: we stamp a flag;
//     discard sites consult it. Deferred enforcement.
func registerNecropotence(r *Registry) {
	r.OnETB("Necropotence", necropotenceETB)
	r.OnActivated("Necropotence", necropotenceActivate)
}

func necropotenceETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "necropotence_etb"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	seat := perm.Controller
	gs.Flags["necropotence_seat_"+intToStr(seat)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"skip_draw":   1,
		"discard_to":  "exile",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"skip_draw_step_and_discard_to_exile_flags_latent_phase_loop_not_wired")
}

// necroExiled tracks cards exiled face-down by Necropotence awaiting
// end-of-turn return to hand. Keyed by seat (one entry per activation).
// Package-level (not on Permanent) to avoid state.go touches.
var necroExiled = map[int][]*gameengine.Card{}

func necropotenceActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "necropotence_pay_one_life"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
		return
	}
	// Pay 1 life.
	s.Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "necropotence_activation_cost",
		},
	})
	// Exile top card face down.
	c := s.Library[0]
	gameengine.MoveCard(gs, c, seat, "library", "exile", "face-down-exile")
	c.FaceDown = true
	necroExiled[seat] = append(necroExiled[seat], c)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"exiled_card": c.DisplayName(),
		"life_after":  s.Life,
	})
	// Register delayed end-step trigger to return the exiled cards to
	// hand. We register one trigger per activation (a batch approach
	// would collapse them; per-activation is more faithful to the
	// oracle text, which says "that card" singular per activation).
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seat,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			necropotenceEndStep(gs, seat, c)
		},
	})
	// SBA-visible life loss for consistency with Ad Nauseam.
	_ = gs.CheckEnd()
}

// necropotenceEndStep is the delayed-trigger callback: move the
// previously-exiled card to the seat's hand.
func necropotenceEndStep(gs *gameengine.GameState, seat int, card *gameengine.Card) {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// Find & confirm in exile. We don't enforce "still in exile"
	// (i.e. if something else exiled-and-moved the card, we no-op).
	for _, c := range s.Exile {
		if c == card {
			gameengine.MoveCard(gs, card, seat, "exile", "hand", "return-from-exile")
			gs.LogEvent(gameengine.Event{
				Kind:   "return_to_hand",
				Seat:   seat,
				Target: seat,
				Source: "Necropotence",
				Details: map[string]interface{}{
					"card":   card.DisplayName(),
					"reason": "necropotence_end_step",
				},
			})
			return
		}
	}
	// Card was moved elsewhere by another effect — no-op.
}
