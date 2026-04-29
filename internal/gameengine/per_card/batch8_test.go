package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Sacrifice Outlets
// ============================================================================

func TestAshnodsAltar_SacCreatureAdd2Mana(t *testing.T) {
	gs := newGame(t, 2)
	altar := addPerm(gs, 0, "Ashnod's Altar", "artifact")
	victim := addPerm(gs, 0, "Llanowar Elves", "creature")
	startMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, altar, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	if gs.Seats[0].ManaPool != startMana+2 {
		t.Errorf("expected mana pool %d, got %d", startMana+2, gs.Seats[0].ManaPool)
	}
	// Victim should be gone from battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestPhyrexianAltar_SacCreatureAdd1Mana(t *testing.T) {
	gs := newGame(t, 2)
	altar := addPerm(gs, 0, "Phyrexian Altar", "artifact")
	victim := addPerm(gs, 0, "Birds of Paradise", "creature")
	startMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, altar, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	if gs.Seats[0].ManaPool != startMana+1 {
		t.Errorf("expected mana pool %d, got %d", startMana+1, gs.Seats[0].ManaPool)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestVisceraSeer_SacCreatureScry1(t *testing.T) {
	gs := newGame(t, 2)
	seer := addPerm(gs, 0, "Viscera Seer", "creature")
	victim := addPerm(gs, 0, "Llanowar Elves", "creature")
	addLibrary(gs, 0, "A", "B")

	gameengine.InvokeActivatedHook(gs, seer, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	// Victim should be sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
	// Scry event should have fired.
	if hasEvent(gs, "scry") < 1 {
		t.Error("expected scry event from Viscera Seer")
	}
}

func TestCarrionFeeder_SacCreatureAddCounter(t *testing.T) {
	gs := newGame(t, 2)
	feeder := addPerm(gs, 0, "Carrion Feeder", "creature")
	victim := addPerm(gs, 0, "Llanowar Elves", "creature")

	gameengine.InvokeActivatedHook(gs, feeder, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	if feeder.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter on Carrion Feeder, got %d", feeder.Counters["+1/+1"])
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestAltarOfDementia_SacCreatureMillTarget(t *testing.T) {
	gs := newGame(t, 2)
	altar := addPerm(gs, 0, "Altar of Dementia", "artifact")
	// Creature with power 3.
	victim := addPerm(gs, 0, "Grizzly Bears", "creature")
	victim.Card.BasePower = 3
	addLibrary(gs, 1, "A", "B", "C", "D", "E")

	gameengine.InvokeActivatedHook(gs, altar, 0, map[string]interface{}{
		"creature_perm": victim,
		"target_seat":   1,
	})

	// Should mill 3 from opponent.
	if len(gs.Seats[1].Library) != 2 {
		t.Errorf("expected 2 cards in opponent's library after milling 3, got %d", len(gs.Seats[1].Library))
	}
	if len(gs.Seats[1].Graveyard) != 3 {
		t.Errorf("expected 3 cards in graveyard, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestGoblinBombardment_SacCreatureDeal1Damage(t *testing.T) {
	gs := newGame(t, 2)
	bombardment := addPerm(gs, 0, "Goblin Bombardment", "enchantment")
	victim := addPerm(gs, 0, "Goblin Token", "creature")
	gs.Seats[1].Life = 20

	gameengine.InvokeActivatedHook(gs, bombardment, 0, map[string]interface{}{
		"creature_perm": victim,
		"target_seat":   1,
	})

	if gs.Seats[1].Life != 19 {
		t.Errorf("expected opponent life 19, got %d", gs.Seats[1].Life)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestYahenni_SacOtherCreatureGainIndestructible(t *testing.T) {
	gs := newGame(t, 2)
	yahenni := addPerm(gs, 0, "Yahenni, Undying Partisan", "creature")
	victim := addPerm(gs, 0, "Llanowar Elves", "creature")

	gameengine.InvokeActivatedHook(gs, yahenni, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	// Yahenni should have indestructible.
	if yahenni.Flags["indestructible"] != 1 {
		t.Error("expected Yahenni to have indestructible flag")
	}
	// Victim should be gone.
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestYahenni_CantSacSelf(t *testing.T) {
	gs := newGame(t, 2)
	yahenni := addPerm(gs, 0, "Yahenni, Undying Partisan", "creature")
	// No other creatures on battlefield.

	gameengine.InvokeActivatedHook(gs, yahenni, 0, nil)

	// Should fail gracefully — no creature to sacrifice.
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when no other creature to sacrifice")
	}
}

func TestWoeStrider_ETBCreatesGoatToken(t *testing.T) {
	gs := newGame(t, 2)
	strider := addPerm(gs, 0, "Woe Strider", "creature")

	gameengine.InvokeETBHook(gs, strider)

	// Should have a Goat token on the battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Goat" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Goat token on battlefield after Woe Strider ETB")
	}
}

func TestWoeStrider_SacOtherCreatureScry1(t *testing.T) {
	gs := newGame(t, 2)
	strider := addPerm(gs, 0, "Woe Strider", "creature")
	victim := addPerm(gs, 0, "Goblin Token", "creature")
	addLibrary(gs, 0, "A", "B")

	gameengine.InvokeActivatedHook(gs, strider, 0, map[string]interface{}{
		"creature_perm": victim,
	})

	if hasEvent(gs, "scry") < 1 {
		t.Error("expected scry event from Woe Strider")
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p == victim {
			t.Error("victim should have been sacrificed")
		}
	}
}

func TestSacOutlet_NoCreatureFailsGracefully(t *testing.T) {
	gs := newGame(t, 2)
	altar := addPerm(gs, 0, "Ashnod's Altar", "artifact")
	// No creatures on battlefield.

	gameengine.InvokeActivatedHook(gs, altar, 0, nil)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when no creature to sacrifice")
	}
}

// ============================================================================
// Board Wipes
// ============================================================================

func TestWrathOfGod_DestroysAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Serra Angel", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact") // should survive
	addPerm(gs, 1, "Tarmogoyf", "creature")
	addPerm(gs, 1, "Sword of Fire and Ice", "artifact", "equipment") // should survive

	card := addCard(gs, 0, "Wrath of God", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All creatures gone, artifacts remain.
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				t.Errorf("creature %s should have been destroyed", p.Card.DisplayName())
			}
		}
	}
	// Artifacts should remain.
	artifactCount := 0
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.IsArtifact() {
				artifactCount++
			}
		}
	}
	if artifactCount != 2 {
		t.Errorf("expected 2 artifacts to survive Wrath, got %d", artifactCount)
	}
}

func TestDamnation_DestroysAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addPerm(gs, 1, "Llanowar Elves", "creature")

	card := addCard(gs, 0, "Damnation", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				t.Errorf("creature %s should have been destroyed", p.Card.DisplayName())
			}
		}
	}
}

