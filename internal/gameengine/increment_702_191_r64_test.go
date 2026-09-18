package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func mkIncrementCreature(gs *GameState, seat int, name string, p, t int) *Permanent {
	perm := addBattlefield(gs, seat, name, p, t, "creature")
	perm.Card.AST = &gameast.CardAST{
		Name:      name,
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "increment"}},
	}
	return perm
}

// TestIncrement_702_191 pins the Increment keyword: on a cast, a creature the
// caster controls gets a +1/+1 counter iff the mana spent exceeds its power OR
// its toughness.
func TestIncrement_702_191(t *testing.T) {
	gs := newCombatGame(t)
	inc := mkIncrementCreature(gs, 0, "Increment Bear", 2, 2)

	// mana spent 3 > power/toughness 2 → +1/+1
	ApplyIncrementTriggers(gs, 0, 3)
	if inc.Counters["+1/+1"] != 1 {
		t.Fatalf("mana 3 vs 2/2 should add a +1/+1 counter, got %d", inc.Counters["+1/+1"])
	}

	// now it's effectively 3/3; mana spent 1 is not > 3 → no further counter
	ApplyIncrementTriggers(gs, 0, 1)
	if inc.Counters["+1/+1"] != 1 {
		t.Errorf("low mana-spent must not add another counter, got %d", inc.Counters["+1/+1"])
	}

	// mana spent 3 vs current 3/3: 3 > 3 is false → still no add
	ApplyIncrementTriggers(gs, 0, 3)
	if inc.Counters["+1/+1"] != 1 {
		t.Errorf("mana equal to both stats (not greater) must not add, got %d", inc.Counters["+1/+1"])
	}

	// mana spent 4 > 3 → adds again
	ApplyIncrementTriggers(gs, 0, 4)
	if inc.Counters["+1/+1"] != 2 {
		t.Errorf("mana 4 vs 3/3 should add a second counter, got %d", inc.Counters["+1/+1"])
	}

	// a non-Increment creature is never affected
	plain := addBattlefield(gs, 0, "Plain Bear", 2, 2, "creature")
	ApplyIncrementTriggers(gs, 0, 9)
	if plain.Counters["+1/+1"] != 0 {
		t.Error("a creature without the Increment keyword must not get a counter")
	}
}
