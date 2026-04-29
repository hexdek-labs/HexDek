package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSimicBasilisk wires Simic Basilisk's activated ability.
//
// Oracle: "{1}{G}: Until end of turn, target creature with a +1/+1 counter
// on it gains 'Whenever this creature deals combat damage to a creature,
// destroy that creature at end of combat.'"
//
// Graft 3 is handled by the generic graft keyword (keywords_misc.go).
// This handler covers the activated ability that grants basilisk-like
// deathtouch-to-creatures to any creature with a +1/+1 counter.
func registerSimicBasilisk(r *Registry) {
	r.OnActivated("Simic Basilisk", simicBasiliskActivated)
}

func simicBasiliskActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	controller := src.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	// Find target creature with a +1/+1 counter controlled by us.
	var target *gameengine.Permanent
	for _, p := range gs.Seats[controller].Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() || p == src {
			continue
		}
		if p.Flags != nil && p.Flags["counter:+1/+1"] > 0 {
			target = p
			break
		}
	}
	if target == nil {
		return
	}

	// Grant the basilisk ability as a flag until end of turn.
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["basilisk_granted"] = 1

	// Register a delayed trigger: at end of combat, destroy any creature
	// marked by the basilisk combat hook.
	gs.DelayedTriggers = append(gs.DelayedTriggers, &gameengine.DelayedTrigger{
		TriggerAt:      "end_of_combat",
		ControllerSeat: controller,
		SourceCardName: "Simic Basilisk",
		CreatedTurn:    gs.Turn,
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if target.Flags != nil {
				delete(target.Flags, "basilisk_combat_hit")
			}
			for _, seat := range gs.Seats {
				if seat == nil {
					continue
				}
				snap := make([]*gameengine.Permanent, len(seat.Battlefield))
				copy(snap, seat.Battlefield)
				for _, p := range snap {
					if p.Flags != nil && p.Flags["basilisk_marked_destroy"] > 0 {
						delete(p.Flags, "basilisk_marked_destroy")
						gameengine.DestroyPermanent(gs, p, src)
					}
				}
			}
		},
	})

	gs.LogEvent(gameengine.Event{
		Kind:   "basilisk_ability_granted",
		Seat:   controller,
		Source: "Simic Basilisk",
		Details: map[string]interface{}{
			"target": target.Card.DisplayName(),
		},
	})
}

