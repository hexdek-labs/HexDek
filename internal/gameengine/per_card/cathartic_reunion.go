package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// cathartic_reunion.go — per_card handler for Cathartic Reunion.
//
// Oracle text (Sorcery, {1}{R}):
//
//	As an additional cost to cast this spell, discard two cards.
//	Draw three cards.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so on resolution Cathartic Reunion drew nothing — a key
// graveyard-fuel / dig spell running dead. Verified inert: no registered
// handler for "Cathartic Reunion".
//
// Implemented here (OnResolve): draw three cards for the caster. The
// "discard two cards" additional cost is paid at cast time by the cost
// pipeline (a pre-existing, separate concern); this handler is only the
// resolve-time draw, mirroring how the other discard-cost looters split
// cost (cast) from effect (resolve).

func init() {
	registerCatharticReunion(Global())
	AddResetHook(registerCatharticReunion)
}

func registerCatharticReunion(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Cathartic Reunion", catharticReunionResolve)
}

func catharticReunionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cathartic_reunion"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawn := 0
	for i := 0; i < 3; i++ {
		if drawOne(gs, seat, "Cathartic Reunion") != nil {
			drawn++
		}
	}
	emit(gs, slug, "Cathartic Reunion", map[string]interface{}{
		"seat":  seat,
		"drawn": drawn,
	})
}
