package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// self_harm_signals_r60_test.go — pins the R60 round 16+ extension to
// the self-trigger response framework. Three new self-harm scenarios:
//   1. Self-mill lethal (deck-out)
//   2. Stated-life-loss lethal
// And the OR-chain dispatcher in shouldCounterOwnTrigger that wires
// both into the existing draw-damage path.

// mkMillTriggerSource builds a permanent whose oracle text describes
// a mill effect (used as a triggered StackItem source).
func mkMillTriggerSource(name, oracle string) *gameengine.Permanent {
	c := &gameengine.Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Triggered{Raw: oracle},
			},
		},
	}
	return &gameengine.Permanent{Card: c, Flags: map[string]int{}}
}

// -----------------------------------------------------------------------
// extractNumberAfter
// -----------------------------------------------------------------------

func TestExtractNumberAfter_NeedleMissing(t *testing.T) {
	if got := extractNumberAfter("nothing here", "mill "); got != 0 {
		t.Errorf("missing needle should return 0; got %d", got)
	}
}

func TestExtractNumberAfter_DigitNumber(t *testing.T) {
	if got := extractNumberAfter("target player mills 5 cards", "mills "); got != 5 {
		t.Errorf("digit should be parsed; got %d", got)
	}
}

func TestExtractNumberAfter_MultiDigit(t *testing.T) {
	if got := extractNumberAfter("target player mills 12 cards", "mills "); got != 12 {
		t.Errorf("multi-digit should be parsed; got %d", got)
	}
}

func TestExtractNumberAfter_WordNumber(t *testing.T) {
	if got := extractNumberAfter("draw three cards", "draw "); got != 3 {
		t.Errorf("English word should map; got %d", got)
	}
}

func TestExtractNumberAfter_WordBoundaryMatters(t *testing.T) {
	// "threefold" should NOT match "three" (word-boundary check).
	if got := extractNumberAfter("threefold sun", ""); got != 0 {
		t.Errorf("word-boundary check should reject 'threefold'; got %d", got)
	}
}

// -----------------------------------------------------------------------
// estimateMillTriggerCardCount
// -----------------------------------------------------------------------

func TestEstimateMillTriggerCardCount_NilZero(t *testing.T) {
	if got := estimateMillTriggerCardCount(nil); got != 0 {
		t.Errorf("nil top should return 0; got %d", got)
	}
}

func TestEstimateMillTriggerCardCount_NoSourceZero(t *testing.T) {
	if got := estimateMillTriggerCardCount(&gameengine.StackItem{Kind: "triggered"}); got != 0 {
		t.Errorf("no source should return 0; got %d", got)
	}
}

func TestEstimateMillTriggerCardCount_MillN(t *testing.T) {
	src := mkMillTriggerSource("Hedron Crab",
		"landfall — whenever a land enters under your control, target player mills 3 cards")
	if got := estimateMillTriggerCardCount(&gameengine.StackItem{Source: src}); got != 3 {
		t.Errorf("'mills 3 cards' should return 3; got %d", got)
	}
}

func TestEstimateMillTriggerCardCount_MillACard(t *testing.T) {
	src := mkMillTriggerSource("Mesmeric Orb",
		"whenever a permanent becomes untapped, that permanent's controller mills a card")
	// "mill a card" idiom — engine renders as 1.
	if got := estimateMillTriggerCardCount(&gameengine.StackItem{Source: src}); got != 1 {
		t.Errorf("'mill a card' should return 1; got %d", got)
	}
}

func TestEstimateMillTriggerCardCount_PutsTopN(t *testing.T) {
	src := mkMillTriggerSource("Old-Style Mill",
		"whenever this creature attacks, puts the top 4 cards of target player's library into their graveyard")
	if got := estimateMillTriggerCardCount(&gameengine.StackItem{Source: src}); got != 4 {
		t.Errorf("legacy 'puts the top N cards' should return 4; got %d", got)
	}
}

// -----------------------------------------------------------------------
// triggerCausesLethalMill
// -----------------------------------------------------------------------

func TestTriggerCausesLethalMill_OppControlledNoMatch(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Library = make([]*gameengine.Card, 5)
	src := mkMillTriggerSource("Hedron Crab",
		"whenever a land enters, target player mills 10 cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 1, Source: src}
	if triggerCausesLethalMill(gs, 0, top) {
		t.Error("opp-controlled trigger should not match self-mill gate")
	}
}

func TestTriggerCausesLethalMill_LibraryAbundant(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Library = make([]*gameengine.Card, 50)
	src := mkMillTriggerSource("Hedron Crab",
		"target player mills 3 cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if triggerCausesLethalMill(gs, 0, top) {
		t.Error("3 mill vs 50 library should NOT be lethal")
	}
}

func TestTriggerCausesLethalMill_DeckOutLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Library = make([]*gameengine.Card, 3) // exactly 3
	src := mkMillTriggerSource("Hedron Crab",
		"target player mills 3 cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if !triggerCausesLethalMill(gs, 0, top) {
		t.Error("3 mill vs 3-card library should be lethal (next draw fails §704.5b)")
	}
}

func TestTriggerCausesLethalMill_OverMill(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Library = make([]*gameengine.Card, 2)
	src := mkMillTriggerSource("Hedron Crab",
		"target player mills 5 cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if !triggerCausesLethalMill(gs, 0, top) {
		t.Error("over-mill (5 vs 2) should be lethal")
	}
}

// -----------------------------------------------------------------------
// estimateLifeLossTriggerAmount
// -----------------------------------------------------------------------

