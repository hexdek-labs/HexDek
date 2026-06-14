package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 HAT-RESPONSE-BEHAVIOR audit.
//
// Root cause behind "our hats aren't interacting with each other" (7174n1c):
// the tournament driver floats mana into ManaPool ONLY for the active player
// (runMainPhase → tapAllManaSources), and the pool is drained every untap
// (§500.4). A DEFENDER therefore has ManaPool == 0 during an opponent's turn,
// even with a hand full of counterspells and a battlefield full of untapped
// lands. PriorityRound's payment gate (`s.ManaPool < cost → skip`) then makes
// EVERY reactive response — counterspells included — silently impossible in
// real games. The entire ChooseResponse / GetResponse reactive layer is dead
// code in practice.
//
// Fix: PriorityRound now funds an affordable response by tapping the
// responder's own untapped mana sources at payment time (lands first, then
// mana artifacts), exactly as a player would tap to cast at instant speed.
// The affordability gates use AvailableManaEstimate (potential mana) so a
// counter that's payable-by-tapping is actually proposed and cast.

// makeEmptyCostCounter builds an instant whose body is a CounterSpell —
// Layout-1 (empty-cost Activated), the parser-artifact spell-body shape that
// counterSpellEffect recognizes for instants/sorceries.
func makeEmptyCostCounter(name string, cmc int, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		CMC:   cmc,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{},
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}},
				},
			},
		},
	}
}

func untappedLand(name string, owner int) *Permanent {
	return &Permanent{
		Card:       &Card{Name: name, Owner: owner, Types: []string{"land"}},
		Controller: owner,
		Owner:      owner,
		Tapped:     false,
		Flags:      map[string]int{},
	}
}

// TestPriorityRound_FundsDefenderCounterFromUntappedLands is the core
// regression: a defender with ManaPool==0 but two untapped lands MUST be able
// to counter an opponent's spell. Pre-fix this failed (counter never paid).
func TestPriorityRound_FundsDefenderCounterFromUntappedLands(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	// Defender (seat 1): counter in hand, two untapped lands, ZERO floating
	// mana — exactly the state on an opponent's turn after §500.4 drain.
	counter := makeEmptyCostCounter("Test Counterspell", 2, 1)
	gs.Seats[1].Hand = []*Card{counter}
	gs.Seats[1].ManaPool = 0
	gs.Seats[1].Battlefield = []*Permanent{
		untappedLand("Island", 1),
		untappedLand("Island", 1),
	}

	// Active player (seat 0) puts a spell on the stack.
	threat := &Card{Name: "Opposing Threat", Owner: 0, CMC: 3, Types: []string{"creature"}}
	PushStackItem(gs, &StackItem{Controller: 0, Card: threat})

	PriorityRound(gs)

	// The counter must now be on the stack (cast in response), the original
	// threat must still be under it, and the defender's lands must be tapped.
	if len(gs.Stack) != 2 {
		t.Fatalf("expected stack depth 2 (threat + counter), got %d", len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if top.Card == nil || top.Card.Name != "Test Counterspell" || top.Controller != 1 {
		t.Fatalf("expected defender's counter on top, got %+v", top)
	}
	// Counter left the defender's hand.
	for _, c := range gs.Seats[1].Hand {
		if c == counter {
			t.Fatal("counter should have left the defender's hand when cast")
		}
	}
	// Mana was actually tapped to pay for it.
	tapped := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p.Tapped {
			tapped++
		}
	}
	if tapped < 2 {
		t.Fatalf("expected >=2 lands tapped to fund the 2-cost counter, got %d", tapped)
	}
}

// TestPriorityRound_NoManaSourcesCannotRespond confirms the fix does NOT
// conjure mana: a defender with no floating mana AND no untapped sources
// cannot counter.
func TestPriorityRound_NoManaSourcesCannotRespond(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	counter := makeEmptyCostCounter("Test Counterspell", 2, 1)
	gs.Seats[1].Hand = []*Card{counter}
	gs.Seats[1].ManaPool = 0
	gs.Seats[1].Battlefield = nil // no lands, no rocks

	threat := &Card{Name: "Opposing Threat", Owner: 0, CMC: 3, Types: []string{"creature"}}
	PushStackItem(gs, &StackItem{Controller: 0, Card: threat})

	PriorityRound(gs)

	if len(gs.Stack) != 1 {
		t.Fatalf("expected stack depth 1 (no affordable response), got %d", len(gs.Stack))
	}
	if len(gs.Seats[1].Hand) != 1 {
		t.Fatal("counter should remain in hand when unaffordable")
	}
}

// TestPriorityRound_PreFloatedManaStillPays guards the legacy path: a defender
// who already has floating mana (e.g. test fixtures, or a seat that floated
// mana deliberately) pays without tapping any lands.
func TestPriorityRound_PreFloatedManaStillPays(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	counter := makeEmptyCostCounter("Test Counterspell", 2, 1)
	gs.Seats[1].Hand = []*Card{counter}
	gs.Seats[1].ManaPool = 2
	land := untappedLand("Island", 1)
	gs.Seats[1].Battlefield = []*Permanent{land}

	threat := &Card{Name: "Opposing Threat", Owner: 0, CMC: 3, Types: []string{"creature"}}
	PushStackItem(gs, &StackItem{Controller: 0, Card: threat})

	PriorityRound(gs)

	if len(gs.Stack) != 2 {
		t.Fatalf("expected stack depth 2, got %d", len(gs.Stack))
	}
	if land.Tapped {
		t.Fatal("pre-floated mana should pay without tapping lands")
	}
}
