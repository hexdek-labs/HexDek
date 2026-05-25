package hat

import (
	"testing"
)

// eval_weights_round3_r60_test.go — Round 3 of the R60 archetype-
// weight audit (companion to PR #179 / #181 which retuned Storm + Mill
// and Voltron + Aristocrats). Final pass over the remaining tunable
// archetypes; picked Reanimator, Spellslinger, and Tribal as the three
// with the largest gameplan-vs-weight gap.

// -----------------------------------------------------------------------------
// Reanimator: GraveyardValue stays highest; combo / single-threat /
// activated-reanimator dials rise.
// -----------------------------------------------------------------------------

func TestReanimatorWeights_GraveyardValueRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeReanimator)
	a := w.AsArray()
	idxGY := 7 // GraveyardValue slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxGY {
		t.Fatalf("Reanimator's highest weight should be GraveyardValue (idx %d), got idx %d val %.2f",
			idxGY, bestIdx, best)
	}
}

func TestReanimatorWeights_ComboProximityBoosted(t *testing.T) {
	// Worldgorger Dragon + Animate Dead / Karmic Guide + Reveillark /
	// Karador persist loops are real combo lines.
	w := DefaultWeightsForArchetype(ArchetypeReanimator)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 1.0 {
		t.Errorf("Reanimator ComboProximity should be ≥ 1.0 (Worldgorger lines); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("Reanimator ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

func TestReanimatorWeights_ThreatExposureBoosted(t *testing.T) {
	// Same single-threat shape as Voltron — losing the reanimated
	// fatty to spot removal is the whole game.
	w := DefaultWeightsForArchetype(ArchetypeReanimator)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ThreatExposure < 1.1 {
		t.Errorf("Reanimator ThreatExposure should be ≥ 1.1 (single-threat plan); got %.2f",
			w.ThreatExposure)
	}
	if w.ThreatExposure <= mid.ThreatExposure {
		t.Errorf("Reanimator ThreatExposure (%.2f) should exceed midrange (%.2f)",
			w.ThreatExposure, mid.ThreatExposure)
	}
}

func TestReanimatorWeights_ActivationTempoBoosted(t *testing.T) {
	// Sheoldred The Apocalypse / Volrath / Karador / Lord Windgrace
	// are activated reanimators; dredge / surveil are activations too.
	w := DefaultWeightsForArchetype(ArchetypeReanimator)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ActivationTempo < 0.5 {
		t.Errorf("Reanimator ActivationTempo should be ≥ 0.5 (activated reanimators); got %.2f",
			w.ActivationTempo)
	}
	if w.ActivationTempo <= mid.ActivationTempo {
		t.Errorf("Reanimator ActivationTempo (%.2f) should exceed midrange (%.2f)",
			w.ActivationTempo, mid.ActivationTempo)
	}
}

// -----------------------------------------------------------------------------
// Spellslinger: CardAdvantage stays highest; combo / ritual / Isochron
// dials rise.
// -----------------------------------------------------------------------------

func TestSpellslingerWeights_CardAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeSpellslinger)
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
		t.Fatalf("Spellslinger's highest weight should be CardAdvantage (idx %d), got idx %d val %.2f",
			idxCA, bestIdx, best)
	}
}

