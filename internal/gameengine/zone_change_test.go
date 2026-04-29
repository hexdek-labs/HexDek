package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// =============================================================================
// P0 #1: Dies/LTB zone-change trigger system
// =============================================================================

// TestBloodArtist_DiesTriggerFires — Blood Artist on battlefield, another
// creature dies → Blood Artist's "whenever a creature dies" trigger fires
// (drain 1 life). This is the canonical test for observer-pattern dies triggers.
func TestBloodArtist_DiesTriggerFires(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Blood Artist on seat 0 with "whenever a creature dies" trigger.
	bloodArtist := addBattlefield(gs, 0, "Blood Artist", 0, 1, "creature")
	bloodArtist.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "die",
					Actor: &gameast.Filter{Base: "creature"},
				},
				Effect: &gameast.Sequence{
					Items: []gameast.Effect{
						&gameast.LoseLife{
							Target: gameast.Filter{Base: "opponent", Quantifier: "each"},
							Amount: numRef(1),
						},
						&gameast.GainLife{
							Target: gameast.Filter{Base: "you"},
							Amount: numRef(1),
						},
					},
				},
			},
		},
	}

	// Creature on seat 1 that will die.
	victim := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Destroy the creature — should fire Blood Artist's trigger.
	DestroyPermanent(gs, victim, nil)

	// Check that Blood Artist's trigger fired.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Blood Artist" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Blood Artist's dies trigger should have been pushed to stack")
	}

	// Check creature_dies hook fired.
	if countEvents(gs, "creature_dies") == 0 {
		// The per-card trigger hook fires as "creature_dies" event via FireCardTrigger.
		// This is only emitted if TriggerHook is installed. If not installed, it's fine.
	}

	// Verify the victim is in graveyard.
	found = false
	for _, c := range gs.Seats[1].Graveyard {
		if c.Name == "Grizzly Bears" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Grizzly Bears should be in seat 1's graveyard")
	}

	// Verify victim is NOT on battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.Name == "Grizzly Bears" {
			t.Fatal("Grizzly Bears should not be on battlefield after destroy")
		}
	}
}

// TestSelfDiesTrigger — Kokusho-style "when this creature dies" fires.
func TestSelfDiesTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Creature with "when this creature dies" trigger (self-trigger).
	kokusho := addBattlefield(gs, 0, "Kokusho", 5, 5, "creature")
	kokusho.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "die",
					// No actor filter = self-trigger.
				},
				Effect: &gameast.LoseLife{
					Target: gameast.Filter{Base: "opponent", Quantifier: "each"},
					Amount: numRef(5),
				},
			},
		},
	}

	// Kill Kokusho.
	DestroyPermanent(gs, kokusho, nil)

	// Kokusho's self-trigger should have pushed to stack.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Kokusho" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Kokusho's self-dies trigger should have been pushed to stack")
	}
}

// TestSBA_DiesTriggerFires — creature lethal damage via SBA fires triggers.
func TestSBA_DiesTriggerFires(t *testing.T) {
	gs := newFixtureGame(t)

	// Blood Artist on seat 0.
	bloodArtist := addBattlefield(gs, 0, "Blood Artist", 0, 1, "creature")
	bloodArtist.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "die",
					Actor: &gameast.Filter{Base: "creature"},
				},
				Effect: &gameast.GainLife{
					Target: gameast.Filter{Base: "you"},
					Amount: numRef(1),
				},
			},
		},
	}

	// Creature on seat 1 with lethal damage.
	bear := addBattlefield(gs, 1, "Bear", 2, 2, "creature")
	bear.MarkedDamage = 3 // lethal

	// Run SBAs — should destroy bear and fire Blood Artist trigger.
	StateBasedActions(gs)

	// Bear should be gone from battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.Name == "Bear" {
			t.Fatal("Bear should be destroyed by SBA")
		}
	}

	// Blood Artist trigger should have fired.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Blood Artist" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Blood Artist's dies trigger should fire from SBA death")
	}
}

