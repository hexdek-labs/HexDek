package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// necromantic_selection_test.go — regression pins for the Necromantic
// Selection handler. Before this handler the spell parsed to three bare
// `custom` nodes and did nothing on resolve.

func bfCreatures(gs *gameengine.GameState, seat int) []*gameengine.Permanent {
	var out []*gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsCreature() {
			out = append(out, p)
		}
	}
	return out
}

func TestNecromanticSelection_WipesAndReanimatesStrongestUnderControl(t *testing.T) {
	gs := newGame(t, 2)

	// Caster's weak creature.
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	bear.Card.BasePower, bear.Card.BaseToughness = 2, 2

	// Opponent's strong creature — should be the one reanimated.
	maw := addPerm(gs, 1, "Colossal Dreadmaw", "creature", "dinosaur")
	maw.Card.BasePower, maw.Card.BaseToughness = 6, 6

	// Opponent's mid creature.
	ogre := addPerm(gs, 1, "Hill Giant", "creature", "giant")
	ogre.Card.BasePower, ogre.Card.BaseToughness = 3, 3

	// A creature ALREADY in a graveyard (not put there "this way") with
	// the highest power of all — must NOT be eligible for return.
	preDead := addCard(gs, 1, "Ancient Titan", "creature", "giant")
	preDead.BasePower, preDead.BaseToughness = 10, 10
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, preDead)

	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       &gameengine.Card{Name: "Necromantic Selection", Owner: 0, Types: []string{"sorcery"}},
		CastZone:   "hand",
	}
	necromanticSelectionResolve(gs, item)

	// Opponent has no creatures left.
	if got := bfCreatures(gs, 1); len(got) != 0 {
		t.Fatalf("seat 1 should have 0 creatures after wipe, got %d", len(got))
	}

	// Caster controls exactly one creature — the reanimated Dreadmaw.
	mine := bfCreatures(gs, 0)
	if len(mine) != 1 {
		t.Fatalf("seat 0 should control exactly 1 creature (the reanimated one), got %d", len(mine))
	}
	got := mine[0]
	if got.Card.DisplayName() != "Colossal Dreadmaw" {
		t.Fatalf("expected strongest DIED creature (Colossal Dreadmaw) reanimated, got %q", got.Card.DisplayName())
	}
	if got.Controller != 0 {
		t.Fatalf("reanimated creature must be under caster control (seat 0), got %d", got.Controller)
	}

	// It's a black Zombie in addition to its types/colors.
	hasB, hasZombie := false, false
	for _, c := range got.Card.Colors {
		if c == "B" {
			hasB = true
		}
	}
	for _, ty := range got.Card.Types {
		if ty == "zombie" {
			hasZombie = true
		}
	}
	if !hasB {
		t.Errorf("reanimated creature must gain black color, colors=%v", got.Card.Colors)
	}
	if !hasZombie {
		t.Errorf("reanimated creature must gain Zombie type, types=%v", got.Card.Types)
	}

	// The pre-existing graveyard creature (higher power, but not put there
	// "this way") must remain in the graveyard, not reanimated.
	stillDead := false
	for _, c := range gs.Seats[1].Graveyard {
		if c == preDead {
			stillDead = true
		}
	}
	if !stillDead {
		t.Error("pre-existing graveyard creature must NOT be eligible for return (it stays in graveyard)")
	}
}

func TestNecromanticSelection_NoCreatures_NoPanic(t *testing.T) {
	gs := newGame(t, 2)
	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       &gameengine.Card{Name: "Necromantic Selection", Owner: 0, Types: []string{"sorcery"}},
		CastZone:   "hand",
	}
	necromanticSelectionResolve(gs, item) // must not panic / no return
	if got := bfCreatures(gs, 0); len(got) != 0 {
		t.Fatalf("no creatures should exist, got %d", len(got))
	}
}
