package hat

import (
	"testing"
)

// eval_weights_round4_r60_test.go — Round 4 of the R60 archetype-
// weight audit (companions: PR #179 Storm+Mill, #181 Voltron+
// Aristocrats, #185 Reanimator+Spellslinger+Tribal). Picked the three
// remaining profiles with the largest signal-to-noise improvements
// available: Stax (ArtifactSynergy / StackInteraction), LandsMatter
// (GraveyardValue / ComboProximity), Blink (ExileZoneAssets /
// ComboProximity).

// -----------------------------------------------------------------------------
// Stax: StaxLockProgress stays highest; artifact + stack-protection rise.
// -----------------------------------------------------------------------------

func TestStaxWeights_StaxLockProgressRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeStax)
	a := w.AsArray()
	idxStaxLock := 19 // StaxLockProgress slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxStaxLock {
		t.Fatalf("Stax's highest weight should be StaxLockProgress (idx %d), got idx %d val %.2f",
			idxStaxLock, bestIdx, best)
	}
}

func TestStaxWeights_ArtifactSynergyBoosted(t *testing.T) {
	// Winter Orb / Static Orb / Stasis / Smokestack / Sphere of
	// Resistance / Tangle Wire / Trinisphere / Chalice of the Void —
	// most stax pieces are artifacts. 0.6 was way under-weighted.
	w := DefaultWeightsForArchetype(ArchetypeStax)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ArtifactSynergy < 1.1 {
		t.Errorf("Stax ArtifactSynergy should be ≥ 1.1 (Winter Orb / Static Orb / Stasis); got %.2f",
			w.ArtifactSynergy)
	}
	if w.ArtifactSynergy <= mid.ArtifactSynergy {
		t.Errorf("Stax ArtifactSynergy (%.2f) should exceed midrange (%.2f)",
			w.ArtifactSynergy, mid.ArtifactSynergy)
	}
}

func TestStaxWeights_StackInteractionBoosted(t *testing.T) {
	// Lock pieces eat removal and counters — protecting them is the
	// difference between winning a long game and conceding turn 12.
	w := DefaultWeightsForArchetype(ArchetypeStax)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.StackInteraction < 1.0 {
		t.Errorf("Stax StackInteraction should be ≥ 1.0 (protect lock pieces); got %.2f",
			w.StackInteraction)
	}
	if w.StackInteraction <= mid.StackInteraction {
		t.Errorf("Stax StackInteraction (%.2f) should exceed midrange (%.2f)",
			w.StackInteraction, mid.StackInteraction)
	}
}

// -----------------------------------------------------------------------------
// LandsMatter: ManaAdvantage stays highest; graveyard + combo rise.
// -----------------------------------------------------------------------------

func TestLandsMatterWeights_ManaAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeLandsMatter)
	a := w.AsArray()
	idxMana := 2 // ManaAdvantage slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxMana {
		t.Fatalf("LandsMatter's highest weight should be ManaAdvantage (idx %d), got idx %d val %.2f",
			idxMana, bestIdx, best)
	}
}

func TestLandsMatterWeights_GraveyardValueBoosted(t *testing.T) {
	// Crucible of Worlds / Ramunap Excavator / Splendid Reclamation /
	// World Shaper / Titania / Worm Harvest / Lord Windgrace -3 / Gitrog
	// Monster — recurring lands from GY IS the gameplan.
	w := DefaultWeightsForArchetype(ArchetypeLandsMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.GraveyardValue < 1.2 {
		t.Errorf("LandsMatter GraveyardValue should be ≥ 1.2 (Crucible / Excavator / Titania); got %.2f",
			w.GraveyardValue)
	}
	if w.GraveyardValue <= mid.GraveyardValue {
		t.Errorf("LandsMatter GraveyardValue (%.2f) should exceed midrange (%.2f)",
			w.GraveyardValue, mid.GraveyardValue)
	}
}

func TestLandsMatterWeights_ComboProximityBoosted(t *testing.T) {
	// Lotus Field + Mystic Sanctuary, Field of the Dead, Scapeshift,
	// Valakut, Dakmor Salvage + Gitrog Monster.
	w := DefaultWeightsForArchetype(ArchetypeLandsMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 0.6 {
		t.Errorf("LandsMatter ComboProximity should be ≥ 0.6 (Scapeshift / Gitrog); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("LandsMatter ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

// -----------------------------------------------------------------------------
// Blink: CardAdvantage stays highest; exile + combo rise.
// -----------------------------------------------------------------------------

func TestBlinkWeights_CardAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeBlink)
	a := w.AsArray()
	idxCA := 1 // CardAdvantage slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxCA {
		t.Fatalf("Blink's highest weight should be CardAdvantage (idx %d), got idx %d val %.2f",
			idxCA, bestIdx, best)
	}
}

func TestBlinkWeights_ExileZoneAssetsBoosted(t *testing.T) {
	// Yorion Sky Nomad's exile pile IS the deck's working resource;
	// Brago King Eternal exiles and returns; Soulherder accumulates
	// +1/+1 from exiled-and-returned creatures.
	w := DefaultWeightsForArchetype(ArchetypeBlink)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ExileZoneAssets < 0.9 {
		t.Errorf("Blink ExileZoneAssets should be ≥ 0.9 (Yorion / Soulherder / Brago); got %.2f",
			w.ExileZoneAssets)
	}
	if w.ExileZoneAssets <= mid.ExileZoneAssets {
		t.Errorf("Blink ExileZoneAssets (%.2f) should exceed midrange (%.2f)",
			w.ExileZoneAssets, mid.ExileZoneAssets)
	}
}

func TestBlinkWeights_ComboProximityBoosted(t *testing.T) {
	// Felidar Guardian + Saheeli Rai (infinite tokens), Restoration
	// Angel + Kiki-Jiki (infinite haste copies), Emiel + Tundra Wolves
	// family, Deadeye Navigator + Peregrine Drake.
	w := DefaultWeightsForArchetype(ArchetypeBlink)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 0.9 {
		t.Errorf("Blink ComboProximity should be ≥ 0.9 (Felidar+Saheeli / Resto+Kiki); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("Blink ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

// -----------------------------------------------------------------------------
// Distinct-from-midrange invariant for all three
// -----------------------------------------------------------------------------

func TestArchetypeWeightsRound4_AllThreeDistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	for _, arch := range []string{ArchetypeStax, ArchetypeLandsMatter, ArchetypeBlink} {
		w := DefaultWeightsForArchetype(arch)
		if w == mid {
			t.Errorf("archetype %q weights should not equal midrange after round 4", arch)
		}
	}
}
