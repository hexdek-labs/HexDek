package hat

import (
	"testing"
)

// eval_weights_burn_r60_test.go — R60 Burn-off-generic profile pins.
// Pre-fix Burn decks (Torbran, Heartless Hidetsugu, Toralf, Krenko)
// classified as Aggro / Midrange / Spellslinger fingerprints and
// inherited those profiles' weights, mis-pricing the three axes that
// define Burn: LifeResource (opponent life pressure), ComboProximity
// (Burn sequences, doesn't assemble), and ThreatExposure (direct-
// damage sources are the wincon).

// TestBurnWeights_LifeResourceIsSignatureDimension pins LifeResource
// as the single highest weight in the Burn profile. Per directive
// "Weight LifeResource(opponent_life) high" — Burn's whole gameplan is
// closing the life delta.
func TestBurnWeights_LifeResourceIsSignatureDimension(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeBurn)
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
		t.Fatalf("Burn's highest weight should be LifeResource (idx %d), got idx %d val %.2f",
			idxLife, bestIdx, best)
	}
	if w.LifeResource < 1.5 {
		t.Errorf("Burn LifeResource should be ≥ 1.5 (signature dim); got %.2f",
			w.LifeResource)
	}
}

// TestBurnWeights_LifeResourceExceedsAggro — Aggro also pressures life
// but trades through combat. Burn's life-delta focus is more singular
// (every damage spell, no creature-trade tax) so LifeResource should
// strictly exceed Aggro's 0.8.
func TestBurnWeights_LifeResourceExceedsAggro(t *testing.T) {
	burn := DefaultWeightsForArchetype(ArchetypeBurn)
	aggro := DefaultWeightsForArchetype(ArchetypeAggro)
	if burn.LifeResource <= aggro.LifeResource {
		t.Errorf("Burn LifeResource (%.2f) should exceed Aggro (%.2f) — Burn's whole gameplan is the life delta",
			burn.LifeResource, aggro.LifeResource)
	}
}

// TestBurnWeights_ComboProximityIsFloored — Burn sequences burn
// spells, it does NOT assemble combo pieces. The dimension should be
// pushed below every other archetype that actually combos (Combo at
// 2.0, Storm at 1.8, Mill at 1.4, Spellslinger at 1.0, Aristocrats
// at 1.0, Reanimator at 1.1, Blink at 1.0, LandsMatter at 0.7).
func TestBurnWeights_ComboProximityIsFloored(t *testing.T) {
	burn := DefaultWeightsForArchetype(ArchetypeBurn)
	if burn.ComboProximity > 0.2 {
		t.Errorf("Burn ComboProximity should be ≤ 0.2 (Burn sequences, doesn't assemble); got %.2f",
			burn.ComboProximity)
	}
	// Sanity: it should be at-or-below Aggro's floor (0.1). Aggro and
	// Voltron are the other "no combo plan" profiles.
	aggro := DefaultWeightsForArchetype(ArchetypeAggro)
	if burn.ComboProximity > aggro.ComboProximity {
		t.Errorf("Burn ComboProximity (%.2f) should not exceed Aggro's floor (%.2f) — both archetypes have no assembly plan",
			burn.ComboProximity, aggro.ComboProximity)
	}
}

// TestBurnWeights_ThreatExposureProtectsDamageSources — Burn's
// "threats" are the damage sources themselves (Bonecrusher Giant,
// Eidolon, Torbran multiplier, Toralf copy, Neheb post-combat mana).
// Losing one to a Path is losing a full damage windowʼs worth of
// pressure — must protect them more aggressively than a generic
// creature deck.
func TestBurnWeights_ThreatExposureProtectsDamageSources(t *testing.T) {
	burn := DefaultWeightsForArchetype(ArchetypeBurn)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	control := DefaultWeightsForArchetype(ArchetypeControl)
	if burn.ThreatExposure < 1.2 {
		t.Errorf("Burn ThreatExposure should be ≥ 1.2 (Torbran / Toralf / Bonecrusher / Eidolon protection); got %.2f",
			burn.ThreatExposure)
	}
	if burn.ThreatExposure <= mid.ThreatExposure {
		t.Errorf("Burn ThreatExposure (%.2f) should exceed midrange (%.2f)",
			burn.ThreatExposure, mid.ThreatExposure)
	}
	// Control's ThreatExposure (1.2) protects answers; Burn protects
	// the win itself, so Burn should exceed Control too.
	if burn.ThreatExposure <= control.ThreatExposure {
		t.Errorf("Burn ThreatExposure (%.2f) should exceed Control (%.2f) — Burn protects wincon pieces, Control protects answers",
			burn.ThreatExposure, control.ThreatExposure)
	}
}

// TestBurnWeights_DistinctFromMidrange — the whole point of adding
// Burn off generic. Pre-fix lookup returned midrange weights; this
// pins that the dispatch now resolves to a unique profile.
func TestBurnWeights_DistinctFromMidrange(t *testing.T) {
	burn := DefaultWeightsForArchetype(ArchetypeBurn)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if burn == mid {
		t.Fatalf("Burn weights must not equal midrange (the whole point of adding the profile)")
	}
}

// TestBurnWeights_RegisteredInDispatch — the string lookup must hit.
// A typo in the map key (e.g. "burn " with trailing space) would
// silently fall through to midrange.
func TestBurnWeights_RegisteredInDispatch(t *testing.T) {
	if _, ok := archetypeWeights[ArchetypeBurn]; !ok {
		t.Fatalf("ArchetypeBurn (%q) is not registered in archetypeWeights map", ArchetypeBurn)
	}
}

// TestBurnWeights_LegacyMidrangeOverrideStillWorks — LegacyMidrangeOnly
// must override Burn just like every other archetype, for A/B baseline
// tournament runs.
func TestBurnWeights_LegacyMidrangeOverrideStillWorks(t *testing.T) {
	prev := LegacyMidrangeOnly
	defer func() { LegacyMidrangeOnly = prev }()
	LegacyMidrangeOnly = true
	if DefaultWeightsForArchetype(ArchetypeBurn) != archetypeWeights[ArchetypeMidrange] {
		t.Errorf("LegacyMidrangeOnly should collapse Burn lookup to midrange weights")
	}
}
