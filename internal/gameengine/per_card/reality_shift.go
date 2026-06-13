package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Reality Shift — {1}{U} Instant.
//
//	Exile target creature. Its controller manifests the top card of their
//	library.
//
// A widely-played blue removal staple (80 decks in the pool). The whole
// effect parsed to two inert `custom` slugs, so it resolved to a no-op —
// the targeted creature stayed put. This handler exiles the target and has
// ITS CONTROLLER (not the caster) manifest the top card.
func init() {
	registerRealityShift(Global())
	AddResetHook(registerRealityShift)
}

func registerRealityShift(r *Registry) {
	r.OnResolve("Reality Shift", realityShiftResolve)
}

func realityShiftResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "reality_shift"
	if gs == nil || item == nil {
		return
	}
	var target *gameengine.Permanent
	for _, t := range item.Targets {
		if t.Kind == gameengine.TargetKindPermanent && t.Permanent != nil && t.Permanent.IsCreature() {
			target = t.Permanent
			break
		}
	}
	if target == nil || target.Card == nil {
		emitFail(gs, slug, "Reality Shift", "no_creature_target", nil)
		return
	}
	controller := target.Controller
	name := target.Card.DisplayName()
	if !gameengine.ExilePermanent(gs, target, nil) {
		emitFail(gs, slug, "Reality Shift", "exile_failed", nil)
		return
	}
	manifested := gameengine.ApplyManifestTop(gs, controller, 1)
	emit(gs, slug, "Reality Shift", map[string]interface{}{
		"exiled":           name,
		"controller":       controller,
		"manifested_cards": manifested,
	})
}
