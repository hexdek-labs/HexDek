package hat

import (
	"testing"
)

// lifegain_profile_r60_test.go — R60 follow-up tune for Lifegain.
// Pre-fix the profile had LifeResource=1.8 (correctly highest but
// under-anchored vs. the 2.0 signature pattern every other custom
// profile uses), BoardPresence=0.9 (midrange-tier despite Ajani's
// Pridemate / Karlov / Heliod growing with lifegain), and ThreatExposure
// =0.6 (ignored the chained dependency on lifelink creatures).

func TestLifegainWeights_LifeResourceAnchoredAt2(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	if w.LifeResource < 2.0 {
		t.Errorf("Lifegain LifeResource should anchor at 2.0 (signature pattern); got %.2f",
			w.LifeResource)
	}
}

func TestLifegainWeights_LifeResourceRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	a := w.AsArray()
	idxLife := 3 // LifeResource slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxLife {
		t.Fatalf("Lifegain's highest weight should be LifeResource (idx %d), got idx %d val %.2f",
			idxLife, bestIdx, best)
	}
}

func TestLifegainWeights_BoardPresenceBoosted(t *testing.T) {
	// Pridemate / Karlov / Heliod creatures scale with lifegain — they
	// ARE the closer. Should clear midrange's BoardPresence.
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < 1.1 {
		t.Errorf("Lifegain BoardPresence should be ≥ 1.1 (Pridemate/Karlov/Heliod scaling); got %.2f",
			w.BoardPresence)
	}
	if w.BoardPresence <= mid.BoardPresence {
		t.Errorf("Lifegain BoardPresence (%.2f) should exceed midrange (%.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
}

func TestLifegainWeights_ThreatExposureBoosted(t *testing.T) {
	// Soul Warden / Heliod / Bishop of Wings / Trelasarra / Crested
	// Sunmare are the lifelink/triggered-lifegain engine. Losing one
	// stalls the wincon chain. ThreatExposure should be ≥ midrange.
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ThreatExposure < 0.9 {
		t.Errorf("Lifegain ThreatExposure should be ≥ 0.9 (protect lifelink engine); got %.2f",
			w.ThreatExposure)
	}
	if w.ThreatExposure < mid.ThreatExposure {
		t.Errorf("Lifegain ThreatExposure (%.2f) should at least match midrange (%.2f)",
			w.ThreatExposure, mid.ThreatExposure)
	}
}

// Sanity: DrainEngine remains at the R60-era tuned value so the
// Sanguine Bond / Vito / Defiler of Flesh drain wincon stays
// represented. Pinning the existing value flags any future regression.
func TestLifegainWeights_DrainEngineRemainsTuned(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	if w.DrainEngine < 1.0 {
		t.Errorf("Lifegain DrainEngine should remain ≥ 1.0 (Sanguine Bond / Vito); got %.2f",
			w.DrainEngine)
	}
}

func TestLifegainWeights_DistinctFromMidrange(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeLifegain)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w == m {
		t.Fatal("Lifegain weights should not equal midrange")
	}
}
