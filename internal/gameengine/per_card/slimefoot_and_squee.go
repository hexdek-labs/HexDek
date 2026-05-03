package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// registerSlimefootAndSquee wires Slimefoot and Squee.
//
// Oracle text:
//
//	Whenever Slimefoot and Squee enters or whenever another creature you
//	control dies, create a 1/1 green Saproling creature token.
//	{1}{B}{R}{G}, Sacrifice a Saproling: Return Slimefoot and Squee from
//	your graveyard to the battlefield. Activate only as a sorcery.
//
// Implementation:
//   - OnETB — create a 1/1 green Saproling token when Slimefoot and Squee
//     enters the battlefield.
//   - OnTrigger("creature_dies") — when another creature controlled by
//     Slimefoot and Squee's controller dies, create a 1/1 green Saproling
//     token. Gate: dying creature is not Slimefoot and Squee itself, and
//     the dying creature's controller matches S&S's controller.
//   - Activated ability ({1}{B}{R}{G}, Sac Saproling: return from graveyard
//     to battlefield, sorcery-speed) — emitPartial; graveyard-return
//     activated abilities are not yet wired into the activation dispatch.
func registerSlimefootAndSquee(r *Registry) {
	r.OnETB("Slimefoot and Squee", slimefootAndSqueeETB)
	r.OnTrigger("Slimefoot and Squee", "creature_dies", slimefootAndSqueeCreatureDies)
}

func slimefootAndSqueeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "slimefoot_and_squee_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	gameengine.CreateCreatureToken(gs, seat, "Saproling", []string{"creature", "saproling"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"token": "Saproling",
	})
}

func slimefootAndSqueeCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "slimefoot_and_squee_creature_dies"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat := perm.Controller

	// Only triggers on creatures controlled by S&S's controller.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != seat {
		return
	}

	// "Another creature" — the dying creature must not be Slimefoot and Squee itself.
	if dyingPerm, ok := ctx["perm"].(*gameengine.Permanent); ok && dyingPerm == perm {
		return
	}

	gameengine.CreateCreatureToken(gs, seat, "Saproling", []string{"creature", "saproling"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"token": "Saproling",
	})
}
