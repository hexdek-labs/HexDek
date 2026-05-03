package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNivMizzetParun wires Niv-Mizzet, Parun.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	This spell can't be countered.
//	Flying
//	Whenever you draw a card, Niv-Mizzet, Parun deals 1 damage to any target.
//	Whenever a player casts an instant or sorcery spell, you draw a card.
//
// Implementation:
//   - "This spell can't be countered" and Flying are handled by the AST
//     keyword pipeline.
//   - OnTrigger "player_would_draw": fires before every draw event
//     (FireDrawTriggerObservers in cast_counts.go). Gate on draw_seat ==
//     perm.Controller (only the controller's own draws trigger the ping).
//     Deals 1 damage to the opponent with the highest life total via
//     direct Life subtraction + "damage" event.
//   - OnTrigger "instant_or_sorcery_cast": fires when ANY player casts an
//     instant or sorcery (no caster_seat gate). Controller draws one card
//     via drawOne. This triggers the draw ability above, creating a chain.
//     The engine's trigger_depth guard (max 8) prevents infinite loops
//     when combined with repeated instant/sorcery casts.
//
// Coverage gap: draws that bypass FireDrawTriggerObservers entirely (e.g.
// direct library→hand moves not routed through the standard draw path)
// will not fire player_would_draw and therefore will not trigger the
// damage ping. emitPartial flags the gap on ETB.
func registerNivMizzetParun(r *Registry) {
	r.OnETB("Niv-Mizzet, Parun", nivMizzetParunETB)
	r.OnTrigger("Niv-Mizzet, Parun", "player_would_draw", nivMizzetParunDraw)
	r.OnTrigger("Niv-Mizzet, Parun", "instant_or_sorcery_cast", nivMizzetParunSpellCast)
}

func nivMizzetParunETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "niv_mizzet_parun_coverage_gap"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"draws_not_routed_through_fire_draw_trigger_observers_not_tracked")
}

// nivMizzetParunDraw — "Whenever you draw a card, Niv-Mizzet, Parun deals
// 1 damage to any target." Targets the opponent with the highest life total.
func nivMizzetParunDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "niv_mizzet_parun_draw_damage"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	drawSeat, ok := ctx["draw_seat"].(int)
	if !ok || drawSeat < 0 || drawSeat >= len(gs.Seats) {
		return
	}

	// Only trigger on controller's own draws.
	if drawSeat != perm.Controller {
		return
	}

	// Pick the opponent with the highest life total as target.
	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	target := opps[0]
	bestLife := gs.Seats[target].Life
	for _, o := range opps[1:] {
		if gs.Seats[o].Life > bestLife {
			bestLife = gs.Seats[o].Life
			target = o
		}
	}

	// Deal 1 damage.
	gs.Seats[target].Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   perm.Controller,
		Target: target,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "niv_mizzet_parun_draw_trigger",
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"draw_seat":   drawSeat,
		"target_seat": target,
		"damage":      1,
	})

	_ = gs.CheckEnd()
}

// nivMizzetParunSpellCast — "Whenever a player casts an instant or sorcery
// spell, you draw a card." No caster_seat gate — triggers on ALL players'
// instant/sorcery casts. This feeds into the draw trigger above.
func nivMizzetParunSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "niv_mizzet_parun_spell_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	casterSeat, _ := ctx["caster_seat"].(int)
	spellName, _ := ctx["spell_name"].(string)

	// Draw one card for Niv-Mizzet's controller.
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"caster_seat": casterSeat,
		"spell_name":  spellName,
		"drawn":       drawnName,
	})
}
