package per_card

import (
	"fmt"
	"strconv"

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
	gs.UnregisterReplacementsForPermanent(perm)
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
	// Combat-path replacement (gs.DamageReplacements is consulted by
	// combat.go's deal-damage routine).
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "lightning_stagger_defender",
		Fn: func(g *gameengine.GameState, dctx *gameengine.DamageContext) {
			if dctx == nil {
				return
			}
			if g.Turn > expiresOnTurn {
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
	// Generic-event replacement (gs.Replacements via FireEvent; consulted
	// by FireDamageEvent and any other "would_be_dealt_damage" emitter).
	// R60 followup: the combat-path replacement above isn't seen by the
	// generic FireEvent dispatcher — register a sibling ReplacementEffect
	// so non-combat damage routed through FireDamageEvent (instants,
	// triggered abilities) also gets doubled while the stagger is armed.
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_be_dealt_damage",
		HandlerID:      "Lightning, Army of One:stagger_double:" + strconv.Itoa(perm.Timestamp) + ":seat" + strconv.Itoa(defender),
		SourcePerm:     perm,
		ControllerSeat: perm.Controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(g *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Count() <= 0 {
				return false
			}
			if g.Turn > expiresOnTurn {
				return false
			}
			return ev.TargetSeat == defender
		},
		ApplyFn: func(g *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.SetCount(ev.Count() * 2)
		},
	})
	// Diagnostic flag so audit tooling (and TestStubsBatchH_Lightning)
	// can see the active stagger window per defender seat.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags[fmt.Sprintf("lightning_stagger_seat%d_until_turn", defender)] = expiresOnTurn
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"defender_seat": defender,
		"expires_turn":  expiresOnTurn,
	})
}
