package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJanJansenChaosCrafter wires Jan Jansen, Chaos Crafter.
//
// Oracle text:
//
//	Haste
//	{T}, Sacrifice an artifact creature: Create two Treasure tokens.
//	{T}, Sacrifice a noncreature artifact: Create two 1/1 colorless
//	Construct artifact creature tokens.
//
// Implementation (R53 batch N port):
//   - OnActivated with two abilityIdx variants (0 and 1). Both require
//     the source to be untapped.
//   - Ability 0: sac the lowest-CMC friendly artifact creature →
//     mint 2 Treasure tokens.
//   - Ability 1: sac the lowest-CMC friendly noncreature artifact →
//     mint 2 1/1 colorless Construct artifact creature tokens.
func registerJanJansenChaosCrafter(r *Registry) {
	r.OnActivated("Jan Jansen, Chaos Crafter", janJansenActivated)
}

func janJansenActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jan_jansen_activated"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	pickArtifact := func(creatureOnly, noncreatureOnly bool) *gameengine.Permanent {
		var best *gameengine.Permanent
		bestCMC := 1 << 30
		for _, p := range seat.Battlefield {
			if p == nil || p == src || p.Card == nil {
				continue
			}
			if !cardHasType(p.Card, "artifact") {
				continue
			}
			isCreature := p.IsCreature()
			if creatureOnly && !isCreature {
				continue
			}
			if noncreatureOnly && isCreature {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc < bestCMC {
				bestCMC = cmc
				best = p
			}
		}
		return best
	}
	var victim *gameengine.Permanent
	switch abilityIdx {
	case 0:
		victim = pickArtifact(true, false)
	case 1:
		victim = pickArtifact(false, true)
	default:
		return
	}
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_sacrifice_target", map[string]interface{}{
			"ability_idx": abilityIdx,
		})
		return
	}
	src.Tapped = true
	gameengine.SacrificePermanent(gs, victim, "jan_jansen_sac")
	switch abilityIdx {
	case 0:
		for i := 0; i < 2; i++ {
			gameengine.CreateTreasureToken(gs, src.Controller)
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":       src.Controller,
			"sacrificed": victim.Card.DisplayName(),
			"minted":     "treasure x2",
		})
	case 1:
		for i := 0; i < 2; i++ {
			gameengine.CreateCreatureToken(gs, src.Controller, "Construct Token",
				[]string{"artifact", "creature", "construct"}, 1, 1)
		}
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":       src.Controller,
			"sacrificed": victim.Card.DisplayName(),
			"minted":     "construct x2",
		})
	}
}
