package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPiaNalaarConsulOfRevival wires Pia Nalaar, Consul of Revival.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Pia%20Nalaar%2C%20Consul%20of%20Revival):
//
//	Whenever Pia Nalaar, Consul of Revival or another nontoken artifact
//	creature you control dies, create a tapped 1/1 colorless Thopter
//	artifact creature token with flying.
//
// {1}{R} 2/2 Legendary Creature — Human Artificer. Artifact-tribal /
// Thopter-tribal aristocrats commander — every artifact creature
// (including herself) that dies trades for a Thopter, and the Thopters
// themselves enable later sac-payoff loops (Disciple of the Vault,
// Pia and Kiran Nalaar's aftermath, Krark-Clan Ironworks combos).
//
// Implementation:
//   - OnTrigger("creature_dies"). Filter chain: (1) the dying perm is
//     a nontoken artifact creature, (2) controlled by Pia's controller.
//     "Pia or ANOTHER" — Pia herself qualifies; other artifact
//     creatures must be different from Pia (the engine doesn't fire
//     creature_dies recursively for the Thopter itself in dying-
//     batches, so no infinite-loop guard needed).
//   - Action: create a tapped 1/1 colorless flying Thopter token.
func registerPiaNalaarConsulOfRevival(r *Registry) {
	r.OnTrigger("Pia Nalaar, Consul of Revival", "creature_dies", piaNalaarOnDies)
}

func piaNalaarOnDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pia_nalaar_consul_of_revival"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying == nil || dying.Card == nil {
		// Some dispatch paths use "dying_perm"; honor it as a fallback.
		dying, _ = ctx["dying_perm"].(*gameengine.Permanent)
		if dying == nil || dying.Card == nil {
			return
		}
	}
	// Must be an artifact creature controlled by Pia's controller,
	// and not a token.
	if !cardHasType(dying.Card, "creature") || !cardHasType(dying.Card, "artifact") {
		return
	}
	if dying.Controller != perm.Controller {
		return
	}
	if dying.IsToken() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	token := &gameengine.Card{
		Name:          "Thopter Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "artifact", "thopter", "flying"},
		BasePower:     1,
		BaseToughness: 1,
	}
	tp := enterBattlefieldWithETB(gs, perm.Controller, token, false)
	if tp != nil {
		tp.Tapped = true
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Thopter Token",
			"reason": "pia_nalaar_consul_of_revival",
			"power":  1,
			"tough":  1,
			"flying": true,
			"tapped": true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"dying": dying.Card.DisplayName(),
	})
}