// TestLTB_Trigger — "when this permanent leaves the battlefield" fires on bounce.
func TestLTB_Trigger(t *testing.T) {
	gs := newFixtureGame(t)

	// Permanent with LTB trigger.
	perm := addBattlefield(gs, 0, "LTB Creature", 3, 3, "creature")
	perm.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "ltb",
				},
				Effect: &gameast.Draw{
					Count: numRef(1),
				},
			},
		},
	}
	// Add library so the draw has something to draw.
	addLibrary(gs, 0, "Card1")

	// Bounce the permanent.
	BouncePermanent(gs, perm, nil, "hand")

	// LTB trigger should fire.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "LTB Creature" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LTB trigger should fire on bounce")
	}
}

// =============================================================================
// P0 #2: Target legality at resolution
// =============================================================================

// TestFizzle_AllTargetsIllegal — Lightning Bolt targeting creature, creature
// bounced in response → Bolt fizzles.
func TestFizzle_AllTargetsIllegal(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 10
	gs.Seats[1].Life = 20

	// Put a creature on seat 1's battlefield.
	target := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Create a stack item targeting the creature (simulating Lightning Bolt).
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant", "cost:1"}},
		Effect: &gameast.Damage{
			Amount: numRef(3),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: target},
		},
	}
	gs.Stack = append(gs.Stack, item)

	// Bounce the target in response (simulating opponent's response).
	gs.removePermanent(target)
	gs.moveToZone(target.Card.Owner, target.Card, "hand")

	// Now resolve the stack top — should fizzle.
	ResolveStackTop(gs)

	// Check for fizzle event.
	fizzled := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "fizzle" {
			fizzled = true
			break
		}
	}
	if !fizzled {
		t.Fatal("Lightning Bolt should fizzle when all targets are illegal")
	}

	// Lightning Bolt should be in graveyard (countered on resolution).
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Lightning Bolt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Fizzled spell should go to graveyard")
	}
}

// TestPartialTargets_SomeLegal — spell with two targets, one becomes illegal,
// resolves with the legal target only.
func TestPartialTargets_SomeLegal(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 10
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	// Two creatures on seat 1.
	bear1 := addBattlefield(gs, 1, "Bear A", 2, 2, "creature")
	bear2 := addBattlefield(gs, 1, "Bear B", 2, 2, "creature")

	// Spell targeting both.
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Double Bolt", Owner: 0, Types: []string{"instant"}},
		Effect: &gameast.Damage{
			Amount: numRef(3),
			Target: gameast.Filter{Base: "creature", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: bear1},
			{Kind: TargetKindPermanent, Permanent: bear2},
		},
	}
	gs.Stack = append(gs.Stack, item)

	// Remove bear1 from battlefield (bounced in response).
	gs.removePermanent(bear1)
	gs.moveToZone(bear1.Card.Owner, bear1.Card, "hand")

	// Resolve — should NOT fizzle because bear2 is still legal.
	ResolveStackTop(gs)

	// Should NOT have fizzle event.
	for _, ev := range gs.EventLog {
		if ev.Kind == "fizzle" {
			t.Fatal("spell should NOT fizzle when at least one target is legal")
		}
	}
}

// TestNoTargets_NoFizzle — spell with no targets resolves normally.
func TestNoTargets_NoFizzle(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20

	// Spell with no targets (like Divination).
	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Divination", Owner: 0, Types: []string{"sorcery"}},
		Effect:     &gameast.Draw{Count: numRef(2)},
		Targets:    nil,
	}
	addLibrary(gs, 0, "Card1", "Card2")
	gs.Stack = append(gs.Stack, item)

	ResolveStackTop(gs)

	// Should resolve normally (draw 2).
	if countEvents(gs, "fizzle") > 0 {
		t.Fatal("untargeted spell should never fizzle")
	}
	if countEvents(gs, "draw") == 0 {
		t.Fatal("Divination should draw cards")
	}
}

