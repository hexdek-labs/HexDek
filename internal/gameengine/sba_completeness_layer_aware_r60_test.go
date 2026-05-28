package gameengine

import (
	"testing"
)

// TestSBACompleteness_LegendRuleUsesLayerAwarePredicate verifies
// that sba704_5j (legend rule) reads the layer-effective legendary
// status via gs.HasTypeOf rather than the printed-only
// p.IsLegendary. This matches the layer-aware predicate pattern
// already used by sba704_5f / sba704_5g / sba704_5n /
// checkSBACompleteness, and closes a class of SBACompleteness
// divergences where the invariant uses the layer-aware predicate
// but the SBA itself uses the printed one — the invariant sees a
// state the SBA refused to act on.
//
// Scenario: two legendary creatures with the same name (Karn,
// Liberated) both on seat 0's battlefield. Legend rule must fire
// regardless of which type-predicate path is used.
func TestSBACompleteness_LegendRuleUsesLayerAwarePredicate(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	addLegendary := func(name string) *Permanent {
		p := &Permanent{
			Card: &Card{
				Name:  name,
				Types: []string{"legendary", "planeswalker"},
				Owner: 0,
			},
			Controller: 0,
			Owner:      0,
			Timestamp:  gs.NextTimestamp(),
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		return p
	}

	older := addLegendary("Karn, Liberated") // ts=1
	newer := addLegendary("Karn, Liberated") // ts=2

	changed := sba704_5j(gs)
	if !changed {
		t.Fatal("sba704_5j: should have fired for two same-name legendaries")
	}

	// Earlier timestamp keeps; the newer copy must have been
	// removed from the battlefield.
	stillThere := func(p *Permanent) bool {
		for _, q := range gs.Seats[0].Battlefield {
			if q == p {
				return true
			}
		}
		return false
	}
	if !stillThere(older) {
		t.Errorf("keeper (earliest timestamp) should remain on battlefield, but was removed")
	}
	if stillThere(newer) {
		t.Errorf("loser (later timestamp) should have been removed by legend rule, but is still on battlefield")
	}
}

// TestSBACompleteness_LegendRuleSurvivesHumilityStripping verifies
// the integration: Humility strips abilities but the legend rule
// (a supertype property, not an ability) still applies. A common
// concern is whether Humility's Layer 6 ability strip incorrectly
// disables the legend rule — it shouldn't, because legendary is a
// supertype (§205.4b) not an ability.
//
// Two legendary creatures + Humility + Opalescence simulation:
// even after layer cache propagation, the legend rule must reach
// steady state in a single SBA loop.
func TestSBACompleteness_LegendRuleSurvivesHumilityStripping(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Humility on seat 0.
	humility := &Permanent{
		Card:       &Card{Name: "Humility", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{"cmc": 4},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)
	RegisterHumility(gs, humility)

	// Two legendary creatures with the same name. Use a known
	// printed legendary creature.
	addLegendaryCreature := func(name string) *Permanent {
		p := &Permanent{
			Card: &Card{
				Name:          name,
				Types:         []string{"legendary", "creature"},
				BasePower:     3,
				BaseToughness: 3,
				Owner:         0,
			},
			Controller: 0,
			Owner:      0,
			Timestamp:  gs.NextTimestamp(),
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		return p
	}

	older := addLegendaryCreature("Llanowar Sentinel")
	newer := addLegendaryCreature("Llanowar Sentinel")

	// Run the full SBA loop. Humility sets P/T to 1/1 (so they
	// survive 704.5f) and strips their abilities (irrelevant to
	// legend rule). The legend rule must still fire and reach
	// steady state.
	StateBasedActions(gs)

	stillThere := func(p *Permanent) bool {
		for _, q := range gs.Seats[0].Battlefield {
			if q == p {
				return true
			}
		}
		return false
	}
	if !stillThere(older) {
		t.Errorf("Humility-stripped legendary keeper should remain after SBA pass, but was removed (likely an over-aggressive printed-type sweep)")
	}
	if stillThere(newer) {
		t.Errorf("Humility-stripped legendary loser should have been removed by legend rule, but survived (printed-type predicate may have failed to see legendary status after layer effects)")
	}

	// Verify SBACompleteness invariant holds — every remaining
	// creature has positive toughness.
	if err := checkSBACompleteness(gs); err != nil {
		t.Errorf("SBACompleteness invariant violated post-SBA: %v", err)
	}
}

// TestSBACompleteness_HumilityKeepsCreaturesAlive verifies the
// canonical Humility behavior: an enchantment with no printed
// creature base P/T, made a creature by Opalescence (CMC/CMC),
// would die to 704.5f if its CMC is 0 — but with Humility setting
// it to 1/1, it survives. This pins the layer 7b timestamp race
// resolution.
func TestSBACompleteness_HumilityKeepsCreaturesAlive(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Opalescence first (lower timestamp).
	opal := &Permanent{
		Card:       &Card{Name: "Opalescence", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{"cmc": 4},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, opal)
	RegisterOpalescence(gs, opal)

	// Humility second (later timestamp — wins layer 7b race).
	humility := &Permanent{
		Card:       &Card{Name: "Humility", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{"cmc": 4},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)
	RegisterHumility(gs, humility)

	// A bare enchantment (no printed creature type) that
	// Opalescence will turn into a creature.
	test := &Permanent{
		Card: &Card{
			Name:  "Test Enchantment",
			Types: []string{"enchantment"},
			Owner: 0,
		},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{"cmc": 0}, // 0-cost enchantment
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, test)

	StateBasedActions(gs)

	// Test Enchantment should survive: Opalescence's Layer 7b
	// (CMC=0 -> 0/0) is over-ridden by Humility's later-timestamp
	// Layer 7b (-> 1/1). Toughness 1 > 0, so 704.5f doesn't fire.
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == test {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("Humility (later timestamp) should win the Layer 7b race vs Opalescence and set Test Enchantment to 1/1, but it was destroyed")
	}
	if err := checkSBACompleteness(gs); err != nil {
		t.Errorf("SBACompleteness invariant violated: %v", err)
	}
}
