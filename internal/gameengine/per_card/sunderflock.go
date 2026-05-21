package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSunderflock wires Sunderflock (Muninn parser-gap #25, 46,939 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{7}{U}{U}
//	Creature — Elemental
//	This spell costs {X} less to cast, where X is the greatest mana
//	value among Elementals you control.
//	Flying
//	When this creature enters, if you cast it, return all non-Elemental
//	creatures to their owners' hands.
//
// Implementation:
//   - OnETB: if was_cast, snapshot every non-Elemental creature across
//     all battlefields and BouncePermanent each (routes through ZCT and
//     commander redirect).
//   - The cost reduction is a §601.2f cost modifier that requires hooking
//     into CalculateTotalCost; there's no per_card extension point for
//     that today. Logged via emitPartial.
//   - Flying handled by the engine's keyword pipeline.
func registerSunderflock(r *Registry) {
	r.OnETB("Sunderflock", sunderflockETB)
}

func sunderflockETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sunderflock_etb_bounce"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["was_cast"] == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"bounced": 0,
			"reason":  "not_cast",
		})
		return
	}

	var victims []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsCreature() {
				continue
			}
			if cardHasType(p.Card, "elemental") {
				continue
			}
			victims = append(victims, p)
		}
	}
	bounced := 0
	for _, p := range victims {
		gameengine.BouncePermanent(gs, p, perm, "hand")
		bounced++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"bounced": bounced,
	})
	// Cost reduction is wired in cost_modifiers.go under "Sunderflock".
}
