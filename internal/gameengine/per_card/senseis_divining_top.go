package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSenseisDiviningTop wires up Sensei's Divining Top.
//
// Oracle text:
//
//	{T}: Look at the top three cards of your library, then put them
//	back in any order.
//	{T}: Draw a card, then put Sensei's Divining Top on top of its
//	owner's library.
//	{1}: Return Sensei's Divining Top to its owner's hand.
//
// The most universal "enable scry" card in cEDH. Paired with any
// shuffle effect (fetchlands, tutor, Brainstorm), Top offers 3-deep
// knowledge refresh every activation. Not a combo piece in modern
// cEDH (combo'd top lists are too expensive to go over 4 mana a turn)
// but played in every control shell.
//
// Batch #2 scope:
//   - OnActivated(0, ...) — "look + reorder" mode. MVP: take top 3
//     cards, re-sort in a way that's a no-op in the observation model
//     (library is already deterministic). We log the activation for
//     credit assignment but don't physically shuffle.
//   - OnActivated(1, ...) — "draw + place on top" mode. Draw 1 card,
//     then remove Top from battlefield and put at top of its owner's
//     library.
//   - OnActivated(2, ...) — "{1}: return to hand" mode.
//
// Tap enforcement is lax for activation 0 and 1 (caller pays tap).
// The return-to-hand mode (2) doesn't need a tap.
func registerSenseisDiviningTop(r *Registry) {
	r.OnActivated("Sensei's Divining Top", senseisDiviningTopActivate)
}

func senseisDiviningTopActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	switch abilityIdx {
	case 0:
		// Look + reorder top 3. MVP: shuffle the top 3 using gs.Rng
		// (deterministic under the game's seed). The real policy would
		// be exposed to Hat.
		const slug = "senseis_top_look_three"
		n := 3
		if n > len(s.Library) {
			n = len(s.Library)
		}
		if n > 1 {
			rng := gs.Rng
			if rng == nil {
				rng = rand.New(rand.NewSource(1))
			}
			top := append([]*gameengine.Card(nil), s.Library[:n]...)
			rng.Shuffle(len(top), func(i, j int) { top[i], top[j] = top[j], top[i] })
			copy(s.Library[:n], top)
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"viewed":      n,
			"library_len": len(s.Library),
		})
	case 1:
		// Draw + place on top of library via BouncePermanent (dest=library_top)
		// for proper zone-change handling: replacement effects, LTB triggers,
		// commander redirect.
		const slug = "senseis_top_draw_and_self_top"
		drawOne(gs, seat, src.Card.DisplayName())
		if !gameengine.BouncePermanent(gs, src, src, "library_top") {
			emitFail(gs, slug, src.Card.DisplayName(), "not_on_battlefield", nil)
			return
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":  seat,
			"owner": src.Owner,
		})
	case 2:
		// {1}: return Top from battlefield to owner's hand via BouncePermanent
		// for proper zone-change handling.
		const slug = "senseis_top_bounce_self"
		if !gameengine.BouncePermanent(gs, src, src, "hand") {
			emitFail(gs, slug, src.Card.DisplayName(), "not_on_battlefield", nil)
			return
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":  seat,
			"owner": src.Owner,
		})
	}
}
