package hat

import "testing"

// eval_weights_round5_r60_test.go — R60-13. Aristocrats sac/persist
// audit follow-up: BoardPresence + GraveyardValue + ComboProximity
// pushed past their round-2 floors after wins started leaning on
// infinite-sac lines (Mikaeus+Triskelion, Persist+sac+drain) that
// the evaluator was under-counting.

func TestAristocratsR60_13_BoardPresenceFloorPushed(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeAristocrats)
	if w.BoardPresence < 1.3 {
		t.Errorf("Aristocrats BoardPresence must be ≥ 1.3 (sac fodder throttle); got %.2f",
			w.BoardPresence)
	}
}

func TestAristocratsR60_13_GraveyardValueFloorPushed(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeAristocrats)
	if w.GraveyardValue < 1.5 {
		t.Errorf("Aristocrats GraveyardValue must be ≥ 1.5 (persist bodies live in GY); got %.2f",
			w.GraveyardValue)
	}
	// Should clearly exceed Reanimator-adjacent value-engine baselines.
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.GraveyardValue <= mid.GraveyardValue*2 {
		t.Errorf("Aristocrats GraveyardValue (%.2f) should be > 2x midrange (%.2f) — GY IS the engine",
			w.GraveyardValue, mid.GraveyardValue)
	}
}

func TestAristocratsR60_13_ComboProximityFloorPushed(t *testing.T) {
	// Mikaeus+Triskelion / Mikaeus+Walking Ballista / Persist creature+
	// Melira/Mikaeus+sac outlet / Reassembling Skeleton+Phyrexian Altar+
	// Pitiless Plunderer — infinite-sac lines the deck reaches into.
	w := DefaultWeightsForArchetype(ArchetypeAristocrats)
	if w.ComboProximity < 1.4 {
		t.Errorf("Aristocrats ComboProximity must be ≥ 1.4 (infinite-sac lines); got %.2f",
			w.ComboProximity)
	}
	// Stays below the Combo headline (2.0) — DrainEngine remains the
	// signature dial for the archetype.
	if w.ComboProximity >= 2.0 {
		t.Errorf("Aristocrats ComboProximity (%.2f) should stay below dedicated Combo (2.0); DrainEngine is the headline",
			w.ComboProximity)
	}
}

func TestAristocratsR60_13_DrainEngineStillHeadline(t *testing.T) {
	// Bump invariant: after pushing three secondary dials up, DrainEngine
	// must remain the single highest weight in the profile.
	w := DefaultWeightsForArchetype(ArchetypeAristocrats)
	a := w.AsArray()
	idxDrain := 8
	for i, v := range a {
		if i == idxDrain {
			continue
		}
		if v >= a[idxDrain] {
			t.Errorf("dim %d (%.2f) reached/exceeded DrainEngine (%.2f) — DrainEngine must stay the headline after r60-13 bump",
				i, v, a[idxDrain])
		}
	}
}
