package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// cyber_conversion.go — per_card handler for Cyber Conversion (Doctor Who,
// 2023).
//
// Oracle text (Instant, {2}{U}):
//
//	Turn target creature face down. (It's a 2/2 Cyberman artifact creature.)
//
// Why a per_card handler: "turn target creature face down" had no engine
// primitive — no mint path turned an EXISTING battlefield creature face
// down, so this spell was inert. Piece 2 of the unified face-down overlay
// redesign adds gameengine.TurnPermanentFaceDown, which keeps the real card
// as the permanent of record under the "cyber" template (2/2 colorless
// Cyberman artifact creature, not hidden, no controller turn-up).
//
// Mode policy (AI): shrinking the opponent's biggest threat to a vanilla
// 2/2 (stripping its abilities) is the point of the card, so target the
// highest-power opponent creature. Fizzles only when no opponent controls a
// creature.
func init() {
	registerCyberConversion(Global())
	AddResetHook(registerCyberConversion)
}

func registerCyberConversion(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Cyber Conversion", cyberConversionResolve)
}

func cyberConversionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cyber_conversion"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	target := bestOpponentCreatureByPower(gs, seat)
	if target == nil {
		emitFail(gs, slug, "Cyber Conversion", "no_legal_target", nil)
		return
	}
	name := target.Card.DisplayName()
	if !gameengine.TurnPermanentFaceDown(gs, target, "cyber") {
		// Already face down (or invalid) — nothing legal to do.
		emitFail(gs, slug, "Cyber Conversion", "target_already_face_down", map[string]interface{}{
			"target": name,
		})
		return
	}
	emit(gs, slug, "Cyber Conversion", map[string]interface{}{
		"seat": seat, "target": name, "template": "cyber",
	})
	_ = gs.CheckEnd()
}

// bestOpponentCreatureByPower returns the highest-power creature an
// opponent of `controller` controls, skipping any that are already face
// down (turning a 2/2 face-down creature face down again is a no-op).
func bestOpponentCreatureByPower(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	var best *gameengine.Permanent
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Card.FaceDown {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	return best
}
