package gameengine

import "testing"

func armiesControlled(gs *GameState, seat int) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == "army" {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// (a)/(d) No Army → create one 0/0 BLACK [subtype] Army token, then the counters.
func TestAmass_NoArmy_CreatesBlackArmyWithCounters(t *testing.T) {
	gs := newCombatGame(t)
	army := Amass(gs, 0, 2, "zombie")
	if army == nil {
		t.Fatal("Amass should create/return an Army")
	}
	if got := armiesControlled(gs, 0); len(got) != 1 {
		t.Fatalf("amass with no Army should create exactly one Army, got %d", len(got))
	}
	if army.Card.Name != "Zombie Army" {
		t.Fatalf("Army token name should be 'Zombie Army', got %q", army.Card.Name)
	}
	// Type line: creature + zombie + army.
	for _, want := range []string{"creature", "zombie", "army"} {
		found := false
		for _, ty := range army.Card.Types {
			if ty == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("Army token Types missing %q: %v", want, army.Card.Types)
		}
	}
	// §701.43c — black.
	if len(army.Card.Colors) != 1 || army.Card.Colors[0] != "B" {
		t.Fatalf("Army token must be black, got colors %v", army.Card.Colors)
	}
	if army.Counters["+1/+1"] != 2 {
		t.Fatalf("amass 2 should place 2 +1/+1 counters, got %d", army.Counters["+1/+1"])
	}
}

// (b)/(c) Existing Army → NO new token; all N counters grow the SAME Army.
func TestAmass_ExistingArmy_GrowsSameNoNewToken(t *testing.T) {
	gs := newCombatGame(t)
	first := Amass(gs, 0, 1, "zombie") // creates the army
	second := Amass(gs, 0, 3, "zombie")
	if first != second {
		t.Fatal("a second amass must grow the SAME Army, not create a new one")
	}
	if got := armiesControlled(gs, 0); len(got) != 1 {
		t.Fatalf("amass with an existing Army must not create another, got %d Armies", len(got))
	}
	if first.Counters["+1/+1"] != 4 {
		t.Fatalf("two amasses (1 + 3) should leave 4 counters on the one Army, got %d", first.Counters["+1/+1"])
	}
}

// (f) +1/+1 counter doublers apply to the amassed counters.
func TestAmass_CounterDoublersApply(t *testing.T) {
	gs := newCombatGame(t)
	// Pre-place a bare Army the seat controls.
	army := addBattlefield(gs, 0, "Zombie Army", 0, 0, "creature", "zombie", "army")
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	got := Amass(gs, 0, 3, "zombie")
	if got != army {
		t.Fatal("amass should grow the existing Army")
	}
	// Doubling Season doubles the 3 amassed counters → 6.
	if army.Counters["+1/+1"] != 6 {
		t.Fatalf("amass 3 with Doubling Season should place 6 +1/+1 counters, got %d", army.Counters["+1/+1"])
	}
}

// Orc subtype + blackness for the Orc Army path (Sauron etc.).
func TestAmass_OrcSubtypeBlack(t *testing.T) {
	gs := newCombatGame(t)
	army := Amass(gs, 0, 1, "orc")
	if army == nil || army.Card.Name != "Orc Army" {
		t.Fatalf("amass Orcs should make an 'Orc Army', got %v", army)
	}
	if len(army.Card.Colors) != 1 || army.Card.Colors[0] != "B" {
		t.Fatalf("Orc Army must be black, got %v", army.Card.Colors)
	}
}
