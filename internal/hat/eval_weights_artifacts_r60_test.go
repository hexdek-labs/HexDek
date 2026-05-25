package hat

import (
	"testing"
)

// eval_weights_artifacts_r60_test.go — R60 audit retune of the Artifacts
// archetype. ArtifactSynergy=2.0 was already the signature anchor, but
// ManaAdvantage (rocks), ComboProximity (infinite mana via rocks), and
// BoardPresence (metalcraft) were close to midrange values despite being
// core to the deck. Pin the lifts so future round-N tunes can't regress
// them back to generic.

func TestArtifactsWeights_ArtifactSynergyRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeArtifacts)
	a := w.AsArray()
	idxAS := 9 // ArtifactSynergy slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxAS {
		t.Fatalf("Artifacts' highest weight should be ArtifactSynergy (idx %d), got idx %d val %.2f",
			idxAS, bestIdx, best)
	}
}

func TestArtifactsWeights_ManaAdvantageBoosted(t *testing.T) {
	// Sol Ring / Mana Crypt / Mana Vault / Grim Monolith / Basalt
	// Monolith / Thran Dynamo / signets / talismans — the rock package
	// IS the mana base.
	w := DefaultWeightsForArchetype(ArchetypeArtifacts)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ManaAdvantage < 1.3 {
		t.Errorf("Artifacts ManaAdvantage should be ≥ 1.3 (rock package); got %.2f",
			w.ManaAdvantage)
	}
	if w.ManaAdvantage <= mid.ManaAdvantage {
		t.Errorf("Artifacts ManaAdvantage (%.2f) should exceed midrange (%.2f)",
			w.ManaAdvantage, mid.ManaAdvantage)
	}
}

func TestArtifactsWeights_ComboProximityBoosted_InfiniteManaViaRocks(t *testing.T) {
	// Basalt Monolith + Forsaken Monument, Grim Monolith + Power Artifact /
	// Rings of Brighthearth, KCI + Scrap Trawler loops, Isochron Scepter
	// + Dramatic Reversal in artifact-mana shells — these are the deck's
	// actual win lines, not occasional wins.
	w := DefaultWeightsForArchetype(ArchetypeArtifacts)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 1.2 {
		t.Errorf("Artifacts ComboProximity should be ≥ 1.2 (infinite mana via rocks); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("Artifacts ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

func TestArtifactsWeights_BoardPresenceBoosted_Metalcraft(t *testing.T) {
	// Mox Opal counts battlefield artifacts; Cranial Plating scales with
	// artifact count; Etched Champion / Inkmoth Nexus / Arcbound Ravager
	// care about parallel artifact bodies. More board artifacts = more
	// of the rest of the deck functions.
	w := DefaultWeightsForArchetype(ArchetypeArtifacts)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < 1.0 {
		t.Errorf("Artifacts BoardPresence should be ≥ 1.0 (metalcraft / Mox Opal); got %.2f",
			w.BoardPresence)
	}
	// Midrange BoardPresence is 1.0; Artifacts should at least match it
	// since metalcraft makes the artifact count load-bearing.
	if w.BoardPresence < mid.BoardPresence {
		t.Errorf("Artifacts BoardPresence (%.2f) should at least match midrange (%.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
}

func TestArtifactsWeights_DistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	w := DefaultWeightsForArchetype(ArchetypeArtifacts)
	if w == mid {
		t.Fatalf("Artifacts weights should not equal midrange after R60 retune")
	}
}
