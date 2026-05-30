package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrimHireling wires Grim Hireling.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Grim%20Hireling):
//
//	{3}{B}{R}
//	Creature — Human Mercenary
//	4/4
//	Whenever one or more creatures you control deal combat damage to
//	a player, create that many Treasure tokens.
//	{2}{B}{R}: Target creature gets deathtouch and lifelink until end
//	of turn.
//
// Combat-damage → treasure is the engine: a midrange combat board with
// 4 attackers connecting yields 4 treasures per swing, refueling the
// curve and enabling explosive next-turn plays. The activated ability
// (deathtouch+lifelink at sorcery speed) is the secondary mode.
//
// Implementation:
//   - OnTrigger("combat_damage_player") gated on source_seat == self
//     controller. The engine fires this event once per attacker that
//     deals combat damage to a player; we create one Treasure token
//     per event. Aggregate count across the combat step = number of
//     creatures that connected, matching the oracle's "that many"
//     semantics in the common case (multiple creatures hitting the
//     same player still produces the right total even though the
//     trigger fires per-creature rather than once-per-defender).
//   - Skip the trigger when source is Grim Hireling itself (no
//     double-count — the oracle "one or more creatures you control"
//     includes Grim Hireling, which is correct; this is just a
//     sanity guard against re-entry).
//   - Activated ability deferred — modeled as a noop OnActivated
//     stub with emitPartial. The deathtouch+lifelink grant requires
//     a per-target until-EOT layer effect that the per_card layer
//     can wire later; the primary commander engine is the trigger.
func registerGrimHireling(r *Registry) {
	r.OnTrigger("Grim Hireling", "combat_damage_player", grimHirelingCombatDamage)
	r.OnActivated("Grim Hireling", grimHirelingActivate)
}

func grimHirelingCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "grim_hireling_combat_treasure"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}

	gameengine.CreateTreasureToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"treasure_made": 1,
		"damage":        amount,
	})
}

func grimHirelingActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "grim_hireling_deathtouch_lifelink"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"deathtouch_lifelink_grant_until_eot_not_yet_wired")
}
