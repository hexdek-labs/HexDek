package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTajicBladeOfTheLegion wires Tajic, Blade of the Legion.
//
// Oracle text:
//
//	Indestructible
//	Battalion — Whenever Tajic and at least two other creatures attack,
//	Tajic gets +5/+5 until end of turn.
//
// Implementation:
//   - "creature_attacks" trigger gated on Tajic himself. On each fire,
//     count Tajic's controller's attacking creatures across the
//     battlefield. If >= 3 attackers (Tajic + 2 others), pump +5/+5
//     until end of turn via runtime flags. Idempotent per turn — once
//     Tajic has the buff this turn, additional triggers are no-ops.
//   - Indestructible static handled by AST keyword pipeline.
func registerTajicBladeOfTheLegion(r *Registry) {
	r.OnETB("Tajic, Blade of the Legion", tajicBladeOfTheLegionETB)
	r.OnTrigger("Tajic, Blade of the Legion", "creature_attacks", tajicBladeOfTheLegionAttacks)
}

func tajicBladeOfTheLegionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tajic_blade_etb"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["indestructible"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func tajicBladeOfTheLegionAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tajic_battalion_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["tajic_battalion_pumped_this_turn"] == 1 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	attacking := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Flags != nil && p.Flags["attacking"] == 1 {
			attacking++
		}
	}
	if attacking < 3 {
		return
	}
	pumpUntilEOT(gs, perm, 5, 5)
	perm.Flags["plus_power_until_eot"] += 5
	perm.Flags["plus_toughness_until_eot"] += 5
	perm.Flags["tajic_battalion_pumped_this_turn"] = 1
	gs.InvalidateCharacteristicsCache()
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if perm.Flags == nil {
				return
			}
			delete(perm.Flags, "tajic_battalion_pumped_this_turn")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"attackers": attacking,
		"buff":      "+5/+5 EOT",
	})
}
