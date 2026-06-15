package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrunnTheLonelyKing wires Grunn, the Lonely King.
//
// Oracle text:
//
//	Kicker {3}
//	If Grunn was kicked, it enters with five +1/+1 counters on it.
//	Whenever Grunn attacks alone, double its power and toughness until end of turn.
//
// Kicker ETB is handled by the generic AST conditional resolver.
// This handler covers the "attacks alone → double P/T" trigger.
func registerGrunnTheLonelyKing(r *Registry) {
	r.OnTrigger("Grunn, the Lonely King", "combat_attackers_declared", grunnAttacksAlone)
}

func grunnAttacksAlone(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "grunn_the_lonely_king_attacks_alone"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// CR §508.4a — "Grunn attacks alone" requires Grunn to be the ONLY
	// DECLARED attacker. A Grunn put onto the battlefield attacking (§506.3)
	// was never declared, so it doesn't "attack alone"; and a token entering
	// attacking next to a lone declared Grunn doesn't break alone-ness. Use
	// WasDeclaredAttacker() to agree with exalted's declared count.
	if !perm.WasDeclaredAttacker() {
		return
	}
	attackerCount := 0
	for _, p := range s.Battlefield {
		if p != nil && p.WasDeclaredAttacker() {
			attackerCount++
		}
	}
	if attackerCount != 1 {
		return
	}
	curPower := perm.Power()
	curTough := perm.Toughness()
	perm.Modifications = append(perm.Modifications, gameengine.Modification{
		Power:     curPower,
		Toughness: curTough,
		Duration:  "until_end_of_turn",
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"doubled_power": curPower * 2,
		"doubled_tough": curTough * 2,
	})
}
