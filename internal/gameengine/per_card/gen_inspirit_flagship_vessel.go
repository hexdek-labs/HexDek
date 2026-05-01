package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInspiritFlagshipVessel wires Inspirit, Flagship Vessel.
//
// Oracle text:
//
//   Station (Tap another creature you control: Put charge counters equal to its power on this Spacecraft. Station only as a sorcery. It's an artifact creature at 8+.)
//   1+ | At the beginning of combat on your turn, put your choice of a +1/+1 counter or two charge counters on up to one other target artifact.
//   8+ | Flying
//   Other artifacts you control have hexproof and indestructible.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerInspiritFlagshipVessel(r *Registry) {
	r.OnETB("Inspirit, Flagship Vessel", inspiritFlagshipVesselStaticETB)
}

func inspiritFlagshipVesselStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "inspirit_flagship_vessel_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