func TestWrathOfGod_IndestructibleSurvives(t *testing.T) {
	gs := newGame(t, 2)
	god := addPerm(gs, 0, "Darksteel Colossus", "creature", "artifact")
	god.Flags["indestructible"] = 1
	addPerm(gs, 1, "Llanowar Elves", "creature")

	card := addCard(gs, 0, "Wrath of God", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Indestructible creature should survive.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == god {
			found = true
		}
	}
	if !found {
		t.Error("indestructible creature should survive Wrath of God")
	}
	// Non-indestructible creature should be destroyed.
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsCreature() {
			t.Error("non-indestructible creature should be destroyed")
		}
	}
}

func TestToxicDeluge_KillsThroughIndestructible(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40

	god := addPerm(gs, 0, "Darksteel Colossus", "creature")
	god.Flags["indestructible"] = 1
	god.Card.BaseToughness = 5
	elf := addPerm(gs, 1, "Llanowar Elves", "creature")
	elf.Card.BaseToughness = 1

	card := addCard(gs, 0, "Toxic Deluge", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card, ChosenX: 5}
	gameengine.InvokeResolveHook(gs, item)

	// Life should be reduced by 5.
	if gs.Seats[0].Life != 35 {
		t.Errorf("expected life 35 after paying 5, got %d", gs.Seats[0].Life)
	}

	// Both creatures should have -5/-5 modification.
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				// Check that modifications were applied.
				hasBuff := false
				for _, m := range p.Modifications {
					if m.Toughness == -5 {
						hasBuff = true
					}
				}
				if !hasBuff {
					t.Errorf("creature %s should have -5/-5 modification", p.Card.DisplayName())
				}
			}
		}
	}
}

