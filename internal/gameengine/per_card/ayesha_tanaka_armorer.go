package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAyeshaTanakaArmorer wires Ayesha Tanaka, Armorer.
//
// Oracle text:
//
//	Whenever Ayesha Tanaka attacks, look at the top four cards of your
//	library. You may put any number of artifact cards with mana value
//	less than or equal to Ayesha Tanaka's power from among them onto
//	the battlefield tapped. Put the rest on the bottom of your library
//	in a random order.
//	Ayesha Tanaka can't be blocked as long as defending player controls
//	three or more artifacts.
func registerAyeshaTanakaArmorer(r *Registry) {
	r.OnTrigger("Ayesha Tanaka, Armorer", "attacks", ayeshaAttacks)
}

func ayeshaAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ayesha_tanaka_top4_artifacts"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attackerSeat, _ := ctx["seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	power := perm.Power()
	look := 4
	if look > len(seat.Library) {
		look = len(seat.Library)
	}
	// Snapshot top N (Wave 2 multi-step migration — no pre-splice per
	// iteration). Walk the snapshot; for played cards let
	// enterBattlefieldWithETB → createPermanent sweep them from library
	// + fire ETB cascade (cleaner than MoveCard("library"→"battlefield"),
	// which races with createPermanent's dedup). For passed cards, rotate
	// top→bottom (within-zone reorder, no zone change).
	top := append([]*gameengine.Card(nil), seat.Library[:look]...)
	playedCount := 0
	for _, c := range top {
		if c == nil {
			continue
		}
		if cardHasType(c, "artifact") && gameengine.ManaCostOf(c) <= power {
			enterBattlefieldWithETB(gs, perm.Controller, c, true)
			playedCount++
			continue
		}
		if len(seat.Library) > 0 && seat.Library[0] == c {
			seat.Library = seat.Library[1:]
			seat.Library = append(seat.Library, c)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"looked":    look,
		"played":    playedCount,
		"power_cap": power,
	})
}
