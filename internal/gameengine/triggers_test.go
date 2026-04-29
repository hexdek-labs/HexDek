package gameengine

// Wave 3c tests — APNAP trigger ordering.
//
// Verifies:
//   - Active player's triggers are ordered first (pushed first = resolve last)
//   - Non-active players ordered in turn order after active
//   - Hat.OrderTriggers is called for intra-group ordering
//   - Single-controller set passes through unchanged
//   - Empty / nil inputs are handled gracefully

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// reverseTriggerHat is a test Hat that reverses trigger order to verify
// the engine actually calls OrderTriggers.
type reverseTriggerHat struct {
	GreedyHatStub
}

func (h *reverseTriggerHat) OrderTriggers(gs *GameState, seatIdx int, triggers []*StackItem) []*StackItem {
	if len(triggers) <= 1 {
		return triggers
	}
	out := make([]*StackItem, len(triggers))
	for i, t := range triggers {
		out[len(triggers)-1-i] = t
	}
	return out
}

// GreedyHatStub is a minimal Hat that satisfies the interface with no-op
// methods. Used as a base for test hats.
type GreedyHatStub struct{}

func (*GreedyHatStub) ChooseMulligan(gs *GameState, seatIdx int, hand []*Card) bool { return false }
func (*GreedyHatStub) ChooseLandToPlay(gs *GameState, seatIdx int, lands []*Card) *Card {
	return nil
}
func (*GreedyHatStub) ChooseCastFromHand(gs *GameState, seatIdx int, castable []*Card) *Card {
	return nil
}
func (*GreedyHatStub) ChooseActivation(gs *GameState, seatIdx int, options []Activation) *Activation {
	return nil
}
func (*GreedyHatStub) ChooseAttackers(gs *GameState, seatIdx int, legal []*Permanent) []*Permanent {
	return nil
}
func (*GreedyHatStub) ChooseAttackTarget(gs *GameState, seatIdx int, attacker *Permanent, legalDefenders []int) int {
	return 0
}
func (*GreedyHatStub) AssignBlockers(gs *GameState, seatIdx int, attackers []*Permanent) map[*Permanent][]*Permanent {
	return nil
}
func (*GreedyHatStub) ChooseResponse(gs *GameState, seatIdx int, stackTop *StackItem) *StackItem {
	return nil
}
func (*GreedyHatStub) ChooseTarget(gs *GameState, seatIdx int, filter gameast.Filter, legal []Target) Target {
	if len(legal) > 0 {
		return legal[0]
	}
	return Target{}
}
func (*GreedyHatStub) ChooseMode(gs *GameState, seatIdx int, modes []gameast.Effect) int { return 0 }
func (*GreedyHatStub) ShouldCastCommander(gs *GameState, seatIdx int, commanderName string, tax int) bool {
	return false
}
func (*GreedyHatStub) ShouldRedirectCommanderZone(gs *GameState, seatIdx int, commander *Card, to string) bool {
	return false
}
func (*GreedyHatStub) OrderReplacements(gs *GameState, seatIdx int, candidates []*ReplacementEffect) []*ReplacementEffect {
	return candidates
}
func (*GreedyHatStub) ChooseDiscard(gs *GameState, seatIdx int, hand []*Card, n int) []*Card {
	return nil
}
func (*GreedyHatStub) OrderTriggers(gs *GameState, seatIdx int, triggers []*StackItem) []*StackItem {
	return triggers
}
func (*GreedyHatStub) ChooseX(gs *GameState, seatIdx int, card *Card, availableMana int) int {
	return availableMana
}
func (*GreedyHatStub) ChooseBottomCards(gs *GameState, seatIdx int, hand []*Card, count int) []*Card {
	if count > len(hand) {
		count = len(hand)
	}
	return hand[:count]
}
func (*GreedyHatStub) ChooseScry(gs *GameState, seatIdx int, cards []*Card) (top []*Card, bottom []*Card) {
	return cards, nil
}
func (*GreedyHatStub) ChooseSurveil(gs *GameState, seatIdx int, cards []*Card) (graveyard []*Card, top []*Card) {
	return nil, cards
}
func (*GreedyHatStub) ChoosePutBack(gs *GameState, seatIdx int, hand []*Card, count int) []*Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	if count > len(hand) {
		count = len(hand)
	}
	return hand[:count]
}
func (*GreedyHatStub) ObserveEvent(gs *GameState, seatIdx int, event *Event) {}
func (*GreedyHatStub) ShouldConcede(gs *GameState, seatIdx int) bool        { return false }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOrderTriggersAPNAP_ActiveFirst(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 1 // seat 1 is active

	// Create triggers from various seats.
	tA := &StackItem{Controller: 1, Card: &Card{Name: "Trigger-AP"}, Kind: "triggered"}
	tB := &StackItem{Controller: 2, Card: &Card{Name: "Trigger-NAP2"}, Kind: "triggered"}
	tC := &StackItem{Controller: 3, Card: &Card{Name: "Trigger-NAP3"}, Kind: "triggered"}
	tD := &StackItem{Controller: 0, Card: &Card{Name: "Trigger-NAP0"}, Kind: "triggered"}

	// Input order is arbitrary.
	triggers := []*StackItem{tC, tD, tA, tB}

	result := OrderTriggersAPNAP(gs, triggers)

	if len(result) != 4 {
		t.Fatalf("expected 4 triggers, got %d", len(result))
	}

	// APNAP order starting from active=1: 1, 2, 3, 0
	// Active player (seat 1) triggers pushed first (resolves last).
	if result[0].Controller != 1 {
		t.Fatalf("first push should be active player (seat 1), got seat %d", result[0].Controller)
	}
	if result[1].Controller != 2 {
		t.Fatalf("second push should be seat 2, got seat %d", result[1].Controller)
	}
	if result[2].Controller != 3 {
		t.Fatalf("third push should be seat 3, got seat %d", result[2].Controller)
	}
	if result[3].Controller != 0 {
		t.Fatalf("fourth push should be seat 0, got seat %d", result[3].Controller)
	}
}

