package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Cantrips — cheap card-selection spells.
//
// These are the backbone of cEDH: low-cost spells that draw cards and/or
// manipulate the top of the library. They need real Scry/Surveil/draw
// infrastructure to work correctly.
// ============================================================================

// --- Brainstorm ---
//
// Oracle text:
//   Draw three cards, then put two cards from your hand on top of your
//   library in any order.
//
// U instant. The most iconic cantrip. Requires Hat.ChoosePutBack.
func registerBrainstorm(r *Registry) {
	r.OnResolve("Brainstorm", brainstormResolve)
}

func brainstormResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "brainstorm"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Draw 3 cards.
	drawn := 0
	for i := 0; i < 3; i++ {
		if len(s.Library) > 0 {
			card := s.Library[0]
			gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
			drawn++
		}
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "draw",
		Seat:   seat,
		Source: "Brainstorm",
		Amount: drawn,
	})

	// Put 2 cards from hand on top of library in any order.
	// Use Hat.ChoosePutBack to decide which cards.
	putBackCount := 2
	if len(s.Hand) < putBackCount {
		putBackCount = len(s.Hand)
	}
	if putBackCount > 0 && s.Hat != nil {
		chosen := s.Hat.ChoosePutBack(gs, seat, s.Hand, putBackCount)
		if len(chosen) == putBackCount {
			// Route each chosen card through MoveCard so §614 replacements
			// and hand-leave triggers fire. Iterate in REVERSE so the first
			// element of chosen ends up as the new top of library (each
			// MoveCard uses library_top, which prepends; reversing gives
			// the desired final ordering).
			for i := len(chosen) - 1; i >= 0; i-- {
				gameengine.MoveCard(gs, chosen[i], seat, "hand", "library_top", "tuck-top")
			}
		}
	}

	emit(gs, slug, "Brainstorm", map[string]interface{}{
		"seat":     seat,
		"drawn":    drawn,
		"put_back": putBackCount,
	})
}

// --- Ponder ---
//
// Oracle text:
//   Look at the top three cards of your library, then put them back in
//   any order. You may shuffle your library.
//   Draw a card.
//
// U sorcery. Uses ChooseScry for the reorder decision (functionally
// equivalent). MVP: scry 3 then draw 1.
func registerPonder(r *Registry) {
	r.OnResolve("Ponder", ponderResolve)
}

func ponderResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "ponder"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Look at top 3 and reorder. Scry is a reasonable proxy: Hat decides
	// which stay on top (in order) and which go to bottom.
	// Ponder also has "you may shuffle" — MVP: Hat scry decision covers
	// this intent (bottom cards = functionally similar to shuffle-away).
	gameengine.Scry(gs, seat, 3)

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Source: "Ponder",
			Amount: 1,
		})
	}

	emit(gs, slug, "Ponder", map[string]interface{}{
		"seat": seat,
	})
}

// --- Preordain ---
//
// Oracle text:
//   Scry 2, then draw a card.
//
// U sorcery.
func registerPreordain(r *Registry) {
	r.OnResolve("Preordain", preordainResolve)
}

func preordainResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "preordain"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Scry 2.
	gameengine.Scry(gs, seat, 2)

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Source: "Preordain",
			Amount: 1,
		})
	}

	emit(gs, slug, "Preordain", map[string]interface{}{
		"seat": seat,
	})
}

// --- Gitaxian Probe ---
//
// Oracle text:
//   ({U/P} can be paid with either {U} or 2 life.)
//   Look at target player's hand.
//   Draw a card.
//
// Phyrexian U sorcery. Often "free" at 2 life.
func registerGitaxianProbe(r *Registry) {
	r.OnResolve("Gitaxian Probe", gitaxianProbeResolve)
}

func gitaxianProbeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "gitaxian_probe"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Look at target opponent's hand. MVP: log-only reveal (we don't
	// expose hidden information to the Hat yet).
	targetSeat := -1
	for _, opp := range gs.Opponents(seat) {
		targetSeat = opp
		break
	}
	if targetSeat >= 0 && targetSeat < len(gs.Seats) {
		gs.LogEvent(gameengine.Event{
			Kind:   "look_at_hand",
			Seat:   seat,
			Target: targetSeat,
			Source: "Gitaxian Probe",
			Amount: len(gs.Seats[targetSeat].Hand),
		})
	}

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Source: "Gitaxian Probe",
			Amount: 1,
		})
	}

	emit(gs, slug, "Gitaxian Probe", map[string]interface{}{
		"seat":        seat,
		"target_seat": targetSeat,
	})
}

// --- Opt ---
//
// Oracle text:
//   Scry 1, then draw a card.
//
// U instant.
func registerOpt(r *Registry) {
	r.OnResolve("Opt", optResolve)
}

func optResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "opt"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Scry 1.
	gameengine.Scry(gs, seat, 1)

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Source: "Opt",
			Amount: 1,
		})
	}

	emit(gs, slug, "Opt", map[string]interface{}{
		"seat": seat,
	})
}

// --- Consider ---
//
// Oracle text:
//   Look at the top card of your library. You may put that card into
//   your graveyard.
//   Draw a card.
//
// U instant. Functionally surveil 1, then draw 1.
func registerConsider(r *Registry) {
	r.OnResolve("Consider", considerResolve)
}

func considerResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "consider"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Surveil 1 (look at top, may put into graveyard).
	gameengine.Surveil(gs, seat, 1)

	// Draw a card.
	if len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "hand", "draw")
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Source: "Consider",
			Amount: 1,
		})
	}

	emit(gs, slug, "Consider", map[string]interface{}{
		"seat": seat,
	})
}

// ============================================================================
// Helper
// ============================================================================

// removeCardFromSlice removes the first occurrence of card (by pointer
// identity) from the slice.
func removeCardFromSlice(hand *[]*gameengine.Card, card *gameengine.Card) {
	for i, c := range *hand {
		if c == card {
			*hand = append((*hand)[:i], (*hand)[i+1:]...)
			return
		}
	}
}
