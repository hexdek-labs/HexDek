package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAngelsGrace wires up Angel's Grace.
//
// Oracle text:
//
//	Cast this spell only if you could cast a sorcery.
//	[Split Second]
//	Until end of turn, your life total can't change, and damage
//	doesn't cause you to lose the game.
//
// (Text varies by printing; the canonical wording used in cEDH decks
// is "You can't lose the game this turn and your opponents can't win
// the game this turn. Until end of turn, damage that would reduce your
// life total to less than 1 reduces it to 1 instead.")
//
// cEDH pairing: Angel's Grace + Ad Nauseam. Grace keeps us alive
// through Ad Nauseam's life cost even to 0-or-less, enabling the
// storm storm kill. Without Grace, Ad Nauseam self-caps at ~1 life.
//
// Batch #2 scope:
//   - OnResolve: stamp gs.Flags["angels_grace_eot_seat_N"] = 1 so that
//     Ad Nauseam's resolve handler (see ad_nauseam.go) removes its
//     self-preservation stop-condition. Register a delayed end-step
//     trigger that clears the flag.
//   - Does NOT implement the actual "can't lose + damage reduces to 1"
//     replacement (would need to intercept would_lose_game +
//     would_take_damage replacements). Log partial.
//
// The Ad Nauseam synergy — the primary reason cEDH decks pack Grace —
// IS implemented: Ad Nauseam reads the flag and keeps flipping.
func registerAngelsGrace(r *Registry) {
	r.OnResolve("Angel's Grace", angelsGraceResolve)
}

func angelsGraceResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "angels_grace_eot"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	key := "angels_grace_eot_seat_" + intToStr(seat)
	gs.Flags[key] = 1
	gs.LogEvent(gameengine.Event{
		Kind:   "protection_granted",
		Seat:   seat,
		Target: seat,
		Source: item.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "614",
			"effect": "cant_lose_the_game_until_eot",
		},
	})
	// End-of-turn cleanup.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seat,
		SourceCardName: item.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			delete(gs.Flags, key)
			gs.LogEvent(gameengine.Event{
				Kind:   "protection_ended",
				Seat:   seat,
				Target: seat,
				Source: "Angel's Grace",
				Details: map[string]interface{}{
					"rule":   "614",
					"reason": "end_of_turn",
				},
			})
		},
	})
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, item.Card.DisplayName(),
		"cant_lose_and_damage_to_one_replacement_not_fully_wired_only_ad_nauseam_integration")
}
