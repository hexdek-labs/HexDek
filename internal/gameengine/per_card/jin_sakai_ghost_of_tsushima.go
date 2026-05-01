package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJinSakaiGhostOfTsushima wires Jin Sakai, Ghost of Tsushima.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Whenever Jin Sakai deals combat damage to a player, draw a card.
//	Whenever a creature you control attacks a player, if no other
//	creatures are attacking that player, choose one —
//	  • Standoff — It gains double strike until end of turn.
//	  • Ghost — It can't be blocked this turn.
//
// Implementation:
//   - "combat_damage_player" trigger: source must be Jin Sakai → draw.
//   - "creature_attacks" trigger: when an attacker controlled by Jin
//     Sakai's controller targets a defender alone (no other declared
//     attacker has the same defender seat), pick the better mode and
//     stamp the runtime keyword flag on the attacker. The choice
//     heuristic prefers Standoff (double strike) for raw damage but
//     flips to Ghost (unblockable) when the attacker already has
//     double strike — Ghost is otherwise functionally equivalent or
//     worse versus an unblocked attacker.
func registerJinSakaiGhostOfTsushima(r *Registry) {
	r.OnTrigger("Jin Sakai, Ghost of Tsushima", "combat_damage_player", jinSakaiCombatDamageDraw)
	r.OnTrigger("Jin Sakai, Ghost of Tsushima", "creature_attacks", jinSakaiAttackModal)
}

func jinSakaiCombatDamageDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jin_sakai_combat_damage_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != perm.Card.DisplayName() {
		return
	}
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"drawn_card": drawnName,
	})
}

func jinSakaiAttackModal(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jin_sakai_attack_modal"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Controller != perm.Controller {
		return
	}
	defenderSeat, ok := gameengine.AttackerDefender(atk)
	if !ok {
		return
	}

	// "if no other creatures are attacking that player"
	otherAttackers := 0
	seat := gs.Seats[perm.Controller]
	if seat != nil {
		for _, p := range seat.Battlefield {
			if p == nil || p == atk {
				continue
			}
			if p.Flags == nil || p.Flags["attacking"] != 1 {
				continue
			}
			d, has := gameengine.AttackerDefender(p)
			if !has || d != defenderSeat {
				continue
			}
			otherAttackers++
		}
	}
	if otherAttackers > 0 {
		return
	}

	if atk.Flags == nil {
		atk.Flags = map[string]int{}
	}
	mode := "standoff"
	if atk.HasKeyword("double strike") {
		mode = "ghost"
	}
	switch mode {
	case "standoff":
		atk.Flags["kw:double strike"] = 1
	case "ghost":
		atk.Flags["unblockable"] = 1
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker":      atk.Card.DisplayName(),
		"defender_seat": defenderSeat,
		"mode":          mode,
	})
}
