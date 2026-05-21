package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCurseOfBloodletting wires Curse of Bloodletting.
//
// Oracle text (Scryfall, verified — Innistrad, {3}{R}):
//
//	Enchant player
//	If a source would deal damage to enchanted player, it deals
//	double that damage to that player instead.
//
// Implementation (R55 — damage replacement primitive):
//   - This is an Aura attached to a player. HexDek's Permanent.AttachedTo
//     only models permanent-targets cleanly, so we use a seat-level
//     flag (perm.Flags["enchanted_player_seat"]) populated by the
//     ETB. AI policy: target the lowest-life opponent (most leverage
//     on the doubling).
//   - ETB registers a damage-replacement closure that doubles damage
//     when ctx.TargetSeat matches the enchanted seat AND it's player
//     damage (DamageCombatPlayer / DamageNonCombatPlayer). Damage to
//     the enchanted player's permanents is NOT doubled (printed text:
//     "deal damage to enchanted PLAYER").
//   - LTB unregisters via UnregisterDamageReplacementsForPermanent.
//   - Multiple Curses (one per attached player) stack via independent
//     closures keyed off the seat flag.
func registerCurseOfBloodletting(r *Registry) {
	r.OnETB("Curse of Bloodletting", curseOfBloodlettingETBAttachAndRegister)
	r.OnTrigger("Curse of Bloodletting", "permanent_ltb", curseOfBloodlettingLTBUnregister)
}

func curseOfBloodlettingETBAttachAndRegister(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "curse_of_bloodletting_double_player_damage"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// AI policy: pick the lowest-life opponent.
	target := -1
	bestLife := 1 << 30
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = i
		}
	}
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_to_curse", nil)
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["curse_enchanted_player_seat"] = target + 1
	cursedSeat := target

	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "curse_of_bloodletting_double_to_enchanted_player",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Amount <= 0 {
				return
			}
			// Only fires for damage to the enchanted PLAYER, not to
			// the player's permanents.
			if ctx.Kind != gameengine.DamageCombatPlayer && ctx.Kind != gameengine.DamageNonCombatPlayer {
				return
			}
			if ctx.TargetSeat != cursedSeat {
				return
			}
			ctx.Amount *= 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"cursed_seat":  cursedSeat,
		"cursed_life":  bestLife,
	})
}

func curseOfBloodlettingLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}
