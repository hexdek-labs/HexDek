package gameengine

import (
	"testing"
)

// r63 depth-frontier (loki seed 7 / game 3548, max-turns 120):
// seat 2 died to Glacial Chasm's cumulative upkeep MID-upkeep, then its own
// "your upkeep" trigger (Braid of Fire) still fired, pushed onto the stack,
// and resolved — adding {R}{R}{R} to the eliminated seat's pool. The chaos
// invariant tripped ResourceConservation: "seat is Lost but has ManaPool=3".
//
// CR §800.4a: once a player leaves the game, every ability they control
// ceases to exist — including triggers that fire after the elimination
// sweep already ran. These tests pin the three guards added for the class:
//   - PushPerCardTrigger / PushTriggeredAbilityWithIf cease at formation
//   - AddMana / AddRestrictedMana refuse to refill a departed seat's pool

func TestAddMana_EliminatedSeatHoldsNoMana(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	seat := gs.Seats[2]
	seat.LeftGame = true
	seat.ManaPool = 0

	AddMana(gs, seat, "R", 3, "Braid of Fire")
	if seat.ManaPool != 0 {
		t.Fatalf("AddMana to a LeftGame seat should be a no-op; got ManaPool=%d", seat.ManaPool)
	}

	AddRestrictedMana(gs, seat, 2, "R", "", "Braid of Fire")
	if seat.ManaPool != 0 {
		t.Fatalf("AddRestrictedMana to a LeftGame seat should be a no-op; got ManaPool=%d", seat.ManaPool)
	}
}

func TestAddMana_LivingSeatStillGainsMana(t *testing.T) {
	// Guard the guard: a living seat must be unaffected.
	gs := newMultiplayerGame(t, 4)
	seat := gs.Seats[2]
	seat.ManaPool = 0
	AddMana(gs, seat, "R", 3, "Braid of Fire")
	if seat.ManaPool != 3 {
		t.Fatalf("AddMana to a living seat should add 3; got ManaPool=%d", seat.ManaPool)
	}
}

func TestPushPerCardTrigger_CeasesForLeftGameController(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	seat := gs.Seats[2]
	seat.LeftGame = true

	perm := &Permanent{
		Card: &Card{
			Name: "Braid of Fire", TypeLine: "Enchantment",
			Types: []string{"enchantment"}, Owner: 2,
		},
		Controller: 2, Owner: 2,
		Counters: map[string]int{"age": 3}, Flags: map[string]int{},
	}

	ran := false
	handler := func(g *GameState, p *Permanent, ctx map[string]interface{}) {
		ran = true
		AddMana(g, g.Seats[p.Controller], "R", 3, "Braid of Fire")
	}

	stackBefore := len(gs.Stack)
	PushPerCardTrigger(gs, perm, handler, map[string]interface{}{"active_seat": 2})

	if ran {
		t.Fatalf("per-card trigger handler ran for a controller who left the game (§800.4a)")
	}
	if len(gs.Stack) != stackBefore {
		t.Fatalf("ceased trigger must not be pushed onto the stack; stack grew %d -> %d", stackBefore, len(gs.Stack))
	}
	if seat.ManaPool != 0 {
		t.Fatalf("eliminated seat gained mana from a ceased trigger; ManaPool=%d", seat.ManaPool)
	}
}

func TestPushPerCardTrigger_LivingControllerStillFires(t *testing.T) {
	// Guard the guard: a living controller's trigger must still resolve.
	gs := newMultiplayerGame(t, 4)
	gs.Active = 2
	perm := &Permanent{
		Card: &Card{
			Name: "Braid of Fire", TypeLine: "Enchantment",
			Types: []string{"enchantment"}, Owner: 2,
		},
		Controller: 2, Owner: 2,
		Counters: map[string]int{"age": 3}, Flags: map[string]int{},
	}
	gs.Seats[2].Battlefield = append(gs.Seats[2].Battlefield, perm)

	ran := false
	handler := func(g *GameState, p *Permanent, ctx map[string]interface{}) {
		ran = true
		AddMana(g, g.Seats[p.Controller], "R", 3, "Braid of Fire")
	}
	PushPerCardTrigger(gs, perm, handler, map[string]interface{}{"active_seat": 2})

	if !ran {
		t.Fatalf("per-card trigger handler should run for a living controller")
	}
	if gs.Seats[2].ManaPool != 3 {
		t.Fatalf("living controller's Braid of Fire should add 3 mana; got %d", gs.Seats[2].ManaPool)
	}
}
