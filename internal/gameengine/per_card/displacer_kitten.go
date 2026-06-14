package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDisplacerKitten wires up Displacer Kitten.
//
// Oracle text:
//
//	Whenever you cast a noncreature spell, you may exile another
//	target nonland permanent you control, then return it to the
//	battlefield under its owner's control.
//
// The combo engine for a lot of cEDH builds. Paired with Cloudstone
// Curio (ETB triggers that bounce back) or any mana-positive ETB
// creature (Deadeye Navigator, Peregrine Drake), it goes infinite.
//
// Implementation:
//   - OnTrigger("noncreature_spell_cast") — pick a nonland permanent
//     the Kitten's controller controls (not Kitten herself unless
//     no other target), flicker it (remove + re-add as a new
//     Permanent so ETB triggers re-fire).
//   - Flicker policy: prefer a mana-rock / ETB creature. MVP: pick the
//     highest-timestamp non-Kitten nonland permanent (most recent ETB
//     has the most interesting re-ETB payoff usually).
func registerDisplacerKitten(r *Registry) {
	r.OnTrigger("Displacer Kitten", "noncreature_spell_cast", displacerKittenOnCast)
}

func displacerKittenOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "displacer_kitten_flicker"
	if gs == nil || perm == nil {
		return
	}
	// "Whenever YOU cast" — scope to the Kitten's controller.
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	seat := perm.Controller
	// Find a target: nonland permanent controlled by the Kitten's
	// controller, NOT the Kitten itself. Pick highest-timestamp.
	var target *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p == perm {
			continue
		}
		if p.IsLand() {
			continue
		}
		if target == nil || p.Timestamp > target.Timestamp {
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_flicker_target", nil)
		return
	}

	// Flicker: remove, then re-add as a fresh Permanent. This re-fires
	// ETB triggers — which is the whole point.
	//
	// NOTE: This is zone-conservation-neutral — the card goes from
	// battlefield -> battlefield atomically (never enters exile/graveyard).
	// We use raw removePermanent here intentionally because the card is
	// immediately re-placed on the battlefield via createPermanent below.
	// Proper zone-change helpers (ExilePermanent + re-enter) would
	// double-count the zone write.
	card := target.Card
	isToken := target.IsToken()
	// CR — "return it to the battlefield under YOUR control": the new object
	// is controlled by the Kitten's controller, NOT routed back to its owner.
	// (Returning to owner handed a blinked stolen permanent back to its
	// opponent.) Owner stays card.Owner via createPermanent.
	controlSeat := seat
	removePermanent(gs, target)
	// Clean up replacement/continuous effects for the leaving permanent.
	gs.UnregisterReplacementsForPermanent(target)
	gs.UnregisterContinuousEffectsForPermanent(target)
	// Fire LTB triggers for the departing permanent.
	gameengine.FireZoneChangeTriggers(gs, target, card, "battlefield", "exile")
	// Detach any auras/equipment that were attached to the flickered carrier.
	// Blink re-enters as a NEW permanent (createPermanent below), so attached
	// objects don't follow — they would point at a no-longer-on-BF perm and
	// trip checkAttachmentConsistency in the window before SBA §704.5m/n
	// cleans them up (Loki r41/r57 cluster).
	gameengine.DetachAll(gs, target)
	// CR §111.7 / §704.5d — a blinked token ceases to exist and does NOT return.
	if isToken {
		gs.LogEvent(gameengine.Event{
			Kind:   "flicker_token_ceased",
			Seat:   seat,
			Source: card.DisplayName(),
			Details: map[string]interface{}{"rule": "111.7", "via": "displacer_kitten"},
		})
		return
	}
	newPerm := createPermanent(gs, controlSeat, card, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "flicker",
		Seat:   seat,
		Target: controlSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": card.DisplayName(),
			"reason":      "displacer_kitten",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"flickered_card": card.DisplayName(),
	})

	// Fire the FULL canonical ETB cascade (AST ETB triggers, continuous-effect
	// registration, ETB counters, §614 doublers) — the old subset missed them.
	if newPerm != nil {
		gameengine.RegisterReplacementsForPermanent(gs, newPerm)
		gameengine.FirePermanentETBTriggers(gs, newPerm)
	}
}
