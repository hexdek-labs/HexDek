package gameengine

import (
	"math/rand"
	"testing"
)

// ---------------------------------------------------------------------------
// ZoneConservation
// ---------------------------------------------------------------------------

func TestZoneConservation_BaselineRecorded(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// Add some cards to zones.
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Island"})
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, &Card{Name: "Mountain"})

	err := checkZoneConservation(gs)
	if err != nil {
		t.Fatalf("first call should establish baseline, got error: %v", err)
	}
	// Baseline should be recorded.
	if gs.Flags["_zone_conservation_total"] != 3 {
		t.Fatalf("expected baseline 3, got %d", gs.Flags["_zone_conservation_total"])
	}
}

func TestZoneConservation_Pass(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Island"})

	// First call: establish baseline.
	checkZoneConservation(gs)

	// Move card from library to hand (legal zone change).
	card := gs.Seats[0].Library[0]
	gs.Seats[0].Library = gs.Seats[0].Library[1:]
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	err := checkZoneConservation(gs)
	if err != nil {
		t.Fatalf("zone change should preserve total, got: %v", err)
	}
}

func TestZoneConservation_ViolationNegativeDelta(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Island"})

	// First call: establish baseline (total=2).
	checkZoneConservation(gs)

	// Remove a card without moving it anywhere (bug simulation).
	gs.Seats[0].Library = gs.Seats[0].Library[:0]

	err := checkZoneConservation(gs)
	if err == nil {
		t.Fatal("should detect missing card (negative delta)")
	}
}

func TestZoneConservation_SmallPositiveDelta_OK(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})

	// Establish baseline (total=1).
	checkZoneConservation(gs)

	// Add a card (simulate copy effect creating a real card).
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Copy of Forest"})

	err := checkZoneConservation(gs)
	if err != nil {
		t.Fatalf("small positive delta from copy effects should be tolerated, got: %v", err)
	}
}

func TestZoneConservation_TokensIgnored(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})

	checkZoneConservation(gs) // baseline = 1

	// Create a token — should not affect conservation count.
	token := &Permanent{
		Card:       &Card{Name: "Treasure Token", Types: []string{"token", "artifact"}},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, token)

	err := checkZoneConservation(gs)
	if err != nil {
		t.Fatalf("tokens should be ignored, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LifeConsistency
// ---------------------------------------------------------------------------

func TestLifeConsistency_Pass(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = -5
	gs.Seats[1].Lost = true

	err := checkLifeConsistency(gs)
	if err != nil {
		t.Fatalf("negative life with Lost=true should pass, got: %v", err)
	}
}

func TestLifeConsistency_Violation(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Life = -3

	err := checkLifeConsistency(gs)
	if err == nil {
		t.Fatal("should detect negative life without Lost flag")
	}
}

// ---------------------------------------------------------------------------
// SBACompleteness
// ---------------------------------------------------------------------------

func TestSBACompleteness_CreatureZeroToughness(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	perm := &Permanent{
		Card: &Card{
			Name:          "Dud",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 0,
		},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	err := checkSBACompleteness(gs)
	if err == nil {
		t.Fatal("should detect zero-toughness creature on battlefield")
	}
}

func TestSBACompleteness_PhasedOutIgnored(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	perm := &Permanent{
		Card: &Card{
			Name:          "Phased Dud",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 0,
		},
		Controller: 0,
		PhasedOut:  true,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	err := checkSBACompleteness(gs)
	if err != nil {
		t.Fatalf("phased-out creature should be skipped, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// StackIntegrity
// ---------------------------------------------------------------------------

func TestStackIntegrity_EmptyAtMainPhase(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Step = "precombat_main"

	err := checkStackIntegrity(gs)
	if err != nil {
		t.Fatalf("empty stack at main phase should pass, got: %v", err)
	}
}

func TestStackIntegrity_NonEmptyAtMainPhase(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Step = "precombat_main"
	gs.Stack = append(gs.Stack, &StackItem{
		Card: &Card{Name: "Lightning Bolt"},
	})

	err := checkStackIntegrity(gs)
	if err == nil {
		t.Fatal("should detect non-empty stack at main phase boundary")
	}
}

func TestStackIntegrity_NonEmptyDuringCombat_OK(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Step = "declare_attackers"
	gs.Stack = append(gs.Stack, &StackItem{
		Card: &Card{Name: "Lightning Bolt"},
	})

	err := checkStackIntegrity(gs)
	if err != nil {
		t.Fatalf("stack during combat should be OK, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ManaPoolNonNegative
// ---------------------------------------------------------------------------

func TestManaPoolNonNegative_Pass(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].ManaPool = 5

	err := checkManaPoolNonNegative(gs)
	if err != nil {
		t.Fatalf("positive mana should pass, got: %v", err)
	}
}

func TestManaPoolNonNegative_Violation(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].ManaPool = -1

	err := checkManaPoolNonNegative(gs)
	if err == nil {
		t.Fatal("should detect negative mana pool")
	}
}

func TestManaPoolNonNegative_TypedPool(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Mana = &ColoredManaPool{W: 3, U: -1}

	err := checkManaPoolNonNegative(gs)
	if err == nil {
		t.Fatal("should detect negative typed mana")
	}
}

// ---------------------------------------------------------------------------
// CommanderDamageMonotonic
// ---------------------------------------------------------------------------

func TestCommanderDamageMonotonic_Pass(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderDamage[1] = map[string]int{"Krenko": 5}

	// First call — baseline recorded.
	err := checkCommanderDamageMonotonic(gs)
	if err != nil {
		t.Fatalf("first call should pass, got: %v", err)
	}

	// Increase — should pass.
	gs.Seats[0].CommanderDamage[1]["Krenko"] = 10
	err = checkCommanderDamageMonotonic(gs)
	if err != nil {
		t.Fatalf("increase should pass, got: %v", err)
	}
}

func TestCommanderDamageMonotonic_Violation(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.CommanderFormat = true
	gs.Seats[0].CommanderDamage[1] = map[string]int{"Krenko": 10}

	checkCommanderDamageMonotonic(gs) // baseline = 10

	// Decrease — should fail.
	gs.Seats[0].CommanderDamage[1]["Krenko"] = 5
	err := checkCommanderDamageMonotonic(gs)
	if err == nil {
		t.Fatal("should detect commander damage decrease")
	}
}

// ---------------------------------------------------------------------------
// LayerIdempotency
// ---------------------------------------------------------------------------

func TestLayerIdempotency_Pass(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	perm := &Permanent{
		Card: &Card{
			Name:          "Grizzly Bears",
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
		},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	err := checkLayerIdempotency(gs)
	if err != nil {
		t.Fatalf("simple permanent should be idempotent, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// RunAllInvariants
// ---------------------------------------------------------------------------

func TestRunAllInvariants_CleanState(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	violations := RunAllInvariants(gs)
	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("unexpected violation: %s — %s", v.Name, v.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// Diagnostic helpers
// ---------------------------------------------------------------------------

func TestGameStateSummary(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Life = 15
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})
	summary := GameStateSummary(gs)
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
}

func TestRecentEvents(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.LogEvent(Event{Kind: "test_event", Seat: 0, Source: "Test"})
	lines := RecentEvents(gs, 5)
	if len(lines) != 1 {
		t.Fatalf("expected 1 event, got %d", len(lines))
	}
}
