package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// evacuation.go — per_card handler for Evacuation.
//
// Oracle text (Instant, {3}{U}{U}):
//
//	Return all creatures to their owners' hands.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so Evacuation — a marquee blue mass-bounce reset — returned
// nothing. Verified inert: no registered handler for "Evacuation".
//
// Implemented here (OnResolve): snapshot every creature on every
// battlefield, then route each through the canonical BouncePermanent to
// its owner's hand (tokens cease per §111.7, zone-change triggers fire).

func init() {
	registerEvacuation(Global())
	AddResetHook(registerEvacuation)
}

func registerEvacuation(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Evacuation", evacuationResolve)
}

func evacuationResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "evacuation"
	if gs == nil || item == nil {
		return
	}
	var creatures []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
	}
	bounced := 0
	for _, p := range creatures {
		if gameengine.BouncePermanent(gs, p, nil, "hand") {
			bounced++
		}
	}
	emit(gs, slug, "Evacuation", map[string]interface{}{
		"seat":    item.Controller,
		"bounced": bounced,
	})
	_ = gs.CheckEnd()
}
