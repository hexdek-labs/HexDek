package gameengine

import (
	"testing"
)

// r63 eliminated-seat deep sweep — delayed-trigger leak (CR §800.4a).
//
// A §603.7 delayed triggered ability ("at the beginning of the next end
// step / your next upkeep / end of combat / on <event>, do X") sits in
// gs.DelayedTriggers until its boundary. The leave-game sweep purged the
// stack and the §603.3b pending-trigger batch, but never touched the
// delayed-trigger pool — so a delayed trigger controlled by a departed
// seat still fired at the next matching boundary, acting on a player who
// has left the game (adding mana → ResourceConservation, drawing, making
// tokens). Same class as the Myr Moonvessel pending-trigger leak and the
// Braid of Fire mana leak, one pool over.
//
// These pin the three guards added for the class:
//   - HandleSeatElimination purges gs.DelayedTriggers for the leaver
//   - FireDelayedTriggers skips a left-game controller at a boundary
//   - FireEventDelayedTriggers skips a left-game controller on an event

func TestHandleSeatElimination_PurgesDelayedTriggers(t *testing.T) {
	gs := newMultiplayerGame(t, 4)

	fired := false
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: 2,
		SourceCardName: "Some End-Step Effect",
		EffectFn: func(g *GameState) {
			fired = true
			AddMana(g, g.Seats[2], "R", 3, "Some End-Step Effect")
		},
	})
	// A survivor's delayed trigger must be preserved.
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: 1,
		SourceCardName: "Survivor Effect",
		EffectFn:       func(g *GameState) {},
	})

	gs.Seats[2].Lost = true
	HandleSeatElimination(gs, 2)

	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.ControllerSeat == 2 && !dt.Consumed {
			t.Fatalf("eliminated seat's delayed trigger survived the §800.4a sweep")
		}
	}
	survived := false
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.ControllerSeat == 1 && !dt.Consumed {
			survived = true
		}
	}
	if !survived {
		t.Fatalf("a living seat's delayed trigger was wrongly purged")
	}

	// Drive an end step: the purged trigger must not fire.
	FireDelayedTriggers(gs, "ending", "end")
	if fired {
		t.Fatalf("purged delayed trigger fired at the end step for a departed seat")
	}
	if gs.Seats[2].ManaPool != 0 {
		t.Fatalf("eliminated seat gained mana from a leaked delayed trigger; ManaPool=%d", gs.Seats[2].ManaPool)
	}
}

func TestFireDelayedTriggers_SkipsLeftGameController(t *testing.T) {
	// Defense in depth: even if a delayed trigger slips past the sweep
	// (controller leaves via a non-HandleSeatElimination path), the
	// boundary fire must skip a left-game controller.
	gs := newMultiplayerGame(t, 4)
	fired := false
	gs.DelayedTriggers = append(gs.DelayedTriggers, &DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: 2,
		SourceCardName: "Leaked Effect",
		EffectFn:       func(g *GameState) { fired = true },
	})
	gs.Seats[2].LeftGame = true

	FireDelayedTriggers(gs, "ending", "end")
	if fired {
		t.Fatalf("delayed trigger fired at boundary for a LeftGame controller (§800.4a)")
	}
}

func TestFireDelayedTriggers_LivingControllerStillFires(t *testing.T) {
	// Guard the guard.
	gs := newMultiplayerGame(t, 4)
	gs.Active = 2
	fired := false
	gs.DelayedTriggers = append(gs.DelayedTriggers, &DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: 2,
		SourceCardName: "Live Effect",
		EffectFn:       func(g *GameState) { fired = true },
	})
	FireDelayedTriggers(gs, "ending", "end")
	if !fired {
		t.Fatalf("living controller's delayed trigger should fire at the end step")
	}
}

func TestFireEventDelayedTriggers_SkipsLeftGameController(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	fired := false
	gs.DelayedTriggers = append(gs.DelayedTriggers, &DelayedTrigger{
		TriggerAt:      "on_event",
		ControllerSeat: 2,
		SourceCardName: "Leaked On-Event Effect",
		ConditionFn:    func(g *GameState, ev *Event) bool { return true },
		EffectFn:       func(g *GameState) { fired = true },
	})
	gs.Seats[2].LeftGame = true

	FireEventDelayedTriggers(gs, &Event{Kind: "card_drawn", Seat: 0})
	if fired {
		t.Fatalf("on_event delayed trigger fired for a LeftGame controller (§800.4a)")
	}
}

func TestFireEventDelayedTriggers_LivingControllerStillFires(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	fired := false
	gs.DelayedTriggers = append(gs.DelayedTriggers, &DelayedTrigger{
		TriggerAt:      "on_event",
		ControllerSeat: 2,
		SourceCardName: "Live On-Event Effect",
		ConditionFn:    func(g *GameState, ev *Event) bool { return true },
		EffectFn:       func(g *GameState) { fired = true },
	})
	FireEventDelayedTriggers(gs, &Event{Kind: "card_drawn", Seat: 0})
	if !fired {
		t.Fatalf("living controller's on_event delayed trigger should fire")
	}
}
