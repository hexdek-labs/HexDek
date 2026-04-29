package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// =============================================================================
// PARTIAL #1: Mirage Mirror — EOT copy-effect revert
//
// Mirage Mirror's activation uses CopyPermanentLayered with
// DurationEndOfTurn. At cleanup, ScanExpiredDurations removes the
// layer-1 copy effect, and the mirror reverts to its printed
// characteristics (a non-creature artifact).
// =============================================================================

func TestMirageMirror_CopiesCreatureThenRevertsAtCleanup(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 10

	// Mirage Mirror is a non-creature artifact.
	mirror := addPerm(gs, 0, "Mirage Mirror", "artifact")

	// Target: a big creature.
	dragon := addPerm(gs, 1, "Shivan Dragon", "creature")
	dragon.Card.BasePower = 5
	dragon.Card.BaseToughness = 5

	// Activate Mirage Mirror.
	gameengine.InvokeActivatedHook(gs, mirror, 0, nil)

	// After activation, mirror should have copied the dragon's
	// characteristics via the layer-1 copy.
	chars := gameengine.GetEffectiveCharacteristics(gs, mirror)
	if chars.Name != "Shivan Dragon" {
		t.Errorf("Mirage Mirror should copy name; got %q", chars.Name)
	}
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("Mirage Mirror should copy P/T; got %d/%d", chars.Power, chars.Toughness)
	}

	// Simulate cleanup step — expire EOT effects.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	gs.InvalidateCharacteristicsCache()

	// After cleanup, mirror should revert to its printed characteristics.
	chars2 := gameengine.GetEffectiveCharacteristics(gs, mirror)
	if chars2.Name != "Mirage Mirror" {
		t.Errorf("After cleanup, Mirage Mirror should revert to original name; got %q", chars2.Name)
	}
	// The mirror's printed card has 0/0 or no P/T (it's a non-creature artifact).
	// Key test: it should NOT still be 5/5.
	if chars2.Power == 5 && chars2.Toughness == 5 {
		t.Errorf("After cleanup, Mirage Mirror should NOT retain dragon's 5/5 P/T")
	}
}

func TestMirageMirror_RevertsToNonCreature(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].ManaPool = 10

	mirror := addPerm(gs, 0, "Mirage Mirror", "artifact")
	target := addPerm(gs, 1, "Serra Angel", "creature")
	target.Card.BasePower = 4
	target.Card.BaseToughness = 4

	gameengine.InvokeActivatedHook(gs, mirror, 0, nil)

	// During the turn, mirror is a copy of Serra Angel (a creature).
	chars := gameengine.GetEffectiveCharacteristics(gs, mirror)
	hasCreature := false
	for _, tp := range chars.Types {
		if tp == "creature" {
			hasCreature = true
			break
		}
	}
	if !hasCreature {
		t.Errorf("Mirage Mirror as copy of Serra Angel should be a creature; types=%v", chars.Types)
	}

	// At cleanup, revert.
	gameengine.ScanExpiredDurations(gs, "ending", "cleanup")
	gs.InvalidateCharacteristicsCache()

	chars2 := gameengine.GetEffectiveCharacteristics(gs, mirror)
	hasCreature2 := false
	for _, tp := range chars2.Types {
		if tp == "creature" {
			hasCreature2 = true
			break
		}
	}
	if hasCreature2 {
		t.Errorf("After cleanup, Mirage Mirror should NOT be a creature; types=%v", chars2.Types)
	}

	// It should still be an artifact.
	hasArtifact := false
	for _, tp := range chars2.Types {
		if tp == "artifact" {
			hasArtifact = true
			break
		}
	}
	if !hasArtifact {
		t.Errorf("After cleanup, Mirage Mirror should still be an artifact; types=%v", chars2.Types)
	}
}

// =============================================================================
// PARTIAL #2: Release to the Wind — cast-from-exile via ZoneCastPermission
//
// When Release to the Wind exiles a permanent, it registers a
// ZoneCastPermission granting the card's owner the ability to cast it
// from exile for free (ManaCost = 0).
// =============================================================================

