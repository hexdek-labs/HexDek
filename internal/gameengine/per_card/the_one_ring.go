package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheOneRing wires up The One Ring.
//
// Oracle text:
//
//   Indestructible
//   When The One Ring enters the battlefield, if you cast it, you gain
//   protection from everything until your next turn.
//   At the beginning of your upkeep, you lose 1 life for each burden
//   counter on The One Ring.
//   {T}: Put a burden counter on The One Ring, then draw a card for
//   each burden counter on The One Ring.
//
// cEDH staple. The key mechanics:
//   1. ETB: protection from everything until your next turn (implemented
//      as a prevention shield; MVP: set a flag).
//   2. Upkeep: lose 1 life per burden counter.
//   3. Activated: tap, add burden counter, draw N cards.
func registerTheOneRing(r *Registry) {
	r.OnETB("The One Ring", theOneRingETB)
	r.OnActivated("The One Ring", theOneRingActivated)
	r.OnTrigger("The One Ring", "upkeep_controller", theOneRingUpkeep)
}

func theOneRingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_one_ring_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller

	// Grant protection from everything until your next turn.
	// MVP: we stamp a prevention shield that blocks all damage to this
	// player until their next turn. Full "protection from everything"
	// (can't be targeted, dealt damage, blocked, enchanted/equipped by)
	// is broader than we model; we cover the damage prevention aspect.
	gameengine.AddPreventionShield(gs, gameengine.PreventionShield{
		TargetSeat: seat,
		Amount:     -1, // unlimited — prevent all damage
		SourceCard: "The One Ring",
	})
	// Also stamp a game flag so the phase loop can expire the shield.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["one_ring_protection_seat_"+intToStr(seat)] = gs.Turn

	// Also set an indestructible flag on the permanent itself.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["indestructible"] = 1

	emit(gs, slug, "The One Ring", map[string]interface{}{
		"seat":       seat,
		"protection": "until_your_next_turn",
	})
}

func theOneRingActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_one_ring_draw"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "The One Ring", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Tap.
	src.Tapped = true

	// Put a burden counter on The One Ring.
	src.AddCounter("burden", 1)

	// Draw cards equal to burden counters.
	burdens := src.Counters["burden"]
	drawn := 0
	for i := 0; i < burdens; i++ {
		c := drawOne(gs, seat, "The One Ring")
		if c != nil {
			drawn++
		}
	}

	emit(gs, slug, "The One Ring", map[string]interface{}{
		"seat":     seat,
		"burdens":  burdens,
		"drawn":    drawn,
	})
}

func theOneRingUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_one_ring_upkeep"
	if gs == nil || perm == nil {
		return
	}
	// Only fires during controller's upkeep.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	burdens := 0
	if perm.Counters != nil {
		burdens = perm.Counters["burden"]
	}
	if burdens <= 0 {
		return
	}
	// Lose 1 life per burden counter.
	gs.Seats[seat].Life -= burdens
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "The One Ring",
		Amount: burdens,
		Details: map[string]interface{}{
			"reason":  "burden_counters",
			"burdens": burdens,
		},
	})
	emit(gs, slug, "The One Ring", map[string]interface{}{
		"seat":      seat,
		"burdens":   burdens,
		"life_lost": burdens,
		"life_now":  gs.Seats[seat].Life,
	})
	_ = gs.CheckEnd()
}
