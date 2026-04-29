package gameengine

// Phase 5 tests — stack + priority system.
//
// Uses the fixture helpers from resolve_test.go (newFixtureGame,
// addBattlefield, countEvents, lastEventOfKind) and combat_test.go
// (addCreature, addCardWithAbility). All fixtures are synthetic.
//
// CR references throughout cite data/rules/MagicCompRules-20260227.txt.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Phase 5 fixture helpers.
// ---------------------------------------------------------------------------

// addHandCard puts a card on seat's hand with the given Types (cost tag
// goes in Types as "cost:N"). Returns the Card pointer.
func addHandCard(gs *GameState, seat int, name string, cost int, types ...string) *Card {
	t := []string{}
	t = append(t, types...)
	if cost > 0 {
		t = append(t, "cost:"+itoa(cost))
	}
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: t,
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// addHandCardWithEffect is like addHandCard but also wires a single
// Activated ability with the given Effect as the "spell effect"
// (Activated with empty cost = spell-effect pattern per playloop.py).
func addHandCardWithEffect(gs *GameState, seat int, name string, cost int, eff gameast.Effect, types ...string) *Card {
	c := addHandCard(gs, seat, name, cost, types...)
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{Effect: eff},
		},
	}
	c.AST = ast
	return c
}

// addCounterspellInHand puts a counterspell (2U for Counterspell, etc.) in
// seat's hand. Returns the Card pointer.
func addCounterspellInHand(gs *GameState, seat int, name string, cost int) *Card {
	eff := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell", Targeted: true},
	}
	return addHandCardWithEffect(gs, seat, name, cost, eff, "instant")
}

// itoa moved to activation.go as a non-test package-level function
// (needed by zone_cast.go, resolve.go, and activation.go).

// ---------------------------------------------------------------------------
// 1. Basic CastSpell → stack push + resolve.
// ---------------------------------------------------------------------------

func TestStack_CastSpell_PushesAndResolves(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")

	if err := CastSpell(gs, 0, bolt, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// Stack should be empty after resolution.
	if len(gs.Stack) != 0 {
		t.Errorf("expected empty stack, got %d items", len(gs.Stack))
	}
	// Opponent life should be 17 (20 - 3).
	if gs.Seats[1].Life != 17 {
		t.Errorf("expected opponent life 17, got %d", gs.Seats[1].Life)
	}
	// Card should be in graveyard.
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected 1 card in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if countEvents(gs, "stack_push") == 0 {
		t.Errorf("expected stack_push event")
	}
	if countEvents(gs, "stack_resolve") == 0 {
		t.Errorf("expected stack_resolve event")
	}
}

// ---------------------------------------------------------------------------
// 2. Cast with no mana returns CastError.
// ---------------------------------------------------------------------------

func TestStack_CastSpell_InsufficientMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 0
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")

	err := CastSpell(gs, 0, bolt, nil)
	if err == nil {
		t.Fatalf("expected CastError for insufficient mana")
	}
	// Card should still be in hand (state restored per CR §601.2e).
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected card back in hand on failed cast, got %d",
			len(gs.Seats[0].Hand))
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("damage should not have applied")
	}
}

// ---------------------------------------------------------------------------
// 3. ManaPool decreases on cast.
// ---------------------------------------------------------------------------

func TestStack_CastSpell_ManaDeducted(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	bolt := addHandCardWithEffect(gs, 0, "Shock", 2,
		&gameast.Damage{
			Amount: *gameast.NumInt(2),
			Target: gameast.TargetOpponent(),
		}, "instant")

	_ = CastSpell(gs, 0, bolt, nil)
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana left, got %d", gs.Seats[0].ManaPool)
	}
}

// ---------------------------------------------------------------------------
// 4. Opponent Counterspell → spell is countered, source to graveyard.
// ---------------------------------------------------------------------------

