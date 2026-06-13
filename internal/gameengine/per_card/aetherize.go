package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// aetherize.go — per_card handler for Aetherize.
//
// Oracle text (Instant, {3}{U}):
//
//	Return all attacking creatures to their owner's hand.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so Aetherize — a premier "fog + tempo blowout" against an
// alpha strike — returned nothing. Verified inert: no registered handler
// for "Aetherize".
//
// Implemented here (OnResolve): snapshot every attacking creature
// (p.IsAttacking()) and route each through BouncePermanent to its
// owner's hand.

func init() {
	registerAetherize(Global())
	AddResetHook(registerAetherize)
}

func registerAetherize(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Aetherize", aetherizeResolve)
}

func aetherizeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "aetherize"
	if gs == nil || item == nil {
		return
	}
	var attackers []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() && p.IsAttacking() {
				attackers = append(attackers, p)
			}
		}
	}
	bounced := 0
	for _, p := range attackers {
		if gameengine.BouncePermanent(gs, p, nil, "hand") {
			bounced++
		}
	}
	emit(gs, slug, "Aetherize", map[string]interface{}{
		"seat":    item.Controller,
		"bounced": bounced,
	})
	_ = gs.CheckEnd()
}
