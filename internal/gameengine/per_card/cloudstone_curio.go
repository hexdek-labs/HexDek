package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCloudstoneCurio wires up Cloudstone Curio.
//
// Oracle text:
//
//	Whenever a nonland permanent enters the battlefield under your
//	control, you may return another nonland permanent you control
//	to its owner's hand.
//
// Paired with Displacer Kitten or any ETB-value creature, this
// compounds: each ETB bounces a prior permanent; you can then re-cast
// it for another ETB + bounce, generating infinite ETB triggers if
// there's a mana-positive piece in the chain.
//
// Implementation:
//   - OnTrigger("nonland_permanent_etb") — when the Curio's controller
//     ETBs a nonland permanent, pick ANOTHER nonland permanent they
//     control and bounce it to its owner's hand.
//   - Target policy: prefer the OLDEST nonland (lowest timestamp) that
//     isn't the Curio itself or the permanent just entering. This
//     mirrors typical cEDH lines (bounce a mana rock to re-play it for
//     another ETB).
func registerCloudstoneCurio(r *Registry) {
	r.OnTrigger("Cloudstone Curio", "nonland_permanent_etb", cloudstoneCurioOnETB)
}

func cloudstoneCurioOnETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cloudstone_curio_bounce"
	if gs == nil || perm == nil {
		return
	}
	// "Under your control" — scope to Curio's controller.
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	// Can't trigger on Curio entering itself (oracle says "another").
	if entering == perm {
		return
	}
	seat := perm.Controller

	// Find bounce target: oldest non-Curio, non-entering, nonland
	// permanent the controller controls.
	var target *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p == perm || p == entering {
			continue
		}
		if p.IsLand() {
			continue
		}
		if target == nil || p.Timestamp < target.Timestamp {
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_bounce_target", map[string]interface{}{
			"entering": entering.Card.DisplayName(),
		})
		return
	}
	// Route through BouncePermanent for proper zone-change handling:
	// replacement effects, LTB triggers, commander redirect.
	gameengine.BouncePermanent(gs, target, perm, "hand")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"bounced_card": target.Card.DisplayName(),
		"entering":     entering.Card.DisplayName(),
	})
}