func TestStack_Counterspell_OpponentCounters(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // Bolt cost
	gs.Seats[1].ManaPool = 2 // Counterspell cost
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")
	cs := addCounterspellInHand(gs, 1, "Counterspell", 2)
	_ = cs

	if err := CastSpell(gs, 0, bolt, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	// Opponent life unchanged — damage never resolved.
	if gs.Seats[1].Life != 20 {
		t.Errorf("expected life 20 (countered), got %d", gs.Seats[1].Life)
	}
	// Bolt should be in seat-0 graveyard (countered → owner's graveyard).
	inGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Lightning Bolt" {
			inGY = true
			break
		}
	}
	if !inGY {
		t.Errorf("expected Bolt in seat 0 graveyard after counter")
	}
	// Counterspell also resolved → should be in seat-1 graveyard.
	inGY2 := false
	for _, c := range gs.Seats[1].Graveyard {
		if c.Name == "Counterspell" {
			inGY2 = true
			break
		}
	}
	if !inGY2 {
		t.Errorf("expected Counterspell in seat 1 graveyard after resolution")
	}
}

// ---------------------------------------------------------------------------
// 5. Counter-counter war: opp-Negate counters seat-1's Counterspell.
//    Bolt resolves after all counters settle.
// ---------------------------------------------------------------------------

func TestStack_CounterWar_TwoDeep(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	// Seat 0 has Bolt + Negate (to counter seat-1's response).
	gs.Seats[0].ManaPool = 3 // Bolt (1) + Negate (2)
	gs.Seats[1].ManaPool = 2 // Counterspell
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")
	_ = addCounterspellInHand(gs, 0, "Negate", 2)
	_ = addCounterspellInHand(gs, 1, "Counterspell", 2)

	_ = CastSpell(gs, 0, bolt, nil)

	// With the greedy policy, the sequence is:
	//  - Bolt pushed by seat 0.
	//  - Priority: seat 1 counters with Counterspell → stack [Bolt, CS1].
	//    CS1 marks Bolt.Countered=true during its *resolution*, not push.
	//  - Priority re-opens: seat 0 has Negate. Negate counters CS1? It
	//    could — but our GetResponse checks that incoming.Countered is
	//    false before counter-casting. CS1 is not yet countered, so seat
	//    0's Negate will respond → stack [Bolt, CS1, Negate].
	//  - Priority re-opens: nobody has responses left.
	//  - Negate resolves → marks CS1.Countered = true.
	//  - CS1 resolves countered → goes to graveyard, Bolt not marked.
	//  - Bolt resolves → 3 damage to seat 1 (life 17).
	if gs.Seats[1].Life != 17 {
		t.Errorf("expected opp life 17 (Bolt resolved after counter-war), got %d",
			gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// 6. Split-second active — instants can't be cast in response.
// ---------------------------------------------------------------------------

func TestStack_SplitSecond_BlocksResponses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	// Put a split-second spell on the stack manually.
	ssCard := &Card{
		Name:  "Sudden Shock",
		Owner: 0,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
			},
		},
	}
	ssItem := &StackItem{
		ID:         1,
		Controller: 0,
		Card:       ssCard,
	}
	gs.Stack = append(gs.Stack, ssItem)

	if !SplitSecondActive(gs) {
		t.Fatal("expected split_second active")
	}
	// Opponent tries to cast Counterspell → should fail.
	gs.Seats[1].ManaPool = 2
	cs := addCounterspellInHand(gs, 1, "Counterspell", 2)
	err := CastSpell(gs, 1, cs, nil)
	if err == nil {
		t.Fatalf("expected CastError with split_second in play")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "split_second" {
		t.Errorf("expected split_second reason, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 7. Split-second marks non-countered items only.
// ---------------------------------------------------------------------------

func TestStack_SplitSecond_IgnoresCountered(t *testing.T) {
	gs := newFixtureGame(t)
	ssCard := &Card{
		Name: "Sudden Shock", Owner: 0,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
			},
		},
	}
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0, Card: ssCard, Countered: true},
	}
	if SplitSecondActive(gs) {
		t.Error("countered split-second should not lock out responses")
	}
}

// ---------------------------------------------------------------------------
// 8. Teferi-style sorcery-speed restriction blocks opponent responses.
// ---------------------------------------------------------------------------