func TestBlasphemousAct_Deals13DamageToEachCreature(t *testing.T) {
	gs := newGame(t, 2)
	angel := addPerm(gs, 0, "Serra Angel", "creature")
	angel.Card.BaseToughness = 4
	elf := addPerm(gs, 1, "Llanowar Elves", "creature")
	elf.Card.BaseToughness = 1

	card := addCard(gs, 0, "Blasphemous Act", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Both should have 13 marked damage.
	if angel.MarkedDamage != 13 {
		t.Errorf("expected 13 damage on Serra Angel, got %d", angel.MarkedDamage)
	}
	if elf.MarkedDamage != 13 {
		t.Errorf("expected 13 damage on Llanowar Elves, got %d", elf.MarkedDamage)
	}
}

func TestFarewell_ExilesAllTypesAndGraveyards(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addPerm(gs, 1, "Rhystic Study", "enchantment")
	// Land should survive.
	addPerm(gs, 1, "Island", "land")
	// Put some cards in graveyards.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "A", Owner: 0}, {Name: "B", Owner: 0},
	}
	gs.Seats[1].Graveyard = []*gameengine.Card{
		{Name: "C", Owner: 1},
	}

	card := addCard(gs, 0, "Farewell", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// All artifacts, creatures, enchantments should be gone.
	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if p.IsCreature() || p.IsArtifact() || p.IsEnchantment() {
				t.Errorf("%s should have been exiled by Farewell", p.Card.DisplayName())
			}
		}
	}
	// Land should survive.
	landFound := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsLand() {
			landFound = true
		}
	}
	if !landFound {
		t.Error("Island should survive Farewell")
	}
	// Graveyards should be empty (exiled).
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("seat 0 graveyard should be empty, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("seat 1 graveyard should be empty, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestAustereCommand_DestroysAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Serra Angel", "creature")
	addPerm(gs, 1, "Tarmogoyf", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact") // should survive (MVP picks creature modes)

	card := addCard(gs, 0, "Austere Command", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, s := range gs.Seats {
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				t.Errorf("creature %s should have been destroyed", p.Card.DisplayName())
			}
		}
	}
}

// ============================================================================
// Counterspells
// ============================================================================

func TestNegate_CountersNoncreatureSpell(t *testing.T) {
	gs := newGame(t, 2)
	// Put an opponent's noncreature spell on the stack.
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Demonic Tutor", Owner: 1, Types: []string{"sorcery"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Negate", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Error("Negate should have countered the sorcery")
	}
}

func TestNegate_DoesNotCounterCreatureSpell(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Tarmogoyf", Owner: 1, Types: []string{"creature"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Negate", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if oppSpell.Countered {
		t.Error("Negate should NOT counter creature spells")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed for no valid target")
	}
}

func TestSwanSong_CountersInstantAndCreatesBird(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Counterspell", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Swan Song", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Error("Swan Song should have countered the instant")
	}
	// Opponent should have a 2/2 Bird token.
	birdFound := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Bird" {
			birdFound = true
			if p.Card.BasePower != 2 || p.Card.BaseToughness != 2 {
				t.Errorf("expected 2/2 Bird, got %d/%d", p.Card.BasePower, p.Card.BaseToughness)
			}
		}
	}
	if !birdFound {
		t.Error("expected Bird token on opponent's battlefield")
	}
}

func TestDispel_OnlyCountersInstants(t *testing.T) {
	gs := newGame(t, 2)
	sorcery := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Demonic Tutor", Owner: 1, Types: []string{"sorcery"}},
	}
	gs.Stack = append(gs.Stack, sorcery)

	card := addCard(gs, 0, "Dispel", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if sorcery.Countered {
		t.Error("Dispel should NOT counter sorceries")
	}
}

func TestManaDrain_CountersAndGrantsMana(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Tooth and Nail",
			Owner: 1,
			Types: []string{"sorcery", "cost:7"},
			CMC:   7,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Mana Drain", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Error("Mana Drain should have countered the spell")
	}
	// Should have registered a delayed trigger for mana.
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected delayed trigger for Mana Drain mana gain")
	}
}