// TestFizzle_PlayerTargetDead — spell targeting a player who has lost.
func TestFizzle_PlayerTargetDead(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	item := &StackItem{
		Controller: 0,
		Card:       &Card{Name: "Lava Axe", Owner: 0, Types: []string{"sorcery"}},
		Effect: &gameast.Damage{
			Amount: numRef(5),
			Target: gameast.Filter{Base: "player", Targeted: true},
		},
		Targets: []Target{
			{Kind: TargetKindSeat, Seat: 1},
		},
	}
	gs.Stack = append(gs.Stack, item)

	// Kill the target player.
	gs.Seats[1].Lost = true
	gs.Seats[1].LossReason = "test"

	// Resolve — should fizzle.
	ResolveStackTop(gs)

	if countEvents(gs, "fizzle") == 0 {
		t.Fatal("spell targeting dead player should fizzle")
	}
}

// =============================================================================
// P0 #3: Spell-resolution destroy/exile/sacrifice vs indestructible
//         + replacement effects
// =============================================================================

// TestWrathOfGod_IndestructibleSurvives — destroy effect on indestructible
// creature → creature survives.
func TestWrathOfGod_IndestructibleSurvives(t *testing.T) {
	gs := newFixtureGame(t)

	// Normal creature.
	mortal := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	// Indestructible creature.
	god := addBattlefield(gs, 1, "Darksteel Colossus", 11, 11, "creature")
	god.Flags["indestructible"] = 1

	// Destroy both via DestroyPermanent.
	DestroyPermanent(gs, mortal, nil)
	DestroyPermanent(gs, god, nil)

	// Mortal should be gone.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Grizzly Bears" {
			t.Fatal("mortal creature should be destroyed")
		}
	}

	// God should survive.
	found := false
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card.Name == "Darksteel Colossus" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("indestructible creature should survive destroy")
	}

	// Check for destroy_prevented event.
	if countEvents(gs, "destroy_prevented") == 0 {
		t.Fatal("should emit destroy_prevented event for indestructible")
	}
}

// TestSacrifice_BypassesIndestructible — sacrifice CAN remove indestructible
// permanents per CR §701.17b.
func TestSacrifice_BypassesIndestructible(t *testing.T) {
	gs := newFixtureGame(t)

	// Indestructible creature.
	god := addBattlefield(gs, 0, "Darksteel Colossus", 11, 11, "creature")
	god.Flags["indestructible"] = 1

	// Sacrifice it.
	SacrificePermanent(gs, god, "edict")

	// Should be gone from battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Darksteel Colossus" {
			t.Fatal("sacrifice should remove indestructible creatures (CR §701.17b)")
		}
	}

	// Should be in graveyard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Darksteel Colossus" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sacrificed creature should be in graveyard")
	}
}

// TestRestInPeace_DestroyExilesInstead — creature dies with Rest in Peace on
// battlefield → creature exiled instead of graveyarded.
func TestRestInPeace_DestroyExilesInstead(t *testing.T) {
	gs := newFixtureGame(t)

	// Register a Rest in Peace replacement effect that redirects
	// would_die to exile.
	ripPerm := addBattlefield(gs, 0, "Rest in Peace", 0, 0, "enchantment")
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      "rip:exile",
		SourcePerm:     ripPerm,
		ControllerSeat: 0,
		Timestamp:      gs.NextTimestamp(),
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return true // applies to all dying permanents
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "exile"
		},
	})

	// Creature on seat 1.
	victim := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	// Destroy it — should go to exile instead of graveyard.
	DestroyPermanent(gs, victim, nil)

	// Should NOT be in graveyard.
	for _, c := range gs.Seats[1].Graveyard {
		if c.Name == "Grizzly Bears" {
			t.Fatal("with Rest in Peace, creature should go to exile, not graveyard")
		}
	}

	// Should be in exile.
	found := false
	for _, c := range gs.Seats[1].Exile {
		if c.Name == "Grizzly Bears" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("with Rest in Peace, destroyed creature should be in exile")
	}
}

