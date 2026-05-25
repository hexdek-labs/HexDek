package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// self_trigger_response_r60_test.go — pins the R60 round 15+ self-
// trigger response edge case. Three layers:
//   1. controllerHasDamageOnDrawPunisher detects punishers in play
//   2. estimateDrawTriggerCardCount reads the trigger source's text
//   3. shouldCounterOwnTrigger composes the gate
// Plus one ChooseResponse integration test confirming the counter
// path fires end-to-end.

// mkPunisher drops a permanent with a punisher oracle text on seat.
func mkPunisher(gs *gameengine.GameState, controller int, name, oracle string) {
	c := &gameengine.Card{
		Name:  name,
		Types: []string{"enchantment"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Triggered{Raw: oracle},
			},
		},
	}
	gs.Seats[controller].Battlefield = append(gs.Seats[controller].Battlefield,
		&gameengine.Permanent{Card: c, Controller: controller, Owner: controller, Flags: map[string]int{}})
}

// mkDrawTriggerSource builds a permanent whose oracle text describes a
// draw trigger (used as the source of a triggered StackItem).
func mkDrawTriggerSource(name, oracle string) *gameengine.Permanent {
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
// controllerHasDamageOnDrawPunisher
// -----------------------------------------------------------------------

func TestControllerHasDamageOnDrawPunisher_NoPunisherZero(t *testing.T) {
	gs := newTestGame(t, 2)
	if got := controllerHasDamageOnDrawPunisher(gs, 0); got != 0 {
		t.Errorf("empty battlefield should be 0; got %d", got)
	}
}

func TestControllerHasDamageOnDrawPunisher_UnderworldDreamsFires(t *testing.T) {
	gs := newTestGame(t, 2)
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	if got := controllerHasDamageOnDrawPunisher(gs, 0); got != 1 {
		t.Errorf("Underworld Dreams (name match) should return 1; got %d", got)
	}
}

func TestControllerHasDamageOnDrawPunisher_SpitefulVisionsHigherDamage(t *testing.T) {
	gs := newTestGame(t, 2)
	mkPunisher(gs, 0, "Spiteful Visions",
		"at the beginning of each player's draw step, that player draws an additional card and spiteful visions deals 1 damage to that player")
	if got := controllerHasDamageOnDrawPunisher(gs, 0); got != 2 {
		t.Errorf("Spiteful Visions should return 2 per draw; got %d", got)
	}
}

func TestControllerHasDamageOnDrawPunisher_StacksAcrossPunishers(t *testing.T) {
	gs := newTestGame(t, 2)
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	mkPunisher(gs, 0, "Curse of Wizardry",
		"whenever a player draws a card, that player loses 1 life")
	// 1 (Underworld) + 1 (Curse via "draws a card ... loses") = 2.
	if got := controllerHasDamageOnDrawPunisher(gs, 0); got != 2 {
		t.Errorf("Underworld Dreams + Curse of Wizardry should stack to 2; got %d", got)
	}
}

func TestControllerHasDamageOnDrawPunisher_OnlyOppDrawsDoesNotMatch(t *testing.T) {
	gs := newTestGame(t, 2)
	mkPunisher(gs, 0, "Some Asymmetric Punisher",
		"whenever an opponent draws a card, that player loses 1 life")
	// "Opponent draws" without name match — that punisher hits opps, not us.
	// Our scan filters for "draws ... loses" or known names; "an opponent
	// draws ... that player loses" matches neither user-draw needles nor a
	// canonical name, so it should return 0.
	if got := controllerHasDamageOnDrawPunisher(gs, 0); got != 0 {
		t.Errorf("opp-only punisher (no name match, no symmetric phrase) should be 0; got %d", got)
	}
}

// -----------------------------------------------------------------------
// estimateDrawTriggerCardCount
// -----------------------------------------------------------------------

func TestEstimateDrawTriggerCardCount_NilTopZero(t *testing.T) {
	if got := estimateDrawTriggerCardCount(nil); got != 0 {
		t.Errorf("nil top should be 0; got %d", got)
	}
}

func TestEstimateDrawTriggerCardCount_NoSourceZero(t *testing.T) {
	top := &gameengine.StackItem{Kind: "triggered"}
	if got := estimateDrawTriggerCardCount(top); got != 0 {
		t.Errorf("top with no source should be 0; got %d", got)
	}
}

func TestEstimateDrawTriggerCardCount_OneCard(t *testing.T) {
	src := mkDrawTriggerSource("Drawer",
		"when this creature enters the battlefield, draw a card")
	top := &gameengine.StackItem{Kind: "triggered", Source: src}
	if got := estimateDrawTriggerCardCount(top); got != 1 {
		t.Errorf("draw-a-card should be 1; got %d", got)
	}
}

func TestEstimateDrawTriggerCardCount_TwoCards(t *testing.T) {
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Source: src}
	if got := estimateDrawTriggerCardCount(top); got != 2 {
		t.Errorf("draw two should be 2; got %d", got)
	}
}

