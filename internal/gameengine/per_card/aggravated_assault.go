package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAggravatedAssault wires up Aggravated Assault.
//
// Oracle text:
//
//	{3}{R}{R}: Untap all creatures you control. After this main phase,
//	there is an additional combat phase followed by an additional main
//	phase. Activate only during your main phase.
//
// In the engine this generates extra combats via PendingExtraCombats +
// untaps all creatures. The turn loop already consumes PendingExtraCombats.
func registerAggravatedAssault(r *Registry) {
	r.OnActivated("Aggravated Assault", aggravatedAssaultActivated)
}

func aggravatedAssaultActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "aggravated_assault_extra_combat"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller

	// Check it's our main phase.
	if gs.Active != seat {
		emitFail(gs, slug, "Aggravated Assault", "not_your_turn", nil)
		return
	}

	// Pay {3}{R}{R} = 5 mana.
	s := gs.Seats[seat]
	if s.ManaPool < 5 {
		emitFail(gs, slug, "Aggravated Assault", "insufficient_mana", nil)
		return
	}
	s.ManaPool -= 5
	gameengine.SyncManaAfterSpend(s)

	// Untap all creatures we control.
	untapped := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsCreature() || !p.Tapped {
			continue
		}
		p.Tapped = false
		untapped++
	}

	// Queue extra combat + extra main phase.
	gs.PendingExtraCombats++

	emit(gs, slug, "Aggravated Assault", map[string]interface{}{
		"seat":              seat,
		"creatures_untapped": untapped,
		"extra_combats":     gs.PendingExtraCombats,
	})
}
