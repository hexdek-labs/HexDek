package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGreenSunsZenith wires Green Sun's Zenith.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Green%20Sun%27s%20Zenith):
//
//	Search your library for a green creature card with mana value X or
//	less, put it onto the battlefield, then shuffle. Shuffle Green Sun's
//	Zenith into its owner's library.
//
// {X}{G} Instant. The premier mono-G creature tutor — finds Dryad
// Arbor at X=0 for a one-mana ramp, finds combo pieces at X=N. The
// shuffle-into-library clause keeps it re-castable across the game,
// which is what separates it from Chord/Worldly — repeated tutor uses
// rather than a one-shot.
//
// Implementation:
//   - OnResolve. ChosenX on the StackItem carries the X value the
//     caster paid. Fall back to 0 when unset (test fixtures, or a
//     synthesized item) — X=0 finds Dryad Arbor / nothing.
//   - Picker: highest-CMC GREEN creature with CMC <= X. "Green" is
//     detected via the "pip:G" tag in Card.Types (per_card test
//     fixture convention) or color identity if present; CMC via
//     cardCMC.
//   - MoveCard library→battlefield with ETB; shuffle library.
//   - Shuffle GSZ into library: manually move item.Card from stack
//     to library and shuffle. The engine's post-resolve MoveCard
//     (stack→graveyard) silently no-ops because removeCardFromZone
//     finds nothing in stack — the card has already moved.
func registerGreenSunsZenith(r *Registry) {
	r.OnResolve("Green Sun's Zenith", greenSunsZenithResolve)
}

func greenSunsZenithResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "green_suns_zenith"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	x := item.ChosenX
	if x < 0 {
		x = 0
	}

	// Pick highest-CMC green creature with CMC <= X.
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		if !cardIsGreen(c) {
			continue
		}
		cmc := cardCMC(c)
		if cmc > x {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		card := s.Library[bestIdx]
		s.Library = append(s.Library[:bestIdx], s.Library[bestIdx+1:]...)
		enterBattlefieldWithETB(gs, seat, card, false)
		emit(gs, slug, "Green Sun's Zenith", map[string]interface{}{
			"seat":       seat,
			"x":          x,
			"into_play":  card.DisplayName(),
			"target_cmc": bestCMC,
		})
	} else {
		emitFail(gs, slug, "Green Sun's Zenith", "no_legal_target", map[string]interface{}{
			"x":        x,
			"lib_size": len(s.Library),
		})
	}

	// Shuffle GSZ itself into its owner's library. Move from stack to
	// library directly; the post-resolve MoveCard(stack→graveyard) in
	// stack.go silently no-ops since the card has already left stack.
	if item.Card != nil {
		ownerSeat := item.Card.Owner
		if ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
			gameengine.MoveCard(gs, item.Card, ownerSeat, "stack", "library", "gsz_shuffle_into_library")
			shuffleLibraryPerCard(gs, ownerSeat)
		}
	}

	// Shuffle the searched library too (printed "then shuffle").
	shuffleLibraryPerCard(gs, seat)
}

// cardIsGreen reports whether a card is green. Per_card layer doesn't
// see ColorIdentity, so we check for "pip:G" type tags or "green" /
// "color:G" tags that test fixtures use.
func cardIsGreen(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		switch t {
		case "pip:G", "color:G", "green":
			return true
		}
	}
	return false
}
