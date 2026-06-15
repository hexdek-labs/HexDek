package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §107.3 / §608.2h — the X chosen when a spell is put on the stack is
// LOCKED and used as the effect amount. Regression for the r63 X-spell
// probe: nothing copied StackItem.ChosenX into gs.Flags["x"] at resolution,
// so every generic-AST X-spell (one without a bespoke per_card handler)
// resolved with X=0 — Fireball dealt 0, "draw X" drew 0, "X tokens" made 0.

func xs_game() *GameState { return NewGameState(2, rand.New(rand.NewSource(4)), nil) }

func xs_resolveTop(gs *GameState, item *StackItem) {
	gs.Stack = append(gs.Stack, item)
	ResolveStackTop(gs)
}

// (a)/(d): "deal X damage to target player" deals the chosen X.
func TestXSpell_DealXDamage_UsesChosenX(t *testing.T) {
	gs := xs_game()
	eff := &gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "x"},
		Target: gameast.Filter{Base: "player", Targeted: true}}
	item := &StackItem{Controller: 0,
		Card:    &Card{Name: "Fireball", Owner: 0, Types: []string{"sorcery"}},
		Effect:  eff, ChosenX: 5,
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}}}
	life0 := gs.Seats[1].Life
	xs_resolveTop(gs, item)
	if gs.Seats[1].Life != life0-5 {
		t.Errorf("X=5 Fireball must deal 5; defender %d -> %d", life0, gs.Seats[1].Life)
	}
}

// (a): X=0 deals nothing (the choice is respected, not a default).
func TestXSpell_ZeroX_DealsNothing(t *testing.T) {
	gs := xs_game()
	eff := &gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "x"},
		Target: gameast.Filter{Base: "player", Targeted: true}}
	item := &StackItem{Controller: 0,
		Card:    &Card{Name: "Fireball", Owner: 0, Types: []string{"sorcery"}},
		Effect:  eff, ChosenX: 0,
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}}}
	life0 := gs.Seats[1].Life
	xs_resolveTop(gs, item)
	if gs.Seats[1].Life != life0 {
		t.Errorf("X=0 must deal 0; defender %d -> %d", life0, gs.Seats[1].Life)
	}
}

// gs.Flags["x"] is restored after resolution — a later non-X spell must not
// inherit a stale X.
func TestXSpell_XValue_NotLeakedAfterResolution(t *testing.T) {
	gs := xs_game()
	gs.Flags = map[string]int{}
	eff := &gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "x"},
		Target: gameast.Filter{Base: "player", Targeted: true}}
	item := &StackItem{Controller: 0,
		Card:    &Card{Name: "Fireball", Owner: 0, Types: []string{"sorcery"}},
		Effect:  eff, ChosenX: 7,
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}}}
	xs_resolveTop(gs, item)
	if v, ok := gs.Flags["x"]; ok && v != 0 {
		t.Errorf("gs.Flags[x] must be cleared after resolution, leaked %d", v)
	}
}

// (f): a COPY of an X spell uses the original's chosen X.
func TestXSpell_Copy_InheritsChosenX(t *testing.T) {
	gs := xs_game()
	// Original Fireball with X=4 sitting on the stack, targeting seat 1.
	orig := &StackItem{Controller: 0,
		Card:    &Card{Name: "Fireball", Owner: 0, Types: []string{"sorcery"}},
		Effect:  &gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "x"}, Target: gameast.Filter{Base: "player", Targeted: true}},
		ChosenX: 4, Kind: "spell",
		Targets: []Target{{Kind: TargetKindSeat, Seat: 1}}}
	gs.Stack = append(gs.Stack, orig)

	// Fork/Twincast: copy the spell.
	src := &Permanent{Card: &Card{Name: "Twincast", Owner: 0}, Controller: 0, Owner: 0, Flags: map[string]int{}}
	resolveCopySpell(gs, src, &gameast.CopySpell{MayChooseNewTargets: true})

	// The copy is on top of the stack — resolve it; it must deal 4 (not 0).
	life0 := gs.Seats[1].Life
	ResolveStackTop(gs)
	if gs.Seats[1].Life != life0-4 {
		t.Errorf("copy of an X=4 Fireball must deal 4; defender %d -> %d", life0, gs.Seats[1].Life)
	}
}