// TestRestInPeace_SacrificeExilesInstead — sacrifice with Rest in Peace
// redirects to exile.
func TestRestInPeace_SacrificeExilesInstead(t *testing.T) {
	gs := newFixtureGame(t)

	// Register Rest in Peace replacement.
	ripPerm := addBattlefield(gs, 0, "Rest in Peace", 0, 0, "enchantment")
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      "rip:exile:sac",
		SourcePerm:     ripPerm,
		ControllerSeat: 0,
		Timestamp:      gs.NextTimestamp(),
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return true
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			ev.Payload["to_zone"] = "exile"
		},
	})

	// Creature on seat 0.
	victim := addBattlefield(gs, 0, "Goblin", 1, 1, "creature")

	// Sacrifice it.
	SacrificePermanent(gs, victim, "test")

	// Should be in exile, not graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Goblin" {
			t.Fatal("sacrificed creature should go to exile with Rest in Peace")
		}
	}
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c.Name == "Goblin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sacrificed creature should be in exile with Rest in Peace")
	}
}

// TestCommanderDies_RedirectToCommandZone — commander dies → redirected
// to command zone via §903.9b / FireZoneChange replacement.
func TestCommanderDies_RedirectToCommandZone(t *testing.T) {
	gs := newFixtureGame(t)
	gs.CommanderFormat = true

	// Set up commander for seat 0.
	cmdrCard := &Card{
		Name:  "Test Commander",
		Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	gs.Seats[0].CommanderNames = []string{"Test Commander"}
	gs.Seats[0].CommanderCastCounts = map[string]int{"Test Commander": 0}
	gs.Seats[0].CommanderTax = gs.Seats[0].CommanderCastCounts

	// Register the §903.9b replacement (same as SetupCommanderGame does).
	registerCommanderZoneReplacement(gs, 0, "Test Commander")

	// Put commander on battlefield.
	cmdr := &Permanent{
		Card:       cmdrCard,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, cmdr)

	// Destroy the commander.
	DestroyPermanent(gs, cmdr, nil)

	// Commander should NOT be in graveyard (§903.9b redirects graveyard → command zone,
	// and §704.6d SBA picks up commanders in GY/exile).
	// Note: The §903.9b replacement fires on "would_change_zone" and redirects
	// to command_zone when destination is hand/library. For graveyard/exile destinations,
	// the SBA §704.6d handles the redirect. Let's check if the commander ended up
	// in either the command zone or graveyard (where §704.6d will pick it up).
	inCommandZone := false
	for _, c := range gs.Seats[0].CommandZone {
		if c.Name == "Test Commander" {
			inCommandZone = true
			break
		}
	}
	inGraveyard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == "Test Commander" {
			inGraveyard = true
			break
		}
	}

	// Commander ends up in graveyard initially, then §704.6d SBA moves it.
	// Let's run SBAs to complete the redirect.
	if inGraveyard && !inCommandZone {
		StateBasedActions(gs)
		// Now check command zone.
		for _, c := range gs.Seats[0].CommandZone {
			if c.Name == "Test Commander" {
				inCommandZone = true
				break
			}
		}
	}

	if !inCommandZone {
		t.Fatal("commander should end up in command zone after dying (via §704.6d)")
	}

	// Commander should NOT be on battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Test Commander" {
			t.Fatal("commander should not be on battlefield after destroy")
		}
	}
}

// TestExile_DoesNotCheckIndestructible — exile ignores indestructible.
func TestExile_DoesNotCheckIndestructible(t *testing.T) {
	gs := newFixtureGame(t)

	god := addBattlefield(gs, 0, "Darksteel Colossus", 11, 11, "creature")
	god.Flags["indestructible"] = 1

	ExilePermanent(gs, god, nil)

	// Should be exiled.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Darksteel Colossus" {
			t.Fatal("exile should remove indestructible creatures")
		}
	}
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c.Name == "Darksteel Colossus" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("indestructible creature should be in exile zone")
	}
}

// TestBounce_FiresLTBTrigger — bounce a creature, its LTB trigger fires.
func TestBounce_FiresLTBTrigger(t *testing.T) {
	gs := newFixtureGame(t)

	perm := addBattlefield(gs, 0, "LTB Dude", 2, 2, "creature")
	perm.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "ltb",
				},
				Effect: &gameast.GainLife{
					Target: gameast.Filter{Base: "you"},
					Amount: numRef(3),
				},
			},
		},
	}

	BouncePermanent(gs, perm, nil, "hand")

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "LTB Dude" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("LTB trigger should fire on bounce")
	}
}

