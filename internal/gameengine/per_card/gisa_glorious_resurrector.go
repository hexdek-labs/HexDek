package per_card

import (
	"sync"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// gisaExiledPile tracks creature cards that "would die → exile instead"
// took from opponents. Keyed by Gisa *Permanent; value is the slice of
// captured *Card. Upkeep recycles the pile onto Gisa's battlefield.
var gisaExiledPile sync.Map

// registerGisaGloriousResurrector wires Gisa, Glorious Resurrector.
//
// Oracle text:
//
//	If a creature an opponent controls would die, exile it instead.
//	At the beginning of your upkeep, put all creature cards exiled with
//	Gisa onto the battlefield under your control. They gain decayed.
//
// The "would die → exile instead" replacement is engine-level and
// requires replacement-effect registration. We track exiled-with-Gisa
// via seat flag and pull from a side pile.
func registerGisaGloriousResurrector(r *Registry) {
	r.OnTrigger("Gisa, Glorious Resurrector", "upkeep", gisaResurrectorUpkeep)
	r.OnTrigger("Gisa, Glorious Resurrector", "creature_dies", gisaResurrectorDies)
}

// gisaResurrectorDies approximates "would die → exile instead". The
// engine fires creature_dies AFTER the creature has been moved to the
// graveyard; we retroactively pull it into Gisa's exile pile. The
// strategic outcome (opponent creature is unavailable for reanimation
// + reappears under our control next upkeep) matches the printed text,
// even though the timing isn't a true §614 replacement. R52 batchM.
func gisaResurrectorDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gisa_resurrector_exile_instead"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if dyingCard == nil {
		return
	}
	// Only fires for opponent creatures.
	if controllerSeat == perm.Controller {
		return
	}
	// Lift from the dying seat's graveyard if it's still there.
	ownerSeat := dyingCard.Owner
	if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
		ownerSeat = controllerSeat
	}
	if ownerSeat >= 0 && ownerSeat < len(gs.Seats) && gs.Seats[ownerSeat] != nil {
		gy := gs.Seats[ownerSeat].Graveyard
		for i, c := range gy {
			if c == dyingCard {
				gs.Seats[ownerSeat].Graveyard = append(gy[:i], gy[i+1:]...)
				break
			}
		}
	}
	// Stash to our exile pile (we'll re-spawn at upkeep).
	gs.Seats[perm.Controller].Exile = append(gs.Seats[perm.Controller].Exile, dyingCard)
	var pile []*gameengine.Card
	if v, ok := gisaExiledPile.Load(perm); ok {
		pile, _ = v.([]*gameengine.Card)
	}
	pile = append(pile, dyingCard)
	gisaExiledPile.Store(perm, pile)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"exiled":   dyingCard.DisplayName(),
		"from_seat": controllerSeat,
	})
}

// gisaResurrectorUpkeep returns every captured creature to Gisa's
// controller's battlefield with the "decayed" flag set. Decayed
// creatures can't block and self-sacrifice when they attack —
// stamping kw:decayed is the AST-layer marker for those riders.
func gisaResurrectorUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gisa_resurrector_upkeep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	v, ok := gisaExiledPile.Load(perm)
	if !ok {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"reanimated": 0,
		})
		return
	}
	pile, _ := v.([]*gameengine.Card)
	if len(pile) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"reanimated": 0,
		})
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	returned := 0
	for _, c := range pile {
		if c == nil {
			continue
		}
		// Remove from our exile zone if still there.
		for i, ec := range seat.Exile {
			if ec == c {
				seat.Exile = append(seat.Exile[:i], seat.Exile[i+1:]...)
				break
			}
		}
		newPerm := enterBattlefieldWithETB(gs, perm.Controller, c, false)
		if newPerm != nil {
			if newPerm.Flags == nil {
				newPerm.Flags = map[string]int{}
			}
			newPerm.Flags["kw:decayed"] = 1
			returned++
		}
	}
	gisaExiledPile.Delete(perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"reanimated": returned,
	})
}