func TestStack_Teferi_BlocksOppInstantResponses(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	// Seat 0 controls Teferi — a Static with opp_sorcery_speed_only.
	teferi := addBattlefield(gs, 0, "Teferi, Time Raveler", 1, 4,
		"creature", "planeswalker")
	teferi.Card.AST = &gameast.CardAST{
		Name: "Teferi, Time Raveler",
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "opp_sorcery_speed_only",
				},
				Raw: "Each opponent can cast spells only any time they could cast a sorcery.",
			},
		},
	}

	// Seat 0 casts Bolt — opponent should NOT be able to counter.
	gs.Seats[0].ManaPool = 1
	gs.Seats[1].ManaPool = 2
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")
	_ = addCounterspellInHand(gs, 1, "Counterspell", 2)

	_ = CastSpell(gs, 0, bolt, nil)
	if gs.Seats[1].Life != 17 {
		t.Errorf("expected life 17 (Teferi blocks counter), got %d", gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// 9. Teferi doesn't restrict its own controller.
// ---------------------------------------------------------------------------

func TestStack_Teferi_SelfNotRestricted(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 1 // opp is active casting
	teferi := addBattlefield(gs, 0, "Teferi, Time Raveler", 1, 4,
		"creature", "planeswalker")
	teferi.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{ModKind: "opp_sorcery_speed_only"},
			},
		},
	}

	// Seat 0 (Teferi's controller) is NOT restricted.
	if OppRestrictsDefenderToSorcerySpeed(gs, 0) {
		t.Error("Teferi's controller should not be restricted by own static")
	}
	// Seat 1 IS restricted.
	if !OppRestrictsDefenderToSorcerySpeed(gs, 1) {
		t.Error("opponent should be restricted by Teferi")
	}
}

// ---------------------------------------------------------------------------
// 10. Priority pass with all-pass → top resolves.
// ---------------------------------------------------------------------------

func TestStack_PriorityRound_AllPass(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	// No cards in hand → no responses → round returns immediately.
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0, Card: &Card{Name: "X", Owner: 0}},
	}
	PriorityRound(gs)
	// Stack still has its item (PriorityRound doesn't resolve; the caller
	// resolves after the round ends).
	if len(gs.Stack) != 1 {
		t.Errorf("expected stack untouched, got size %d", len(gs.Stack))
	}
	if countEvents(gs, "priority_pass") == 0 {
		t.Error("expected priority_pass events")
	}
}

// ---------------------------------------------------------------------------
// 11. Stack is LIFO — last pushed resolves first.
// ---------------------------------------------------------------------------

func TestStack_LIFO_Ordering(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "A", 0, 0, "instant")
	b := addBattlefield(gs, 0, "B", 0, 0, "instant")

	// Build a Damage effect that targets seat 1 for 1 each.
	eff1 := &gameast.Damage{
		Amount: *gameast.NumInt(1),
		Target: gameast.TargetOpponent(),
	}
	eff2 := &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.TargetOpponent(),
	}

	PushStackItem(gs, &StackItem{
		Controller: 0, Source: a, Card: a.Card, Effect: eff1,
	})
	PushStackItem(gs, &StackItem{
		Controller: 0, Source: b, Card: b.Card, Effect: eff2,
	})

	// Resolve top (B with 2 damage) first.
	ResolveStackTop(gs)
	if gs.Seats[1].Life != 18 {
		t.Errorf("expected life 18 after B resolves (2 dmg), got %d",
			gs.Seats[1].Life)
	}
	ResolveStackTop(gs)
	if gs.Seats[1].Life != 17 {
		t.Errorf("expected life 17 after A (1 dmg) resolves, got %d",
			gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// 12. ResolveStackTop on countered item → card to graveyard, effect skipped.
// ---------------------------------------------------------------------------

func TestStack_ResolveCounteredItem(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	eff := &gameast.Damage{
		Amount: *gameast.NumInt(3),
		Target: gameast.TargetOpponent(),
	}
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0, Card: card, Effect: eff, Countered: true},
	}
	ResolveStackTop(gs)
	if gs.Seats[1].Life != 20 {
		t.Errorf("countered spell should not deal damage, got life %d",
			gs.Seats[1].Life)
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected countered spell in graveyard")
	}
}

// ---------------------------------------------------------------------------
// 13. SBA fires between stack resolutions — creature at 0 tough dies.
// ---------------------------------------------------------------------------

