package gameengine

// CR §704.5j — the legend rule keys on a permanent's NAME, which is its
// EFFECTIVE (layer-applied) name, not its printed one. Regression for the
// r63 legend-rule probe: sba704_5j grouped by p.Card.DisplayName() (printed)
// while gating legendary-ness on the layer-aware HasSupertypeOf, so a
// TEMPORARY copy of a legend (Cytoshape / Mirrorweave — CopyPermanentLayered
// with a non-permanent duration leaves p.Card untouched) read as legendary
// with the copied name in the layer system but kept its old printed name,
// and the rule never fired.

import "testing"

// A temporary copy of your legend (Card pointer NOT swapped — only the
// layer-1 copy effect applies) must trigger the legend rule.
func TestLegendRule_TemporaryCopyOfLegend_Fires(t *testing.T) {
	gs := newFixtureGame(t)
	orig := addBattlefield(gs, 0, "Kenrith, the Returned King", 5, 5, "creature", "legendary")
	clone := addBattlefield(gs, 0, "Cloudshift Bear", 2, 2, "creature")

	// "becomes a copy of Kenrith until end of turn" — temporary, no Card swap.
	CopyPermanentLayered(gs, clone, orig, DurationEndOfTurn)
	gs.InvalidateCharacteristicsCache()

	// Sanity: the clone is now an effective legendary Kenrith but kept its
	// printed name.
	if !gs.HasSupertypeOf(clone, "legendary") {
		t.Fatal("temporary copy of a legend must be legendary")
	}
	if clone.Card.DisplayName() == orig.Card.DisplayName() {
		t.Fatal("fixture: the Card pointer should NOT be swapped for a temporary copy")
	}

	StateBasedActions(gs)

	live := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == orig || p == clone {
			live++
		}
	}
	if live != 1 {
		t.Errorf("legend rule must fire on a temporary copy of your legend; %d of the two survived (want 1)", live)
	}
	if countEvents(gs, "sba_704_5j_keep") == 0 {
		t.Error("expected an sba_704_5j_keep event for the temporary-copy legend rule")
	}
}

// Control: two DIFFERENT-named legends coexist (the layered-name change
// must not over-group).
func TestLegendRule_DifferentNamedLegendsCoexist(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Kenrith, the Returned King", 5, 5, "creature", "legendary")
	addBattlefield(gs, 0, "Atraxa, Praetors' Voice", 4, 4, "creature", "legendary")
	StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Errorf("two different-named legends must coexist, got %d", len(gs.Seats[0].Battlefield))
	}
}

// Control: a PERMANENT clone (Card pointer swapped) still triggers the rule
// (guards the layered-name read for the common path).
func TestLegendRule_PermanentCloneOfLegend_Fires(t *testing.T) {
	gs := newFixtureGame(t)
	orig := addBattlefield(gs, 0, "Kenrith, the Returned King", 5, 5, "creature", "legendary")
	clone := addBattlefield(gs, 0, "Clone", 0, 0, "creature")
	CopyPermanentLayered(gs, clone, orig, DurationPermanent)
	gs.InvalidateCharacteristicsCache()

	StateBasedActions(gs)
	live := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == orig || p == clone {
			live++
		}
	}
	if live != 1 {
		t.Errorf("permanent clone of a legend must trigger the legend rule; %d survived (want 1)", live)
	}
}
