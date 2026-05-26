package hat

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestMCTSHat_InterfaceSatisfaction is a COMPILE-TIME assertion that
// MCTSHat satisfies the gameengine.Hat interface. The check is the
// `var _ Iface = (*T)(nil)` pattern at line 1 of the function — if
// MCTSHat ever loses a method, the file fails to compile, and
// `go test` reports the error before any test runs. The function
// body intentionally has no runtime assertion; Phase 2B's no-
// assertion sweep correctly flags this — flagged-as-intentional via
// this comment.
func TestMCTSHat_InterfaceSatisfaction(t *testing.T) {
	var _ gameengine.Hat = (*MCTSHat)(nil)
}

func TestMCTSHat_Budget0_DelegatesToInner(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 0)

	// With budget 0, MCTSHat should behave identically to inner hat.
	castable := []*gameengine.Card{
		newTestCardMinimal("Big Spell", []string{"sorcery"}, 5, nil),
		newTestCardMinimal("Small Spell", []string{"instant"}, 1, nil),
	}
	gs.Seats[0].Hand = castable
	gs.Seats[0].ManaPool = 5

	innerChoice := inner.ChooseCastFromHand(gs, 0, castable)
	mctsChoice := mcts.ChooseCastFromHand(gs, 0, castable)

	if innerChoice == nil || mctsChoice == nil {
		t.Fatal("both should pick a card")
	}
	if innerChoice.DisplayName() != mctsChoice.DisplayName() {
		t.Errorf("budget=0 should match inner: inner=%s mcts=%s",
			innerChoice.DisplayName(), mctsChoice.DisplayName())
	}
}

func TestMCTSHat_PrefersCombopiece(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Oracle", "Consultation"}, Type: "infinite"},
		},
	}

	inner := NewPokerHat()
	mcts := NewMCTSHat(inner, sp, 50)

	// Hand has a combo piece + a generic card. Both castable.
	oracle := newTestCardMinimal("Oracle", []string{"creature"}, 2, nil)
	generic := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)

	// Put Consultation on battlefield so combo is completable.
	consultCard := newTestCardMinimal("Consultation", []string{"instant"}, 1, nil)
	newTestPermanent(gs.Seats[0], consultCard, 0, 0)

	castable := []*gameengine.Card{oracle, generic}
	gs.Seats[0].Hand = castable
	gs.Seats[0].ManaPool = 2

	choice := mcts.ChooseCastFromHand(gs, 0, castable)
	if choice == nil {
		t.Fatal("should pick a card")
	}
	if choice.DisplayName() != "Oracle" {
		t.Errorf("should prefer combo piece Oracle, got %s", choice.DisplayName())
	}
}

func TestMCTSHat_AttackersAdjustedByPosition(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50)

	// Give seat 0 creatures to attack with.
	c1 := newTestCardMinimal("Flyer", []string{"creature"}, 3, nil)
	p1 := newTestPermanent(gs.Seats[0], c1, 3, 3)
	addKeyword(p1, "flying")

	c2 := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
	p2 := newTestPermanent(gs.Seats[0], c2, 2, 2)

	legal := []*gameengine.Permanent{p1, p2}
	attackers := mcts.ChooseAttackers(gs, 0, legal)

	// Should return at least one attacker.
	if len(attackers) == 0 {
		t.Error("MCTSHat should select at least one attacker")
	}

	// Flying creature should be selected (evasion bonus).
	hasFlyer := false
	for _, a := range attackers {
		if a.Card.DisplayName() == "Flyer" {
			hasFlyer = true
		}
	}
	if !hasFlyer {
		t.Error("flying creature should be preferred attacker")
	}
}

