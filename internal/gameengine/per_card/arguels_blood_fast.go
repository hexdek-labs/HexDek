package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArguelsBloodFast wires Arguel's Blood Fast // Temple of Aclazotz
// (Muninn parser-gap #97, ~3.9K hits).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Arguel's Blood Fast — {1}{B} Legendary Enchantment
//	{1}{B}, Pay 2 life: Draw a card.
//	At the beginning of your upkeep, if you have 5 or less life, you
//	may transform Arguel's Blood Fast.
//
//	Temple of Aclazotz (back) — Legendary Land
//	{T}: Add {B}.
//	{T}, Sacrifice a creature: You gain life equal to the sacrificed
//	creature's toughness.
//
// Implementation:
//   - OnActivated index 0: pay 2 life, draw 1. Mana cost {1}{B} is
//     enforced by the activation pipeline.
//   - upkeep_controller: if controller's life <= 5, emit
//     transform-eligible partial (DFC transform is engine-level).
func registerArguelsBloodFast(r *Registry) {
	r.OnActivated("Arguel's Blood Fast", arguelsBloodFastActivate)
	r.OnTrigger("Arguel's Blood Fast", "upkeep_controller", arguelsBloodFastUpkeep)
}

func arguelsBloodFastActivate(gs *gameengine.GameState, src *gameengine.Permanent, idx int, ctx map[string]interface{}) {
	const slug = "arguels_blood_fast_draw"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	if s.Life < 3 {
		emitFail(gs, slug, "Arguel's Blood Fast", "insufficient_life", map[string]interface{}{
			"life": s.Life,
		})
		return
	}
	gameengine.LoseLife(gs, seat, 2, "Arguel's Blood Fast")
	drawn := drawOne(gs, seat, "Arguel's Blood Fast")
	name := ""
	if drawn != nil {
		name = drawn.DisplayName()
	}
	emit(gs, slug, "Arguel's Blood Fast", map[string]interface{}{
		"seat":  seat,
		"drawn": name,
	})
}

func arguelsBloodFastUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "arguels_blood_fast_upkeep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	life := gs.Seats[perm.Controller].Life
	if life > 5 {
		emit(gs, slug, "Arguel's Blood Fast", map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"life":      life,
		})
		return
	}
	// R51 batch J: promote breadcrumb. The engine exposes
	// TransformPermanent (used by Cecil's darkness threshold + others);
	// call it directly so Arguel's flips to Temple of Aclazotz when the
	// 5-or-less-life condition holds. Idempotent on already-transformed
	// permanents (TransformPermanent self-skips).
	if !perm.Transformed {
		gameengine.TransformPermanent(gs, perm, "arguels_blood_fast_low_life")
	}
	emit(gs, slug, "Arguel's Blood Fast", map[string]interface{}{
		"seat":        perm.Controller,
		"triggered":   true,
		"life":        life,
		"transformed": perm.Transformed,
	})
}
