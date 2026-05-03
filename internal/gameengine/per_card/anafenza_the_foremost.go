package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnafenzaTheForemost wires Anafenza, the Foremost.
//
// Oracle text (Khans of Tarkir, {W}{B}{G}, 4/4 Legendary Creature —
// Human Soldier):
//
//	Whenever Anafenza, the Foremost attacks, put a +1/+1 counter on
//	another target tapped creature you control.
//	If a nontoken creature an opponent controls would die, exile it instead.
//
// Implementation:
//   - OnTrigger("creature_attacks"): gate on atk == perm (Anafenza herself
//     is attacking). Find the best "another tapped creature you control"
//     — highest-power tapped creature on Anafenza's controller's
//     battlefield that isn't Anafenza. Place one +1/+1 counter on it via
//     AddCounter and fire counter_placed so proliferate observers and
//     Shalai/Hallar-style triggers see the event.
//   - Replacement effect ("nontoken creature an opponent controls would die
//     → exile instead"): this is a static replacement effect, not a
//     triggered ability. Replacement effects are not dispatched through the
//     per-card trigger path. emitPartial flags the gap; the AST engine or
//     a future replacement-effect framework would need to handle it.
//
// emitPartial: death-replacement (nontoken opp-creature exile instead)
// not implemented — requires replacement-effect framework.
func registerAnafenzaTheForemost(r *Registry) {
	r.OnTrigger("Anafenza, the Foremost", "creature_attacks", anafenzaTheForemost)
}

func anafenzaTheForemost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anafenza_the_foremost_attack_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: trigger only fires when Anafenza herself is the attacker.
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Find the best "another tapped creature you control" — highest power
	// among tapped creatures on the battlefield that are not Anafenza.
	var best *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !p.Tapped {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}

	if best == nil {
		// No valid target — Anafenza's trigger fizzles (no other tapped
		// creature you control). Log and emit the partial for the
		// death-replacement clause.
		emitFail(gs, slug, perm.Card.DisplayName(),
			"no_other_tapped_creature_you_control", nil)
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"death_replacement_exile_nontoken_opp_creature_not_implemented")
		return
	}

	// Place one +1/+1 counter on the target.
	best.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	// Fire counter_placed so engine observers (Shalai and Hallar, etc.)
	// see the event.
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_placed",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Target: best.Controller,
		Details: map[string]interface{}{
			"slug":         slug,
			"target_perm":  best.Card.DisplayName(),
			"counter_type": "+1/+1",
			"amount":       1,
			"total":        best.Counters["+1/+1"],
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target":       best.Card.DisplayName(),
		"counter_type": "+1/+1",
		"amount":       1,
		"total":        best.Counters["+1/+1"],
	})

	// The exile-instead-of-die replacement effect is not implemented here.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"death_replacement_exile_nontoken_opp_creature_not_implemented")
}