func TestReleaseToTheWind_RegistersZoneCastGrant(t *testing.T) {
	gs := newGame(t, 2)

	// Opponent has a creature.
	creature := addPerm(gs, 1, "Serra Angel", "creature")
	creature.Card.Owner = 1

	// Cast Release to the Wind.
	card := addCard(gs, 0, "Release to the Wind", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// The creature's card should now be in exile.
	found := false
	var exiledCard *gameengine.Card
	for _, c := range gs.Seats[1].Exile {
		if c.DisplayName() == "Serra Angel" {
			found = true
			exiledCard = c
			break
		}
	}
	if !found {
		t.Fatal("Serra Angel should be in exile after Release to the Wind")
	}

	// A ZoneCastGrant should be registered for the exiled card.
	grant := gameengine.GetZoneCastGrant(gs, exiledCard)
	if grant == nil {
		t.Fatal("ZoneCastGrant should be registered for the exiled card")
	}
	if grant.Zone != "exile" {
		t.Errorf("grant zone should be 'exile', got %q", grant.Zone)
	}
	if grant.ManaCost != 0 {
		t.Errorf("grant mana cost should be 0 (free cast), got %d", grant.ManaCost)
	}
	if grant.RequireController != 1 {
		t.Errorf("grant should be restricted to seat 1 (owner), got %d", grant.RequireController)
	}
	if grant.SourceName != "Release to the Wind" {
		t.Errorf("grant source name should be 'Release to the Wind', got %q", grant.SourceName)
	}

	// Verify the zone_cast_grant_registered event was logged.
	foundEvent := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "zone_cast_grant_registered" {
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		t.Error("expected zone_cast_grant_registered event")
	}
}

func TestReleaseToTheWind_CanCastFromExileForFree(t *testing.T) {
	gs := newGame(t, 2)

	// Opponent has a creature.
	creature := addPerm(gs, 1, "Serra Angel", "creature")
	creature.Card.Owner = 1
	creature.Card.CMC = 5
	creature.Card.BasePower = 4
	creature.Card.BaseToughness = 4

	// Cast Release to the Wind targeting the creature.
	card := addCard(gs, 0, "Release to the Wind", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Get the exiled card.
	var exiledCard *gameengine.Card
	for _, c := range gs.Seats[1].Exile {
		if c.DisplayName() == "Serra Angel" {
			exiledCard = c
			break
		}
	}
	if exiledCard == nil {
		t.Fatal("Serra Angel should be in exile")
	}

	// The owner (seat 1) should be able to cast it from exile for free.
	grant := gameengine.GetZoneCastGrant(gs, exiledCard)
	if grant == nil {
		t.Fatal("ZoneCastGrant should exist")
	}

	// Verify the permission allows casting from exile.
	perms := []*gameengine.ZoneCastPermission{grant}
	found := gameengine.CanCastFromZone(gs, 1, exiledCard, "exile", perms)
	if found == nil {
		t.Error("owner should be able to cast from exile with the grant (0 mana)")
	}

	// Seat 0 should NOT be able to use this grant (RequireController = 1).
	found2 := gameengine.CanCastFromZone(gs, 0, exiledCard, "exile", perms)
	if found2 != nil {
		t.Error("non-owner should NOT be able to cast with this grant")
	}
}

// =============================================================================
// PARTIAL #3: Ad-hoc delayed trigger framework expansion
//
// Verify that ALL per-card handlers that register delayed triggers
// use the general gs.RegisterDelayedTrigger framework (not ad-hoc
// logic). This is verified by checking that the DelayedTriggers
// slice is populated after each handler fires.
// =============================================================================

func TestDelayedTriggerFramework_SneakAttack_UsesGeneralFramework(t *testing.T) {
	gs := newGame(t, 2)
	sneakAttack := addPerm(gs, 0, "Sneak Attack", "creature")
	_ = sneakAttack

	// Add a creature to hand.
	creature := &gameengine.Card{
		Name:          "Griselbrand",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     7,
		BaseToughness: 7,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, creature)

	gameengine.InvokeActivatedHook(gs, sneakAttack, 0, nil)

	// Should have registered a delayed trigger via the general framework.
	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("Sneak Attack should register a delayed trigger via RegisterDelayedTrigger")
	}
	dt := gs.DelayedTriggers[0]
	if dt.TriggerAt != "next_end_step" {
		t.Errorf("Sneak Attack delayed trigger should fire at 'next_end_step', got %q", dt.TriggerAt)
	}
	if dt.SourceCardName != "Sneak Attack" {
		t.Errorf("source should be 'Sneak Attack', got %q", dt.SourceCardName)
	}
}

func TestDelayedTriggerFramework_PactOfNegation_UsesGeneralFramework(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Pact of Negation", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	gameengine.InvokeCastHook(gs, item)

	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("Pact of Negation should register a delayed trigger via RegisterDelayedTrigger")
	}
	dt := gs.DelayedTriggers[0]
	if dt.TriggerAt != "your_next_upkeep" {
		t.Errorf("Pact delayed trigger should fire at 'your_next_upkeep', got %q", dt.TriggerAt)
	}
}