func TestStack_SBABetweenResolutions(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1
	// Seat 1 has a 1-toughness creature.
	bear := addBattlefield(gs, 1, "Bear", 1, 1, "creature")
	_ = bear

	// Seat 0 casts Shock for 1 damage at bear.
	eff := &gameast.Damage{
		Amount: *gameast.NumInt(1),
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	shock := addHandCardWithEffect(gs, 0, "Shock", 1, eff, "instant")
	_ = CastSpell(gs, 0, shock, nil)

	// After SBAs: bear should be in graveyard.
	if len(gs.Seats[1].Graveyard) < 1 {
		t.Errorf("expected bear in graveyard after SBAs")
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("expected bear removed from battlefield, got %d",
			len(gs.Seats[1].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// 14. GetResponse returns nil when no counterspell in hand.
// ---------------------------------------------------------------------------

func TestStack_GetResponse_NoCounterspell(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[1].ManaPool = 2
	addHandCard(gs, 1, "Grizzly Bears", 2, "creature")
	item := &StackItem{
		ID:         1,
		Controller: 0,
		Card:       &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}},
	}
	resp := GetResponse(gs, 1, item)
	if resp != nil {
		t.Errorf("expected nil response (no counterspell), got %+v", resp)
	}
}

// ---------------------------------------------------------------------------
// 15. GetResponse returns nil when can't afford.
// ---------------------------------------------------------------------------

func TestStack_GetResponse_NoMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[1].ManaPool = 1 // Can't afford 2-cost CS
	_ = addCounterspellInHand(gs, 1, "Counterspell", 2)
	item := &StackItem{
		ID: 1, Controller: 0,
		Card: &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}},
	}
	resp := GetResponse(gs, 1, item)
	if resp != nil {
		t.Errorf("expected nil response (can't afford), got %+v", resp)
	}
}

// ---------------------------------------------------------------------------
// 16. GetResponse skips own spells.
// ---------------------------------------------------------------------------

func TestStack_GetResponse_OwnSpellNotCountered(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 2
	_ = addCounterspellInHand(gs, 0, "Counterspell", 2)
	// Seat 0's own spell on stack.
	item := &StackItem{
		ID: 1, Controller: 0,
		Card: &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}},
	}
	resp := GetResponse(gs, 0, item)
	if resp != nil {
		t.Errorf("shouldn't counter own spell, got %+v", resp)
	}
}

// ---------------------------------------------------------------------------
// 17. PushTriggeredAbility creates stack item + resolves.
// ---------------------------------------------------------------------------

func TestStack_PushTriggeredAbility(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Impulsive Pyromancer", 1, 1, "creature")
	eff := &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.TargetOpponent(),
	}

	preLife := gs.Seats[1].Life
	PushTriggeredAbility(gs, src, eff)
	if gs.Seats[1].Life != preLife-2 {
		t.Errorf("expected 2 damage from triggered ability, life=%d",
			gs.Seats[1].Life)
	}
	if countEvents(gs, "stack_push") == 0 {
		t.Error("expected stack_push event for triggered ability")
	}
}

// ---------------------------------------------------------------------------
// 18. Combat damage trigger goes on stack — not resolved inline.
// ---------------------------------------------------------------------------

func TestStack_CombatDamageTriggerOnStack(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	// Build a creature with "whenever this deals damage, draw a card".
	drawTrigger := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "deal_combat_damage"},
		Effect: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	attacker := addCardWithAbility(gs, 0, "Curiosity Bear", 2, 2, drawTrigger)
	_ = attacker
	addLibrary(gs, 0, "CardA")

	CombatPhase(gs)

	// One card should have been drawn via the trigger.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card drawn from combat-damage trigger, got %d",
			len(gs.Seats[0].Hand))
	}
	// Verify the trigger went through the stack (stack_push event with
	// source=Curiosity Bear).
	sawPush := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Curiosity Bear" {
			sawPush = true
			break
		}
	}
	if !sawPush {
		t.Error("expected stack_push for Curiosity Bear combat-damage trigger")
	}
}

// ---------------------------------------------------------------------------
// 19. ResolveStackTop on empty stack is a no-op.
// ---------------------------------------------------------------------------

func TestStack_ResolveEmptyStack(t *testing.T) {
	gs := newFixtureGame(t)
	ResolveStackTop(gs) // should not panic
	if len(gs.Stack) != 0 {
		t.Errorf("empty stack should stay empty")
	}
}

// ---------------------------------------------------------------------------
// 20. CastSpell with nil card fails.
// ---------------------------------------------------------------------------

func TestStack_CastSpell_NilCard(t *testing.T) {
	gs := newFixtureGame(t)
	if err := CastSpell(gs, 0, nil, nil); err == nil {
		t.Error("expected error for nil card")
	}
}

