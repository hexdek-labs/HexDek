package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Geist of Saint Traft — {1}{W}{U} 2/2 Legendary Creature — Spirit Cleric.
//
//   Hexproof
//   Whenever Geist of Saint Traft attacks, create a 4/4 white Angel
//   creature token with flying that's tapped and attacking. Exile that
//   token at end of combat.
//
// Pattern mirrors altair_ibn_la_ahad.go: spawn-then-delayed-exile via
// RegisterDelayedTrigger with TriggerAt="end_of_combat". Hexproof is
// keyword-pipeline. Angel token enters tapped + attacking; defender_seat
// carried from ctx so the token attacks the same opponent as Geist.

func init() {
	registerGeistOfSaintTraft(Global())
	AddResetHook(registerGeistOfSaintTraft)
}

func registerGeistOfSaintTraft(r *Registry) {
	r.OnTrigger("Geist of Saint Traft", "creature_attacks", geistAttack)
}

func geistAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "geist_attack_angel_token"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	token := gameengine.CreateCreatureToken(gs, perm.Controller, "Angel Token",
		[]string{"creature", "angel", "pip:W", "flying"}, 4, 4)
	if token == nil {
		return
	}
	token.Tapped = true
	token.Flags["attacking"] = 1
	token.SummoningSick = false
	if defenderSeat, ok := ctx["defender_seat"].(int); ok {
		token.Flags["attacking_defender_seat"] = defenderSeat + 1
	}
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_combat",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName() + " (angel exile)",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gameengine.ExilePermanent(gs, token, perm)
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Angel Token (4/4 flying, tapped+attacking, exiles EOC)",
	})
}
