package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// reactive_removal_r61_test.go — r61 PR-7 reactive instant-speed removal.
//
// These tests pin the conservative reactive-interaction win: a non-active
// seat with an affordable instant-speed removal spell and a genuine threat
// on an opponent's battlefield fires the removal during the opponent's
// turn (its only reactive priority window is ChooseResponse). When there's
// no good target, it passes (returns nil). The one-shot guard proves it
// never offers an endless chain against the same stack item.

// removalInstant builds an instant whose AST spell body is a Destroy
// targeting a creature — the shape isTargetedRemoval recognizes.
func removalInstant(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"instant"}, cmc,
		&gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.Destroy{
						Target: gameast.Filter{Base: "creature", Targeted: true},
					},
				},
			},
		})
}

// TestYggReactiveRemoval_FiresAtThreatOnOpponentTurn — the core win: AI
// has an affordable removal instant and an opponent has a big creature; an
// opponent's spell is on the stack, so the AI gets a response window. It
// returns the removal.
func TestYggReactiveRemoval_FiresAtThreatOnOpponentTurn(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5

	// Seat 0 (the defender) has an affordable removal instant.
	bolt := removalInstant("Murder", 2)
	gs.Seats[0].Hand = []*gameengine.Card{bolt}
	gameengine.AddMana(gs, gs.Seats[0], "any", 5, "test")

	// Seat 1 has a genuine threat on board (power 6 → bestOpponentRemovalScore 0.60).
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wurm", []string{"creature"}, 6, nil), 6, 6)

	// An opponent spell is resolving — this is what gives seat 0 a response window.
	top := &gameengine.StackItem{
		ID:         42,
		Controller: 1,
		Card:       newTestCardMinimal("EnemySpell", []string{"sorcery"}, 3, nil),
		Effect:     &gameast.Damage{},
	}

	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("AI should fire reactive removal at a big threat during opponent's turn")
	}
	if got.Card == nil || got.Card.DisplayName() != "Murder" {
		t.Fatalf("expected reactive removal Murder; got %v", got.Card)
	}
	if got.Controller != 0 {
		t.Fatalf("reactive removal should be controlled by the defender (seat 0); got %d", got.Controller)
	}
}

// TestYggReactiveRemoval_PassesWhenNoGoodTarget — no threatening permanent
// exists (only a small vanilla creature), so the AI passes (returns nil)
// and does NOT cast removal at chaff.
func TestYggReactiveRemoval_PassesWhenNoGoodTarget(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5

	gs.Seats[0].Hand = []*gameengine.Card{removalInstant("Murder", 2)}
	gameengine.AddMana(gs, gs.Seats[0], "any", 5, "test")

	// Seat 1 has only a small vanilla 1/1 — below the threat threshold.
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Bird", []string{"creature"}, 1, nil), 1, 1)

	top := &gameengine.StackItem{
		ID:         7,
		Controller: 1,
		Card:       newTestCardMinimal("EnemySpell", []string{"sorcery"}, 3, nil),
		Effect:     &gameast.Damage{},
	}

	got := h.ChooseResponse(gs, 0, top)
	if got != nil {
		t.Fatalf("AI should PASS (no good removal target); got %v", got.Card)
	}
}

// TestYggReactiveRemoval_OneShotGuard_NoLoop — after the AI fires reactive
// removal against a stack item, a SECOND ChooseResponse call against the
// SAME stack item (same ID) returns nil even though the threat and a
// second removal are still in hand. This is the anti-loop guarantee: the
// AI can't chain reactive responses against the same incoming item.
func TestYggReactiveRemoval_OneShotGuard_NoLoop(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5

	// Two removal instants in hand and plenty of mana — without the guard
	// the AI could fire a second one against the same item.
	gs.Seats[0].Hand = []*gameengine.Card{
		removalInstant("Murder", 2),
		removalInstant("Doom Blade", 2),
	}
	gameengine.AddMana(gs, gs.Seats[0], "any", 10, "test")

	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wurm", []string{"creature"}, 6, nil), 6, 6)

	top := &gameengine.StackItem{
		ID:         99,
		Controller: 1,
		Card:       newTestCardMinimal("EnemySpell", []string{"sorcery"}, 3, nil),
		Effect:     &gameast.Damage{},
	}

	first := h.ChooseResponse(gs, 0, top)
	if first == nil {
		t.Fatal("first reactive removal should fire")
	}
	// Simulate the engine removing the cast card from hand (PriorityRound
	// does this centrally). The second removal + threat remain.
	for i, c := range gs.Seats[0].Hand {
		if c == first.Card {
			gs.Seats[0].Hand = append(gs.Seats[0].Hand[:i], gs.Seats[0].Hand[i+1:]...)
			break
		}
	}

	second := h.ChooseResponse(gs, 0, top)
	if second != nil {
		t.Fatalf("one-shot guard violated: AI offered a SECOND reactive play against the same stack item (%v) — loop risk", second.Card)
	}
}

