package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_trigger_grants_r60.go — R60 sweep continuation:
// trigger-grant family for single-target flashback grants.
//
// Cards covered:
//
//   - Archmage's Newt — Salamander Mount; combat-damage trigger grants
//     flashback to a target i/s in the controller's graveyard at the
//     card's mana cost; flashback {0} instead if Newt is saddled.
//
//   - The Fugitive Doctor — Time Lord Doctor; attack trigger, may
//     sacrifice a Clue. When you do, grant flashback {2}{R}{G} (CMC 4
//     in engine-units) to a target i/s in your graveyard.
//
//   - Lost in Memories — Aura; enchanted creature gets +1/+1 and a
//     granted combat-damage trigger that grants flashback (printed
//     mana cost) to a target i/s in your graveyard.
//
// All three reuse GrantFlashbackUntilEOT or its cost-override sibling
// GrantFlashbackUntilEOTWithCost from keywords_flashback.go. Target
// selection policy (until full target-prompt plumbing lands): pick the
// highest-CMC eligible card in the controller's graveyard. Falls back
// to a no-op + emitFail when the graveyard has nothing eligible.

func init() {
	registerTLAFlashbackTriggerGrantsR60(Global())
	AddResetHook(registerTLAFlashbackTriggerGrantsR60)
}

func registerTLAFlashbackTriggerGrantsR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Archmage's Newt", "combat_damage_player", archmagesNewtCombatDamage)
	r.OnTrigger("The Fugitive Doctor", "attacks", fugitiveDoctorAttacks)
	r.OnTrigger("Lost in Memories", "combat_damage_player", lostInMemoriesCombatDamage)
}

// -----------------------------------------------------------------------------
// Archmage's Newt
// -----------------------------------------------------------------------------
//
// Oracle text ({1}{U} Creature — Salamander Mount):
//
//	Whenever this creature deals combat damage to a player, target
//	instant or sorcery card in your graveyard gains flashback until end
//	of turn. The flashback cost is equal to its mana cost. That card
//	gains flashback {0} until end of turn instead if this creature is
//	saddled.
//	Saddle 3
//
// Implementation:
//   - combat_damage_player fires once per source-defender pair. We gate
//     on source_card matching Newt's name and source_seat matching the
//     controller (so the trigger only fires when this Newt is the
//     damager — combat.go ctx doesn't carry a Permanent pointer so we
//     use name+seat).
//   - PermIsSaddled drives the cost branch: saddled → flashback {0},
//     unsaddled → printed mana cost. The Saddle 3 cost is the AST
//     keyword pipeline's surface; we only read the resulting flag.

func archmagesNewtCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archmages_newt_flashback_grant"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(seat.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_target_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	saddled := gameengine.PermIsSaddled(perm)
	if saddled {
		gameengine.GrantFlashbackUntilEOTWithCost(gs, target, perm.Controller, perm.Card.DisplayName(), 0)
	} else {
		gameengine.GrantFlashbackUntilEOT(gs, target, perm.Controller, perm.Card.DisplayName())
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"target":  target.DisplayName(),
		"saddled": saddled,
	})
}

// -----------------------------------------------------------------------------
// The Fugitive Doctor
// -----------------------------------------------------------------------------
//
// Oracle text ({3}{R}{G} Legendary Creature — Time Lord Doctor):
//
//	When The Fugitive Doctor enters, investigate.
//	Whenever The Fugitive Doctor attacks, you may sacrifice a Clue.
//	When you do, target instant or sorcery card in your graveyard
//	gains flashback {2}{R}{G} until end of turn.
//
// Implementation:
//   - OnTrigger("attacks") with the attacker_perm == perm gate (same
//     shape as Backdraft Hellkite in tla_flashback_grants_r60.go).
//   - "May sacrifice a Clue": opt-in iff (a) the controller has at
//     least one Clue token on the battlefield, and (b) there is an
//     eligible i/s target in the graveyard. Without an eligible target
//     the Clue is wasted, so we abstain.
//   - On opt-in, SacrificePermanent the Clue, then grant flashback at
//     cost 4 ({2}{R}{G} = 4 mana value).
//   - The ETB investigate trigger is wired by the AST keyword pipeline.

func fugitiveDoctorAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fugitive_doctor_flashback_grant"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(seat.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_target_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	clue := findFugitiveDoctorClue(seat)
	if clue == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_clue_to_sacrifice", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.SacrificePermanent(gs, clue, "the_fugitive_doctor_attack_trigger")
	gameengine.GrantFlashbackUntilEOTWithCost(gs, target, perm.Controller, perm.Card.DisplayName(), 4)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.DisplayName(),
		"cost":   4,
	})
}

func findFugitiveDoctorClue(seat *gameengine.Seat) *gameengine.Permanent {
	if seat == nil {
		return nil
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "clue") {
			return p
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Lost in Memories
// -----------------------------------------------------------------------------
//
// Oracle text ({1}{R} Enchantment — Aura):
//
//	Flash
//	Enchant creature you control
//	Enchanted creature gets +1/+1 and has "Whenever this creature deals
//	combat damage to a player, target instant or sorcery card in your
//	graveyard gains flashback until end of turn. The flashback cost is
//	equal to its mana cost."
//
// Implementation:
//   - The granted combat-damage trigger lives on the enchanted creature,
//     but we hook the trigger on Lost in Memories itself (the Aura) and
//     match against ctx["source_card"] == AttachedTo's card name. That
//     way we only need a single OnTrigger registration and don't need
//     dynamic ability-injection plumbing.
//   - +1/+1 is granted by the AST static-modification pipeline (Auras
//     emit a layer-7c P/T modification automatically); we don't touch
//     P/T here.
//   - "your graveyard" is the enchanted-creature's-controller's
//     graveyard, which is the Aura's controller's graveyard (Aura must
//     be attached to a creature its controller controls per "enchant
//     creature you control").

func lostInMemoriesCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lost_in_memories_flashback_grant"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	if perm.AttachedTo == nil || perm.AttachedTo.Card == nil {
		return
	}
	sourceCard, _ := ctx["source_card"].(string)
	if sourceCard != perm.AttachedTo.Card.DisplayName() {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.AttachedTo.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(seat.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_target_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.GrantFlashbackUntilEOT(gs, target, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"target":           target.DisplayName(),
		"enchanted_source": perm.AttachedTo.Card.DisplayName(),
	})
}
