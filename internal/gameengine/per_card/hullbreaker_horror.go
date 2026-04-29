package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHullbreakerHorror wires up Hullbreaker Horror.
//
// Oracle text:
//
//	Flash. Ward {3}.
//	This spell can't be countered.
//	Whenever you cast another spell, return target nonland permanent
//	to its owner's hand.
//
// Y'shtola + Kinnan combo piece. Gets unbounded value from "whenever
// you cast" loops — paired with Kinnan (untap mana rocks), every cast
// bounces a permanent (ideally an opponent's problem).
//
// Implementation:
//   - OnTrigger("spell_cast") — when the Horror's controller casts
//     another spell, pick a target nonland permanent (ideally an
//     opponent's) and bounce it.
//   - Target policy: prefer opponents' nonland permanents; if none,
//     bounce a friendly (Cloudstone-style re-play loop).
func registerHullbreakerHorror(r *Registry) {
	r.OnTrigger("Hullbreaker Horror", "spell_cast", hullbreakerHorrorOnCast)
}

func hullbreakerHorrorOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hullbreaker_horror_bounce"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	// "Another spell" — the Horror itself is an exception when it's
	// the spell on the stack. We approximate by comparing the spell
	// name; if the cast was the Horror itself (entering as a spell),
	// skip.
	if spellName, _ := ctx["spell_name"].(string); spellName == perm.Card.DisplayName() {
		return
	}

	seat := perm.Controller
	// Pick target: first opponent's highest-timestamp nonland
	// permanent (their newest threat/mana rock).
	var target *gameengine.Permanent
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.IsLand() {
				continue
			}
			if target == nil || p.Timestamp > target.Timestamp {
				target = p
			}
		}
	}
	if target == nil {
		// Fall back to friendly (enables re-play loops).
		for _, p := range gs.Seats[seat].Battlefield {
			if p == nil || p == perm || p.IsLand() {
				continue
			}
			if target == nil || p.Timestamp < target.Timestamp {
				target = p
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_bounce_target", nil)
		return
	}
	// Route through BouncePermanent for proper zone-change handling:
	// replacement effects, LTB triggers, commander redirect.
	gameengine.BouncePermanent(gs, target, perm, "hand")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"bounced_card": target.Card.DisplayName(),
		"owner_seat":   target.Owner,
	})
}
