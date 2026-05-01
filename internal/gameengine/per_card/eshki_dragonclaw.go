package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEshkiDragonclaw wires Eshki, Dragonclaw.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Vigilance, trample, ward {1}
//	At the beginning of combat on your turn, if you've cast both a
//	creature spell and a noncreature spell this turn, draw a card and
//	put two +1/+1 counters on Eshki Dragonclaw.
//
// Implementation:
//   - "creature_spell_cast" / "noncreature_spell_cast": when Eshki's
//     controller casts the matching spell type, stamp a turn-keyed flag
//     on Eshki's Permanent.Flags. We use gs.Turn as the value so reads
//     pin to the current turn (mirrors tymna's pattern).
//   - "combat_begin": at the beginning of combat on Eshki's controller's
//     turn, intervening-if check both flags. If both are set this turn,
//     draw a card and add two +1/+1 counters to Eshki.
//   - The keyword stack (vigilance, trample, ward {1}) is intrinsic to
//     the card AST — no per-card runtime work needed.
func registerEshkiDragonclaw(r *Registry) {
	r.OnTrigger("Eshki Dragonclaw", "creature_spell_cast", eshkiCreatureSpellCast)
	r.OnTrigger("Eshki Dragonclaw", "noncreature_spell_cast", eshkiNoncreatureSpellCast)
	r.OnTrigger("Eshki Dragonclaw", "combat_begin", eshkiBeginningOfCombat)
}

func eshkiCreatureSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["eshki_creature_cast_turn"] = gs.Turn + 1
}

func eshkiNoncreatureSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["eshki_noncreature_cast_turn"] = gs.Turn + 1
}

func eshkiBeginningOfCombat(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "eshki_dragonclaw_combat_payoff"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	turnKey := gs.Turn + 1
	if perm.Flags == nil ||
		perm.Flags["eshki_creature_cast_turn"] != turnKey ||
		perm.Flags["eshki_noncreature_cast_turn"] != turnKey {
		return
	}

	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	perm.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()

	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"drawn_card": drawnName,
		"counters":   perm.Counters["+1/+1"],
	})
}
