package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBolassCitadel wires up Bolas's Citadel.
//
// Oracle text:
//
//	You may look at the top card of your library any time.
//	You may play lands and cast spells from the top of your library.
//	If you cast a spell this way, pay life equal to its mana value
//	rather than pay its mana cost.
//	{T}, Sacrifice ten nontoken permanents: Each opponent loses 10
//	life.
//
// THE Aetherflux Reservoir combo: Citadel lets you cast from the top
// paying life instead of mana, Aetherflux lifegains on every cast, so
// a sufficiently low-curve top-of-library produces a loop that gains
// 1-per-cast life and eventually activates Aetherflux for 50 damage.
//
// Batch #2 scope:
//   - OnActivated(0, ...): the "play top of library paying life" mode.
//     We DON'T implement the full cast-from-top cycle (that needs
//     cast-from-zone plumbing the engine doesn't have yet). Instead,
//     we draw the top card into hand and log a partial — downstream
//     zone-cast work can route through this.
//   - OnActivated(1, ...): the sac-10-nontoken mode. Each opponent
//     loses 10 life. Sacrifice cost is assumed paid by the caller.
//   - OnETB: set gs.Flags["bolas_citadel_active_seat_N"] = 1 so the
//     downstream cast-from-top logic can gate its life-cost
//     substitution.
//
// The "you may look at the top card" clause is a no-op in the current
// observation model (no hidden information yet).
func registerBolassCitadel(r *Registry) {
	r.OnETB("Bolas's Citadel", bolassCitadelETB)
	r.OnActivated("Bolas's Citadel", bolassCitadelActivate)
}

func bolassCitadelETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bolass_citadel_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["bolas_citadel_active_seat_"+intToStr(seat)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cast_from_top_of_library_paying_life_not_fully_wired_zone_cast_plumbing_missing")
}

func bolassCitadelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	switch abilityIdx {
	case 0:
		// "Play top of library for life" mode. MVP: move top card of
		// library into hand and pay life = its CMC. Downstream work
		// will swap this for the full cast-from-top pipeline once
		// zone-cast plumbing lands.
		const slug = "bolass_citadel_play_top"
		if len(s.Library) == 0 {
			emitFail(gs, slug, src.Card.DisplayName(), "library_empty", nil)
			return
		}
		c := s.Library[0]
		cmc := cardCMC(c)
		gameengine.MoveCard(gs, c, seat, "library", "hand", "effect")
		s.Life -= cmc
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   seat,
			Target: seat,
			Source: src.Card.DisplayName(),
			Amount: cmc,
			Details: map[string]interface{}{
				"reason":      "bolass_citadel_cast_top_paying_life",
				"card_played": c.DisplayName(),
				"cmc":         cmc,
			},
		})
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"card_played": c.DisplayName(),
			"life_paid":   cmc,
			"life_after":  s.Life,
		})
		emitPartial(gs, slug, src.Card.DisplayName(),
			"top_card_placed_in_hand_rather_than_cast_stack_zonecast_not_implemented")
		_ = gs.CheckEnd()
	case 1:
		// Sacrifice-10 → each opponent loses 10 life.
		const slug = "bolass_citadel_sac_ten"
		// We don't enforce the sac cost here (caller pays). Effect only.
		for _, opp := range gs.Opponents(seat) {
			os := gs.Seats[opp]
			os.Life -= 10
			gs.LogEvent(gameengine.Event{
				Kind:   "lose_life",
				Seat:   seat,
				Target: opp,
				Source: src.Card.DisplayName(),
				Amount: 10,
				Details: map[string]interface{}{
					"reason": "bolass_citadel_activated",
				},
			})
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":           seat,
			"opponents_hit":  len(gs.Opponents(seat)),
			"damage_per_opp": 10,
		})
		_ = gs.CheckEnd()
	}
}