func TestArcaneDenial_CountersAndRegistersDelayedDraws(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Lightning Bolt", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Arcane Denial", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Error("Arcane Denial should have countered the spell")
	}
	// Should have registered 2 delayed triggers (opponent draws 2, caster draws 1).
	if len(gs.DelayedTriggers) < 2 {
		t.Errorf("expected 2 delayed triggers, got %d", len(gs.DelayedTriggers))
	}
}

func TestDovinsVeto_CountersNoncreatureAndCantBeCountered(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: "Cyclonic Rift", Owner: 1, Types: []string{"instant"}},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Dovin's Veto", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	// Fire the cast hook first (sets cannot_be_countered).
	gameengine.InvokeCastHook(gs, item)
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Error("Dovin's Veto should have countered the noncreature spell")
	}
	// Verify the "can't be countered" flag was set.
	if item.CostMeta == nil || item.CostMeta["cannot_be_countered"] != true {
		t.Error("expected cannot_be_countered flag on Dovin's Veto")
	}
}

func TestCounterspell_EmptyStackFails(t *testing.T) {
	gs := newGame(t, 2)
	// Stack is empty.

	card := addCard(gs, 0, "Negate", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Error("expected per_card_failed when stack is empty")
	}
}

func TestCounterspell_DoesNotCounterOwnSpell(t *testing.T) {
	gs := newGame(t, 2)
	// Only caster's own spell on stack.
	ownSpell := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Ponder", Owner: 0, Types: []string{"sorcery"}},
	}
	gs.Stack = append(gs.Stack, ownSpell)

	card := addCard(gs, 0, "Negate", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if ownSpell.Countered {
		t.Error("should NOT counter own spells")
	}
}

// ============================================================================
// Cantrips
// ============================================================================

func TestBrainstorm_Draw3PutBack2(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Existing1", Owner: 0},
	}

	card := addCard(gs, 0, "Brainstorm", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Started with 1 in hand, drew 3 (= 4), put back 2 (= 2).
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards in hand after Brainstorm, got %d", len(gs.Seats[0].Hand))
	}
	// Library: started with 5, drew 3 (= 2), put back 2 (= 4).
	if len(gs.Seats[0].Library) != 4 {
		t.Errorf("expected 4 cards in library after Brainstorm, got %d", len(gs.Seats[0].Library))
	}
}

func TestPonder_Scry3Draw1(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	card := addCard(gs, 0, "Ponder", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Should have drawn 1 card.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after Ponder, got %d", len(gs.Seats[0].Hand))
	}
	// Scry event should have fired.
	if hasEvent(gs, "scry") < 1 {
		t.Error("expected scry event from Ponder")
	}
}

func TestPreordain_Scry2Draw1(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	addLibrary(gs, 0, "A", "B", "C", "D")

	card := addCard(gs, 0, "Preordain", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after Preordain, got %d", len(gs.Seats[0].Hand))
	}
	if hasEvent(gs, "scry") < 1 {
		t.Error("expected scry event from Preordain")
	}
}

func TestOpt_Scry1Draw1(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	addLibrary(gs, 0, "A", "B")

	card := addCard(gs, 0, "Opt", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after Opt, got %d", len(gs.Seats[0].Hand))
	}
}

func TestConsider_Surveil1Draw1(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	addLibrary(gs, 0, "A", "B", "C")

	card := addCard(gs, 0, "Consider", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand after Consider, got %d", len(gs.Seats[0].Hand))
	}
	if hasEvent(gs, "surveil") < 1 {
		t.Error("expected surveil event from Consider")
	}
}

func TestGitaxianProbe_RevealsHandAndDraws(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "Secret Card", Owner: 1},
	}
	addLibrary(gs, 0, "A", "B")

	card := addCard(gs, 0, "Gitaxian Probe", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Should have drawn 1 card.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
	// Should have a look_at_hand event.
	if hasEvent(gs, "look_at_hand") < 1 {
		t.Error("expected look_at_hand event from Gitaxian Probe")
	}
}