func TestOrderTriggersAPNAP_TwoPlayerGame(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0

	tA := &StackItem{Controller: 0, Card: &Card{Name: "AP-trigger"}, Kind: "triggered"}
	tB := &StackItem{Controller: 1, Card: &Card{Name: "NAP-trigger"}, Kind: "triggered"}

	result := OrderTriggersAPNAP(gs, []*StackItem{tB, tA})

	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].Controller != 0 {
		t.Fatal("active player should be first")
	}
	if result[1].Controller != 1 {
		t.Fatal("non-active player should be second")
	}
}

func TestOrderTriggersAPNAP_MultipleSameController(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0

	tA1 := &StackItem{Controller: 0, Card: &Card{Name: "AP-1"}, Kind: "triggered"}
	tA2 := &StackItem{Controller: 0, Card: &Card{Name: "AP-2"}, Kind: "triggered"}
	tB1 := &StackItem{Controller: 1, Card: &Card{Name: "NAP-1"}, Kind: "triggered"}

	result := OrderTriggersAPNAP(gs, []*StackItem{tB1, tA2, tA1})

	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	// AP triggers come first.
	if result[0].Controller != 0 || result[1].Controller != 0 {
		t.Fatal("both AP triggers should be grouped first")
	}
	if result[2].Controller != 1 {
		t.Fatal("NAP trigger should be last")
	}
}

func TestOrderTriggersAPNAP_HatOrderTriggersCalled(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0

	// Install a reverse hat on seat 0.
	gs.Seats[0].Hat = &reverseTriggerHat{}
	gs.Seats[1].Hat = &GreedyHatStub{}

	tA1 := &StackItem{Controller: 0, Card: &Card{Name: "AP-First"}, Kind: "triggered"}
	tA2 := &StackItem{Controller: 0, Card: &Card{Name: "AP-Second"}, Kind: "triggered"}
	tB := &StackItem{Controller: 1, Card: &Card{Name: "NAP"}, Kind: "triggered"}

	result := OrderTriggersAPNAP(gs, []*StackItem{tA1, tA2, tB})

	if len(result) != 3 {
		t.Fatalf("expected 3, got %d", len(result))
	}
	// The reverse hat should have reversed seat 0's triggers.
	if result[0].Card.Name != "AP-Second" {
		t.Fatalf("expected reversed order, first AP trigger should be AP-Second, got %s", result[0].Card.Name)
	}
	if result[1].Card.Name != "AP-First" {
		t.Fatalf("expected reversed order, second AP trigger should be AP-First, got %s", result[1].Card.Name)
	}
	if result[2].Card.Name != "NAP" {
		t.Fatalf("NAP trigger should be last, got %s", result[2].Card.Name)
	}
}

func TestOrderTriggersAPNAP_EmptyInput(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	result := OrderTriggersAPNAP(gs, nil)
	if result != nil {
		t.Fatal("nil input should return nil")
	}

	result = OrderTriggersAPNAP(gs, []*StackItem{})
	if len(result) != 0 {
		t.Fatal("empty input should return empty")
	}
}

func TestOrderTriggersAPNAP_SingleTrigger(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	tA := &StackItem{Controller: 0, Card: &Card{Name: "only"}, Kind: "triggered"}
	result := OrderTriggersAPNAP(gs, []*StackItem{tA})

	if len(result) != 1 || result[0] != tA {
		t.Fatal("single trigger should pass through unchanged")
	}
}

func TestPushSimultaneousTriggers_PushesInOrder(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	tA := &StackItem{Controller: 0, Card: &Card{Name: "AP-trig"}}
	tB := &StackItem{Controller: 1, Card: &Card{Name: "NAP-trig"}}

	PushSimultaneousTriggers(gs, []*StackItem{tB, tA})

	// Stack should have 2 items. AP pushed first (bottom), NAP pushed
	// second (top). Top resolves first per LIFO.
	if len(gs.Stack) != 2 {
		t.Fatalf("expected 2 stack items, got %d", len(gs.Stack))
	}
	// Stack bottom (index 0) is AP's trigger.
	if gs.Stack[0].Card.Name != "AP-trig" {
		t.Fatalf("bottom should be AP trigger, got %s", gs.Stack[0].Card.Name)
	}
	// Stack top (index 1) is NAP's trigger.
	if gs.Stack[1].Card.Name != "NAP-trig" {
		t.Fatalf("top should be NAP trigger, got %s", gs.Stack[1].Card.Name)
	}

	if !hasEventOfKind(gs, "triggers_ordered") {
		t.Fatal("expected triggers_ordered event")
	}
}