func TestDelayedTriggerFramework_AllPacts_UseGeneralFramework(t *testing.T) {
	pacts := []struct {
		name      string
		triggerAt string
	}{
		{"Pact of the Titan", "your_next_upkeep"},
		{"Slaughter Pact", "your_next_upkeep"},
		{"Intervention Pact", "your_next_upkeep"},
		{"Summoner's Pact", "your_next_upkeep"},
	}
	for _, pact := range pacts {
		t.Run(pact.name, func(t *testing.T) {
			gs := newGame(t, 2)
			// Add a target for Slaughter Pact.
			if pact.name == "Slaughter Pact" {
				addPerm(gs, 1, "Serra Angel", "creature")
			}
			// Add a green creature for Summoner's Pact.
			if pact.name == "Summoner's Pact" {
				c := &gameengine.Card{
					Name:   "Craterhoof Behemoth",
					Owner:  0,
					Types:  []string{"creature"},
					Colors: []string{"G"},
				}
				gs.Seats[0].Library = append(gs.Seats[0].Library, c)
			}

			card := addCard(gs, 0, pact.name, "instant")
			item := &gameengine.StackItem{Controller: 0, Card: card}
			gameengine.InvokeCastHook(gs, item)

			if len(gs.DelayedTriggers) == 0 {
				t.Fatalf("%s should register a delayed trigger", pact.name)
			}
			dt := gs.DelayedTriggers[0]
			if dt.TriggerAt != pact.triggerAt {
				t.Errorf("%s trigger should fire at %q, got %q", pact.name, pact.triggerAt, dt.TriggerAt)
			}
			if dt.SourceCardName != pact.name {
				t.Errorf("source should be %q, got %q", pact.name, dt.SourceCardName)
			}
		})
	}
}

// =============================================================================
// PARTIAL #4: Delayed trigger condition evaluation at fire time
//
// Verify that EffectFn closures read CURRENT game state (the *GameState
// pointer passed at fire time), not a stale snapshot from registration.
// =============================================================================

func TestDelayedTrigger_ConditionEvaluatesAtFireTime_NotRegistrationTime(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	// Register a delayed trigger whose EffectFn reads the CURRENT
	// gs.Seats[0].Life at fire time.
	var lifeCaptured int
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: 0,
		SourceCardName: "Test Pact",
		EffectFn: func(gs *gameengine.GameState) {
			// This reads the CURRENT game state, not a stale copy.
			lifeCaptured = gs.Seats[0].Life
		},
	})

	// At registration time, life is 20.
	if gs.Seats[0].Life != 20 {
		t.Fatalf("expected life 20 at registration, got %d", gs.Seats[0].Life)
	}

	// Now change life before firing.
	gs.Seats[0].Life = 5

	// Advance turn and fire at upkeep.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")

	// The closure should have captured life=5 (current state), not 20.
	if lifeCaptured != 5 {
		t.Errorf("EffectFn should read current state at fire time; captured life=%d, want 5", lifeCaptured)
	}
}

func TestDelayedTrigger_PactPayment_UsesCurrentMana(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	// Cast Pact of Negation — registers delayed trigger.
	card := addCard(gs, 0, "Pact of Negation", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// At registration: seat 0 has 0 mana (can't pay).
	// Now give seat 0 enough mana BEFORE firing.
	gs.Seats[0].ManaPool = 10

	// Advance to next upkeep and fire.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")

	// Seat 0 should have PAID (not lost), because EffectFn reads
	// current mana pool (10) at fire time.
	if gs.Seats[0].Lost {
		t.Errorf("Seat 0 should have paid the pact (had 10 mana); got Lost=true, reason=%q",
			gs.Seats[0].LossReason)
	}
	// Mana should be 5 (paid 5 from 10).
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("expected mana 5 after paying 3UU=5, got %d", gs.Seats[0].ManaPool)
	}
}

func TestDelayedTrigger_PactLose_WhenInsufficientMana(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	card := addCard(gs, 0, "Pact of Negation", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Don't give any mana.
	gs.Seats[0].ManaPool = 0

	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")

	if !gs.Seats[0].Lost {
		t.Error("Seat 0 should lose when unable to pay pact cost")
	}
}

// =============================================================================
// PARTIAL #5: Pact cycle beyond Pact of Negation
//
// All 4 additional Pacts: Pact of the Titan, Slaughter Pact,
// Intervention Pact, Summoner's Pact. Same pattern: cast for free,
// delayed trigger at next upkeep "pay {cost} or lose the game."
// =============================================================================

func TestPactOfTheTitan_CreatesTokenAndDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	card := addCard(gs, 0, "Pact of the Titan", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Should create a 4/4 token.
	foundToken := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Giant Token" {
			if p.Card.BasePower == 4 && p.Card.BaseToughness == 4 {
				foundToken = true
			}
		}
	}
	if !foundToken {
		t.Error("Pact of the Titan should create a 4/4 Giant token")
	}

	// Should register a delayed trigger.
	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("expected delayed trigger")
	}

	// Fire the trigger without mana -> lose.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")
	if !gs.Seats[0].Lost {
		t.Error("should lose when unable to pay Pact of the Titan cost")
	}
}

