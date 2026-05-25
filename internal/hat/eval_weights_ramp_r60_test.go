package hat

import (
	"testing"
)

// eval_weights_ramp_r60_test.go — R60 Ramp retune. Pre-fix Ramp was
// indistinguishable from a generic "we play lands" profile on every
// axis except ManaAdvantage: BoardPresence 0.6 ranked the deck's
// big-creature finishers (Craterhoof / Vorinclex / Eldrazi titans)
// below midrange's 1.0, LifeResource 0.5 made the hat eager to trade
// life for mana even though ramp needs to survive the ramp turns, and
// ManaAdvantage 1.8 was the same under-anchor bug Lifegain had pre-
// R60 (every other custom profile pins its signature dimension at the
// 2.0 anchor).

func TestRampWeights_ManaAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeRamp)
	a := w.AsArray()
	const idxMana = 2 // ManaAdvantage slot in AsArray
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxMana {
		t.Fatalf("Ramp's highest weight should be ManaAdvantage (idx %d), got idx %d val %.2f",
			idxMana, bestIdx, best)
	}
}

func TestRampWeights_ManaAdvantageAnchoredAt2(t *testing.T) {
	// Every other custom profile anchors its signature dimension at 2.0
	// (Storm/Mill/Voltron/Aristocrats/Reanimator/Spellslinger/Tribal/
	// Stax/Selfmill/Enchantress/Artifacts/Lifegain). Ramp's 1.8 was the
	// odd one out.
	w := DefaultWeightsForArchetype(ArchetypeRamp)
	if w.ManaAdvantage < 2.0 {
		t.Errorf("Ramp ManaAdvantage should anchor at 2.0 like other custom profiles; got %.2f",
			w.ManaAdvantage)
	}
}

func TestRampWeights_BoardPresenceBigCreatureDensity(t *testing.T) {
	// Ramp dumps fatties: Craterhoof Behemoth, Vorinclex Voice of
	// Hunger, Avenger of Zendikar, Ulamog / Kozilek / Emrakul,
	// Worldspine Wurm, End-Raze Forerunners, Hornet Queen, Apex
	// Devastator. A ramp deck with 9 mana and no body has failed its
	// gameplan. Should sit above midrange's 1.0.
	w := DefaultWeightsForArchetype(ArchetypeRamp)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < 1.0 {
		t.Errorf("Ramp BoardPresence should be ≥ 1.0 (big_creature_density: Craterhoof / Vorinclex / Eldrazi); got %.2f",
			w.BoardPresence)
	}
	if w.BoardPresence <= mid.BoardPresence {
		t.Errorf("Ramp BoardPresence (%.2f) should exceed midrange (%.2f) — big finishers matter MORE than midrange creatures",
			w.BoardPresence, mid.BoardPresence)
	}
}

func TestRampWeights_LifeResourceNeutral(t *testing.T) {
	// Neutral = midrange-tier. Ramp isn't Storm/Mill — it can't trade
	// life freely because it needs to SURVIVE the early ramp turns
	// before the fatty lands. But it's also not Lifegain (2.0) or Aggro
	// (0.8 with low-life-as-clock framing). 0.5 was too low; should be
	// ~= midrange.
	w := DefaultWeightsForArchetype(ArchetypeRamp)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.LifeResource < mid.LifeResource-0.05 {
		t.Errorf("Ramp LifeResource should be ≥ midrange-neutral (%.2f); got %.2f",
			mid.LifeResource, w.LifeResource)
	}
	if w.LifeResource > mid.LifeResource+0.05 {
		t.Errorf("Ramp LifeResource should be midrange-neutral (%.2f), not boosted; got %.2f",
			mid.LifeResource, w.LifeResource)
	}
}

func TestRampWeights_DistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	w := DefaultWeightsForArchetype(ArchetypeRamp)
	if w == mid {
		t.Fatalf("Ramp weights should not equal midrange after R60 retune")
	}
}
