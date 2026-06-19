package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// demonstrate_r63_test.go — Demonstrate (CR §702.144, Strixhaven). The
// ApplyDemonstrate implementation existed but had NO caller — demonstrate was
// fully inert (casting made no copies). These pin both the copy semantics and
// the cast-path wiring.

func demoGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(3, rand.New(rand.NewSource(17)), nil)
	for _, s := range gs.Seats {
		s.Life = 20
	}
	gs.Active = 0
	return gs
}

func demoSpellItem(gs *GameState, seat int) *StackItem {
	card := &Card{Name: "Demo Spell", Owner: seat, Types: []string{"instant"}, AST: &gameast.CardAST{Name: "Demo Spell"}}
	item := &StackItem{Controller: seat, Card: card, Kind: "spell"}
	PushStackItem(gs, item)
	return item
}

// (1) Opt-in → two copies: one under the controller, one under the chosen opponent.
func TestDemonstrate_TwoCopies(t *testing.T) {
	gs := demoGame(t)
	item := demoSpellItem(gs, 0)
	stackBefore := len(gs.Stack)

	n := ApplyDemonstrate(gs, item, 0, func() bool { return true }, func() int { return 2 })

	if n != 2 {
		t.Fatalf("opt-in should create 2 copies, got %d", n)
	}
	if len(gs.Stack) != stackBefore+2 {
		t.Fatalf("two copies should be on the stack above the original, stack grew by %d", len(gs.Stack)-stackBefore)
	}
	top := gs.Stack[len(gs.Stack)-1]
	mid := gs.Stack[len(gs.Stack)-2]
	if !top.IsCopy || !mid.IsCopy {
		t.Fatal("demonstrate copies must be flagged IsCopy (CR §707.10)")
	}
	controllers := map[int]bool{top.Controller: true, mid.Controller: true}
	if !controllers[0] || !controllers[2] {
		t.Fatalf("expected copies for controller(0) and chosen opponent(2), got controllers %v", controllers)
	}
}

// (2) Decline → no copies.
func TestDemonstrate_DeclineNoCopies(t *testing.T) {
	gs := demoGame(t)
	item := demoSpellItem(gs, 0)
	if n := ApplyDemonstrate(gs, item, 0, func() bool { return false }, nil); n != 0 {
		t.Fatalf("declining should create 0 copies, got %d", n)
	}
}

// (3) No living opponent → only the controller copy (CR §702.144a).
func TestDemonstrate_NoOpponentControllerOnly(t *testing.T) {
	gs := demoGame(t)
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	item := demoSpellItem(gs, 0)
	if n := ApplyDemonstrate(gs, item, 0, func() bool { return true }, nil); n != 1 {
		t.Fatalf("with no living opponent only the controller copy is made, got %d", n)
	}
}

// (4) The chosen opponent (not the controller) gets a copy under THEIR control.
func TestDemonstrate_OpponentChoiceHonored(t *testing.T) {
	gs := demoGame(t)
	item := demoSpellItem(gs, 0)
	ApplyDemonstrate(gs, item, 0, func() bool { return true }, func() int { return 2 })
	found := false
	for _, si := range gs.Stack {
		if si.IsCopy && si.Controller == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("the chosen opponent (seat 2) should control a demonstrate copy")
	}
}

// (5) Integration: casting a demonstrate card fires the cast-path hook and mints
// both copies — the behavior that was inert (ApplyDemonstrate had no caller).
func TestDemonstrate_WiredIntoCast(t *testing.T) {
	gs := demoGame(t)
	gs.Seats[0].ManaPool = 10
	card := &Card{
		Name:  "Demonstrate Bolt",
		Owner: 0,
		Types: []string{"instant", "cost:2"},
		AST: &gameast.CardAST{
			Name:      "Demonstrate Bolt",
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "demonstrate"}},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if c := countEvents(gs, "demonstrate_copy"); c != 2 {
		t.Fatalf("casting a demonstrate spell should mint 2 copies, got %d demonstrate_copy events", c)
	}
	if countEvents(gs, "demonstrate_opt_in") != 1 {
		t.Fatal("expected a demonstrate_opt_in event proving the cast-path hook fired")
	}
}