// ---------------------------------------------------------------------------
// 21. CastSpell not in hand fails.
// ---------------------------------------------------------------------------

func TestStack_CastSpell_NotInHand(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 1
	card := &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	// NOT added to hand.
	err := CastSpell(gs, 0, card, nil)
	if err == nil {
		t.Error("expected error for card not in hand")
	}
	if ce, ok := err.(*CastError); !ok || ce.Reason != "not_in_hand" {
		t.Errorf("expected not_in_hand reason, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 22. Triggered ability from ETB (via combat) goes on stack.
// ---------------------------------------------------------------------------

func TestStack_PushStackItem_AssignsID(t *testing.T) {
	gs := newFixtureGame(t)
	item1 := PushStackItem(gs, &StackItem{Controller: 0})
	item2 := PushStackItem(gs, &StackItem{Controller: 0})
	if item1.ID == 0 || item2.ID == 0 {
		t.Error("expected non-zero IDs")
	}
	if item1.ID == item2.ID {
		t.Errorf("expected distinct IDs, got %d/%d", item1.ID, item2.ID)
	}
}

// ---------------------------------------------------------------------------
// 23. SplitSecondActive — multiple stack items, one with split_second.
// ---------------------------------------------------------------------------

func TestStack_SplitSecondActive_MixedStack(t *testing.T) {
	gs := newFixtureGame(t)
	// A normal spell on stack.
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0,
			Card: &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}},
	}
	if SplitSecondActive(gs) {
		t.Error("no split-second expected here")
	}
	// Add a split-second spell.
	ssCard := &Card{
		Name: "Sudden Shock", Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
			},
		},
	}
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 2, Controller: 0, Card: ssCard,
	})
	if !SplitSecondActive(gs) {
		t.Error("expected split-second active")
	}
}

// ---------------------------------------------------------------------------
// 24. Priority skipped event fires under split-second.
// ---------------------------------------------------------------------------

func TestStack_PriorityRound_SplitSecondSkip(t *testing.T) {
	gs := newFixtureGame(t)
	ssCard := &Card{
		Name: "Krosan Grip", Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "split_second"},
			},
		},
	}
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0, Card: ssCard},
	}
	// Opponent has a counterspell in hand with mana to spare.
	gs.Seats[1].ManaPool = 2
	_ = addCounterspellInHand(gs, 1, "Counterspell", 2)

	PriorityRound(gs)
	// No response should have been cast.
	if len(gs.Stack) != 1 {
		t.Errorf("expected stack unchanged under split_second, got %d items",
			len(gs.Stack))
	}
	// Card should still be in seat 1's hand.
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("expected counterspell still in hand, got %d cards",
			len(gs.Seats[1].Hand))
	}
}

// ---------------------------------------------------------------------------
// 25. Bolt → resolves → damage lands on opponent (cast→resolve cycle).
// ---------------------------------------------------------------------------

func TestStack_BoltFullCycle(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1
	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")

	_ = CastSpell(gs, 0, bolt, nil)
	if gs.Seats[1].Life != 17 {
		t.Errorf("life: want 17, got %d", gs.Seats[1].Life)
	}
	// Confirm stack_push + stack_resolve + resolve events in order.
	seenPush := false
	seenResolve := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" {
			seenPush = true
		}
		if ev.Kind == "stack_resolve" && seenPush {
			seenResolve = true
		}
	}
	if !seenResolve {
		t.Error("expected stack_resolve after stack_push")
	}
}

// ---------------------------------------------------------------------------
// 26. PushTriggeredAbility nil guards.
// ---------------------------------------------------------------------------

func TestStack_PushTriggeredAbility_NilGuards(t *testing.T) {
	gs := newFixtureGame(t)
	if r := PushTriggeredAbility(gs, nil, nil); r != nil {
		t.Error("expected nil return on nil source")
	}
	src := addBattlefield(gs, 0, "X", 1, 1, "creature")
	if r := PushTriggeredAbility(gs, src, nil); r != nil {
		t.Error("expected nil return on nil effect")
	}
}

// ---------------------------------------------------------------------------
// 27. Cast permanent spell → resolves to battlefield, not graveyard.
// ---------------------------------------------------------------------------

