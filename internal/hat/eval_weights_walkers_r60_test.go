package hat

import (
	"testing"
)

// eval_weights_walkers_r60_test.go — Round 5 of the R60 archetype-
// weight audit. Planeswalkers (ArchetypeSuperfriends) sat too close to
// the generic midrange shape on the two axes that actually drive
// walker wins and losses: CardAdvantage (every +1 is a draw, every
// ultimate is a game-winning card-advantage swing) and ThreatExposure
// (a single Hero's Downfall / commander connect kills the walker and
// resets the entire +1 ladder). This file pins those bumps + keeps
// PlaneswalkerProgress as the signature dimension.

func TestSuperfriendsWeights_PlaneswalkerProgressRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeSuperfriends)
	a := w.AsArray()
	idxPW := 17 // PlaneswalkerProgress slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxPW {
		t.Fatalf("Superfriends' highest weight should be PlaneswalkerProgress (idx %d), got idx %d val %.2f",
			idxPW, bestIdx, best)
	}
}

func TestSuperfriendsWeights_CardAdvantageBoosted(t *testing.T) {
	// Walker +1s are draw spells (Jace's Ingenuity, Tamiyo, Dack
	// Fayden, Tezzeret the Seeker, Ajani Steadfast); ultimates are
	// game-winning CA swings (Jace TMS mill, Karn Liberated restart,
	// Ugin board wipe, Liliana hand empty). Midrange-tier (1.0) and
	// the prior 1.2 both under-weighted the "tick toward ultimate"
	// win line.
	w := DefaultWeightsForArchetype(ArchetypeSuperfriends)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CardAdvantage < 1.5 {
		t.Errorf("Superfriends CardAdvantage should be ≥ 1.5 (walker +1s + ultimates); got %.2f",
			w.CardAdvantage)
	}
	if w.CardAdvantage <= mid.CardAdvantage {
		t.Errorf("Superfriends CardAdvantage (%.2f) should exceed midrange (%.2f)",
			w.CardAdvantage, mid.CardAdvantage)
	}
}

func TestSuperfriendsWeights_ThreatExposureBoosted(t *testing.T) {
	// Walkers die to creature attacks (loyalty redirect), to single
	// removal (Hero's Downfall, Vraska's Contempt, Anguished Unmaking,
	// Generous Gift), to bolts. Protecting walker loyalty (Heroic
	// Intervention, Teferi's Protection, fog effects, Doubling Season
	// loyalty floors) is the difference between an active ult and a
	// dead walker. 1.0 was a generic creature-deck rating; lifted near
	// Voltron-tier (1.4) since walker death is as devastating as
	// commander removal in Voltron.
	w := DefaultWeightsForArchetype(ArchetypeSuperfriends)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ThreatExposure < 1.4 {
		t.Errorf("Superfriends ThreatExposure should be ≥ 1.4 (protect walker loyalty); got %.2f",
			w.ThreatExposure)
	}
	if w.ThreatExposure <= mid.ThreatExposure {
		t.Errorf("Superfriends ThreatExposure (%.2f) should exceed midrange (%.2f)",
			w.ThreatExposure, mid.ThreatExposure)
	}
}

func TestSuperfriendsWeights_DistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	w := DefaultWeightsForArchetype(ArchetypeSuperfriends)
	if w == mid {
		t.Errorf("Superfriends weights should not equal midrange after round 5")
	}
}
