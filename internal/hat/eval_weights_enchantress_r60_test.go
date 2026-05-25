package hat

import (
	"testing"
)

// eval_weights_enchantress_r60_test.go — R60 Enchantments retune.
// Lifts CardAdvantage (constellation / Enchantress draws) and
// BoardPresence (enchantment density scales Sphere of Safety, All That
// Glitters, Sage's Reverie) above generic midrange while keeping
// EnchantmentSynergy=2.0 as the signature.

func TestEnchantressWeights_EnchantmentSynergyRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeEnchantress)
	a := w.AsArray()
	idxEnchSyn := 10 // EnchantmentSynergy slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxEnchSyn {
		t.Fatalf("Enchantress's highest weight should be EnchantmentSynergy (idx %d), got idx %d val %.2f",
			idxEnchSyn, bestIdx, best)
	}
}

func TestEnchantressWeights_CardAdvantageBoosted(t *testing.T) {
	// Eidolon of Blossoms, Setessan Champion, Argothian / Mesa /
	// Verduran / Femeref / Satyr Enchantress, Sythis Harvest's Hand,
	// Sram Senior Edificer, Enchantress's Presence — constellation
	// triggers and Enchantress draws make every enchantment cast a
	// card. The deck draws its whole library on a typical curve.
	w := DefaultWeightsForArchetype(ArchetypeEnchantress)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CardAdvantage < 1.5 {
		t.Errorf("Enchantress CardAdvantage should be ≥ 1.5 (constellation / Enchantress draws); got %.2f",
			w.CardAdvantage)
	}
	if w.CardAdvantage <= mid.CardAdvantage {
		t.Errorf("Enchantress CardAdvantage (%.2f) should exceed midrange (%.2f)",
			w.CardAdvantage, mid.CardAdvantage)
	}
}

func TestEnchantressWeights_BoardPresenceBoosted(t *testing.T) {
	// Sphere of Safety scales attack-cost fog by # enchantments;
	// Sage's Reverie / All That Glitters / Ethereal Armor scale
	// creature size by # enchantments; Greater Auramancy protects
	// the whole density layer. Enchantment density IS the board.
	w := DefaultWeightsForArchetype(ArchetypeEnchantress)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < 1.1 {
		t.Errorf("Enchantress BoardPresence should be ≥ 1.1 (Sphere of Safety / All That Glitters / density); got %.2f",
			w.BoardPresence)
	}
	if w.BoardPresence <= mid.BoardPresence {
		t.Errorf("Enchantress BoardPresence (%.2f) should exceed midrange (%.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
}

func TestEnchantressWeights_DistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	w := DefaultWeightsForArchetype(ArchetypeEnchantress)
	if w == mid {
		t.Errorf("Enchantress weights should not equal midrange after R60 retune")
	}
}

func TestEnchantressWeights_SignatureSpreadOverSecondary(t *testing.T) {
	// EnchantmentSynergy is the signature anchor at 2.0; the boosted
	// secondaries (CardAdvantage, BoardPresence) must remain strictly
	// below it so the archetype identity stays legible — guards
	// against an accidental flat top.
	w := DefaultWeightsForArchetype(ArchetypeEnchantress)
	if w.CardAdvantage >= w.EnchantmentSynergy {
		t.Errorf("Enchantress CardAdvantage (%.2f) should stay below EnchantmentSynergy (%.2f)",
			w.CardAdvantage, w.EnchantmentSynergy)
	}
	if w.BoardPresence >= w.EnchantmentSynergy {
		t.Errorf("Enchantress BoardPresence (%.2f) should stay below EnchantmentSynergy (%.2f)",
			w.BoardPresence, w.EnchantmentSynergy)
	}
}
