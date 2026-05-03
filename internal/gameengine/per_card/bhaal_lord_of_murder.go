package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBhaalLordOfMurder wires Bhaal, Lord of Murder.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	As long as your life total is less than or equal to half your starting
//	life total, Bhaal has indestructible.
//	Whenever a nontoken creature you control dies, put a +1/+1 counter on
//	target creature an opponent controls. Goad that creature.
//
// Implementation:
//   - OnETB: emitPartial for the conditional indestructible clause.
//     This is a state-based continuous effect ("as long as ...") that the
//     engine's AST/layers pipeline handles; we flag the gap from the
//     per-card layer so Heimdall/Muninn can track it.
//   - OnTrigger("creature_dies"): fires when any creature dies. We gate on:
//     (a) the dying creature is controlled by Bhaal's controller
//         (ctx["controller_seat"] == perm.Controller),
//     (b) the dying creature is NOT Bhaal himself (dyingCard != perm.Card),
//     (c) the dying creature is not a token (dyingPerm.IsToken() == false).
//     Then pick the most threatening opponent creature (highest Power,
//     tiebreak by earliest timestamp), put a +1/+1 counter on it, and goad it.
//     "Goad" is modelled by setting Flags["goaded"] = 1, matching the engine's
//     CR §701.38 implementation in resolve_helpers.go.
func registerBhaalLordOfMurder(r *Registry) {
	r.OnETB("Bhaal, Lord of Murder", bhaalLordOfMurderETB)
	r.OnTrigger("Bhaal, Lord of Murder", "creature_dies", bhaalLordOfMurderCreatureDies)
}

func bhaalLordOfMurderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	// The "as long as your life total is <= half your starting life total,
	// Bhaal has indestructible" clause is a state-dependent continuous effect
	// (CR §613). It is handled by the AST/layers pipeline, not at the per-card
	// layer. Emit a partial so the coverage gap is tracked.
	emitPartial(gs, "bhaal_lord_of_murder_indestructible",
		perm.Card.DisplayName(),
		"conditional_indestructible_state_based_not_enforced_per_card")
}

func bhaalLordOfMurderCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bhaal_lord_of_murder_creature_dies"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: dying creature must be controlled by Bhaal's controller.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}

	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}

	// Gate: dying creature must not be Bhaal himself.
	if dyingCard == perm.Card {
		return
	}

	// Gate: dying creature must not be a token. Tokens cease to exist on dying
	// (CR §704.5d) so there is no "nontoken" event to care about; we still
	// check to be safe.
	if dyingPerm, _ := ctx["perm"].(*gameengine.Permanent); dyingPerm != nil {
		if dyingPerm.IsToken() {
			emitFail(gs, slug, perm.Card.DisplayName(), "token_not_nontoken", map[string]interface{}{
				"dying_card": dyingCard.DisplayName(),
			})
			return
		}
	}

	// Find best target: opponent creature with highest Power. If Power ties,
	// prefer the one with the lowest Timestamp (longest on the battlefield —
	// most likely to be an established threat). A goaded creature is forced to
	// attack and cannot attack its goad-source, making large creatures ideal
	// targets for redirecting damage to other opponents.
	target := bhaalPickGoadTarget(gs, perm.Controller)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_creature", map[string]interface{}{
			"controller":  perm.Controller,
			"dying_card":  dyingCard.DisplayName(),
		})
		return
	}

	// Put a +1/+1 counter on the target.
	if target.Counters == nil {
		target.Counters = map[string]int{}
	}
	target.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	// Fire counter_placed so observers (Shalai and Hallar, Auntie Ool, etc.)
	// see the counter going on.
	gameengine.FireCardTrigger(gs, "counter_placed", map[string]interface{}{
		"target_perm":  target,
		"target_seat":  target.Controller,
		"counter_kind": "+1/+1",
		"amount":       1,
		"source_card":  perm.Card.DisplayName(),
		"source_seat":  perm.Controller,
	})

	// Goad the target (CR §701.38): the creature attacks each combat if able
	// and attacks a player other than the goad-source if able.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["goaded"] = 1

	gs.LogEvent(gameengine.Event{
		Kind:   "goad",
		Seat:   perm.Controller,
		Target: target.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": target.Card.DisplayName(),
			"target_seat": target.Controller,
			"slug":        slug,
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"controller":   perm.Controller,
		"dying_card":   dyingCard.DisplayName(),
		"target_card":  target.Card.DisplayName(),
		"target_seat":  target.Controller,
		"target_power": target.Power(),
		"counters":     target.Counters["+1/+1"],
	})
}

// bhaalPickGoadTarget returns the most disruptive opponent creature to receive
// the +1/+1 counter and goad. Heuristic: highest Power (buffing a big attacker
// maximises chaos for the table when it is forced to attack elsewhere). Ties
// are broken by smallest Timestamp (most-established creature, less likely to
// be a newly-created blocker and more likely to be a meaningful threat).
// Lost seats are skipped.
func bhaalPickGoadTarget(gs *gameengine.GameState, controller int) *gameengine.Permanent {
	if gs == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1 << 30
	bestTS := 1<<62 - 1

	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			pw := p.Power()
			if pw > bestPower || (pw == bestPower && p.Timestamp < bestTS) {
				bestPower = pw
				bestTS = p.Timestamp
				best = p
			}
		}
	}
	return best
}
