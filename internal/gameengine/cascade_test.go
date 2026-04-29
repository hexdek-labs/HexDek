package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestHasCascadeKeyword_True(t *testing.T) {
	card := &Card{
		Name: "Bloodbraid Elf",
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "cascade"},
				&gameast.Keyword{Name: "haste"},
			},
		},
	}
	if !HasCascadeKeyword(card) {
		t.Fatal("expected HasCascadeKeyword to return true")
	}
}

func TestHasCascadeKeyword_False(t *testing.T) {
	card := &Card{
		Name: "Lightning Bolt",
		AST:  &gameast.CardAST{},
	}
	if HasCascadeKeyword(card) {
		t.Fatal("expected HasCascadeKeyword to return false for non-cascade card")
	}
}

func TestHasCascadeKeyword_NilCard(t *testing.T) {
	if HasCascadeKeyword(nil) {
		t.Fatal("expected false for nil card")
	}
}

func TestApplyCascade_FindsNonland(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	// Build a library with lands on top and a nonland underneath.
	gs.Seats[0].Library = []*Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}, CMC: 0},
		{Name: "Mountain", Owner: 0, Types: []string{"land"}, CMC: 0},
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1},
		{Name: "Plains", Owner: 0, Types: []string{"land"}, CMC: 0},
	}

	hit := ApplyCascade(gs, 0, 3, "Bloodbraid Elf")
	if !hit {
		t.Fatal("expected cascade to find Lightning Bolt")
	}

	// Lightning Bolt was cast, so it should be resolved.
	// The two lands + remaining land should be on the bottom of library.
	if len(gs.Seats[0].Library) != 3 {
		t.Fatalf("expected 3 cards in library (2 lands exiled + 1 original), got %d", len(gs.Seats[0].Library))
	}

	if countEvents(gs, "cascade_trigger") == 0 {
		t.Fatal("missing cascade_trigger event")
	}
	if countEvents(gs, "cascade_hit") == 0 {
		t.Fatal("missing cascade_hit event")
	}
}

func TestApplyCascade_Whiff(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	// Library with only lands — cascade should whiff.
	gs.Seats[0].Library = []*Card{
		{Name: "Forest", Owner: 0, Types: []string{"land"}, CMC: 0},
		{Name: "Mountain", Owner: 0, Types: []string{"land"}, CMC: 0},
	}

	hit := ApplyCascade(gs, 0, 3, "Bloodbraid Elf")
	if hit {
		t.Fatal("expected cascade to whiff with only lands in library")
	}
	if countEvents(gs, "cascade_whiff") == 0 {
		t.Fatal("missing cascade_whiff event")
	}
	// All lands should be back on the bottom of the library.
	if len(gs.Seats[0].Library) != 2 {
		t.Fatalf("expected 2 cards in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestApplyCascade_CMCFilter(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	// Library: 5-CMC nonland (too expensive), then 2-CMC nonland (good).
	gs.Seats[0].Library = []*Card{
		{Name: "Expensive Spell", Owner: 0, Types: []string{"sorcery"}, CMC: 5},
		{Name: "Cheap Spell", Owner: 0, Types: []string{"instant"}, CMC: 2},
	}

	hit := ApplyCascade(gs, 0, 4, "Cascade Spell")
	if !hit {
		t.Fatal("expected cascade to find Cheap Spell (CMC 2 < 4)")
	}
	if countEvents(gs, "cascade_hit") == 0 {
		t.Fatal("missing cascade_hit event")
	}
}
