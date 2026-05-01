package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRowanScion wires Rowan, Scion of War.
//
// Oracle text:
//
//	Menace
//	{T}: Spells you cast this turn that are black and/or red cost {X}
//	     less to cast, where X is the amount of life you lost this turn.
//	     Activate only as a sorcery.
//
// The activated ability is a setup move for storm-style turns: pay big
// life (Necropotence draw, Ad Nauseam, Bolas's Citadel, fetch+shock land
// drops) earlier in the turn, tap Rowan, then dump cheap-or-free B/R
// spells. The cost reduction itself is an engine concern (cost_modifiers.go
// would need a "rowan_discount" reader), so this handler just publishes
// the discount value on the permanent's Flags and emits the event for the
// hat / Freya analyzer to read.
//
// Life-lost-this-turn approximation: net loss since this turn's untap
// step, computed as life_at_turn_start - current_life (clamped >=0).
// This mirrors the Book of Vile Darkness pattern (vecna_trilogy.go) and
// uses the seat flag set by tournament/turn.go at the start of each turn.
// Net loss undercounts when life was both lost and gained (e.g. lost 6
// to fetches, gained 3 from Aetherflux); the engine doesn't yet ledger
// gross life-loss separately.
func registerRowanScion(r *Registry) {
	r.OnActivated("Rowan, Scion of War", rowanScionActivate)
}

func rowanScionActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rowan_scion_of_war_discount"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	lifeLost := 0
	if s.Flags != nil {
		if startLife, ok := s.Flags["life_at_turn_start"]; ok && startLife > s.Life {
			lifeLost = startLife - s.Life
		}
	}

	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["rowan_discount"] = lifeLost

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"life_lost": lifeLost,
		"discount":  lifeLost,
		"applies_to": "black_and_or_red_spells_this_turn",
	})
	if lifeLost == 0 {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"no_life_lost_this_turn_discount_is_zero")
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"cost_modifier_reader_for_rowan_discount_not_yet_wired_in_cost_modifiers")
}
