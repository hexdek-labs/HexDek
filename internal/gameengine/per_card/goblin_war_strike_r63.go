package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// goblin_war_strike_r63.go — per_card handler for Goblin War Strike.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Goblin War Strike deals damage to target player or planeswalker
//	equal to the number of Goblins you control.
//
// {R} Sorcery. The "count your Goblins → burn a player" finisher. Parses
// to an inert fallback node (no structured damage), so it dealt ZERO. The
// runtime text fallback has no "deals damage equal to a count" shape, so
// the spell was fully DOA.
//
// Implementation: OnResolve (authoritative — stock dispatch is skipped
// once a resolve hook fires). Count permanents the caster controls with
// the Goblin subtype (layer-aware gs.HasSubtypeOf), deal that much to the
// highest-life opponent (the canonical player pick when no creature/PW
// target is stamped — mirrors Inspired Ultimatum's policy).
//
// NOTE: subtypes are not split into Characteristics.Subtypes by the base
// layer in this engine — the Goblin subtype token lives in the effective
// Types slice, so the count uses gs.HasTypeOf(p, "goblin") (layer-aware),
// not gs.HasSubtypeOf (which reads the rarely-populated Subtypes slice).
func init() {
	registerGoblinWarStrikeR63(Global())
	AddResetHook(registerGoblinWarStrikeR63)
}

func registerGoblinWarStrikeR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Goblin War Strike", goblinWarStrikeResolve)
}

func goblinWarStrikeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "goblin_war_strike"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	goblins := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil {
			continue
		}
		if gs.HasTypeOf(p, "goblin") {
			goblins++
		}
	}

	target := highestLifeOpponent(gs, seat)
	if target >= 0 && goblins > 0 {
		gameengine.DealDamage(gs, target, goblins, "Goblin War Strike")
	}
	emit(gs, slug, "Goblin War Strike", map[string]interface{}{
		"seat": seat, "goblins": goblins, "target": target,
	})
}
