package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArchpriestOfShadows wires Archpriest of Shadows (Muninn parser-
// gap rank ~168, MH3 deathtouch combat-reanimator).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{3}{B}{B}
//	Creature — Human Warlock
//	Backup 1 (When this creature enters, put a +1/+1 counter on target
//	creature. If that's another creature, it gains the following
//	abilities until end of turn.)
//	Deathtouch
//	Whenever this creature deals combat damage to a player or battle,
//	return target creature card from your graveyard to the battlefield.
//
// Implementation:
//   - Deathtouch via AST keyword pipeline.
//   - OnETB: backup 1 — place a +1/+1 counter on the best creature the
//     controller controls (highest power) and stamp deathtouch +
//     reanimate-on-damage flags if that creature is NOT Archpriest. The
//     "until end of turn" granted-abilities cleanup follows the
//     kw:deathtouch + EOT-strip pattern used by vito_thorn_lifelink.
//     The granted reanimate trigger itself is the hard part — we can't
//     register a transient OnTrigger from inside a handler — so we
//     emitPartial about the granted trigger and only stamp deathtouch.
//   - OnTrigger("combat_damage_to_player"): gate on damager being
//     Archpriest itself dealing damage to a player. Reanimate the
//     highest-CMC creature card from controller's graveyard via
//     enterBattlefieldWithETB.
//   - "Or battle" — battles are a recent CR §310 permanent type the
//     engine doesn't yet support fully; the trigger only fires on
//     player damage today.
func registerArchpriestOfShadows(r *Registry) {
	r.OnETB("Archpriest of Shadows", archpriestOfShadowsETB)
	r.OnTrigger("Archpriest of Shadows", "combat_damage_to_player", archpriestOfShadowsCombatDamage)
}

func archpriestOfShadowsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "archpriest_of_shadows_backup"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick the best other creature to backup; fall back to Archpriest itself.
	var target *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			target = p
		}
	}
	if target == nil {
		target = perm
	}
	// Backup's +1/+1 counter is a §122/§616 counter placement, so it must
	// route through the canonical doubler-aware chokepoint: +1/+1 doublers
	// (Doubling Season, Primal Vigor, Hardened Scales, Branching Evolution)
	// apply and the counter_placed trigger fires (Abzan Falconer / Conclave
	// Mentor payoffs). Raw AddCounter bypassed both — the r63 counter-pipeline
	// sweep's canonical-helper-bypass meta-pattern, here on a per_card site.
	// src = the backup creature. PutCountersTriggered invalidates the cache.
	gameengine.PutCountersTriggered(gs, target, "+1/+1", 1, perm)

	grantedDeathtouch := false
	if target != perm {
		// r93: the guard here used to be `!target.HasKeyword("deathtouch")
		// && target.Flags["kw:deathtouch"] == 0` — skip anything that
		// already shows deathtouch from any source. That made Archpriest
		// decline to record its own grant behind another source's, so
		// that source's cleanup took deathtouch away with Archpriest's
		// until-end-of-turn grant still live. The r87 refcount holds the
		// shared flag until the last source revokes.
		//
		// Note the grant path never invalidated either: the only reason
		// a layer-aware read saw the deathtouch at all is that
		// PutCountersTriggered above invalidates as a side effect of
		// placing the +1/+1 counter. That is an upstream call doing this
		// grant's work for it, so it is made explicit here.
		grantedDeathtouch = gameengine.GrantKeywordFlag(target, "deathtouch", archpriestDeathtouchUEOTMarker)
		if grantedDeathtouch {
			gs.InvalidateCharacteristicsCache()
		}
		captured := target
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				if gameengine.RevokeKeywordFlag(captured, "deathtouch", archpriestDeathtouchUEOTMarker) {
					gs.InvalidateCharacteristicsCache()
				}
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"target":         target.Card.DisplayName(),
		"granted_dt":     grantedDeathtouch,
		"target_is_self": target == perm,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"granted_combat_damage_reanimate_trigger_to_backup_target_unwired_pending_transient_trigger_registration")
}

// archpriestDeathtouchUEOTMarker is this trigger's source identity in
// the r87 refcount (keyword_flag_grants_r87.go), replacing the raw
// shared Flags["kw:deathtouch"] stamp.
const archpriestDeathtouchUEOTMarker = "archpriest_of_shadows_deathtouch_ueot"

func archpriestOfShadowsCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archpriest_of_shadows_reanimate"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Engine fires combat_damage_player with source_seat + source_card (name).
	// Legacy ctx["damager_perm"] (a *Permanent) is supported as a fallback for
	// callers that thread it explicitly.
	damager, _ := ctx["damager_perm"].(*gameengine.Permanent)
	if damager == nil {
		srcSeat, hasSeat := ctx["source_seat"].(int)
		srcName, _ := ctx["source_card"].(string)
		if !hasSeat || srcSeat != perm.Controller || srcName != perm.Card.DisplayName() {
			return
		}
	} else if damager != perm {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick highest-CMC creature card in our graveyard.
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "battlefield", "archpriest_reanimate")
	enterBattlefieldWithETB(gs, perm.Controller, card, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"reanimated":   card.DisplayName(),
		"reanimated_cmc": bestCMC,
	})
}
