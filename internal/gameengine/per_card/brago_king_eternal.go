package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBragoKingEternal wires Brago, King Eternal.
//
// Oracle text:
//
//	Flying
//	Whenever Brago deals combat damage to a player, exile any number of
//	target nonland permanents you control, then return those cards to
//	the battlefield under their owner's control.
//
// Implementation:
//   - Flying — AST keyword pipeline.
//   - OnTrigger("combat_damage_player") — when Brago is the source and
//     hits a player, flicker every nonland permanent Brago's controller
//     controls (excluding Brago himself, since blinking him resets his
//     counters/auras and is rarely the desired play). The flicker re-fires
//     ETB triggers, modeling the standard Brago value engine.
//
// Each blink uses the displacer-kitten flicker pattern: detach the
// permanent, fire LTB triggers, drop replacements/continuous effects,
// then re-create the permanent and fire the full ETB cascade.
func registerBragoKingEternal(r *Registry) {
	r.OnTrigger("Brago, King Eternal", "combat_damage_player", bragoBlinkOnCombatDamage)
}

func bragoBlinkOnCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "brago_king_eternal_blink"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceCard, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceCard != "" && sourceCard != perm.Card.DisplayName() {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Snapshot targets first — we mutate the battlefield as we flicker.
	var targets []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		targets = append(targets, p)
	}

	if len(targets) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_blink_targets", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}

	flickered := make([]string, 0, len(targets))
	for _, target := range targets {
		owner := perm.Controller
		card := target.Card
		if card == nil {
			continue
		}
		if card.Owner >= 0 && card.Owner < len(gs.Seats) {
			owner = card.Owner
		}
		if !removePermanent(gs, target) {
			continue
		}
		gs.UnregisterReplacementsForPermanent(target)
		gs.UnregisterContinuousEffectsForPermanent(target)
		gameengine.FireZoneChangeTriggers(gs, target, card, "battlefield", "exile")

		newPerm := createPermanent(gs, owner, card, false)
		if newPerm == nil {
			continue
		}
		flickered = append(flickered, card.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "flicker",
			Seat:   perm.Controller,
			Target: owner,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"target_card": card.DisplayName(),
				"reason":      "brago_combat_blink",
			},
		})
		gameengine.RegisterReplacementsForPermanent(gs, newPerm)
		gameengine.InvokeETBHook(gs, newPerm)
		gameengine.FireObserverETBTriggers(gs, newPerm)
		gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
			"perm":            newPerm,
			"controller_seat": newPerm.Controller,
			"card":            newPerm.Card,
		})
		if !newPerm.IsLand() {
			gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
				"perm":            newPerm,
				"controller_seat": newPerm.Controller,
				"card":            newPerm.Card,
			})
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"flickered": flickered,
		"count":     len(flickered),
	})
}