func TestEstimateLifeLossTriggerAmount_LoseNLife(t *testing.T) {
	src := mkMillTriggerSource("Phyrexian Arena",
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	if got := estimateLifeLossTriggerAmount(&gameengine.StackItem{Source: src}); got != 1 {
		t.Errorf("'you lose 1 life' should return 1; got %d", got)
	}
}

func TestEstimateLifeLossTriggerAmount_LoseTwoLife(t *testing.T) {
	src := mkMillTriggerSource("Bottomless Pit",
		"at the beginning of your upkeep, you lose 2 life")
	if got := estimateLifeLossTriggerAmount(&gameengine.StackItem{Source: src}); got != 2 {
		t.Errorf("'you lose 2 life' should return 2; got %d", got)
	}
}

func TestEstimateLifeLossTriggerAmount_NotLifeText(t *testing.T) {
	// "you lose 1 cards" shouldn't match (wrong noun after the number).
	src := mkMillTriggerSource("False Match",
		"you lose 1 cards from your hand")
	if got := estimateLifeLossTriggerAmount(&gameengine.StackItem{Source: src}); got != 0 {
		t.Errorf("'you lose N cards' should not match life-loss; got %d", got)
	}
}

func TestEstimateLifeLossTriggerAmount_DealsDamageToYou(t *testing.T) {
	src := mkMillTriggerSource("Self Damage",
		"at the beginning of your upkeep, this creature deals 3 damage to you")
	if got := estimateLifeLossTriggerAmount(&gameengine.StackItem{Source: src}); got != 3 {
		t.Errorf("'deals 3 damage to you' should return 3; got %d", got)
	}
}

func TestEstimateLifeLossTriggerAmount_VariableAmountReturnsZero(t *testing.T) {
	// Dark Confidant's "lose life equal to its CMC" is variable —
	// we intentionally don't match this.
	src := mkMillTriggerSource("Dark Confidant",
		"at the beginning of your upkeep, reveal the top card. you lose life equal to its converted mana cost")
	if got := estimateLifeLossTriggerAmount(&gameengine.StackItem{Source: src}); got != 0 {
		t.Errorf("variable 'lose life equal to ...' should be 0 (no projection); got %d", got)
	}
}

// -----------------------------------------------------------------------
// triggerCausesLethalLifeLoss
// -----------------------------------------------------------------------

func TestTriggerCausesLethalLifeLoss_OppControlledNoMatch(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 1
	src := mkMillTriggerSource("Bottomless Pit", "you lose 2 life")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 1, Source: src}
	if triggerCausesLethalLifeLoss(gs, 0, top) {
		t.Error("opp-controlled trigger should not match self-life-loss gate")
	}
}

func TestTriggerCausesLethalLifeLoss_NotLethalLife(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	src := mkMillTriggerSource("Phyrexian Arena", "you lose 1 life")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if triggerCausesLethalLifeLoss(gs, 0, top) {
		t.Error("1 life loss at 40 life should NOT be lethal")
	}
}

func TestTriggerCausesLethalLifeLoss_LethalFires(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 1
	src := mkMillTriggerSource("Phyrexian Arena", "you lose 1 life")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if !triggerCausesLethalLifeLoss(gs, 0, top) {
		t.Error("1 life loss at 1 life should be lethal")
	}
}

func TestTriggerCausesLethalLifeLoss_DealsDamageToYouLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 3
	src := mkMillTriggerSource("Self Damage", "deals 5 damage to you")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	if !triggerCausesLethalLifeLoss(gs, 0, top) {
		t.Error("5 damage to you at 3 life should be lethal")
	}
}

// -----------------------------------------------------------------------
// Composition: shouldCounterOwnTrigger fires on each scenario independently
// -----------------------------------------------------------------------

// TestShouldCounterOwnTrigger_FiresOnSelfMillLethal pins that the OR-
// chain dispatcher hits the mill scenario without needing a punisher
// in play.
func TestShouldCounterOwnTrigger_FiresOnSelfMillLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40 // life isn't the issue
	gs.Seats[0].Library = make([]*gameengine.Card, 2)
	src := mkMillTriggerSource("Hedron Crab", "target player mills 3 cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("self-mill lethal should fire the dispatcher even with no draw punisher")
	}
}

// TestShouldCounterOwnTrigger_FiresOnSelfLifeLossLethal pins the life-
// loss scenario.
func TestShouldCounterOwnTrigger_FiresOnSelfLifeLossLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 1
	src := mkMillTriggerSource("Phyrexian Arena", "you lose 2 life")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("self-life-loss lethal should fire the dispatcher")
	}
}

// TestShouldCounterOwnTrigger_StillFiresOnDrawDamagePunisher confirms
// the existing PR #410 path is unaffected by the dispatcher extraction.
func TestShouldCounterOwnTrigger_StillFiresOnDrawDamagePunisher(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("PR #410 draw-damage path should still fire post-dispatcher refactor")
	}
}

// TestShouldCounterOwnTrigger_NotLethalDoesNotFireAnyScenario confirms
// all three scenarios stay quiet when no individual one is lethal.
func TestShouldCounterOwnTrigger_NotLethalDoesNotFireAnyScenario(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].Library = make([]*gameengine.Card, 30)
	// Punisher present but only 1 dmg per draw; trigger draws 2; total
	// 2 dmg vs 40 life — not lethal.
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("comfortable life + library + 2 damage projection should not fire any scenario")
	}
}
