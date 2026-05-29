package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFrodoSauronsBane wires Frodo, Sauron's Bane (Muninn parser-gap
// #59, 12,865 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{W}
//	Legendary Creature — Halfling Citizen
//	{W/B}{W/B}: If Frodo is a Citizen, it becomes a Halfling Scout with
//	base power and toughness 2/3 and lifelink.
//	{B}{B}{B}: If Frodo is a Scout, it becomes a Halfling Rogue with
//	"Whenever this creature deals combat damage to a player, that
//	player loses the game if the Ring has tempted you four or more
//	times this game. Otherwise, the Ring tempts you."
//
// Implementation:
//   - Three forms tracked via a Permanent.Flags["frodo_form"] integer:
//     0 = Citizen (default at ETB), 1 = Halfling Scout, 2 = Halfling
//     Rogue. Once advanced the form can't go backwards.
//   - OnActivated dispatch by abilityIdx (parser orders the activated
//     abilities by appearance):
//       idx 0 → Citizen → Scout transformation if currently Citizen.
//                Set BasePower/BaseToughness to 2/3 via temp_power +
//                Flags["frodo_p_override"]/Flags["frodo_t_override"];
//                add lifelink keyword via Flags["has_lifelink"]=1.
//       idx 1 → Scout → Rogue transformation if currently Scout. Power/
//                toughness inherit the Scout's 2/3 (printed text doesn't
//                restate p/t for the Rogue form; the activated ability
//                only swaps the keyword text).
//   - combat_damage_player trigger on the Rogue form: if ring level >=
//     4, the dealt-to player loses; otherwise tempt our controller.
//   - The keyword/static-text overlay path is partial — the engine
//     doesn't fully model "becomes a Rogue with [combat-damage trigger]"
//     as a swap-in static. We track form via flags and self-fire the
//     intended trigger on combat_damage_player when the form is right.
func registerFrodoSauronsBane(r *Registry) {
	r.OnETB("Frodo, Sauron's Bane", frodoSauronsBaneETB)
	r.OnActivated("Frodo, Sauron's Bane", frodoSauronsBaneActivate)
	r.OnTrigger("Frodo, Sauron's Bane", "combat_damage_player", frodoSauronsBaneCombatDamage)
}

func frodoSauronsBaneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["frodo_form"] = 0 // Citizen
}

func frodoSauronsBaneActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "frodo_saurons_bane_class_change"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	current := src.Flags["frodo_form"]
	switch abilityIdx {
	case 0:
		// Citizen → Scout.
		if current != 0 {
			emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
				"seat":   src.Controller,
				"form":   current,
				"reason": "not_citizen_anymore",
			})
			return
		}
		src.Flags["frodo_form"] = 1
		src.Flags["frodo_p_override"] = 2
		src.Flags["frodo_t_override"] = 3
		src.Flags["has_lifelink"] = 1
		// Apply via base P/T override + tempPower compute path the
		// engine respects.
		src.Card.BasePower = 2
		src.Card.BaseToughness = 3
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      src.Controller,
			"transform": "citizen_to_scout",
			"pt":        "2/3",
			"lifelink":  true,
		})
	case 1:
		// Scout → Rogue.
		if current != 1 {
			emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
				"seat":   src.Controller,
				"form":   current,
				"reason": "not_scout",
			})
			return
		}
		src.Flags["frodo_form"] = 2
		// Form keeps Scout's 2/3 stats unless cancelled. Drop lifelink
		// per printed Rogue text (only the combat-damage trigger).
		src.Flags["has_lifelink"] = 0
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      src.Controller,
			"transform": "scout_to_rogue",
			"clauses":   "combat_damage_ring_check",
		})
	default:
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":  src.Controller,
			"ability_idx": abilityIdx,
			"reason":     "unknown_ability",
		})
	}
}

func frodoSauronsBaneCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "frodo_saurons_bane_rogue_combat_damage"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["frodo_form"] != 2 {
		// Not in Rogue form — the printed trigger doesn't exist.
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller || sourceName != perm.Card.DisplayName() {
		return
	}
	target, ok := ctx["defender_seat"].(int)
	if !ok || target < 0 || target >= len(gs.Seats) {
		return
	}
	ringLevel := gameengine.GetRingLevel(gs, perm.Controller)
	if ringLevel >= 4 {
		s := gs.Seats[target]
		if s == nil || s.Lost {
			return
		}
		// CR §104.3e: route through canonical helper so §614
		// would_lose_game (Platinum Angel / Angel's Grace) can cancel.
		gameengine.MarkSeatLostByEffect(gs, target, perm.Card.DisplayName()+" — Rogue combat damage with Ring level 4")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"target": target,
			"ring":   ringLevel,
			"effect": "target_loses_the_game",
		})
		_ = gs.CheckEnd()
		return
	}
	gameengine.TheRingTemptsYou(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target,
		"ring":   ringLevel,
		"effect": "ring_tempts_you",
	})
}
