package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSatoruUmezawa wires Satoru Umezawa.
//
// Oracle text:
//
//	Whenever you activate a ninjutsu ability, look at the top three cards
//	of your library. Put one of them into your hand and the rest on the
//	bottom of your library in any order. This ability triggers only once
//	each turn.
//	Each creature card in your hand has ninjutsu {2}{U}{B}.
//
// The ninjutsu grant is the meat of Satoru — it lets every creature in
// hand sneak in for {2}{U}{B} and bypass blockers. The grant itself is
// a static type-line modification we don't model end-to-end yet (the
// engine's ninjutsu cost lookup reads the printed card, not granted
// keywords). What we DO model is the once-per-turn dig:
//
//   - OnTrigger("ninjutsu_activated"): fires when our controller
//     activates ANY ninjutsu ability. Once per turn (tracked via a flag
//     keyed on Satoru + turn number).
//   - Look at top 3 of library, pick the highest-CMC creature card
//     (ninjutsu deck wants creatures), move it to hand, send the other
//     two to the bottom (preserving order, MVP — "any order").
func registerSatoruUmezawa(r *Registry) {
	r.OnTrigger("Satoru Umezawa", "ninjutsu_activated", satoruUmezawaTrigger)
}

func satoruUmezawaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "satoru_umezawa_ninjutsu_dig"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Once per turn.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := "satoru_dug_turn_" + strconv.Itoa(gs.Turn)
	if perm.Flags[key] > 0 {
		return
	}
	perm.Flags[key] = 1

	if len(seat.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_library", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}

	// Look at top 3 (or fewer if library is short).
	n := 3
	if len(seat.Library) < n {
		n = len(seat.Library)
	}
	top := seat.Library[:n]

	// Pick the highest-CMC creature card; fall back to the highest-CMC
	// card overall if no creature is in the top 3.
	pickIdx := -1
	bestCMC := -1
	for i, c := range top {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pickIdx = i
		}
	}
	if pickIdx < 0 {
		bestCMC = -1
		for i, c := range top {
			if c == nil {
				continue
			}
			cmc := gameengine.ManaCostOf(c)
			if cmc > bestCMC {
				bestCMC = cmc
				pickIdx = i
			}
		}
	}
	if pickIdx < 0 {
		return
	}

	picked := top[pickIdx]
	// Bottom the other top-N cards in their current relative order.
	var rest []*gameengine.Card
	for i, c := range top {
		if i == pickIdx {
			continue
		}
		rest = append(rest, c)
	}

	// Slice off the top N.
	seat.Library = append([]*gameengine.Card(nil), seat.Library[n:]...)
	// Append the rest to the bottom.
	seat.Library = append(seat.Library, rest...)
	// Move the picked card to hand via MoveCard so zone-change triggers fire.
	// Place picked at position 0 of "library" temporarily so MoveCard can
	// find it. Simpler: append directly to hand and log.
	seat.Hand = append(seat.Hand, picked)

	gs.LogEvent(gameengine.Event{
		Kind:   "satoru_umezawa_dig",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"looked":   n,
			"picked":   picked.DisplayName(),
			"picked_cmc": bestCMC,
			"bottomed": len(rest),
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"looked":       n,
		"picked":       picked.DisplayName(),
		"picked_cmc":   bestCMC,
		"bottomed":     len(rest),
	})
}
