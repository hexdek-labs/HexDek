package gameengine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// P1 #1: Cost Modification Framework Tests
// ---------------------------------------------------------------------------

func TestCalculateTotalCost_NoModifiers(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	card := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, card, 0)
	if cost != 1 {
		t.Fatalf("expected cost 1, got %d", cost)
	}
}

func TestCalculateTotalCost_ThaliaIncreasesNoncreature(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Opponent controls Thalia.
	thalia := &Permanent{
		Card:       &Card{Name: "Thalia, Guardian of Thraben", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, thalia)

	// Noncreature spell from seat 0 costs +1.
	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 2 {
		t.Fatalf("expected cost 2 (1 base + 1 Thalia), got %d", cost)
	}

	// Creature spell is unaffected.
	bear := &Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}}
	cost = CalculateTotalCost(gs, bear, 0)
	if cost != 2 {
		t.Fatalf("expected creature cost 2 (Thalia doesn't affect), got %d", cost)
	}
}

func TestCalculateTotalCost_SphereOfResistance(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	sphere := &Permanent{
		Card:       &Card{Name: "Sphere of Resistance", Types: []string{"artifact"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, sphere)

	// All spells cost +1.
	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 2 {
		t.Fatalf("expected cost 2 (1 + Sphere), got %d", cost)
	}

	bear := &Card{Name: "Grizzly Bears", Types: []string{"creature", "cost:2"}}
	cost = CalculateTotalCost(gs, bear, 0)
	if cost != 3 {
		t.Fatalf("expected cost 3 (2 + Sphere), got %d", cost)
	}
}

func TestCalculateTotalCost_HelmOfAwakeningReduces(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	helm := &Permanent{
		Card:       &Card{Name: "Helm of Awakening", Types: []string{"artifact"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, helm)

	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:3"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 2 {
		t.Fatalf("expected cost 2 (3 - 1 Helm), got %d", cost)
	}
}

func TestCalculateTotalCost_ReductionFlooredAtZero(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Two Helms reduce by 2 total, but a 1-cost spell floors at 0.
	for i := 0; i < 2; i++ {
		helm := &Permanent{
			Card:       &Card{Name: "Helm of Awakening", Types: []string{"artifact"}},
			Controller: 0,
			Owner:      0,
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, helm)
	}

	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 0 {
		t.Fatalf("expected cost 0 (1 - 2, floor 0), got %d", cost)
	}
}

func TestCalculateTotalCost_Trinisphere(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	trini := &Permanent{
		Card:       &Card{Name: "Trinisphere", Types: []string{"artifact"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, trini)

	// A 1-cost spell is raised to minimum 3.
	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 3 {
		t.Fatalf("expected cost 3 (Trinisphere minimum), got %d", cost)
	}

	// A 5-cost spell is unaffected.
	bomb := &Card{Name: "Big Spell", Types: []string{"sorcery", "cost:5"}}
	cost = CalculateTotalCost(gs, bomb, 0)
	if cost != 5 {
		t.Fatalf("expected cost 5 (above Trinisphere), got %d", cost)
	}
}

func TestCalculateTotalCost_IncreasesThenReductionsThenMinimums(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Thalia (+1 noncreature) + Helm (-1) + Trinisphere (min 3).
	thalia := &Permanent{
		Card:       &Card{Name: "Thalia, Guardian of Thraben", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
	}
	helm := &Permanent{
		Card:       &Card{Name: "Helm of Awakening", Types: []string{"artifact"}},
		Controller: 0,
		Owner:      0,
	}
	trini := &Permanent{
		Card:       &Card{Name: "Trinisphere", Types: []string{"artifact"}},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, thalia, trini)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, helm)

	// Cost: 1 (base) + 1 (Thalia) - 1 (Helm) = 1, then Trinisphere min 3 = 3.
	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 3 {
		t.Fatalf("expected cost 3 (Thalia+Helm cancel, then Trinisphere), got %d", cost)
	}
}

func TestCalculateTotalCost_MedallionReducesColoredSpell(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	medallion := &Permanent{
		Card:       &Card{Name: "Sapphire Medallion", Types: []string{"artifact"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, medallion)

	// Blue spell from seat 0 costs -1.
	counterspell := &Card{Name: "Counterspell", Types: []string{"instant", "cost:2"}, Colors: []string{"U"}}
	cost := CalculateTotalCost(gs, counterspell, 0)
	if cost != 1 {
		t.Fatalf("expected cost 1 (2 - 1 Sapphire Medallion), got %d", cost)
	}

	// Red spell is unaffected.
	redSpell := &Card{Name: "Shock", Types: []string{"instant", "cost:1"}, Colors: []string{"R"}}
	cost = CalculateTotalCost(gs, redSpell, 0)
	if cost != 1 {
		t.Fatalf("expected cost 1 (Sapphire doesn't affect red), got %d", cost)
	}
}

func TestCalculateTotalCost_GoblinElectromancer(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	electro := &Permanent{
		Card:       &Card{Name: "Goblin Electromancer", Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, electro)

	// Instant from seat 0 costs -1.
	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:2"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 1 {
		t.Fatalf("expected cost 1 (2 - 1 Electromancer), got %d", cost)
	}

	// Creature is unaffected.
	bear := &Card{Name: "Bear", Types: []string{"creature", "cost:2"}}
	cost = CalculateTotalCost(gs, bear, 0)
	if cost != 2 {
		t.Fatalf("expected cost 2 (Electromancer doesn't affect creatures), got %d", cost)
	}
}

func TestCalculateTotalCost_FlagBasedModifiers(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// A permanent with cost_increase_noncreature flag.
	perm := &Permanent{
		Card:       &Card{Name: "Custom Taxer", Types: []string{"enchantment"}},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{"cost_increase_noncreature": 2},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, perm)

	bolt := &Card{Name: "Lightning Bolt", Types: []string{"instant", "cost:1"}}
	cost := CalculateTotalCost(gs, bolt, 0)
	if cost != 3 {
		t.Fatalf("expected cost 3 (1 + 2 flag), got %d", cost)
	}
}

func TestApplyCostModifiers_EmptyList(t *testing.T) {
	cost := ApplyCostModifiers(5, nil)
	if cost != 5 {
		t.Fatalf("expected 5 with no mods, got %d", cost)
	}
}