func TestMCTSHat_ObserveEvent_ResetsStats(t *testing.T) {
	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50)
	gs := newTestGame(t, 2)

	// Record some stats.
	mcts.recordAction("cast:Test", 0.5)
	mcts.recordAction("cast:Test", 0.7)
	if mcts.totalVisits != 2 {
		t.Fatalf("expected 2 visits, got %d", mcts.totalVisits)
	}

	// game_start should reset.
	mcts.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	if mcts.totalVisits != 0 {
		t.Errorf("game_start should reset visits, got %d", mcts.totalVisits)
	}
	if len(mcts.actionStats) != 0 {
		t.Errorf("game_start should reset action stats, got %d entries", len(mcts.actionStats))
	}
}

func TestMCTSHat_UCB1_ExplorationBonus(t *testing.T) {
	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50)

	// Unvisited action should get exploration bonus.
	unvisited := mcts.ucb1Score("new_action", 0.5)

	// Visited action.
	mcts.recordAction("old_action", 0.5)
	mcts.recordAction("old_action", 0.5)
	visited := mcts.ucb1Score("old_action", 0.5)

	// Unvisited should score higher due to exploration bonus.
	if unvisited <= visited {
		t.Errorf("unvisited (%.3f) should score higher than visited (%.3f) due to exploration",
			unvisited, visited)
	}
}

func TestMCTSHat_SingleCandidate_DelegatesToInner(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50)

	// Only one castable card — should delegate directly.
	card := newTestCardMinimal("Bolt", []string{"instant"}, 1, nil)
	castable := []*gameengine.Card{card}
	gs.Seats[0].Hand = castable
	gs.Seats[0].ManaPool = 1

	choice := mcts.ChooseCastFromHand(gs, 0, castable)
	if choice == nil || choice.DisplayName() != "Bolt" {
		t.Errorf("single candidate should delegate to inner, got %v", choice)
	}
}

func TestMCTSHat_NilInputsSafe(t *testing.T) {
	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50)
	gs := newTestGame(t, 2)

	// All nil/empty inputs should not panic.
	_ = mcts.ChooseMulligan(gs, 0, nil)
	_ = mcts.ChooseLandToPlay(gs, 0, nil)
	_ = mcts.ChooseCastFromHand(gs, 0, nil)
	_ = mcts.ChooseActivation(gs, 0, nil)
	_ = mcts.ChooseAttackers(gs, 0, nil)
	_ = mcts.ChooseResponse(gs, 0, nil)
	_ = mcts.ChooseDiscard(gs, 0, nil, 0)
	_ = mcts.ChooseMode(gs, 0, nil)
	_ = mcts.ShouldCastCommander(gs, 0, "", 0)
}

func TestMCTSHat_RolloutFallbackWithoutTurnRunner(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 200)
	// TurnRunner is nil — should fall back to evaluator-only mode.

	card := newTestCardMinimal("Bolt", []string{"instant"}, 1, nil)
	card2 := newTestCardMinimal("Bears", []string{"creature"}, 2, nil)
	castable := []*gameengine.Card{card, card2}
	gs.Seats[0].Hand = castable
	gs.Seats[0].ManaPool = 2

	choice := mcts.ChooseCastFromHand(gs, 0, castable)
	if choice == nil {
		t.Error("should pick a card even without TurnRunner")
	}
}

func TestMCTSHat_RolloutWithMockTurnRunner(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 200)
	turnRunnerCalled := 0
	mcts.TurnRunner = func(gs *gameengine.GameState) {
		turnRunnerCalled++
	}

	card := newTestCardMinimal("Bolt", []string{"instant"}, 1, nil)
	card2 := newTestCardMinimal("Bears", []string{"creature"}, 2, nil)
	castable := []*gameengine.Card{card, card2}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 2

	// chooseCastViaRollout may return nil (pass) if rollouts are
	// indistinguishable — that's valid. The key assertion is that
	// the TurnRunner was actually invoked for simulation.
	_ = mcts.ChooseCastFromHand(gs, 0, castable)
	if turnRunnerCalled == 0 {
		t.Error("TurnRunner should be called during rollout simulation")
	}
	// 3 candidates (Bolt, Bears, pass) × up to rolloutDepth turns each.
	if turnRunnerCalled < 3 {
		t.Errorf("expected at least 3 TurnRunner calls (3 candidates), got %d", turnRunnerCalled)
	}
}

