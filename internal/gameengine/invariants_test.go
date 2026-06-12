package gameengine

import (
	"math/rand"
	"testing"
)

// ---------------------------------------------------------------------------
// ZoneConservation
// ---------------------------------------------------------------------------

// The five count-based ZoneConservation tests that used to live here
// pinned the legacy count heuristic (baseline flag + delta tolerance),
// which the r63 Judge CONSERVATION fold DELETED: 499/500 of count-shape
// warnings were strict-census-proven false positives. The replacements
// below pin the InstanceID census paths, and the strict-census prod
// behavior is pinned in hat/feynman_zone_strict_r63_test.go.

// TestZoneConservation_UnmintedIsNoop — without minted InstanceIDs
// (struct-literal fixtures) the conservation check has nothing to say:
// no baseline flag, no error, regardless of count imbalance.
func TestZoneConservation_UnmintedIsNoop(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Forest"})
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{Name: "Island"})

	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("unminted state must be a conservation no-op, got: %v", err)
	}
	if _, ok := gs.Flags["_zone_conservation_total"]; ok {
		t.Fatal("legacy count baseline flag must no longer be written")
	}
	gs.Seats[0].Library = gs.Seats[0].Library[:0] // count imbalance, no identity
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("unminted count imbalance must not warn post-fold, got: %v", err)
	}
}

// TestZoneConservation_CensusDisappearance — a minted card removed from
// every zone is a real disappearance under the strict census.
func TestZoneConservation_CensusDisappearance(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	c := &Card{Name: "Forest", Owner: 0}
	MintOGInstanceID(gs, c)
	gs.Seats[0].Library = append(gs.Seats[0].Library, c)

	if err, authoritative := ZoneConservationStrict(gs); err != nil || !authoritative {
		t.Fatalf("intact minted state must pass strict census (err=%v auth=%v)", err, authoritative)
	}
	gs.Seats[0].Library = gs.Seats[0].Library[:0] // vanish without ceasing
	err, _ := ZoneConservationStrict(gs)
	if err == nil {
		t.Fatal("minted-not-ceased card absent from every zone must fail the strict census")
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
