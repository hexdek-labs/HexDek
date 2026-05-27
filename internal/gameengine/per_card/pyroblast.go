package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPyroblast wires Pyroblast (and Red Elemental Blast — same
// oracle text in the modal era, both are blasted off this file).
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Pyroblast):
//
//	Choose one —
//	  • Counter target spell if it's blue.
//	  • Destroy target permanent if it's blue.
//
// {R} instant. cEDH sideboard staple — single-mana hoser for the
// blue-spell-heavy meta. Counters the spell on resolve only if
// the target was/is blue; destroys the permanent on resolve only
// if the target was/is blue.
//
// Implementation chooses mode at resolve based on what's available:
//   - If a blue spell is on the stack (excluding this Pyroblast),
//     counter it. Reuses findCounterableSpell with cardHasColor(_, "U")
//     filter.
//   - Otherwise, scan all seats' battlefields for the
//     highest-priority blue permanent (any opponent's first, then
//     own if none on opponents) and destroy it via DestroyPermanent.
//   - If no blue target exists in either zone, the spell resolves
//     with no effect (per the "if it's blue" clause — the target
//     was either never blue or non-existent). Emit a fail breadcrumb
//     so the engine logs the no-op.
//
// Red Elemental Blast has identical pre-MH3 effect and the same
// resolution shape; we wire both names to the same handler.
func registerPyroblast(r *Registry) {
	r.OnResolve("Pyroblast", pyroblastResolve)
	r.OnResolve("Red Elemental Blast", pyroblastResolve)
}

func pyroblastResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "pyroblast"
	if gs == nil || item == nil {
		return
	}

	// Mode 1: counter a blue spell on the stack.
	target := findCounterableSpell(gs, item.Controller, func(si *gameengine.StackItem) bool {
		if si == nil || si.Card == nil {
			return false
		}
		return cardHasColor(si.Card, "U")
	})
	if target != nil {
		target.Countered = true
		emitCounter(gs, slug, "Pyroblast", item.Controller, target)
		return
	}

	// Mode 2: destroy a blue permanent. Prefer opponents' blue
	// permanents over own (Pyroblast is targeted removal — typical
	// use is hosing opponent's blue threats).
	var dest *gameengine.Permanent
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == item.Controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasColor(p.Card, "U") {
				dest = p
				break
			}
		}
		if dest != nil {
			break
		}
	}
	if dest == nil {
		// Fall back to own blue permanent if no opponent's blue exists.
		if item.Controller >= 0 && item.Controller < len(gs.Seats) && gs.Seats[item.Controller] != nil {
			for _, p := range gs.Seats[item.Controller].Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if cardHasColor(p.Card, "U") {
					dest = p
					break
				}
			}
		}
	}
	if dest == nil {
		emitFail(gs, slug, item.Card.DisplayName(), "no_blue_target", nil)
		return
	}
	gameengine.DestroyPermanent(gs, dest, nil)
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":      item.Controller,
		"destroyed": dest.Card.DisplayName(),
		"mode":      "destroy_blue_permanent",
	})
}
