package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheReaperKingNoMore wires The Reaper, King No More.
//
// Oracle text:
//
//	When The Reaper enters, put a -1/-1 counter on each of up to two
//	target creatures.
//	Whenever a creature an opponent controls with a -1/-1 counter on
//	it dies, you may put that card onto the battlefield under your
//	control. Do this only once each turn.
//
// Implementation:
//   - ETB: pick up to two highest-power opponent creatures and place
//     a -1/-1 counter on each.
//   - creature_dies: gate to (a) controller is not Reaper's controller,
//     (b) the dying permanent had at least one -1/-1 counter on it
//     before death, and (c) we haven't already stolen this turn (turn
//     flag stored on Reaper.Flags). Move the dying card from its
//     owner's graveyard to Reaper's controller's battlefield, with
//     control set to Reaper's controller.
//
// emitPartial: target-prompt routing is resolved heuristically here
// (largest opponent creatures / always steal); full UI prompt
// integration is engine-side TODO.
func registerTheReaperKingNoMore(r *Registry) {
	r.OnETB("The Reaper, King No More", theReaperETB)
	r.OnTrigger("The Reaper, King No More", "creature_dies", theReaperDies)
}

func theReaperETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_reaper_etb_minus11_counters"
	if gs == nil || perm == nil {
		return
	}
	type cand struct {
		p   *gameengine.Permanent
		pow int
	}
	var cands []cand
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			cands = append(cands, cand{p, gs.PowerOf(p)})
		}
	}
	for k := 0; k < 2 && len(cands) > 0; k++ {
		bestIdx := 0
		for i := 1; i < len(cands); i++ {
			if cands[i].pow > cands[bestIdx].pow {
				bestIdx = i
			}
		}
		cands[bestIdx].p.AddCounter("-1/-1", 1)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"target":      cands[bestIdx].p.Card.DisplayName(),
			"target_seat": cands[bestIdx].p.Controller,
		})
		cands = append(cands[:bestIdx], cands[bestIdx+1:]...)
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"target_choice_resolved_heuristically_largest_opponent_creatures")
}

func theReaperDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_reaper_steal_dying"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// R53 batch O: the once-per-turn gate now lives on the controller's
	// Seat.Flags instead of perm.Flags. The printed text — "Do this
	// only once each turn" — caps the controller, not the source. The
	// old perm.Flags scheme allowed a flicker (perm leaves and comes
	// back with a fresh Flags map) to bypass the cap and steal twice
	// in one turn. Keying off the controller's seat closes that hole.
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if dyingCard == nil || dyingPerm == nil {
		return
	}
	if controllerSeat == perm.Controller {
		return
	}
	hadMinus := false
	if dyingPerm.Counters != nil && dyingPerm.Counters["-1/-1"] > 0 {
		hadMinus = true
	}
	if !hadMinus {
		return
	}
	ownSeat := gs.Seats[perm.Controller]
	if ownSeat == nil {
		return
	}
	if ownSeat.Flags == nil {
		ownSeat.Flags = map[string]int{}
	}
	if ownSeat.Flags["reaper_stole_turn"] == gs.Turn+1 {
		return
	}
	mover := gameengine.MoveCard(gs, dyingCard, dyingCard.Owner, "graveyard", "battlefield", "the_reaper_steal")
	if mover.Permanent != nil {
		mover.Permanent.Controller = perm.Controller
	}
	ownSeat.Flags["reaper_stole_turn"] = gs.Turn + 1 // +1 to avoid 0-collision on turn 0
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"stolen":    dyingCard.DisplayName(),
		"from_seat": controllerSeat,
		"to_seat":   perm.Controller,
	})
}
