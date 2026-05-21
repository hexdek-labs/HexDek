package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLightningArmyOfOne wires Lightning, Army of One.
//
// Oracle text:
//
//	First strike, trample, lifelink
//	Stagger — Whenever Lightning deals combat damage to a player,
//	until your next turn, if a source would deal damage to that
//	player or a permanent that player controls, it deals double that
//	damage instead.
//
// Implementation (R54 — damage replacement primitive):
//   - First strike / trample / lifelink: AST keyword pipeline.
//   - "combat_damage_to_player" trigger gated to attacker ==
//     Lightning: register a DamageReplacement closure on
//     gs.DamageReplacements. The closure filters on
//     (TargetSeat == staggered_defender) AND the source NOT being
//     a Lightning-controller-owned source that's already in the
//     stagger chain (we just gate on the defender's seat; the
//     printed text doesn't restrict source identity beyond "a
//     source"). On match, ctx.Amount *= 2.
//   - "Until your next turn" duration: approximated as current
//     turn + len(seats), captured into the closure. When the
//     current gs.Turn exceeds the captured expiry tick, the
//     closure no-ops itself.
//   - Lightning's LTB unregisters all closures Lightning owns.
//   - A second combat hit to a different defender registers a
//     second independent replacement (the printed "if a source
//     would deal damage to that player" is a per-defender state).
func registerLightningArmyOfOne(r *Registry) {
	r.OnTrigger("Lightning, Army of One", "combat_damage_to_player", lightningStaggerArm)
	r.OnTrigger("Lightning, Army of One", "permanent_ltb", lightningLTBUnregister)
}

func lightningLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}

func lightningStaggerArm(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lightning_stagger_arm"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	defender := -1
	if v, ok := ctx["defender"].(int); ok {
		defender = v
	} else if v, ok := ctx["defender_seat"].(int); ok {
		defender = v
	} else if v, ok := ctx["target_seat"].(int); ok {
		defender = v
	}
	if defender < 0 || defender >= len(gs.Seats) {
		return
	}
	// "Until your next turn" — current turn + one full round.
	expiresOnTurn := gs.Turn + len(gs.Seats)
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "lightning_stagger_defender",
		Fn: func(gs *gameengine.GameState, dctx *gameengine.DamageContext) {
			if dctx == nil {
				return
			}
			if gs.Turn > expiresOnTurn {
				return
			}
			if dctx.TargetSeat != defender {
				return
			}
			if dctx.Amount <= 0 {
				return
			}
			dctx.Amount *= 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"defender_seat": defender,
		"expires_turn":  expiresOnTurn,
	})
}
