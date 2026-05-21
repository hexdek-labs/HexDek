package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCecilDarkKnightCecilRedeemedPaladin wires Cecil, Dark Knight // Cecil, Redeemed Paladin.
//
// Oracle text:
//
//	Front face — Cecil, Dark Knight:
//	  Deathtouch
//	  Darkness — Whenever Cecil deals damage, you lose that much
//	  life. Then if your life total is less than or equal to half
//	  your starting life total, untap Cecil and transform it.
//
//	Back face — Cecil, Redeemed Paladin:
//	  Lifelink
//	  Protect — Whenever Cecil attacks, other attacking creatures
//	  gain indestructible until end of turn.
//
// Implementation:
//   - Deathtouch / lifelink: handled by the AST keyword pipeline.
//   - "combat_damage_to_player" + "noncombat_damage_to_player" +
//     "noncombat_damage_to_creature" triggers gated to source ==
//     Cecil himself: drain Cecil's controller for the damage amount,
//     then if life ≤ StartingLife/2, untap Cecil and transform.
//   - "creature_attacks" trigger gated to attacker == Cecil (back
//     face): grant indestructible to all OTHER attacking creatures
//     we control until end of turn.
//
// emitPartial: full damage-event coverage requires the engine to
// expose a unified "damage_dealt_by_source" feed; we wire the three
// most common damage-to-target events.
func registerCecilDarkKnightCecilRedeemedPaladin(r *Registry) {
	const name = "Cecil, Dark Knight // Cecil, Redeemed Paladin"
	r.OnTrigger(name, "combat_damage_to_player", cecilDamageHook)
	r.OnTrigger(name, "noncombat_damage_to_player", cecilDamageHook)
	r.OnTrigger(name, "noncombat_damage_to_creature", cecilDamageHook)
	r.OnTrigger(name, "creature_attacks", cecilAttackProtect)
	// Bind under each face so triggers dispatched by the engine's
	// face-only Card.Name (e.g., after a transform) still find their
	// handlers. R53 batch O extends the attack-trigger aliases the R44
	// port added with the three damage triggers — without these, a
	// Cecil whose Card.Name resolves to "Cecil, Dark Knight" or
	// "Cecil, Redeemed Paladin" alone would silently skip the
	// Darkness self-drain (lookupCandidates strips DFC composite to
	// front but does NOT compose front-only back to composite).
	r.OnTrigger("Cecil, Dark Knight", "creature_attacks", cecilAttackProtect)
	r.OnTrigger("Cecil, Redeemed Paladin", "creature_attacks", cecilAttackProtect)
	r.OnTrigger("Cecil, Dark Knight", "combat_damage_to_player", cecilDamageHook)
	r.OnTrigger("Cecil, Dark Knight", "noncombat_damage_to_player", cecilDamageHook)
	r.OnTrigger("Cecil, Dark Knight", "noncombat_damage_to_creature", cecilDamageHook)
	r.OnTrigger("Cecil, Redeemed Paladin", "combat_damage_to_player", cecilDamageHook)
	r.OnTrigger("Cecil, Redeemed Paladin", "noncombat_damage_to_player", cecilDamageHook)
	r.OnTrigger("Cecil, Redeemed Paladin", "noncombat_damage_to_creature", cecilDamageHook)
}

func cecilDamageHook(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cecil_darkness_self_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	src, _ := ctx["source_perm"].(*gameengine.Permanent)
	if src == nil {
		src, _ = ctx["attacker_perm"].(*gameengine.Permanent)
	}
	if src != perm {
		return
	}
	amt, _ := ctx["amount"].(int)
	if amt <= 0 {
		return
	}
	gameengine.LoseLife(gs, perm.Controller, amt, "Cecil darkness")
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	startingLife := seat.StartingLife
	if startingLife <= 0 {
		startingLife = 40
	}
	if seat.Life <= startingLife/2 && !perm.Transformed {
		perm.Tapped = false
		gameengine.TransformPermanent(gs, perm, "cecil_darkness_threshold")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"life_lost":   amt,
		"transformed": perm.Transformed,
	})
}

func cecilAttackProtect(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cecil_protect_indestructible"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	// Only fires from the back face (Redeemed Paladin), per oracle.
	if !perm.Transformed {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var granted []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() || !p.IsAttacking() {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["kw:indestructible"] == 0 {
			p.Flags["kw:indestructible"] = 1
			granted = append(granted, p)
		}
	}
	if len(granted) > 0 {
		gs.InvalidateCharacteristicsCache()
		captured := granted
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				for _, p := range captured {
					if p != nil && p.Flags != nil {
						delete(p.Flags, "kw:indestructible")
					}
				}
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"granted": len(granted),
	})
}
