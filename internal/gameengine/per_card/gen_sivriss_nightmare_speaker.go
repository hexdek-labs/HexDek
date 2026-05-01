package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSivrissNightmareSpeaker wires Sivriss, Nightmare Speaker.
//
// Oracle text:
//
//   {T}, Sacrifice another creature or an artifact: For each opponent, you mill a card, then return that card from your graveyard to your hand unless that player pays 3 life. (To mill a card, put the top card of your library into your graveyard.)
//   Choose a Background (You can have a Background as a second commander.)
//
// Auto-generated activated ability handler.
func registerSivrissNightmareSpeaker(r *Registry) {
	r.OnActivated("Sivriss, Nightmare Speaker", sivrissNightmareSpeakerActivate)
}

func sivrissNightmareSpeakerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sivriss_nightmare_speaker_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[src.Controller]
	for i := 0; i < 1; i++ {
		if len(s.Library) == 0 { break }
		card := s.Library[0]
		gameengine.MoveCard(gs, card, src.Controller, "library", "graveyard", "mill")
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