func TestPactOfTheTitan_PaysCost(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	card := addCard(gs, 0, "Pact of the Titan", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Give enough mana to pay {4}{R} = 5.
	gs.Seats[0].ManaPool = 10

	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")

	if gs.Seats[0].Lost {
		t.Error("should NOT lose with enough mana to pay")
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("expected 5 mana remaining after paying 5, got %d", gs.Seats[0].ManaPool)
	}
}

func TestSlaughterPact_DestroysAndDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	// Opponent has a nonblack creature.
	target := addPerm(gs, 1, "Serra Angel", "creature")
	target.Card.Colors = []string{"W"}

	card := addCard(gs, 0, "Slaughter Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Creature should be destroyed (sacrificed).
	foundTarget := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			foundTarget = true
		}
	}
	if foundTarget {
		t.Error("Slaughter Pact should have destroyed the nonblack creature")
	}

	// Delayed trigger registered.
	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("expected delayed trigger for Slaughter Pact")
	}

	// Fire without mana -> lose.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")
	if !gs.Seats[0].Lost {
		t.Error("should lose when unable to pay Slaughter Pact cost")
	}
}

func TestInterventionPact_GainsLifeAndDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0
	startLife := gs.Seats[0].Life

	card := addCard(gs, 0, "Intervention Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Should have gained life.
	if gs.Seats[0].Life <= startLife {
		t.Errorf("Intervention Pact should gain life; before=%d after=%d", startLife, gs.Seats[0].Life)
	}

	// Delayed trigger registered.
	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("expected delayed trigger for Intervention Pact")
	}

	// Fire without mana -> lose.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")
	if !gs.Seats[0].Lost {
		t.Error("should lose when unable to pay Intervention Pact cost")
	}
}

func TestSummonersPact_TutorsAndDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	// Add a green creature to library.
	craterhoof := &gameengine.Card{
		Name:   "Craterhoof Behemoth",
		Owner:  0,
		Types:  []string{"creature"},
		Colors: []string{"G"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, craterhoof)

	card := addCard(gs, 0, "Summoner's Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Craterhoof should now be in hand.
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Craterhoof Behemoth" {
			foundInHand = true
			break
		}
	}
	if !foundInHand {
		t.Error("Summoner's Pact should tutor Craterhoof Behemoth to hand")
	}

	// Library should be empty.
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be empty after tutor, got %d", len(gs.Seats[0].Library))
	}

	// Delayed trigger registered.
	if len(gs.DelayedTriggers) == 0 {
		t.Fatal("expected delayed trigger for Summoner's Pact")
	}

	// Fire without mana -> lose.
	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")
	if !gs.Seats[0].Lost {
		t.Error("should lose when unable to pay Summoner's Pact cost")
	}
}

func TestSummonersPact_PaysCost(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 1
	gs.Active = 0

	craterhoof := &gameengine.Card{
		Name:   "Craterhoof Behemoth",
		Owner:  0,
		Types:  []string{"creature"},
		Colors: []string{"G"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, craterhoof)

	card := addCard(gs, 0, "Summoner's Pact", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeCastHook(gs, item)

	// Give enough mana to pay {2}{G}{G} = 4.
	gs.Seats[0].ManaPool = 10

	gs.Turn = 2
	gs.Active = 0
	gameengine.FireDelayedTriggers(gs, "beginning", "upkeep")

	if gs.Seats[0].Lost {
		t.Error("should NOT lose with enough mana")
	}
	if gs.Seats[0].ManaPool != 6 {
		t.Errorf("expected 6 mana remaining after paying 4, got %d", gs.Seats[0].ManaPool)
	}
}

// =============================================================================
// Verify the SneakAttack delayed trigger fires correctly
// (validates framework item #3 — ad-hoc vs general)
// =============================================================================

func TestSneakAttack_DelayedSacrificeFiresAtEndStep(t *testing.T) {
	gs := newGame(t, 2)
	sneakAttack := addPerm(gs, 0, "Sneak Attack", "creature")
	_ = sneakAttack

	creature := &gameengine.Card{
		Name:          "Griselbrand",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     7,
		BaseToughness: 7,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, creature)

	gameengine.InvokeActivatedHook(gs, sneakAttack, 0, nil)

	// Verify the creature is on the battlefield.
	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Griselbrand" {
			foundOnBF = true
			break
		}
	}
	if !foundOnBF {
		t.Fatal("Griselbrand should be on the battlefield after Sneak Attack")
	}

	// Fire the delayed trigger at end step.
	gameengine.FireDelayedTriggers(gs, "ending", "end_step")

	// Creature should be sacrificed (removed from battlefield).
	foundAfter := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Griselbrand" {
			foundAfter = true
			break
		}
	}
	if foundAfter {
		t.Error("Griselbrand should be sacrificed at end step via delayed trigger")
	}
}
