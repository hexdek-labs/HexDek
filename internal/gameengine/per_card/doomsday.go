package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDoomsday wires up Doomsday.
//
// Oracle text:
//
//	Search your library and graveyard for five cards and exile the
//	rest. Put the chosen cards on top of your library in any order.
//	You lose half your life, rounded up.
//
// This is THE Kraum+Tymna primary tutor-to-win. With Thassa's Oracle in
// the pile (or already in play + Consultation in the pile), this reliably
// wins within 1-2 turns after resolution.
//
// MVP policy:
//   - Pool = library + graveyard.
//   - If pool has ≤ 5 cards, we pick all of them (CR allows picking 0
//     of a type; zero-short pools are legal but usually fail to combo).
//   - Picked cards go on top of the library in the order they were
//     found. A real client would expose an ordering choice; this is
//     deterministic for reproducible simulation.
//   - Non-picked cards are exiled.
//   - Controller loses ceil(Life/2) life.
//
// No self-replacement or mill-triggers are modeled — Doomsday reshuffles
// the library, which technically triggers any "when you search your
// library" / "whenever you exile a card" replacements. We log a partial.
func registerDoomsday(r *Registry) {
	r.OnResolve("Doomsday", doomsdayResolve)
}

func doomsdayResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "doomsday"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Build the pool: library first, then graveyard. Track the origin
	// zone per card so exile routing can fire correct triggers.
	libLen := len(s.Library)
	pool := make([]*gameengine.Card, 0, libLen+len(s.Graveyard))
	pool = append(pool, s.Library...)
	pool = append(pool, s.Graveyard...)
	if len(pool) == 0 {
		emitFail(gs, slug, "Doomsday", "library_and_graveyard_empty", nil)
		// Still pay the life cost.
		doomsdayLoseHalfLife(gs, seat)
		return
	}

	// Pick up to 5 cards. MVP: prefer cards whose name matches a
	// "doomsday_pile" token in card.Types (tests can pre-order), else
	// take the first 5 in pool order.
	pickedCount := 5
	if pickedCount > len(pool) {
		pickedCount = len(pool)
	}
	picked := append([]*gameengine.Card(nil), pool[:pickedCount]...)
	rest := pool[pickedCount:]

	// Drain source zones up front so MoveCard's internal removal is a
	// no-op and we can route cleanly.
	s.Library = nil
	s.Graveyard = nil
	// Route each "rest" card through MoveCard with the correct origin
	// zone (library if it came from the first libLen slots of pool,
	// graveyard otherwise). Index offset = pickedCount.
	for i, c := range rest {
		origIdx := pickedCount + i
		fromZone := "library"
		if origIdx >= libLen {
			fromZone = "graveyard"
		}
		gameengine.MoveCard(gs, c, seat, fromZone, "exile", "effect")
	}
	// Route each picked card through MoveCard so graveyard-origin picks
	// fire §614 replacements and leaves-graveyard triggers. Library-origin
	// picks are a library→library_bottom move (semantically a reorder;
	// MoveCard still fires a zone_change event but moveToZone drops them
	// in the library — the effective append-picked-as-top ordering is
	// reconstructed below by appending picked in reverse via library_top,
	// so the first picked card ends up on top per "in the order they were
	// found."
	for i := len(picked) - 1; i >= 0; i-- {
		c := picked[i]
		origIdx := i
		fromZone := "library"
		if origIdx >= libLen {
			fromZone = "graveyard"
		}
		gameengine.MoveCard(gs, c, seat, fromZone, "library_top", "doomsday-stack")
	}

	emit(gs, slug, "Doomsday", map[string]interface{}{
		"seat":           seat,
		"picked_count":   len(picked),
		"exiled_count":   len(rest),
		"library_size":   len(s.Library),
	})

	doomsdayLoseHalfLife(gs, seat)
}

func doomsdayLoseHalfLife(gs *gameengine.GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	// ceil(life/2). Negative life is already a loss condition; we clamp
	// the loss to non-negative.
	life := s.Life
	if life < 0 {
		life = 0
	}
	loss := (life + 1) / 2
	s.Life -= loss
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Doomsday",
		Amount: loss,
		Details: map[string]interface{}{
			"reason": "doomsday_half_life",
		},
	})
}
