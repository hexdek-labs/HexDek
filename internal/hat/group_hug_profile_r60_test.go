package hat

import (
	"testing"
)

// group_hug_profile_r60_test.go — R60 follow-up. Group Hug
// (Phelddagrif / Zedruu the Greathearted / Kynaios and Tiro / Selvala
// Explorer Returned / Howling Mine packages) was previously a
// Freya-emitted archetype string that fell back to midrange weights.
// Tuned with CardAdvantage anchored at 2.0 (the deck taxes table-wide
// draws via Rhystic Study / Mystic Remora / Smothering Tithe), Life-
// Resource intentionally low (table leaves the pillow-fort player
// alone, taking hits to keep mana up is correct), BoardPresence
// neutral, ThreatExposure / StackInteraction / ToolboxBreadth bumped
// for the political survival game.

func TestGroupHugWeights_DistinctFromMidrange(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w == m {
		t.Fatal("GroupHug must not fall back to midrange — explicit profile required")
	}
}

func TestGroupHugWeights_CardAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	a := w.AsArray()
	idxCA := 1 // CardAdvantage slot per AsArray ordering
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxCA {
		t.Fatalf("GroupHug's highest weight should be CardAdvantage (idx %d), got idx %d val %.2f",
			idxCA, bestIdx, best)
	}
}

func TestGroupHugWeights_CardAdvantageAnchoredAt2(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	if w.CardAdvantage < 2.0 {
		t.Errorf("GroupHug CardAdvantage should anchor at 2.0 (signature pattern); got %.2f",
			w.CardAdvantage)
	}
}

func TestGroupHugWeights_LifeResourceBelowMidrange(t *testing.T) {
	// Pillow-fort survival means the table leaves us alone, so taking
	// damage to keep mana up for another Rhystic Study trigger is
	// correct. Same shape as Mill's R60 follow-up.
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.LifeResource >= mid.LifeResource {
		t.Errorf("GroupHug LifeResource (%.2f) must be < midrange (%.2f) — don't care if hit",
			w.LifeResource, mid.LifeResource)
	}
}

func TestGroupHugWeights_BoardPresenceNeutral(t *testing.T) {
	// Engines + a few creature finishers (Selvala, Phelddagrif) — not
	// a creature flood. Should sit at midrange ± noise, not above or
	// far below.
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < mid.BoardPresence-0.1 || w.BoardPresence > mid.BoardPresence+0.1 {
		t.Errorf("GroupHug BoardPresence (%.2f) should be neutral (within ±0.1 of midrange %.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
}

func TestGroupHugWeights_EnchantmentSynergyBoosted(t *testing.T) {
	// Rhystic Study / Mystic Remora / Smothering Tithe / Sylvan Library
	// / Mind's Eye-as-artifact — the tax engines that make the
	// "everyone draws" trade favorable are mostly enchantments.
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.EnchantmentSynergy <= mid.EnchantmentSynergy {
		t.Errorf("GroupHug EnchantmentSynergy (%.2f) should exceed midrange (%.2f) for Rhystic Study / Smothering Tithe",
			w.EnchantmentSynergy, mid.EnchantmentSynergy)
	}
}

func TestGroupHugWeights_StackInteractionBoosted(t *testing.T) {
	// Late-game kingmaker survival depends on countering the table's
	// inevitable combo attempts.
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.StackInteraction < mid.StackInteraction {
		t.Errorf("GroupHug StackInteraction (%.2f) should at least match midrange (%.2f)",
			w.StackInteraction, mid.StackInteraction)
	}
}

func TestGroupHugWeights_ToolboxBreadthBoosted(t *testing.T) {
	// Political deals / table-state response packages need flexibility.
	w := DefaultWeightsForArchetype(ArchetypeGroupHug)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ToolboxBreadth <= mid.ToolboxBreadth {
		t.Errorf("GroupHug ToolboxBreadth (%.2f) should exceed midrange (%.2f)",
			w.ToolboxBreadth, mid.ToolboxBreadth)
	}
}

// Sanity: the existing fallback-to-midrange test must still treat
// "group hug" as a recognized archetype — not as the unknown branch.
func TestGroupHugWeights_RecognizedByDefaultWeightsForArchetype(t *testing.T) {
	w := DefaultWeightsForArchetype("group hug") // string literal — not the constant
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w == mid {
		t.Fatal("the literal Freya archetype string 'group hug' must dispatch to the new profile, not the midrange fallback")
	}
}
