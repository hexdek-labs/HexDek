package gameengine

import "testing"

// cost_reduction_colored_floor_r63_test.go — audit of CR §601.2f-h
// "abilities that reduce a cost by an amount of GENERIC mana can't reduce that
// cost below the colored requirement" (property a/b of the cost-reduction
// mechanic probe). Pre-fix ApplyCostModifiers floored every reduction at 0
// (total), so a generic reducer could pierce the colored pips — e.g. an
// {R}{R} dragon under three Helms of Awakening computed to 0, castable for
// free with no red mana.

func addHelms(gs *GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, &Permanent{
			Card:       &Card{Name: "Helm of Awakening", Types: []string{"artifact"}},
			Controller: seat, Owner: seat,
		})
	}
}

// (a)/(b): generic reductions floor at the colored-pip requirement, not 0.
func TestCostReduction_ColoredFloor_NotBelowPips(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	addHelms(gs, 0, 3) // -3 generic, total

	// {R}{R} dragon (CMC 2, all colored). Three -1 reductions must NOT take
	// it below {R}{R} = 2.
	dragon := &Card{Name: "Furystoke Drake", Types: []string{"creature", "dragon", "cost:2"},
		ManaCostString: "{R}{R}"}
	if got := CalculateTotalCost(gs, dragon, 0); got != 2 {
		t.Errorf("{R}{R} under -3 generic: want floor 2 (colored), got %d", got)
	}

	// {2}{R}{R} (CMC 4): -3 removes the {2} generic and 1 wasted reduction →
	// floors at {R}{R} = 2.
	mixed := &Card{Name: "Bigger Dragon", Types: []string{"creature", "dragon", "cost:4"},
		ManaCostString: "{2}{R}{R}"}
	if got := CalculateTotalCost(gs, mixed, 0); got != 2 {
		t.Errorf("{2}{R}{R} under -3 generic: want 2 (colored floor), got %d", got)
	}

	// All-generic {3}: reduces fully to 0 (no colored to protect).
	gen := &Card{Name: "Generic Spell", Types: []string{"sorcery", "cost:3"},
		ManaCostString: "{3}"}
	if got := CalculateTotalCost(gs, gen, 0); got != 0 {
		t.Errorf("{3} under -3 generic: want 0, got %d", got)
	}

	// Colorless {C} pip is also protected from generic reduction.
	cless := &Card{Name: "Eldrazi Spawn", Types: []string{"creature", "cost:2"},
		ManaCostString: "{C}{C}"}
	if got := CalculateTotalCost(gs, cless, 0); got != 2 {
		t.Errorf("{C}{C} under -3 generic: want floor 2, got %d", got)
	}
}

// (e): increases apply before reductions, then the colored floor, then minimums.
func TestCostReduction_IncreaseThenReduceThenFloor(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Sphere of Resistance (+1 all) on seat 1; three Helms (-3) on seat 0.
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &Permanent{
		Card: &Card{Name: "Sphere of Resistance", Types: []string{"artifact"}}, Controller: 1, Owner: 1,
	})
	addHelms(gs, 0, 3)

	// {R}{R} (2) +1 Sphere = 3, -3 Helms = 0 → floor at colored {R}{R} = 2.
	dragon := &Card{Name: "Drake", Types: []string{"creature", "cost:2"}, ManaCostString: "{R}{R}"}
	if got := CalculateTotalCost(gs, dragon, 0); got != 2 {
		t.Errorf("{R}{R}+Sphere-3Helms: want 2 (colored floor), got %d", got)
	}
}

// Convoke is color-capable ("{1} or one mana of that creature's color"), so it
// is EXEMPT from the colored floor — it can take a colored cost to 0. Pre-fix
// this worked only because everything floored at 0; the colored-floor fix must
// not regress it.
func TestCostReduction_ConvokeExemptFromColoredFloor(t *testing.T) {
	// applyCostModifiersFloored directly: floor 2 ({G}{G}), one convoke -2.
	convoke := []CostModifier{{Kind: CostModReduction, Amount: 2, Source: "convoke", ColorCapable: true}}
	if got := applyCostModifiersFloored(2, convoke, 2); got != 0 {
		t.Errorf("convoke (color-capable) must reach 0 past the colored floor, got %d", got)
	}
	// A plain generic reducer with the same floor stops at 2.
	generic := []CostModifier{{Kind: CostModReduction, Amount: 2, Source: "helm"}}
	if got := applyCostModifiersFloored(2, generic, 2); got != 2 {
		t.Errorf("generic reducer must floor at colored 2, got %d", got)
	}
	// Mixed: {2}{G}{G} (floor 2), -3 generic + -2 convoke → 0.
	mixed := []CostModifier{
		{Kind: CostModReduction, Amount: 3, Source: "helmx3"},
		{Kind: CostModReduction, Amount: 2, Source: "convoke", ColorCapable: true},
	}
	if got := applyCostModifiersFloored(4, mixed, 2); got != 0 {
		t.Errorf("{2}{G}{G} -3 generic -2 convoke: want 0, got %d", got)
	}
}

// Cards without a ManaCostString (engine tokens / cost:N-only test cards) keep
// the old floor-0 behavior — no regression for the colorless-cost path.
func TestCostReduction_NoManaCostString_FloorsAtZero(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	addHelms(gs, 0, 3)
	tok := &Card{Name: "Token Thing", Types: []string{"creature", "cost:2"}}
	if got := CalculateTotalCost(gs, tok, 0); got != 0 {
		t.Errorf("no-MCS card under -3: want 0 (legacy), got %d", got)
	}
}