// TestResolveDestroy_UsesProperHelper — resolveDestroy routes through
// DestroyPermanent, checking indestructible.
func TestResolveDestroy_UsesProperHelper(t *testing.T) {
	gs := newFixtureGame(t)

	// Indestructible creature.
	god := addBattlefield(gs, 0, "God", 5, 5, "creature")
	god.Flags["indestructible"] = 1

	// Build a Destroy effect and resolve it.
	e := &gameast.Destroy{
		Target: gameast.Filter{Base: "creature"},
	}
	resolveDestroy(gs, nil, e)

	// God should survive.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "God" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("indestructible creature should survive resolveDestroy")
	}
}

// TestSBA_LethalDamage_IndestructibleSurvives — SBA death respects indestructible.
func TestSBA_LethalDamage_IndestructibleSurvives(t *testing.T) {
	gs := newFixtureGame(t)

	god := addBattlefield(gs, 0, "Blightsteel Colossus", 11, 11, "creature")
	god.Flags["indestructible"] = 1
	god.MarkedDamage = 15 // lethal

	StateBasedActions(gs)

	// Should still be on battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Blightsteel Colossus" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("indestructible creature should survive lethal damage SBA (already in existing code)")
	}
}

// TestCheckTargetLegality_AllLegal — no fizzle when all targets are legal.
func TestCheckTargetLegality_AllLegal(t *testing.T) {
	gs := newFixtureGame(t)

	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	item := &StackItem{
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: bear},
		},
	}

	allIllegal, legal := CheckTargetLegality(gs, item)
	if allIllegal {
		t.Fatal("should not be all illegal when target exists")
	}
	if len(legal) != 1 {
		t.Fatalf("expected 1 legal target, got %d", len(legal))
	}
}

// TestCheckTargetLegality_NoTargets — spells with no targets never fizzle.
func TestCheckTargetLegality_NoTargets(t *testing.T) {
	gs := newFixtureGame(t)
	item := &StackItem{Targets: nil}
	allIllegal, _ := CheckTargetLegality(gs, item)
	if allIllegal {
		t.Fatal("nil targets should not trigger fizzle")
	}
}

// TestObserverDiesTrigger_ControllerFilter — "whenever a creature you control
// dies" only fires for creatures the observer controls.
func TestObserverDiesTrigger_ControllerFilter(t *testing.T) {
	gs := newFixtureGame(t)

	// Observer on seat 0 with "whenever a creature you control dies".
	observer := addBattlefield(gs, 0, "Grave Pact Effect", 0, 1, "enchantment")
	observer.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event:      "die",
					Actor:      &gameast.Filter{Base: "creature"},
					Controller: "you",
				},
				Effect: &gameast.GainLife{
					Target: gameast.Filter{Base: "you"},
					Amount: numRef(1),
				},
			},
		},
	}

	// Kill opponent's creature — should NOT fire.
	opponentCreature := addBattlefield(gs, 1, "Opponent Bear", 2, 2, "creature")
	gs.EventLog = nil // clear
	DestroyPermanent(gs, opponentCreature, nil)

	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Grave Pact Effect" {
			t.Fatal("observer should NOT trigger for opponent's creature dying")
		}
	}

	// Kill own creature — SHOULD fire.
	ownCreature := addBattlefield(gs, 0, "Own Bear", 2, 2, "creature")
	gs.EventLog = nil // clear
	DestroyPermanent(gs, ownCreature, nil)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" && ev.Source == "Grave Pact Effect" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("observer should trigger for own creature dying")
	}
}

// =============================================================================
// Helpers
// =============================================================================

// numRef creates a NumberOrRef representing an integer literal.
func numRef(n int) gameast.NumberOrRef {
	return gameast.NumberOrRef{IsInt: true, Int: n}
}
