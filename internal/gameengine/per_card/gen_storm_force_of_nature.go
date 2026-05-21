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
	r.OnTrigger("Storm, Force of Nature", "spell_cast", stormForceOfNatureConsumeGrant)
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

// stormForceOfNatureConsumeGrant fires on every spell_cast event. When
// the controller casts an instant or sorcery while storm_grant_pending
// is set, it consumes the grant — the printed "next instant or sorcery"
// gate. emitPartial flags the cast-pipeline copy-minting boundary
// (R51 batch H port).
func stormForceOfNatureConsumeGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "storm_force_of_nature_consume_grant"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil || seat.Flags["storm_grant_pending"] == 0 {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return
	}
	delete(seat.Flags, "storm_grant_pending")
	spellName, _ := ctx["spell_name"].(string)
	if spellName == "" {
		spellName = card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": spellName,
		"grant": "storm",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"storm_copy_per_prior_spell_actual_minting_needs_cast_pipeline_storm_helper")
}