func TestEstimateDrawTriggerCardCount_ThreeCards(t *testing.T) {
	src := mkDrawTriggerSource("Big Drawer",
		"when this creature enters, draw three cards")
	top := &gameengine.StackItem{Kind: "triggered", Source: src}
	if got := estimateDrawTriggerCardCount(top); got != 3 {
		t.Errorf("draw three should be 3; got %d", got)
	}
}

// -----------------------------------------------------------------------
// shouldCounterOwnTrigger composite gate
// -----------------------------------------------------------------------

func TestShouldCounterOwnTrigger_NoPunisherNoTrigger(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("no punisher in play — should NOT counter own trigger even at 2 life")
	}
}

func TestShouldCounterOwnTrigger_NotLethalDamage(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40 // plenty of life
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("punisher would deal only 2 damage at 40 life — should NOT counter (not lethal)")
	}
}

func TestShouldCounterOwnTrigger_LethalFires(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2 // would die from 2 damage
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if !h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("2 damage at 2 life is lethal — should counter own trigger")
	}
}

func TestShouldCounterOwnTrigger_NotOwnTrigger(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	// Controller = 1 (opp's trigger, not ours).
	top := &gameengine.StackItem{Kind: "triggered", Controller: 1, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("opp-controlled trigger should not be considered for self-counter")
	}
}

func TestShouldCounterOwnTrigger_ActivatedAbilityNotMatched(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	src := mkDrawTriggerSource("Necropotence-shape",
		"pay 1 life: draw a card")
	// activated, not triggered — we intentionally activated this.
	top := &gameengine.StackItem{Kind: "activated", Controller: 0, Source: src}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("activated ability is intentional — should NOT counter ourselves")
	}
}

func TestShouldCounterOwnTrigger_SpellNotMatched(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	// A spell (Kind="spell" or default empty, Card set, no Source)
	spell := &gameengine.Card{
		Name:  "Concentrate",
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: "Concentrate",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "draw three cards"},
			},
		},
	}
	top := &gameengine.StackItem{Card: spell, Controller: 0, Kind: "spell"}
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if h.shouldCounterOwnTrigger(gs, 0, top) {
		t.Error("self-cast spell should NOT be self-countered (different play altogether)")
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseResponse counters self-trigger with affordable counter
// -----------------------------------------------------------------------

// TestChooseResponse_CountersOwnDrawTriggerAtLethal pins the gate change.
// Setup: I'm at 2 life with Underworld Dreams in play, my Mulldrifter ETB
// trigger is on the stack, and I hold a Counterspell with mana to cast.
// Expected: ChooseResponse returns the Counterspell.
func TestChooseResponse_CountersOwnDrawTriggerAtLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	// Counterspell in hand + 2 untapped Islands to pay {U}{U}.
	for i := 0; i < 2; i++ {
		island := &gameengine.Card{
			Name:     "Island",
			Types:    []string{"land"},
			TypeLine: "Basic Land — Island",
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{Card: island, Controller: 0, Owner: 0, Flags: map[string]int{}})
	}
	counter := &gameengine.Card{
		Name:           "Counterspell",
		Types:          []string{"instant", "cost:2"},
		ManaCostString: "{U}{U}",
		AST: &gameast.CardAST{
			Name: "Counterspell",
			// Activated with empty cost — the "spell-body" parser layout
			// counterSpellEffect recognizes as a hand-castable counter
			// (mirrors how Summon-the-School-shape spells get parsed).
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{},
					Effect: &gameast.CounterSpell{},
				},
			},
		},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, counter)

	src := mkDrawTriggerSource("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards")
	top := &gameengine.StackItem{Kind: "triggered", Controller: 0, Source: src}

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil {
		t.Fatal("expected ChooseResponse to counter own draw trigger at lethal; got nil")
	}
	if resp.Card != counter {
		t.Errorf("expected counter to be the Counterspell; got %v", resp.Card)
	}
}

// TestChooseResponse_StillSkipsSelfSpellAtLethal confirms the gate fix
// is NARROW: a self-controlled SPELL (not a trigger) is still skipped
// even at 2 life with a punisher out. We never counter our own spells.
func TestChooseResponse_StillSkipsSelfSpellAtLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	for i := 0; i < 2; i++ {
		island := &gameengine.Card{
			Name: "Island", Types: []string{"land"}, TypeLine: "Basic Land — Island",
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{Card: island, Controller: 0, Owner: 0, Flags: map[string]int{}})
	}
	counter := &gameengine.Card{
		Name: "Counterspell", Types: []string{"instant", "cost:2"}, ManaCostString: "{U}{U}",
		AST: &gameast.CardAST{Name: "Counterspell",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "counter target spell"}}},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, counter)

	// Self-cast Concentrate (spell, not trigger).
	concentrate := &gameengine.Card{
		Name: "Concentrate", Types: []string{"sorcery"},
		AST: &gameast.CardAST{Name: "Concentrate",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "draw three cards"}}},
	}
	top := &gameengine.StackItem{Kind: "spell", Card: concentrate, Controller: 0}

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if resp := h.ChooseResponse(gs, 0, top); resp != nil {
		t.Errorf("self-cast spell should not be self-countered even at lethal; got resp=%v", resp.Card)
	}
}
