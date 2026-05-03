package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArahboRoarOfTheWorld wires Arahbo, Roar of the World.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Eminence — At the beginning of combat on your turn, if Arahbo, Roar
//	of the World is in the command zone or on the battlefield, another
//	target Cat you control gets +3/+3 until end of turn.
//	Whenever another Cat you control attacks, if Arahbo, Roar of the
//	World is on the battlefield, you may pay {1}{G}{W}. If you do, that
//	Cat gains trample and gets +X/+X until end of turn, where X is its
//	power.
//
// Implementation:
//   - "combat_begin": at the start of combat on Arahbo's controller's
//     turn, grant +3/+3 until end of turn to the highest-power other Cat
//     on the battlefield controlled by the same player. The eminence
//     clause (works from command zone) is not dispatched through the
//     standard path — OnTrigger only fires while Arahbo is on the
//     battlefield. The architectural note from Edgar Markov applies here
//     too: command-zone eminence is not tracked; emitPartial flags the
//     gap.
//   - "creature_attacks": when another Cat controlled by Arahbo's
//     controller attacks, if Arahbo is on the battlefield (implicit —
//     the trigger only fires while Arahbo is in play), apply the
//     optional {1}{G}{W} payment. AI decision is always "yes" (pure
//     upside). The +X/+X boost uses the Cat's current power BEFORE the
//     modification is applied (power at trigger time, per oracle). The
//     trample keyword is granted and cleaned up via a delayed "next_end_step"
//     trigger so it expires correctly at cleanup.
//
// emitPartial: command-zone eminence not tracked (standard dispatcher
// limitation — trigger fires for battlefield permanents only).
func registerArahboRoarOfTheWorld(r *Registry) {
	r.OnTrigger("Arahbo, Roar of the World", "combat_begin", arahboCombatBegin)
	r.OnTrigger("Arahbo, Roar of the World", "creature_attacks", arahboCatAttacks)
}

// arahboCombatBegin implements the eminence +3/+3 at beginning of combat.
func arahboCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "arahbo_roar_of_the_world_eminence_combat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	// Eminence note: the standard TriggerHook only fires for on-battlefield
	// permanents. Command-zone eminence is not dispatched here.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"command_zone_eminence_not_tracked")

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Pick the best "another Cat" — highest power, excluding Arahbo himself.
	var best *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == perm {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "cat") {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}
	if best == nil {
		return
	}

	best.Modifications = append(best.Modifications, gameengine.Modification{
		Power:     3,
		Toughness: 3,
		Duration:  "until_end_of_turn",
	})
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
		"boost":  "+3/+3",
	})
}

// arahboCatAttacks implements the optional {1}{G}{W}: trample + +X/+X when
// another Cat attacks.
func arahboCatAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "arahbo_roar_of_the_world_cat_attack_boost"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk == perm {
		return
	}
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	if atk.Card == nil {
		return
	}
	if !cardHasType(atk.Card, "cat") {
		return
	}

	// X = attacker's power at trigger time (before the modification lands).
	x := atk.Power()
	if x <= 0 {
		// +0/+0 and no meaningful trample boost — skip.
		return
	}

	// AI always pays {1}{G}{W} (pure upside, no cost model in sim).
	atk.Modifications = append(atk.Modifications, gameengine.Modification{
		Power:     x,
		Toughness: x,
		Duration:  "until_end_of_turn",
	})

	// Grant trample until end of turn via kw flag + delayed cleanup.
	if atk.Flags == nil {
		atk.Flags = map[string]int{}
	}
	atk.Flags["kw:trample"] = 1

	captured := atk
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:trample")
		},
	})

	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"cat":     atk.Card.DisplayName(),
		"x":       x,
		"boost":   "+X/+X",
		"trample": true,
	})
}