func TestSpellslingerWeights_ComboProximityBoosted(t *testing.T) {
	// Mizzix-Aetherflux storm, Niv-Mizzet Parun loops, Dramatic Reversal
	// + Isochron Scepter — Spellslinger is half combo-shaped.
	w := DefaultWeightsForArchetype(ArchetypeSpellslinger)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 0.9 {
		t.Errorf("Spellslinger ComboProximity should be ≥ 0.9 (Aetherflux / Niv lines); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("Spellslinger ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

func TestSpellslingerWeights_ManaAdvantageBoosted(t *testing.T) {
	// Goblin Electromancer / Birgi / Baral / Jhoira keep spells flowing —
	// raw mana access shapes how many spells per turn we can chain.
	w := DefaultWeightsForArchetype(ArchetypeSpellslinger)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ManaAdvantage < 1.0 {
		t.Errorf("Spellslinger ManaAdvantage should be ≥ 1.0 (cost reduction / rituals); got %.2f",
			w.ManaAdvantage)
	}
	if w.ManaAdvantage <= mid.ManaAdvantage {
		t.Errorf("Spellslinger ManaAdvantage (%.2f) should exceed midrange (%.2f)",
			w.ManaAdvantage, mid.ManaAdvantage)
	}
}

func TestSpellslingerWeights_ActivationTempoBoosted(t *testing.T) {
	// Isochron Scepter is the canonical Spellslinger artifact.
	w := DefaultWeightsForArchetype(ArchetypeSpellslinger)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ActivationTempo < 0.6 {
		t.Errorf("Spellslinger ActivationTempo should be ≥ 0.6 (Isochron); got %.2f",
			w.ActivationTempo)
	}
	if w.ActivationTempo <= mid.ActivationTempo {
		t.Errorf("Spellslinger ActivationTempo (%.2f) should exceed midrange (%.2f)",
			w.ActivationTempo, mid.ActivationTempo)
	}
}

// -----------------------------------------------------------------------------
// Tribal: BoardPresence stays highest; draw / lord-synergy / commander
// dials rise.
// -----------------------------------------------------------------------------

func TestTribalWeights_BoardPresenceRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeTribal)
	a := w.AsArray()
	idxBP := 0 // BoardPresence slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxBP {
		t.Fatalf("Tribal's highest weight should be BoardPresence (idx %d), got idx %d val %.2f",
			idxBP, bestIdx, best)
	}
}

func TestTribalWeights_CardAdvantageBoosted(t *testing.T) {
	// Vanquisher's Banner / Herald's Horn / Beast Whisperer / Kindred
	// Discovery turn every tribal cast into a draw.
	w := DefaultWeightsForArchetype(ArchetypeTribal)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CardAdvantage < 0.9 {
		t.Errorf("Tribal CardAdvantage should be ≥ 0.9 (Vanquisher's Banner family); got %.2f",
			w.CardAdvantage)
	}
	// Tribal CardAdvantage can EQUAL midrange (the deck's draw is
	// situational, not engine-driven like Spellslinger). Just verify
	// the boost happened.
	if w.CardAdvantage < mid.CardAdvantage {
		t.Errorf("Tribal CardAdvantage (%.2f) should at least match midrange (%.2f)",
			w.CardAdvantage, mid.CardAdvantage)
	}
}

func TestTribalWeights_PartnerSynergyBoosted(t *testing.T) {
	// Lord effects (+1/+1 to all Goblins/Elves/Slivers) ARE the tribal
	// payoff. PartnerSynergy is the dial; 0.4 was way too low.
	w := DefaultWeightsForArchetype(ArchetypeTribal)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.PartnerSynergy < 0.7 {
		t.Errorf("Tribal PartnerSynergy should be ≥ 0.7 (lord effects); got %.2f",
			w.PartnerSynergy)
	}
	if w.PartnerSynergy <= mid.PartnerSynergy {
		t.Errorf("Tribal PartnerSynergy (%.2f) should exceed midrange (%.2f)",
			w.PartnerSynergy, mid.PartnerSynergy)
	}
}

func TestTribalWeights_CommanderProgressBoosted(t *testing.T) {
	// Edgar Markov / Reaper King / Krenko / Slimefoot / Karador —
	// tribal commanders are usually the win condition.
	w := DefaultWeightsForArchetype(ArchetypeTribal)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CommanderProgress < 1.1 {
		t.Errorf("Tribal CommanderProgress should be ≥ 1.1 (commander-engine wincons); got %.2f",
			w.CommanderProgress)
	}
	if w.CommanderProgress <= mid.CommanderProgress {
		t.Errorf("Tribal CommanderProgress (%.2f) should exceed midrange (%.2f)",
			w.CommanderProgress, mid.CommanderProgress)
	}
}

// -----------------------------------------------------------------------------
// Distinct-from-midrange + no-regression invariants
// -----------------------------------------------------------------------------

func TestArchetypeWeightsRound3_AllThreeDistinctFromMidrange(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	for _, arch := range []string{ArchetypeReanimator, ArchetypeSpellslinger, ArchetypeTribal} {
		w := DefaultWeightsForArchetype(arch)
		if w == mid {
			t.Errorf("archetype %q weights should not equal midrange after round 3", arch)
		}
	}
}
