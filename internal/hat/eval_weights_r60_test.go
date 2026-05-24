package hat

import (
	"testing"
)

// eval_weights_r60_test.go — R60 audit retune for ArchetypeStorm and
// ArchetypeMill. Pre-R60 both profiles ranked their archetype-defining
// dimension correctly (ComboProximity for both) but under-weighted
// load-bearing secondary dimensions: Storm's ActivationTempo /
// StackInteraction / GraveyardValue stayed at midrange-or-lower
// floors despite rituals + Past in Flames being core to the gameplan,
// and Mill's CardAdvantage / StackInteraction / DrainEngine sat at
// near-default values despite Mill being a Control variant that draws
// to find wincons and protects Bruvac/Tasha.

// -----------------------------------------------------------------------------
// Storm: ComboProximity stays highest; secondary dials rise above midrange
// -----------------------------------------------------------------------------

func TestStormWeights_ComboProximityRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	a := w.AsArray()
	idxCombo := 4 // ComboProximity slot per AsArray ordering
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxCombo {
		t.Fatalf("Storm's highest weight should be ComboProximity (idx %d), got idx %d val %.2f",
			idxCombo, bestIdx, best)
	}
}

func TestStormWeights_ActivationTempoBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ActivationTempo < 1.0 {
		t.Errorf("Storm ActivationTempo should be ≥ 1.0 (rituals + Aetherflux); got %.2f", w.ActivationTempo)
	}
	if w.ActivationTempo <= mid.ActivationTempo {
		t.Errorf("Storm ActivationTempo (%.2f) should exceed midrange (%.2f)",
			w.ActivationTempo, mid.ActivationTempo)
	}
}

func TestStormWeights_StackInteractionBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.StackInteraction < 1.2 {
		t.Errorf("Storm StackInteraction should be ≥ 1.2 (protect the chain); got %.2f",
			w.StackInteraction)
	}
	if w.StackInteraction <= mid.StackInteraction {
		t.Errorf("Storm StackInteraction (%.2f) should exceed midrange (%.2f)",
			w.StackInteraction, mid.StackInteraction)
	}
}

func TestStormWeights_GraveyardValueRecoveryBoosted(t *testing.T) {
	// Past in Flames / Yawgmoth's Will / Mizzix's Mastery second-storm
	// lines mean Storm's graveyard is a resource, not noise.
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	if w.GraveyardValue < 0.5 {
		t.Errorf("Storm GraveyardValue should be ≥ 0.5 (Past in Flames lines); got %.2f",
			w.GraveyardValue)
	}
}

// -----------------------------------------------------------------------------
// Mill: ComboProximity stays anchored; control-variant dials rise
// -----------------------------------------------------------------------------

func TestMillWeights_CardAdvantageBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMill)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CardAdvantage < 1.2 {
		t.Errorf("Mill CardAdvantage should be ≥ 1.2 (draw to find mill spells); got %.2f",
			w.CardAdvantage)
	}
	if w.CardAdvantage <= mid.CardAdvantage {
		t.Errorf("Mill CardAdvantage (%.2f) should exceed midrange (%.2f)",
			w.CardAdvantage, mid.CardAdvantage)
	}
}

func TestMillWeights_StackInteractionBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMill)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.StackInteraction < 1.0 {
		t.Errorf("Mill StackInteraction should be ≥ 1.0 (protect Bruvac/Tasha); got %.2f",
			w.StackInteraction)
	}
	if w.StackInteraction <= mid.StackInteraction {
		t.Errorf("Mill StackInteraction (%.2f) should exceed midrange (%.2f)",
			w.StackInteraction, mid.StackInteraction)
	}
}

func TestMillWeights_DrainEngineBoosted(t *testing.T) {
	// Notion Thief / Hullbreacher / Consecrated Sphinx — flip opponent
	// draws into our value (which doubles as removal-via-mill in Mill
	// shells running wheels).
	w := DefaultWeightsForArchetype(ArchetypeMill)
	if w.DrainEngine < 0.7 {
		t.Errorf("Mill DrainEngine should be ≥ 0.7 (wheel + draw-flip engines); got %.2f",
			w.DrainEngine)
	}
}

func TestMillWeights_ToolboxBreadthBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMill)
	if w.ToolboxBreadth < 0.6 {
		t.Errorf("Mill ToolboxBreadth should be ≥ 0.6 (varied milling packages); got %.2f",
			w.ToolboxBreadth)
	}
}

// -----------------------------------------------------------------------------
// Sanity: existing Freya-distinct test expectations remain satisfied
// -----------------------------------------------------------------------------

func TestStormWeights_StillBeatsMidrangeComboFloor(t *testing.T) {
	// TestDefaultWeightsForArchetype_FreyaArchetypesDistinct already
	// pins Storm.ComboProximity ≥ 1.5 — confirm the retune didn't break
	// it. The R60 tuning explicitly preserved ComboProximity at 1.8.
	w := DefaultWeightsForArchetype(ArchetypeStorm)
	if w.ComboProximity < 1.5 {
		t.Errorf("Storm ComboProximity must remain ≥ 1.5 (existing contract); got %.2f",
			w.ComboProximity)
	}
}

func TestMillWeights_StillBeatsMidrangeComboFloor(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMill)
	if w.ComboProximity < 1.0 {
		t.Errorf("Mill ComboProximity must remain ≥ 1.0 (existing contract); got %.2f",
			w.ComboProximity)
	}
}

// -----------------------------------------------------------------------------
// No silent regression to midrange
// -----------------------------------------------------------------------------

func TestArchetypeWeightsR60_StormDistinctFromMidrange(t *testing.T) {
	s := DefaultWeightsForArchetype(ArchetypeStorm)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)
	if s == m {
		t.Fatal("Storm weights should not equal midrange")
	}
}

func TestArchetypeWeightsR60_MillDistinctFromMidrange(t *testing.T) {
	mill := DefaultWeightsForArchetype(ArchetypeMill)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)
	if mill == m {
		t.Fatal("Mill weights should not equal midrange")
	}
}