// TestYggReactiveRemoval_CounterTakesPriority — when both a counterspell
// and a removal are in hand, the existing counter logic wins (removal only
// fires when no counter is available). Confirms the counterspell path is
// intact.
func TestYggReactiveRemoval_CounterTakesPriority(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5

	counter := newTestCardMinimal("Counterspell", []string{"instant"}, 2,
		&gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell", Targeted: true}},
				},
			},
		})
	gs.Seats[0].Hand = []*gameengine.Card{counter, removalInstant("Murder", 2)}
	gameengine.AddMana(gs, gs.Seats[0], "any", 10, "test")

	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wurm", []string{"creature"}, 6, nil), 6, 6)

	// Use a board wipe so the counter logic's mustCounter ("destroy all")
	// branch fires. OracleTextLower reads the AST's Static.Raw text.
	enemy := newTestCardMinimal("Wrath", []string{"sorcery"}, 4,
		&gameast.CardAST{
			Name: "Wrath",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "destroy all creatures"},
			},
		})
	top := &gameengine.StackItem{
		ID:         11,
		Controller: 1,
		Card:       enemy,
		Effect:     &gameast.Damage{},
	}

	got := h.ChooseResponse(gs, 0, top)
	if got == nil {
		t.Fatal("AI should respond to a board wipe")
	}
	if got.Card == nil || got.Card.DisplayName() != "Counterspell" {
		t.Fatalf("counter should take priority over removal; got %v", got.Card)
	}
}

// TestYggReactiveRemoval_EndToEnd_DestroysThreatAndTerminates drives the
// FULL engine path: an opponent's spell is on the stack, the AI gets a
// response window via PriorityRound, fires reactive removal, and the
// drain loop resolves it — destroying the opponent's creature. Critically,
// the loop TERMINATES (no infinite priority chain). This is the anti-loop
// + correctness proof.
func TestYggReactiveRemoval_EndToEnd_DestroysThreatAndTerminates(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Active = 1 // opponent's turn

	// Defender (seat 0) gets the YggdrasilHat + a removal instant + mana.
	gs.Seats[0].Hat = h
	gs.Seats[0].Hand = []*gameengine.Card{removalInstant("Murder", 2)}
	gameengine.AddMana(gs, gs.Seats[0], "any", 5, "test")

	// Opponent (seat 1) has the threat.
	threat := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Wurm", []string{"creature"}, 6, nil), 6, 6)

	// Opponent's spell on the stack (gives seat 0 a response window).
	gameengine.PushStackItem(gs, &gameengine.StackItem{
		Controller: 1,
		Card:       newTestCardMinimal("EnemySorcery", []string{"sorcery"}, 3, nil),
	})

	// Drive the priority + resolution loop with a hard cap. If the AI
	// looped on its own responses, this would spin past the cap; the
	// assertion that the loop ended well under the cap is the anti-loop
	// proof.
	const maxIters = 50
	iters := 0
	for len(gs.Stack) > 0 && iters < maxIters {
		iters++
		gameengine.PriorityRound(gs)
		if len(gs.Stack) == 0 {
			break
		}
		gameengine.ResolveStackTop(gs)
		gameengine.StateBasedActions(gs)
	}
	if iters >= maxIters {
		t.Fatalf("priority/resolution loop did NOT terminate within %d iters — loop risk", maxIters)
	}

	// The threat should be gone (destroyed by the reactive removal).
	for _, p := range gs.Seats[1].Battlefield {
		if p == threat {
			t.Fatal("reactive removal did not destroy the opponent's threat")
		}
	}
	// The removal should have left the AI's hand.
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.DisplayName() == "Murder" {
			t.Fatal("removal still in hand — it was not actually cast")
		}
	}
}
