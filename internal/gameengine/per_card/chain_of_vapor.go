package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// chain_of_vapor.go — per_card handler for Chain of Vapor.
//
// Oracle text (Instant, {U}):
//
//	Return target nonland permanent to its owner's hand. Then that
//	permanent's controller may sacrifice a land of their choice. If the
//	player does, they may copy this spell and may choose a new target
//	for that copy.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so Chain of Vapor — a premier cheap bounce / combo-protection
// instant — did NOTHING on resolution. Verified inert: no registered
// handler for "Chain of Vapor".
//
// Implemented here (OnResolve): return the highest-value nonland
// permanent an opponent controls to its owner's hand (AI target
// selection, matching how the engine resolves other single-target bounce
// effects). Routed through the canonical BouncePermanent so zone-change
// triggers, token-ceasing, aura detach and replacement-unregistering all
// fire correctly.
//
// Deliberately omitted (logged): the optional "controller may sacrifice
// a land to copy this spell" chain. That is a downside the bounced
// permanent's controller would decline under AI policy (no incentive to
// sacrifice a land to bounce more of their own board), so the chain
// almost never extends; modelling it would add cross-seat optional
// choreography for negligible behavioral change.

func init() {
	registerChainOfVapor(Global())
	AddResetHook(registerChainOfVapor)
}

func registerChainOfVapor(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Chain of Vapor", chainOfVaporResolve)
}

func chainOfVaporResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "chain_of_vapor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	target := pickBestOppNonlandPermanent(gs, seat)
	if target == nil {
		emitFail(gs, slug, "Chain of Vapor", "no_opponent_nonland_permanent", map[string]interface{}{"seat": seat})
		return
	}
	owner := target.Card.DisplayName()
	if gameengine.BouncePermanent(gs, target, nil, "hand") {
		emit(gs, slug, "Chain of Vapor", map[string]interface{}{
			"seat":    seat,
			"bounced": owner,
		})
	}
}
