package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVolatileStormdrake wires Volatile Stormdrake (Muninn parser-gap
// rank ~159, Aetherdrift cheap removal/steal).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{1}{U}
//	Creature — Drake
//	Flying, hexproof from activated and triggered abilities
//	When this creature enters, exchange control of this creature and
//	target creature an opponent controls. If you do, you get
//	{E}{E}{E}{E}, then sacrifice that creature unless you pay an amount
//	of {E} equal to its mana value.
//
// Implementation:
//   - Flying via AST keyword pipeline. "Hexproof from activated and
//     triggered abilities" is a narrow hexproof keyword variant the engine
//     does not yet model — emitPartial.
//   - OnETB: pick best opponent creature to steal — highest power, then
//     CMC; only consider opponents who control at least one creature
//     legally retargetable. Swap controllers (Volatile Stormdrake →
//     opponent, target → us). Stamp 4 energy onto the controller's
//     EnergyPool flag, then attempt to "pay {E} = target.CMC" — if the
//     controller has enough energy after the +4, debit it and keep the
//     creature; otherwise sacrifice the just-acquired creature back to
//     its now-controller (us). The engine has no first-class energy
//     pool yet; we approximate with seat.Flags["energy"].
//   - If no opponent creature exists, the ETB fizzles (Volatile
//     Stormdrake stays, no exchange).
func registerVolatileStormdrake(r *Registry) {
	r.OnETB("Volatile Stormdrake", volatileStormdrakeETB)
}

func volatileStormdrakeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "volatile_stormdrake_exchange"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	mySeat := perm.Controller
	var target *gameengine.Permanent
	bestScore := -1
	var oppSeat int
	for _, opp := range gs.Opponents(mySeat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			score := p.Power()*10 + cardCMC(p.Card)
			if score > bestScore {
				bestScore = score
				target = p
				oppSeat = opp
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_creature", map[string]interface{}{
			"seat": mySeat,
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"hexproof_from_activated_and_triggered_abilities_unwired_pending_partial_hexproof_modelling")
		return
	}
	mine := gs.Seats[mySeat]
	theirs := gs.Seats[oppSeat]
	if mine == nil || theirs == nil {
		return
	}
	// Exchange controllers.
	for i, p := range mine.Battlefield {
		if p == perm {
			mine.Battlefield = append(mine.Battlefield[:i], mine.Battlefield[i+1:]...)
			break
		}
	}
	for i, p := range theirs.Battlefield {
		if p == target {
			theirs.Battlefield = append(theirs.Battlefield[:i], theirs.Battlefield[i+1:]...)
			break
		}
	}
	perm.Controller = oppSeat
	perm.Timestamp = gs.NextTimestamp()
	target.Controller = mySeat
	target.Timestamp = gs.NextTimestamp()
	theirs.Battlefield = append(theirs.Battlefield, perm)
	mine.Battlefield = append(mine.Battlefield, target)
	gs.InvalidateCharacteristicsCache()

	// Energy: +4, then pay target.CMC or sac.
	if mine.Flags == nil {
		mine.Flags = map[string]int{}
	}
	mine.Flags["energy"] += 4
	required := cardCMC(target.Card)
	kept := false
	if mine.Flags["energy"] >= required {
		mine.Flags["energy"] -= required
		kept = true
	} else {
		// Insufficient energy — sacrifice the just-acquired creature.
		gameengine.SacrificePermanent(gs, target, "volatile_stormdrake_no_energy")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          mySeat,
		"exchanged_to":  oppSeat,
		"acquired":      target.Card.DisplayName(),
		"required_e":    required,
		"energy_after":  mine.Flags["energy"],
		"kept_acquired": kept,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"hexproof_from_activated_and_triggered_abilities_unwired_pending_partial_hexproof_modelling")
}
