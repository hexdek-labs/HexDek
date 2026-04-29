package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYarok wires up Yarok, the Desecrated.
//
// Oracle text:
//
//	Deathtouch, lifelink
//	If a permanent entering the battlefield causes a triggered ability
//	of a permanent you control to trigger, that ability triggers an
//	additional time.
//
// This is Panharmonicon for ALL permanents (not just artifacts/creatures).
// Implementation: register a replacement effect on "would_fire_etb_trigger"
// that doubles the trigger count for Yarok's controller, identical to
// Panharmonicon's mechanism in replacement.go.
//
// The replacement effect system handles stacking: if both Yarok AND
// Panharmonicon are on the battlefield, both replacement effects apply
// independently, doubling then tripling (or vice versa) the trigger count.
func registerYarok(r *Registry) {
	r.OnETB("Yarok, the Desecrated", yarokETB)
}

func yarokETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// Register the ETB doubler via the replacement effect system,
	// same as Panharmonicon but sourced from Yarok.
	gameengine.RegisterYarok(gs, perm)
	emit(gs, "yarok", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "etb_trigger_doubler_registered",
	})
}