func TestBrainstorm_EmptyLibraryStillWorks(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hat = &testHat{}
	// Empty library. Should draw 0 and put back 0.
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "X", Owner: 0},
		{Name: "Y", Owner: 0},
	}

	card := addCard(gs, 0, "Brainstorm", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// No cards drawn, but still puts back 2 from hand.
	// Actually: drew 0, so hand is still 2, then puts back 2 = 0 in hand.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected 0 cards in hand (put back 2 with no draw), got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected 2 cards on top of library, got %d", len(gs.Seats[0].Library))
	}
}

// ============================================================================
// Registry check — batch #8 cards registered
// ============================================================================

func TestBatch8_AllCardsRegistered(t *testing.T) {
	sacOutlets := []string{
		"Ashnod's Altar", "Phyrexian Altar", "Viscera Seer",
		"Carrion Feeder", "Altar of Dementia", "Goblin Bombardment",
	}
	for _, name := range sacOutlets {
		if !HasActivated(name) {
			t.Errorf("expected Activated handler for %s", name)
		}
	}

	boardWipes := []string{
		"Wrath of God", "Damnation", "Toxic Deluge",
		"Blasphemous Act", "Vanquish the Horde", "Farewell",
		"Austere Command",
	}
	for _, name := range boardWipes {
		if !HasResolve(name) {
			t.Errorf("expected Resolve handler for %s", name)
		}
	}

	counters := []string{
		"Negate", "Swan Song", "Dovin's Veto",
		"Arcane Denial", "Dispel", "Mana Drain",
	}
	for _, name := range counters {
		if !HasResolve(name) {
			t.Errorf("expected Resolve handler for %s", name)
		}
	}

	cantrips := []string{
		"Brainstorm", "Ponder", "Preordain",
		"Gitaxian Probe", "Opt", "Consider",
	}
	for _, name := range cantrips {
		if !HasResolve(name) {
			t.Errorf("expected Resolve handler for %s", name)
		}
	}

	// ETB handlers.
	etbCards := []string{"Woe Strider"}
	for _, name := range etbCards {
		if !HasETB(name) {
			t.Errorf("expected ETB handler for %s", name)
		}
	}

	// Yahenni has both activated and trigger.
	if !HasActivated("Yahenni, Undying Partisan") {
		t.Error("expected Activated handler for Yahenni")
	}
}

// ============================================================================
// Test Hat — minimal hat for cantrip tests
// ============================================================================

// testHat satisfies gameengine.Hat with minimal implementations
// needed for cantrip tests. Delegates scry/surveil decisions.
type testHat struct{}

func (*testHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	return false
}
func (*testHat) ChooseLandToPlay(gs *gameengine.GameState, seatIdx int, lands []*gameengine.Card) *gameengine.Card {
	return nil
}
func (*testHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	return nil
}
func (*testHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	return nil
}
func (*testHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	return nil
}
func (*testHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) > 0 {
		return legalDefenders[0]
	}
	return 0
}
func (*testHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	return map[*gameengine.Permanent][]*gameengine.Permanent{}
}
func (*testHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, stackTop *gameengine.StackItem) *gameengine.StackItem {
	return nil
}
func (*testHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	if len(legal) > 0 {
		return legal[0]
	}
	return gameengine.Target{Kind: gameengine.TargetKindNone}
}
func (*testHat) ChooseMode(gs *gameengine.GameState, seatIdx int, modes []gameast.Effect) int {
	return 0
}
func (*testHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	return true
}
func (*testHat) ShouldRedirectCommanderZone(gs *gameengine.GameState, seatIdx int, commander *gameengine.Card, to string) bool {
	return true
}
func (*testHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	return candidates
}
func (*testHat) ChooseDiscard(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, n int) []*gameengine.Card {
	return nil
}
func (*testHat) OrderTriggers(gs *gameengine.GameState, seatIdx int, triggers []*gameengine.StackItem) []*gameengine.StackItem {
	return triggers
}
func (*testHat) ChooseX(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, availableMana int) int {
	return availableMana
}
func (*testHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	return nil
}
func (*testHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (top []*gameengine.Card, bottom []*gameengine.Card) {
	return cards, nil
}
func (*testHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (graveyard []*gameengine.Card, top []*gameengine.Card) {
	return nil, cards
}
func (*testHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	if count > len(hand) {
		count = len(hand)
	}
	// Put back the first `count` cards from hand.
	return hand[:count]
}
func (*testHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {}
func (*testHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool { return false }