func TestStack_CastPermanent_NotToGraveyard(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2
	// A creature "spell" with no on-resolution effect (permanent spell).
	// Give it toughness 2 so §704.5f doesn't immediately SBA it into
	// the graveyard after ETB (Phase 12: permanent spells now actually
	// enter the battlefield on resolution).
	creature := addHandCard(gs, 0, "Grizzly Bears", 2, "creature")
	creature.BasePower = 2
	creature.BaseToughness = 2

	_ = CastSpell(gs, 0, creature, nil)
	// Card should NOT be in graveyard — permanent spells resolve to the
	// battlefield per CR §608.3a.
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("permanent spell should not go to graveyard, got %d",
			len(gs.Seats[0].Graveyard))
	}
	// It SHOULD be on the battlefield now.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("permanent spell should be on battlefield, got %d",
			len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// 28. Benchmark: cast→priority→resolve cycle for Lightning Bolt.
// ---------------------------------------------------------------------------

func BenchmarkCastResolve(b *testing.B) {
	// One-time setup: build a pool of cards, cast/reset in the loop.
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gs := NewGameState(2, nil, nil)
		gs.Active = 0
		gs.Seats[0].ManaPool = 1
		gs.Seats[1].Life = 20
		bolt := &Card{
			Name:  "Lightning Bolt",
			Owner: 0,
			Types: []string{"instant", "cost:1"},
			AST: &gameast.CardAST{
				Name: "Lightning Bolt",
				Abilities: []gameast.Ability{
					&gameast.Activated{
						Effect: &gameast.Damage{
							Amount: *gameast.NumInt(3),
							Target: gameast.TargetOpponent(),
						},
					},
				},
			},
		}
		gs.Seats[0].Hand = []*Card{bolt}
		_ = CastSpell(gs, 0, bolt, nil)
	}
}

// =============================================================================
// Wave 4 — X cost tests (CR §107.3).
// =============================================================================

func TestManaCostContainsX_TypesConvention(t *testing.T) {
	card := &Card{Name: "Fireball", Types: []string{"sorcery", "x_cost", "cost:1"}}
	if !ManaCostContainsX(card) {
		t.Fatal("card with x_cost type should be detected as X-cost")
	}
}

func TestManaCostContainsX_NoX(t *testing.T) {
	card := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	if ManaCostContainsX(card) {
		t.Fatal("card without X should not be detected as X-cost")
	}
}

func TestCastSpell_XCostPaysCorrectAmount(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 7

	// Create an X spell with base cost 1 (XR Fireball).
	fireball := &Card{
		Name:  "Fireball",
		Owner: 0,
		Types: []string{"sorcery", "x_cost", "cost:1"},
	}
	// Install a hat that picks X = all available.
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].Hand = []*Card{fireball}

	err := CastSpell(gs, 0, fireball, nil)
	if err != nil {
		t.Fatalf("cast should succeed: %v", err)
	}

	// Base cost = 1, available for X = 7-1 = 6, so total = 7.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana remaining, got %d", gs.Seats[0].ManaPool)
	}

	// Check the stack item has ChosenX set.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "cast" && ev.Source == "Fireball" {
			if x, ok := ev.Details["chosen_x"]; ok {
				if xv, ok2 := x.(int); ok2 && xv == 6 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("cast event should have chosen_x = 6")
	}
}

func TestCastSpell_XCostZeroManaLeft(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 1 // just enough for base cost

	card := &Card{
		Name:  "Walking Ballista",
		Owner: 0,
		Types: []string{"creature", "x_cost", "cost:0"}, // XX but base=0
	}
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].Hand = []*Card{card}

	err := CastSpell(gs, 0, card, nil)
	if err != nil {
		t.Fatalf("cast should succeed: %v", err)
	}

	// Available for X = 1-0 = 1, greedy hat picks X=1, total = 1.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected 0 mana remaining, got %d", gs.Seats[0].ManaPool)
	}
}

func TestCastSpell_XCostInsufficientMana(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 0 // base cost is 2, can't afford

	card := &Card{
		Name:  "Expensive X Spell",
		Owner: 0,
		Types: []string{"sorcery", "x_cost", "cost:2"},
	}
	gs.Seats[0].Hat = &GreedyHatStub{}
	gs.Seats[0].Hand = []*Card{card}

	err := CastSpell(gs, 0, card, nil)
	if err == nil {
		t.Fatal("cast should fail with insufficient mana")
	}
}
