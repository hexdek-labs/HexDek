package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTroubleInPairs wires Trouble in Pairs.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Trouble%20in%20Pairs):
//
//	If an opponent would begin an extra turn, that player skips that
//	turn instead.
//	Whenever an opponent attacks you with two or more creatures, draws
//	their second card each turn, or casts their second spell each
//	turn, you draw a card.
//
// {2}{W}{W} Enchantment. Premium WUBRG-staple-class draw engine from
// MH3. Three independent trigger arms; the extra-turn-skip clause is
// a replacement effect that the engine doesn't yet model per-seat
// (gs.Flags["extra_turns_pending"] is global, no per-seat origin tag),
// so that clause emits a partial here and is the documented gap.
//
// Implementation:
//   - OnTrigger("card_drawn"): if drawer is an opponent AND this is
//     the 2nd card drawn this turn (ctx["nth_this_turn"] == 2), draw.
//   - OnTrigger("spell_cast"): if caster is an opponent AND this cast
//     bumps that seat's per-turn cast count to exactly 2, draw.
//   - OnTrigger("creature_attacks"): scan the attacker's battlefield
//     for declared attackers targeting controller; on the FIRST
//     attacker that pushes the count to >= 2 in this combat, draw.
//     A per-turn-per-attacker flag (trouble_in_pairs_attack_fired_*)
//     gates against repeated fires across multiple-attacker declares.
//
// The "exactly 2" gate on draws and casts mirrors the oracle's
// "their second card / their second spell" phrasing — the trigger
// fires once on the 2nd event, not on the 3rd+ event. This matches
// the rules text's per-event-count reading.
func registerTroubleInPairs(r *Registry) {
	r.OnETB("Trouble in Pairs", troubleInPairsETB)
	r.OnTrigger("Trouble in Pairs", "card_drawn", troubleInPairsOnDraw)
	r.OnTrigger("Trouble in Pairs", "spell_cast", troubleInPairsOnCast)
	r.OnTrigger("Trouble in Pairs", "creature_attacks", troubleInPairsOnAttack)
}

func troubleInPairsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "trouble_in_pairs_etb"
	if gs == nil || perm == nil {
		return
	}
	// Document the extra-turn-skip clause as unimplemented (engine
	// doesn't yet expose per-seat extra-turn-pending tagging).
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"opponent_extra_turn_skip_replacement")
}

func troubleInPairsOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trouble_in_pairs_draw"
	if gs == nil || perm == nil {
		return
	}
	drawer, _ := ctx["drawer_seat"].(int)
	if drawer == perm.Controller {
		return
	}
	if drawer < 0 || drawer >= len(gs.Seats) {
		return
	}
	nth, _ := ctx["nth_this_turn"].(int)
	if nth != 2 {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"drawer_seat": drawer,
		"trigger_arm": "second_draw",
	})
}

func troubleInPairsOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trouble_in_pairs_cast"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster == perm.Controller {
		return
	}
	if caster < 0 || caster >= len(gs.Seats) {
		return
	}
	opp := gs.Seats[caster]
	if opp == nil {
		return
	}
	// RecordCast appends to Casts BEFORE fireCastTriggers (stack.go:436-447),
	// so the just-cast spell is already in the log. Count == 2 means this
	// IS the second cast this turn for that opponent.
	if opp.Turn.SpellsCast != 2 {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster_seat": caster,
		"trigger_arm": "second_cast",
	})
}

func troubleInPairsOnAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "trouble_in_pairs_attack"
	if gs == nil || perm == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil {
		return
	}
	atkSeat, _ := ctx["attacker_seat"].(int)
	if atkSeat == perm.Controller {
		return // opponent-only
	}
	if atkSeat < 0 || atkSeat >= len(gs.Seats) {
		return
	}
	// Confirm this attacker is targeting controller.
	defSeat, ok := gameengine.AttackerDefender(atkPerm)
	if !ok || defSeat != perm.Controller {
		return
	}
	// Count attackers from atkSeat targeting controller. Scan
	// attacker's battlefield for permanents with "attacking" flag
	// AND defender == controller.
	count := 0
	for _, p := range gs.Seats[atkSeat].Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		if p.Flags["attacking"] != 1 {
			continue
		}
		if d, ok := gameengine.AttackerDefender(p); ok && d == perm.Controller {
			count++
		}
	}
	if count < 2 {
		return
	}
	// Per-turn-per-attacker fire-once gate. Key on game turn + attacker
	// seat so the trigger fires once per combat declaration even when
	// creature_attacks fans out per-creature.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gateKey := fmt.Sprintf("trouble_in_pairs_attack_fired_t%d_s%d_v%d",
		gs.Turn, atkSeat, perm.Controller)
	if gs.Flags[gateKey] == 1 {
		return
	}
	gs.Flags[gateKey] = 1

	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"attacker_seat":  atkSeat,
		"attackers_at_you": count,
		"trigger_arm":   "two_attackers",
	})
}
