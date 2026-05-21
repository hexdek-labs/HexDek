package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLyseHext wires Lyse Hext.
//
// Oracle text:
//
//	Prowess (Whenever you cast a noncreature spell, this creature
//	gets +1/+1 until end of turn.)
//	Noncreature spells you cast cost {1} less to cast.
//	As long as you've cast two or more noncreature spells this turn,
//	Lyse Hext has double strike.
//
// Implementation:
//   - Prowess: handled by AST keyword pipeline.
//   - "spell_cast" trigger gated on caster == Lyse's controller AND
//     the cast spell is noncreature: increment a per-turn counter on
//     Lyse.Flags. Once the counter hits 2, set kw:double_strike on
//     Lyse and register a delayed trigger to clear it at the end step.
//
// emitPartial: noncreature cost reduction is a static replacement on
// the cast pipeline — engine-side TODO. We expose the cast-count to
// any downstream observers.
func registerLyseHext(r *Registry) {
	r.OnTrigger("Lyse Hext", "spell_cast", lyseHextSpellCast)
	r.OnTrigger("Lyse Hext", "combat_begin", lyseHextResetIfStale)
}

func lyseHextSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lyse_hext_noncreature_count"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || cardHasType(card, "creature") {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Reset counter on a new turn so we don't carry stale across turns.
	if perm.Flags["lyse_count_turn"] != gs.Turn {
		perm.Flags["lyse_count_turn"] = gs.Turn
		perm.Flags["lyse_noncreature_count"] = 0
		// Also clear any double-strike grant from a prior turn, in case
		// the delayed-trigger cleanup didn't fire.
		delete(perm.Flags, "kw:double_strike")
	}
	perm.Flags["lyse_noncreature_count"]++
	if perm.Flags["lyse_noncreature_count"] >= 2 && perm.Flags["kw:double_strike"] == 0 {
		perm.Flags["kw:double_strike"] = 1
		gs.InvalidateCharacteristicsCache()
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				if perm != nil && perm.Flags != nil {
					delete(perm.Flags, "kw:double_strike")
					gs.InvalidateCharacteristicsCache()
				}
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"granted":          "double_strike",
			"noncreature_casts": perm.Flags["lyse_noncreature_count"],
		})
	}
	// Noncreature cost reduction is wired in cost_modifiers.go under
	// "Lyse Hext" (R49 batchA — pre-batchD); per-cast trigger here just
	// counts noncreature casts for the double-strike threshold.
}

func lyseHextResetIfStale(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	if perm.Flags["lyse_count_turn"] != gs.Turn {
		perm.Flags["lyse_count_turn"] = gs.Turn
		perm.Flags["lyse_noncreature_count"] = 0
		delete(perm.Flags, "kw:double_strike")
	}
}
