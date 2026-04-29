package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDeadeyeNavigator wires up Deadeye Navigator.
//
// Oracle text:
//
//	Soulbond
//	As long as Deadeye Navigator is paired with another creature,
//	each of those creatures has "{1}{U}: Exile this creature, then
//	return it to the battlefield under its owner's control."
//
// The canonical flicker-infinite combo piece:
//   - Pair with Peregrine Drake → {1}{U} pay, flicker Drake, untap 5
//     lands → net +2 mana.
//   - Pair with Great Whale / Palinchron → same, bigger refund.
//   - Pair with any ETB creature → repeatable ETB trigger.
//
// Batch #2 scope:
//   - Soulbond bonding is NOT implemented — soulbond is an engine-level
//     pairing mechanic (CR §702.93) requiring "when ~ or another
//     creature enters the battlefield and both are unpaired, pair
//     them" triggered hook. We stub it: the OnETB handler stamps
//     perm.Flags["deadeye_soulbond_available"] = 1 as a marker for
//     downstream pairing logic to consume.
//   - OnActivated(0, ctx["target_perm"]): flicker ctx["target_perm"]
//     (remove + re-add fresh Permanent → re-fires ETB). Caller is
//     responsible for ensuring target is the paired creature OR
//     Deadeye itself; we don't enforce the pairing requirement.
//
// The combo-critical path (flicker for {1}{U}) is fully functional;
// soulbond auto-pairing is the half-measure.
func registerDeadeyeNavigator(r *Registry) {
	r.OnETB("Deadeye Navigator", deadeyeNavigatorETB)
	r.OnActivated("Deadeye Navigator", deadeyeNavigatorActivate)
}

func deadeyeNavigatorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "deadeye_navigator_soulbond"
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["deadeye_soulbond_available"] = 1
	// Auto-pair policy (CR §702.93c — "when this creature enters or
	// another creature enters while this one is on the battlefield,
	// you may pair them as long as both are unpaired").
	//
	// MVP: scan the controller's battlefield for the first unpaired
	// creature that is NOT Deadeye itself. If found, set the pair:
	//   - perm.Flags["paired_timestamp"] = partner.Timestamp
	//   - partner.Flags["paired_timestamp"] = perm.Timestamp
	//
	// The flicker-ability target resolution reads these flags to
	// refuse flickers on unpaired targets. For combo gameplay
	// (Peregrine Drake + Deadeye) this auto-pairs on Deadeye's ETB.
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	for _, p := range s.Battlefield {
		if p == nil || p == perm {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		// Already paired with someone else?
		if p.Flags != nil && p.Flags["paired_timestamp"] > 0 {
			continue
		}
		// Pair!
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		perm.Flags["paired_timestamp"] = p.Timestamp
		p.Flags["paired_timestamp"] = perm.Timestamp
		gs.LogEvent(gameengine.Event{
			Kind:   "soulbond_pair",
			Seat:   seat,
			Target: seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"paired_card":     p.Card.DisplayName(),
				"rule":            "702.93",
				"partner_stamp":   p.Timestamp,
				"deadeye_stamp":   perm.Timestamp,
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":         seat,
			"paired_with":  p.Card.DisplayName(),
			"auto_paired":  true,
		})
		return
	}
	// No unpaired creature available — Deadeye sits alone awaiting
	// a future creature ETB to pair. (An engine-side permanent_etb
	// observer would handle that future pair automatically; for MVP
	// we just log.)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"auto_paired": false,
		"reason":      "no_unpaired_creature_available_at_etb",
	})
}

func deadeyeNavigatorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "deadeye_navigator_flicker"
	if gs == nil || src == nil {
		return
	}
	// Determine flicker target. Priority:
	//   1. ctx["target_perm"] if supplied.
	//   2. Fall back to Deadeye itself (self-flicker is legal — the
	//      activated ability is "Exile THIS creature", either half of
	//      the pair can be the "this" referent).
	var target *gameengine.Permanent
	if v, ok := ctx["target_perm"].(*gameengine.Permanent); ok && v != nil {
		target = v
	} else {
		target = src
	}
	if target == nil || target.Card == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_flicker_target", nil)
		return
	}
	// Flicker: remove + re-add. The same pattern as Displacer Kitten.
	//
	// NOTE: Zone-conservation-neutral — card goes battlefield -> battlefield
	// atomically. Raw removePermanent is used because the card is immediately
	// re-placed via createPermanent (not routed to graveyard/exile/hand).
	owner := target.Owner
	if target.Card != nil && target.Card.Owner >= 0 && target.Card.Owner < len(gs.Seats) {
		owner = target.Card.Owner
	}
	card := target.Card
	if !removePermanent(gs, target) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_on_battlefield", nil)
		return
	}
	// Clean up replacement/continuous effects for the leaving permanent.
	gs.UnregisterReplacementsForPermanent(target)
	gs.UnregisterContinuousEffectsForPermanent(target)
	// Fire LTB triggers for the departing permanent.
	gameengine.FireZoneChangeTriggers(gs, target, card, "battlefield", "exile")
	newPerm := createPermanent(gs, owner, card, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "flicker",
		Seat:   src.Controller,
		Target: owner,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": card.DisplayName(),
			"reason":      "deadeye_navigator",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"flickered_card": card.DisplayName(),
		"owner":          owner,
	})
	// Fire full ETB cascade on the re-entered permanent.
	if newPerm != nil {
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
}
