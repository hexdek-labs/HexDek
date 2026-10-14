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
		// Also withdraw a double-strike grant left over from a prior
		// turn, in case the delayed-trigger cleanup didn't fire.
		//
		// r93: this used to be a bare delete of the shared
		// Flags["kw:double_strike"], which took away ANY source's
		// double strike — including one that had nothing to do with
		// Lyse and was not stale at all. Revoking only Lyse's own
		// marker is the whole difference.
		if gameengine.RevokeKeywordFlag(perm, "double_strike", lyseDoubleStrikeMarker) {
			gs.InvalidateCharacteristicsCache()
		}
	}
	perm.Flags["lyse_noncreature_count"]++
	if perm.Flags["lyse_noncreature_count"] >= 2 && !gameengine.HasKeywordFlagGrant(perm, "double_strike", lyseDoubleStrikeMarker) {
		// r93: the guard was `perm.Flags["kw:double_strike"] == 0` —
		// "has anyone granted this?" — which both suppressed Lyse's own
		// grant behind an unrelated source and let the stale-clear paths
		// below delete that source's flag. Asking the refcount whether
		// LYSE holds a grant keeps the once-per-turn emit while making
		// the two sources independent.
		if gameengine.GrantKeywordFlag(perm, "double_strike", lyseDoubleStrikeMarker) {
			gs.InvalidateCharacteristicsCache()
		}
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				if gameengine.RevokeKeywordFlag(perm, "double_strike", lyseDoubleStrikeMarker) {
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

// lyseDoubleStrikeMarker is this trigger's source identity in the r87
// refcount (keyword_flag_grants_r87.go). Lyse grants to herself only,
// so one marker is one source; what the refcount buys here is that the
// three separate cleanup paths stop deleting other sources' grants.
const lyseDoubleStrikeMarker = "lyse_hext_double_strike"

func lyseHextResetIfStale(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || perm.Flags == nil {
		return
	}
	if perm.Flags["lyse_count_turn"] != gs.Turn {
		perm.Flags["lyse_count_turn"] = gs.Turn
		perm.Flags["lyse_noncreature_count"] = 0
		// r93: same bare delete as the reset above, same fix.
		if gameengine.RevokeKeywordFlag(perm, "double_strike", lyseDoubleStrikeMarker) {
			gs.InvalidateCharacteristicsCache()
		}
	}
}
