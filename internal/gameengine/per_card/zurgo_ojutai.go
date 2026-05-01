package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZurgoOjutai wires Zurgo and Ojutai.
//
// Oracle text:
//
//	Flying, haste
//	Whenever Zurgo and Ojutai deals combat damage to a player, look at
//	the top three cards of your library. Put one into your hand and the
//	rest on the bottom of your library in any order.
//	Dash {3}{W}{U}{B}
//
// Combat damage trigger: filtered to source == Zurgo and Ojutai. We pick
// the highest-priority card from the top three using a simple heuristic
// (prefer non-land, then highest CMC) and route it to hand. The
// remaining cards stay on the bottom in their revealed order — the AI's
// "any order" choice doesn't matter for the heuristic since they're
// being buried under any future draw.
//
// Dash isn't engine-supported as a first-class alternative cost: cast
// from hand for an alt cost, gain haste, return to hand at next end
// step. emitPartial flags this so audits can find it.
func registerZurgoOjutai(r *Registry) {
	r.OnETB("Zurgo and Ojutai", zurgoOjutaiETB)
	r.OnTrigger("Zurgo and Ojutai", "combat_damage_player", zurgoOjutaiCombatDamage)
}

func zurgoOjutaiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zurgo_ojutai_dash_alt_cost"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"dash_alt_cost_with_haste_and_eot_return_unimplemented")
}

func zurgoOjutaiCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zurgo_ojutai_combat_damage_top_three"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	n := 3
	if len(seat.Library) < n {
		n = len(seat.Library)
	}
	if n == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"revealed": 0,
			"reason":   "library_empty",
		})
		return
	}

	revealed := make([]*gameengine.Card, n)
	copy(revealed, seat.Library[:n])
	seat.Library = seat.Library[n:]

	// Heuristic: prefer non-land, then highest CMC. Fallback to first card.
	pickIdx := 0
	bestScore := -1
	for i, c := range revealed {
		if c == nil {
			continue
		}
		score := cardCMC(c)
		if !cardHasType(c, "land") {
			score += 100
		}
		if score > bestScore {
			bestScore = score
			pickIdx = i
		}
	}
	picked := revealed[pickIdx]
	seat.Hand = append(seat.Hand, picked)
	gs.LogEvent(gameengine.Event{
		Kind:   "draw",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "zurgo_ojutai_pick_top_three",
			"card":   picked.DisplayName(),
		},
	})

	var bottomed []string
	for i, c := range revealed {
		if i == pickIdx || c == nil {
			continue
		}
		seat.Library = append(seat.Library, c)
		bottomed = append(bottomed, c.DisplayName())
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"revealed":      n,
		"taken":         picked.DisplayName(),
		"bottomed":      bottomed,
		"library_left":  len(seat.Library),
	})
}
