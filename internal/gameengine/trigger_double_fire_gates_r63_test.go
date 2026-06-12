package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — PROGRESSION double-fire gates. A card whose printed triggered
// ability is implemented by a per_card handler must resolve EXACTLY
// once: the per_card registration is authoritative and the generic AST
// trigger push is suppressed (mirrors #1059's attack-observer gate).
// Pre-gate, every such card resolved twice (39-finding PROGRESSION
// sweep: Phyrexian Arena drew 2, Gyruda milled 8, Thraben made 2 Clues…).

// installFakePerCardOwner wires temp hook vars: ownership of (cardName,
// ownedEvents) + a TriggerHook that draws one card for seat 0 when any
// owned engine event fires (the "per_card implementation"). Returns a
// restore func.
func installFakePerCardOwner(cardName string, ownedEvents map[string]bool, fireEvents map[string]bool) func() {
	prevHas, prevTrig, prevETB := HasTriggerHook, TriggerHook, HasETBHook
	HasTriggerHook = func(name, event string) bool {
		return name == cardName && ownedEvents[NormalizeEventSingle(event)]
	}
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if fireEvents[NormalizeEventSingle(event)] {
			drawOne(gs, 0)
		}
	}
	HasETBHook = nil
	return func() { HasTriggerHook, TriggerHook, HasETBHook = prevHas, prevTrig, prevETB }
}

// drawOne moves one library card to hand directly (observable delta
// without engine draw side effects).
func drawOne(gs *GameState, seat int) {
	s := gs.Seats[seat]
	if len(s.Library) == 0 {
		return
	}
	s.Hand = append(s.Hand, s.Library[0])
	s.Library = s.Library[1:]
}

// gateFixture builds a 2-seat game with a library, plus a card whose
// AST carries one Triggered ability (event, effect=Draw 1).
func gateFixture(t *testing.T, cardName, trigEvent, raw string) (*GameState, *Permanent) {
	t.Helper()
	gs := NewGameState(2, nil, nil)
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Filler", Owner: 0})
	}
	card := &Card{
		Name: cardName, Owner: 0, Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: cardName,
			Abilities: []gameast.Ability{
				&gameast.Triggered{
					Trigger: gameast.Trigger{Event: trigEvent},
					Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "self"}},
					Raw:     raw,
				},
			},
		},
	}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Flags: map[string]int{}, Counters: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	return gs, perm
}

func handCount(gs *GameState) int { return len(gs.Seats[0].Hand) }

func TestDoubleFireGate_PhaseUpkeep(t *testing.T) {
	gs, _ := gateFixture(t, "Gate Test Arena", "upkeep", "at the beginning of your upkeep, draw a card")
	restore := installFakePerCardOwner("Gate Test Arena",
		map[string]bool{NormalizeEventSingle("upkeep"): true},
		map[string]bool{NormalizeEventSingle("upkeep"): true})
	defer restore()

	gs.Active = 0
	before := handCount(gs)
	FirePhaseTriggers(gs, "beginning", "upkeep")
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("per_card-owned upkeep trigger must resolve exactly once; drew %d", got)
	}
}

func TestDoubleFireGate_PhaseEndStep(t *testing.T) {
	gs, _ := gateFixture(t, "Gate Test Kynaios", "end_step", "at the beginning of your end step, draw a card")
	restore := installFakePerCardOwner("Gate Test Kynaios",
		map[string]bool{NormalizeEventSingle("end_step"): true},
		map[string]bool{NormalizeEventSingle("end_step"): true})
	defer restore()

	gs.Active = 0
	before := handCount(gs)
	FirePhaseTriggers(gs, "ending", "end")
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("per_card-owned end-step trigger must resolve exactly once; drew %d", got)
	}
}

func TestDoubleFireGate_Dies(t *testing.T) {
	gs, perm := gateFixture(t, "Gate Test Wurm", "dies", "when this creature dies, draw a card")
	restore := installFakePerCardOwner("Gate Test Wurm",
		map[string]bool{NormalizeEventSingle("creature_dies"): true},
		map[string]bool{NormalizeEventSingle("creature_dies"): true})
	defer restore()

	// Real death flow removes the permanent from the battlefield BEFORE
	// FireZoneChangeTriggers (the observer scan must not re-see it).
	gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
	before := handCount(gs)
	FireZoneChangeTriggers(gs, perm, perm.Card, "battlefield", "graveyard")
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("per_card-owned dies trigger must resolve exactly once; drew %d", got)
	}
}

func TestDoubleFireGate_SelfAttack(t *testing.T) {
	gs, perm := gateFixture(t, "Gate Test Brimaz", "attacks", "whenever this creature attacks, draw a card")
	restore := installFakePerCardOwner("Gate Test Brimaz",
		map[string]bool{NormalizeEventSingle("creature_attacks"): true},
		map[string]bool{NormalizeEventSingle("creature_attacks"): true})
	defer restore()

	before := handCount(gs)
	fireAttackTriggers(gs, 0, []*Permanent{perm})
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("per_card-owned attack trigger must resolve exactly once; drew %d", got)
	}
}

func TestDoubleFireGate_ETB(t *testing.T) {
	gs, perm := gateFixture(t, "Gate Test Inspector", "etb", "when this creature enters, draw a card")
	restore := installFakePerCardOwner("Gate Test Inspector",
		map[string]bool{NormalizeEventSingle("permanent_etb"): true},
		map[string]bool{NormalizeEventSingle("permanent_etb"): true})
	defer restore()

	before := handCount(gs)
	// FirePermanentETBTriggers fires FireCardTrigger("permanent_etb")
	// itself — the per_card dispatch is part of the same chokepoint.
	FirePermanentETBTriggers(gs, perm)
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("per_card-owned etb trigger must resolve exactly once; drew %d", got)
	}
}

// TestDoubleFireGate_UnownedASTStillFires pins the other side: with NO
// per_card ownership the AST trigger must keep firing (the gate must
// not over-suppress).
func TestDoubleFireGate_UnownedASTStillFires(t *testing.T) {
	gs, _ := gateFixture(t, "Gate Test Vanilla", "upkeep", "at the beginning of your upkeep, draw a card")
	prevHas, prevTrig := HasTriggerHook, TriggerHook
	HasTriggerHook = func(string, string) bool { return false }
	TriggerHook = func(*GameState, string, map[string]interface{}) {}
	defer func() { HasTriggerHook, TriggerHook = prevHas, prevTrig }()

	gs.Active = 0
	before := handCount(gs)
	FirePhaseTriggers(gs, "beginning", "upkeep")
	DrainStack(gs)
	if got := handCount(gs) - before; got != 1 {
		t.Fatalf("unowned AST upkeep trigger must still resolve exactly once; drew %d", got)
	}
}
