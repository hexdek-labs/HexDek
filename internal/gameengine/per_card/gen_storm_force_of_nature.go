package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStormForceOfNature wires Storm, Force of Nature.
//
// Oracle text:
//
//	Flying, vigilance
//	Ceaseless Tempest — Whenever Storm deals combat damage to a player,
//	the next instant or sorcery spell you cast this turn has storm.
//
// Implementation:
//   - "combat_damage_to_player" trigger gated on the source being
//     Storm. Stamps a per-seat "storm_grant_pending" flag the cast
//     pipeline reads on the next instant/sorcery cast that turn.
//   - Cleanup: cleared at end of turn via a delayed trigger so an
//     unused grant doesn't bleed into the next turn.
//   - The actual "copy for each spell cast before it" mechanic lives
//     in the cast pipeline; we surface a partial.
func registerStormForceOfNature(r *Registry) {
	r.OnTrigger("Storm, Force of Nature", "combat_damage_to_player", stormForceOfNatureCombatDamage)
	// R51 batch I: defensive LTB clear of the storm_grant_pending seat
	// flag. The trigger itself queues an EOT delayed trigger to clear
	// the flag, but if Storm leaves play before the delayed trigger
	// fires (e.g. bounced and recast on the same turn), the captured
	// EffectFn still references the old perm — clearing on LTB here
	// ensures the grant is correctly retracted on the source's exit.
	r.OnTrigger("Storm, Force of Nature", "permanent_ltb", stormLTBClearFlag)
	// R60 followup: the printed "the next instant or sorcery spell you
	// cast this turn has storm" requires consuming the per-seat
	// storm_grant_pending flag at the spell-cast event. Creature /
	// land / artifact spells do not consume the grant.
	r.OnTrigger("Storm, Force of Nature", "spell_cast", stormForceOfNatureConsumeGrant)
}

func stormForceOfNatureConsumeGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	if seat.Flags["storm_grant_pending"] <= 0 {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return
	}
	seat.Flags["storm_grant_pending"] = 0
	emit(gs, "storm_force_of_nature_grant_consumed", perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
	})
}

func stormLTBClearFlag(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	delete(seat.Flags, "storm_grant_pending")
}

func stormForceOfNatureCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "storm_force_of_nature_grant_storm"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcPerm, _ := ctx["source_perm"].(*gameengine.Permanent)
	if srcPerm == nil {
		// Fallback: source_seat must match.
		ss, ok := ctx["source_seat"].(int)
		if !ok || ss != perm.Controller {
			return
		}
	} else if srcPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["storm_grant_pending"] = 1
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			s := gs.Seats[perm.Controller]
			if s == nil || s.Flags == nil {
				return
			}
			delete(s.Flags, "storm_grant_pending")
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"storm_keyword_grant_consumption_handled_by_cast_pipeline_at_resolve")
}
