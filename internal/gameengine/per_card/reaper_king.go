package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Reaper King — {2/W}{2/U}{2/B}{2/R}{2/G} 6/6 Legendary Artifact Creature
// — Scarecrow.
//
//   Other Scarecrow creatures you control get +1/+1.
//   Whenever another Scarecrow you control enters, destroy target
//   permanent.
//
// Scarecrow tribal commander — every Scarecrow ETB is a Vindicate.
// Implementation hooks `permanent_etb` (the ETB-of-other event used by
// Ayula / Adrix and Nev / Be'lakor pattern), filters: controller_seat ==
// Reaper's controller, entered != Reaper himself, entered is a Scarecrow,
// then picks the most threatening opposing permanent to destroy.
//
// Target heuristic: prefer opposing creatures with highest Power; fall
// back to any opposing nonland permanent (planeswalkers, enchantments,
// artifacts). Lands deprioritized — Vindicate-style removal typically
// hits threats first. The +1/+1 anthem is engine-static layer territory
// (not handled here; primary archetype payoff is the ETB Vindicate).

func init() {
	registerReaperKing(Global())
	AddResetHook(registerReaperKing)
}

func registerReaperKing(r *Registry) {
	r.OnTrigger("Reaper King", "permanent_etb", reaperKingETB)
}

func reaperKingETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "reaper_king_scarecrow_vindicate"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if !cardHasSubtype(entered.Card, "scarecrow") {
		return
	}
	victim := reaperKingPickVictim(gs, perm.Controller)
	if victim == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"trigger_cause": entered.Card.DisplayName(),
			"reason":        "no_eligible_opposing_permanent",
		})
		return
	}
	gameengine.DestroyPermanent(gs, victim, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"trigger_cause": entered.Card.DisplayName(),
		"destroyed":     victim.Card.DisplayName(),
		"destroyed_seat": victim.Controller,
	})
}

func reaperKingPickVictim(gs *gameengine.GameState, ownerSeat int) *gameengine.Permanent {
	var bestCreature *gameengine.Permanent
	var bestNonLand *gameengine.Permanent
	bestPower := -1
	for i, s := range gs.Seats {
		if s == nil || i == ownerSeat || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsCreature() {
				if pw := p.Power(); pw > bestPower {
					bestPower = pw
					bestCreature = p
				}
				continue
			}
			if !cardHasType(p.Card, "land") && bestNonLand == nil {
				bestNonLand = p
			}
		}
	}
	if bestCreature != nil {
		return bestCreature
	}
	return bestNonLand
}
