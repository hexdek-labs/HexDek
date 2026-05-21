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
// Implementation:
//   - First strike / trample / lifelink: handled by the AST keyword
//     pipeline.
//   - "combat_damage_to_player" trigger gated to attacker == Lightning:
//     arm a "stagger" flag on gs.Flags keyed by the damaged player's
//     seat and Lightning's controller's NEXT turn (current turn + N
//     where N is the seat count, approximating "until your next turn").
//
// R51 batch H port: register a would_be_dealt_damage replacement at
// ETB that doubles damage routed at a staggered player or their
// permanents. The replacement reads the same gs.Flags
// "lightning_stagger_seat<N>_until_turn" key the arm sets and is
// active while gs.Turn is below the expiry turn.
func registerLightningArmyOfOne(r *Registry) {
	r.OnETB("Lightning, Army of One", lightningETBRegisterReplacement)
	r.OnTrigger("Lightning, Army of One", "combat_damage_to_player", lightningStaggerArm)
}

func lightningETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lightning_army_of_one_etb_stagger_replacement"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_be_dealt_damage",
		HandlerID:      "Lightning, Army of One:stagger:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil || ev.Count() <= 0 {
				return false
			}
			// Determine the seat the damage routes to.
			ts := ev.TargetSeat
			if ev.TargetPerm != nil {
				ts = ev.TargetPerm.Controller
			}
			if ts < 0 || ts >= len(gs.Seats) {
				return false
			}
			if gs.Flags == nil {
				return false
			}
			key := fmt.Sprintf("lightning_stagger_seat%d_until_turn", ts)
			expires := gs.Flags[key]
			if expires == 0 || gs.Turn >= expires {
				return false
			}
			return true
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			before := ev.Count()
			ev.SetCount(before * 2)
			ts := ev.TargetSeat
			if ev.TargetPerm != nil {
				ts = ev.TargetPerm.Controller
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Lightning, Army of One",
				Amount: ev.Count(),
				Details: map[string]interface{}{
					"slug":          slug,
					"rule":          "614",
					"effect":        "stagger_double_damage",
					"target_seat":   ts,
					"before":        before,
					"after":         ev.Count(),
				},
			})
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_be_dealt_damage",
	})
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
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Approximate "until your next turn" as current turn + len(seats),
	// since each seat takes one turn before the controller's next turn.
	expiresOnTurn := gs.Turn + len(gs.Seats)
	key := fmt.Sprintf("lightning_stagger_seat%d_until_turn", defender)
	gs.Flags[key] = expiresOnTurn
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"defender_seat":  defender,
		"expires_turn":   expiresOnTurn,
	})
	// Damage-doubling replacement is wired by lightningETBRegisterReplacement.
}