func TestDeterminize_ShufflesOpponentHand(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	// Give opponent (seat 1) a known hand + library.
	hand := []*gameengine.Card{
		newTestCardMinimal("Spell A", []string{"instant"}, 1, nil),
		newTestCardMinimal("Spell B", []string{"sorcery"}, 2, nil),
	}
	lib := []*gameengine.Card{
		newTestCardMinimal("Lib C", []string{"creature"}, 3, nil),
		newTestCardMinimal("Lib D", []string{"creature"}, 4, nil),
		newTestCardMinimal("Lib E", []string{"land"}, 0, nil),
	}
	gs.Seats[1].Hand = hand
	gs.Seats[1].Library = lib

	rng := rand.New(rand.NewSource(42))
	clone := gs.CloneForRollout(rng)
	determinize(clone, 0, rng)

	// Hand size must be preserved.
	if len(clone.Seats[1].Hand) != 2 {
		t.Fatalf("hand size should be 2, got %d", len(clone.Seats[1].Hand))
	}
	// Library size must be preserved.
	if len(clone.Seats[1].Library) != 3 {
		t.Fatalf("library size should be 3, got %d", len(clone.Seats[1].Library))
	}
	// Total pool should contain all original cards.
	allNames := make(map[string]bool)
	for _, c := range clone.Seats[1].Hand {
		allNames[c.DisplayName()] = true
	}
	for _, c := range clone.Seats[1].Library {
		allNames[c.DisplayName()] = true
	}
	if len(allNames) != 5 {
		t.Errorf("expected 5 unique cards in pool, got %d", len(allNames))
	}
}

func TestDeterminize_SkipsPerspectiveSeat(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	myHand := []*gameengine.Card{
		newTestCardMinimal("My Card", []string{"instant"}, 1, nil),
	}
	gs.Seats[0].Hand = myHand

	rng := rand.New(rand.NewSource(42))
	clone := gs.CloneForRollout(rng)
	determinize(clone, 0, rng)

	// Seat 0 is perspective seat — hand should NOT be shuffled.
	if len(clone.Seats[0].Hand) != 1 || clone.Seats[0].Hand[0].DisplayName() != "My Card" {
		t.Error("perspective seat's hand should remain unchanged")
	}
}

func TestMultiRolloutForCard_ReturnsScore(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	spell := newTestCardMinimal("Test Spell", []string{"sorcery"}, 2, nil)
	gs.Seats[0].Hand = []*gameengine.Card{spell}
	gs.Seats[0].ManaPool = 5

	sp := &StrategyProfile{Archetype: ArchetypeMidrange}
	h := NewYggdrasilHat(sp, 200)
	turnRunnerCalled := 0
	h.TurnRunner = func(gs *gameengine.GameState) { turnRunnerCalled++ }
	h.Evaluator = NewEvaluator(sp)

	score := h.multiRolloutForCard(gs, 0, spell, 3)

	// Score should be finite.
	if score != score { // NaN check
		t.Fatal("score should not be NaN")
	}
	// TurnRunner should have been called (3 rollouts × rolloutDepth turns).
	if turnRunnerCalled < 3 {
		t.Errorf("expected at least 3 TurnRunner calls, got %d", turnRunnerCalled)
	}
}

func TestMCTSHat_PokerInnerForwardsModeTransitions(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 3
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	poker := NewPokerHat()
	mcts := NewMCTSHat(poker, nil, 50)
	gs.Seats[0].Hat = mcts

	// Damage events should flow through to PokerHat's ObserveEvent.
	for i := 0; i < 5; i++ {
		mcts.ObserveEvent(gs, 0, &gameengine.Event{Kind: "damage", Seat: 0, Amount: 5})
	}

	// PokerHat should have transitioned to RAISE on low life.
	if poker.Mode != ModeRaise {
		t.Errorf("inner PokerHat should be in RAISE on 3 life, got %v", poker.Mode)
	}
}
