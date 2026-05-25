package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZidaneTantalusThief wires Zidane, Tantalus Thief.
//
// Oracle text (Scryfall, verified):
//
//	When Zidane enters, gain control of target creature an opponent
//	controls until end of turn. Untap it. It gains lifelink and haste
//	until end of turn.
//	Whenever an opponent gains control of a permanent from you, you
//	create a Treasure token.
//
// Implementation (R49 stub port):
//   - ETB control-grab: pick the highest-power opponent creature
//     (kill-priority bias). Move it to controller's battlefield, give
//     it a fresh timestamp, untap, grant kw:lifelink + kw:haste +
//     mark control_until_eot_owner = original-owner for the
//     end-of-turn return. A next_end_step delayed trigger restores
//     control, clears the lifelink/haste flags, and removes the
//     marker. Mirrors the standard control-steal-UEOT pattern.
//   - Opp-gain-control Treasure trigger: the engine doesn't currently
//     emit a card-trigger event when an opponent's effect grabs one
//     of our permanents (the resolve_helpers.go gain_control path
//     logs an event but does not call FireCardTrigger). emitPartial
//     so the gap is visible to audits — when the engine surface
//     lands, this hook is the wire-up point.
func registerZidaneTantalusThief(r *Registry) {
	r.OnETB("Zidane, Tantalus Thief", zidaneTantalusThiefETB)
}

func zidaneTantalusThiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zidane_tantalus_steal_creature_eot"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Pick the highest-power opponent creature with a tiebreak on most
	// recent timestamp (fresh threats are typically the danger).
	var target *gameengine.Permanent
	var origOwner int = -1
	bestPow := -1
	bestTS := -1
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			pow := gs.PowerOf(p)
			if pow > bestPow || (pow == bestPow && p.Timestamp > bestTS) {
				bestPow = pow
				bestTS = p.Timestamp
				target = p
				origOwner = i
			}
		}
	}
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     seatIdx,
			"no_target": true,
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"opp_gain_control_treasure_trigger_event_not_emitted_engine_side")
		return
	}

	// Move the permanent to Zidane's seat. Mirror the in-engine
	// gain_control pattern.
	src := gs.Seats[origOwner]
	for i, p := range src.Battlefield {
		if p == target {
			src.Battlefield = append(src.Battlefield[:i], src.Battlefield[i+1:]...)
			break
		}
	}
	target.Controller = seatIdx
	target.Timestamp = gs.NextTimestamp()
	target.Tapped = false
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["kw:lifelink"] = 1
	target.Flags["kw:haste"] = 1
	gs.Seats[seatIdx].Battlefield = append(gs.Seats[seatIdx].Battlefield, target)
	gs.LogEvent(gameengine.Event{
		Kind:   "gain_control",
		Seat:   seatIdx,
		Target: origOwner,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"target_card": target.Card.DisplayName(),
			"duration":    "until_end_of_turn",
		},
	})

	captured := target
	originalOwner := origOwner
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seatIdx,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Card == nil {
				return
			}
			// CR §611.3c: when a control-changing effect ends, control
			// reverts to the owner ONLY IF the permanent is still on the
			// battlefield. If `captured` has left play (died → graveyard,
			// exiled, bounced to hand, etc.) since Zidane's ETB, there is
			// nothing to give back — the perm ceased to exist as a
			// battlefield object and the control duration's expiry is a
			// no-op per the rules.
			//
			// Loki r60 extreme-stress / seed 1337 game 8921: 40
			// CardIdentity hits because the pre-fix EOT delayed trigger
			// blindly re-added the captured Permanent to originalOwner's
			// battlefield even when its `*Card` had already moved to a
			// different zone (graveyard / exile / hand). The re-add
			// created a fresh Permanent wrapping a `*Card` that was also
			// present in its real post-death zone — same-pointer-in-
			// two-zones CardIdentity violation, persisting until the perm
			// itself was destroyed again.
			//
			// Same defensive pattern as the Adric / Oketra / Gisa /
			// Athreos / The Reaper §704.6d / Athreos PR #200 fixes: when
			// a per_card handler wants to "restore" or "claim" a card
			// across a delay, validate the card is in the expected source
			// state before acting.
			stillOnBattlefield := false
			for _, s := range gs.Seats {
				if s == nil {
					continue
				}
				for _, p := range s.Battlefield {
					if p == captured {
						stillOnBattlefield = true
						break
					}
				}
				if stillOnBattlefield {
					break
				}
			}
			if !stillOnBattlefield {
				// Permanent left the battlefield — control return is a
				// no-op per CR §611.3c. Drop the lifelink/haste flags so
				// they don't ride along with a future reanimation that
				// rewraps the same *Card pointer.
				if captured.Flags != nil {
					delete(captured.Flags, "kw:lifelink")
					delete(captured.Flags, "kw:haste")
				}
				return
			}
			// Pluck from current controller's bf and return to original.
			cur := gs.Seats[captured.Controller]
			if cur != nil {
				for i, p := range cur.Battlefield {
					if p == captured {
						cur.Battlefield = append(cur.Battlefield[:i], cur.Battlefield[i+1:]...)
						break
					}
				}
			}
			if originalOwner >= 0 && originalOwner < len(gs.Seats) {
				captured.Controller = originalOwner
				captured.Timestamp = gs.NextTimestamp()
				gs.Seats[originalOwner].Battlefield = append(gs.Seats[originalOwner].Battlefield, captured)
				gs.LogEvent(gameengine.Event{
					Kind:   "gain_control",
					Seat:   originalOwner,
					Source: perm.Card.DisplayName(),
					Details: map[string]interface{}{
						"target_card": captured.Card.DisplayName(),
						"reason":      "zidane_eot_return",
					},
				})
			}
			if captured.Flags != nil {
				delete(captured.Flags, "kw:lifelink")
				delete(captured.Flags, "kw:haste")
			}
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"target":   target.Card.DisplayName(),
		"from_seat": origOwner,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"opp_gain_control_treasure_trigger_event_not_emitted_engine_side")
}
