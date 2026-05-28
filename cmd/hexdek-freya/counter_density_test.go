package main

import (
	"strings"
	"testing"
)

// r60 add: computeCounterDensity walks report.Profiles and
// classifies each PERMANENT card as a +1/+1 counter producer
// and/or payoff. Theme thresholds:
//
//   producers + payoffs >= 5  → CountersMatterTheme
//   producers + payoffs >= 10 → CountersMatterStrong
//
// Instants/sorceries excluded by design — the theme framing is
// about persistent board presence, not one-shot pumps.

// stubCounterOracle builds an oracleDB where each card has an
// oracle text body keyed by name. Mirrors the existing
// stubOracleWithText pattern.
func stubCounterOracle(entries map[string]string) *oracleDB {
	db := &oracleDB{byName: map[string]*oracleEntry{}}
	for name, text := range entries {
		e := &oracleEntry{Name: name, OracleText: text}
		db.byName[normalizeName(name)] = e
		db.byName[strings.ToLower(strings.TrimSpace(name))] = e
	}
	return db
}

// counterReport builds a minimal FreyaReport with the supplied
// CardProfiles. computeCounterDensity walks Profiles directly so no
// Roles plumbing is needed.
func counterReport(profiles ...CardProfile) *FreyaReport {
	return &FreyaReport{Profiles: profiles}
}

// TestIsCounterProducerOracle_HangarbackWalker pins the anchor
// "enters with N +1/+1 counters" shape.
func TestIsCounterProducerOracle_HangarbackWalker(t *testing.T) {
	ot := "hangarback walker enters with x +1/+1 counters on it, where x is the amount of mana spent to cast it."
	if !isCounterProducerOracle(ot) {
		t.Errorf("Hangarback Walker should be a counter producer")
	}
}

// TestIsCounterProducerOracle_PutCounterPlacement pins the
// active-placement shape.
func TestIsCounterProducerOracle_PutCounterPlacement(t *testing.T) {
	ot := "whenever a creature you control attacks, put a +1/+1 counter on it."
	if !isCounterProducerOracle(ot) {
		t.Errorf("attack-trigger placement should be a counter producer")
	}
}

// TestIsCounterPayoffOracle_HardenedScales pins the canonical
// multiplier replacement: "that many plus one".
func TestIsCounterPayoffOracle_HardenedScales(t *testing.T) {
	ot := "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on it instead."
	if !isCounterPayoffOracle(ot) {
		t.Errorf("Hardened Scales should be a counter payoff")
	}
}

// TestIsCounterPayoffOracle_DoublingSeason pins the "twice that many"
// shape — Doubling Season / Branching Evolution family.
func TestIsCounterPayoffOracle_DoublingSeason(t *testing.T) {
	ot := "if an effect would create one or more tokens under your control, it creates twice that many of those tokens instead. if an effect would put one or more +1/+1 counters on a permanent you control, it puts twice that many +1/+1 counters on that permanent instead."
	if !isCounterPayoffOracle(ot) {
		t.Errorf("Doubling Season should be a counter payoff")
	}
}

// TestIsCounterPayoffOracle_MasterBiomancer pins the "additional
// counter" shape — the Master-Biomancer-style amplifier.
func TestIsCounterPayoffOracle_MasterBiomancer(t *testing.T) {
	ot := "each other creature you control enters with an additional +1/+1 counter on it for each +1/+1 counter on master biomancer."
	if !isCounterPayoffOracle(ot) {
		t.Errorf("Master Biomancer should be a counter payoff")
	}
}

// TestIsCounterPayoffOracle_ModifiedShorthand pins the "modified
// creature" payoff shape — Glistening Dawn / Skrelv's Hive family.
func TestIsCounterPayoffOracle_ModifiedShorthand(t *testing.T) {
	ot := "at the beginning of combat on your turn, each modified creature you control gets +1/+1 until end of turn."
	if !isCounterPayoffOracle(ot) {
		t.Errorf("modified-creature payoff should be a counter payoff")
	}
}

// TestIsCounterPayoffOracle_VanillaCreatureNotPayoff pins the
// negative: a vanilla creature with "+1/+1" in its stat line (not
// oracle) doesn't trigger payoff detection. (Stat lines aren't in
// the oracle text body so this is structurally guaranteed, but the
// test pins the behavior.)
func TestIsCounterPayoffOracle_VanillaCreatureNotPayoff(t *testing.T) {
	if isCounterPayoffOracle("flying") {
		t.Errorf("vanilla flying creature should NOT be a counter payoff")
	}
	if isCounterPayoffOracle("trample") {
		t.Errorf("vanilla trample creature should NOT be a counter payoff")
	}
}

// TestIsCounterRelevantPermanent_SkipsLandsAndInstants pins the
// permanent-type gate: lands and instants/sorceries don't qualify
// regardless of oracle text.
func TestIsCounterRelevantPermanent_SkipsLandsAndInstants(t *testing.T) {
	land := CardProfile{Name: "Llanowar Reborn", TypeLine: "Land", IsLand: true}
	if isCounterRelevantPermanent(land) {
		t.Errorf("land should not be counter-relevant permanent")
	}
	instant := CardProfile{Name: "Triumph of the Hordes", TypeLine: "Sorcery"}
	if isCounterRelevantPermanent(instant) {
		t.Errorf("sorcery should not be counter-relevant permanent")
	}
	inst := CardProfile{Name: "Inspiring Call", TypeLine: "Instant"}
	if isCounterRelevantPermanent(inst) {
		t.Errorf("instant should not be counter-relevant permanent")
	}
	creature := CardProfile{Name: "Hangarback Walker", TypeLine: "Artifact Creature - Construct"}
	if !isCounterRelevantPermanent(creature) {
		t.Errorf("artifact creature should be counter-relevant permanent")
	}
	enchant := CardProfile{Name: "Hardened Scales", TypeLine: "Enchantment"}
	if !isCounterRelevantPermanent(enchant) {
		t.Errorf("enchantment should be counter-relevant permanent")
	}
}

// TestComputeCounterDensity_DedicatedDeckHitsStrong pins the
// end-to-end pipeline on a dedicated counters-tribal deck: 6
// producers + 5 payoffs (total 11) should set CountersMatterStrong.
func TestComputeCounterDensity_DedicatedDeckHitsStrong(t *testing.T) {
	profiles := []CardProfile{
		// Producers (creatures that ETB with counters or place them)
		{Name: "Hangarback Walker", TypeLine: "Artifact Creature"},
		{Name: "Walking Ballista", TypeLine: "Artifact Creature"},
		{Name: "Champion of Lambholt", TypeLine: "Creature"},
		{Name: "Avenger of Zendikar", TypeLine: "Creature"},
		{Name: "Bramble Sovereign", TypeLine: "Creature"},
		{Name: "Tuskguard Captain", TypeLine: "Creature"},
		// Payoffs (multipliers + scale-with)
		{Name: "Hardened Scales", TypeLine: "Enchantment"},
		{Name: "Doubling Season", TypeLine: "Enchantment"},
		{Name: "Branching Evolution", TypeLine: "Enchantment"},
		{Name: "Conclave Mentor", TypeLine: "Creature"},
		{Name: "Cathars' Crusade", TypeLine: "Enchantment"},
	}
	oracle := stubCounterOracle(map[string]string{
		"Hangarback Walker":    "hangarback walker enters with x +1/+1 counters on it.",
		"Walking Ballista":     "walking ballista enters with x +1/+1 counters on it. remove a +1/+1 counter from this: it deals 1 damage to any target.",
		"Champion of Lambholt": "creatures you control with power greater than champion of lambholt's power can't be blocked. whenever another creature enters under your control, put a +1/+1 counter on champion of lambholt.",
		"Avenger of Zendikar":  "when avenger of zendikar enters, create six 0/1 green plant tokens. landfall — at the beginning of combat on your turn, put a +1/+1 counter on each plant you control.",
		"Bramble Sovereign":    "whenever another nontoken creature enters under your control, you may pay {1}{g}. when you do, create a token that's a copy of that creature. that token enters with an additional +1/+1 counter on it.",
		"Tuskguard Captain":    "creatures you control with a +1/+1 counter on them have trample. whenever tuskguard captain attacks, put a +1/+1 counter on target creature you control.",
		"Hardened Scales":      "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on it instead.",
		"Doubling Season":      "if an effect would put one or more +1/+1 counters on a permanent you control, it puts twice that many +1/+1 counters on that permanent instead.",
		"Branching Evolution":  "if one or more +1/+1 counters would be put on a creature you control, twice that many +1/+1 counters are put on that creature instead.",
		"Conclave Mentor":      "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on it instead.",
		"Cathars' Crusade":     "whenever a creature enters under your control, put a +1/+1 counter on each creature you control.",
	})

	dp := &DeckProfile{}
	computeCounterDensity(dp, counterReport(profiles...), oracle)

	totalProducers := len(dp.CounterProducers)
	totalPayoffs := len(dp.CounterPayoffs)
	if totalProducers < 5 {
		t.Errorf("expected >=5 producers; got %d (%v)", totalProducers, dp.CounterProducers)
	}
	if totalPayoffs < 4 {
		t.Errorf("expected >=4 payoffs; got %d (%v)", totalPayoffs, dp.CounterPayoffs)
	}
	if !dp.CountersMatterTheme {
		t.Errorf("dedicated counter deck (%d+%d) should set CountersMatterTheme",
			totalProducers, totalPayoffs)
	}
	if !dp.CountersMatterStrong {
		t.Errorf("dedicated counter deck (%d+%d) should set CountersMatterStrong (>=10 total)",
			totalProducers, totalPayoffs)
	}
}

// TestComputeCounterDensity_BelowFloorNoTheme pins the lower
// threshold: 4 producers + 0 payoffs (total 4 < 5) does NOT set
// CountersMatterTheme. Guards against incidental Llanowar-Reborn
// false positives.
func TestComputeCounterDensity_BelowFloorNoTheme(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Stonecoil Serpent", TypeLine: "Artifact Creature"},
		{Name: "Hangarback Walker", TypeLine: "Artifact Creature"},
		{Name: "Servant of the Scale", TypeLine: "Creature"},
		{Name: "Walking Ballista", TypeLine: "Artifact Creature"},
		// Plus 90+ unrelated cards in a real deck — modeled as one
		// unrelated card here since computeCounterDensity is
		// independent of the deck's total size.
		{Name: "Counterspell", TypeLine: "Instant"},
	}
	oracle := stubCounterOracle(map[string]string{
		"Stonecoil Serpent":    "stonecoil serpent enters with x +1/+1 counters on it.",
		"Hangarback Walker":    "hangarback walker enters with x +1/+1 counters on it.",
		"Servant of the Scale": "servant of the scale enters with x +1/+1 counters on it.",
		"Walking Ballista":     "walking ballista enters with x +1/+1 counters on it.",
		"Counterspell":         "counter target spell.",
	})

	dp := &DeckProfile{}
	computeCounterDensity(dp, counterReport(profiles...), oracle)

	if dp.CountersMatterTheme {
		t.Errorf("4 producers below floor should NOT set CountersMatterTheme (got producers=%v)",
			dp.CounterProducers)
	}
	if dp.CountersMatterStrong {
		t.Errorf("4 producers below floor should NOT set CountersMatterStrong")
	}
	// Counterspell (instant) must NOT appear — permanent-type gate.
	for _, name := range dp.CounterProducers {
		if name == "Counterspell" {
			t.Errorf("instant should not appear as a counter producer; got %v", dp.CounterProducers)
		}
	}
}

// TestComputeCounterDensity_AtFloorTriggersTheme pins the boundary:
// exactly 5 total (producers + payoffs) DOES set CountersMatterTheme.
// The threshold is >=5 not >5.
func TestComputeCounterDensity_AtFloorTriggersTheme(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Hangarback Walker", TypeLine: "Artifact Creature"},
		{Name: "Walking Ballista", TypeLine: "Artifact Creature"},
		{Name: "Champion of Lambholt", TypeLine: "Creature"},
		{Name: "Tuskguard Captain", TypeLine: "Creature"},
		{Name: "Hardened Scales", TypeLine: "Enchantment"},
	}
	oracle := stubCounterOracle(map[string]string{
		"Hangarback Walker":    "hangarback walker enters with x +1/+1 counters on it.",
		"Walking Ballista":     "walking ballista enters with x +1/+1 counters on it.",
		"Champion of Lambholt": "whenever another creature enters under your control, put a +1/+1 counter on champion of lambholt.",
		"Tuskguard Captain":    "whenever tuskguard captain attacks, put a +1/+1 counter on target creature you control.",
		"Hardened Scales":      "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on it instead.",
	})

	dp := &DeckProfile{}
	computeCounterDensity(dp, counterReport(profiles...), oracle)

	total := len(dp.CounterProducers) + len(dp.CounterPayoffs)
	if total < 5 {
		t.Fatalf("test fixture wrong — expected at least 5 detected, got %d (prod=%v pay=%v)",
			total, dp.CounterProducers, dp.CounterPayoffs)
	}
	if !dp.CountersMatterTheme {
		t.Errorf("exactly-floor total (%d) should set CountersMatterTheme (>= threshold)", total)
	}
	if dp.CountersMatterStrong {
		t.Errorf("at-floor total (%d) should NOT set CountersMatterStrong (requires >=10)", total)
	}
}

// TestComputeCounterDensity_BothListsForDualRoleCards pins that
// cards which BOTH produce and scale (Hangarback Walker enters
// with counters AND dies for tokens equal to counters; Walking
// Ballista enters with counters AND pays them out for damage)
// appear in BOTH lists. Hangarback Walker's oracle text doesn't
// have an explicit "for each +1/+1 counter on it" payoff clause,
// but Walking Ballista's "remove a +1/+1 counter from this: it
// deals 1 damage to any target" hits the "remove a +1/+1 counter"
// payoff cue.
func TestComputeCounterDensity_BothListsForDualRoleCards(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Walking Ballista", TypeLine: "Artifact Creature"},
	}
	oracle := stubCounterOracle(map[string]string{
		"Walking Ballista": "walking ballista enters with x +1/+1 counters on it. remove a +1/+1 counter from this: it deals 1 damage to any target.",
	})

	dp := &DeckProfile{}
	computeCounterDensity(dp, counterReport(profiles...), oracle)

	if len(dp.CounterProducers) != 1 || dp.CounterProducers[0] != "Walking Ballista" {
		t.Errorf("Walking Ballista should appear as producer; got %v", dp.CounterProducers)
	}
	if len(dp.CounterPayoffs) != 1 || dp.CounterPayoffs[0] != "Walking Ballista" {
		t.Errorf("Walking Ballista should ALSO appear as payoff (dual-role); got %v", dp.CounterPayoffs)
	}
}

// TestComputeCounterDensity_NilOracleNoOp confirms the defensive
// guard: nil oracle leaves dp fields untouched.
func TestComputeCounterDensity_NilOracleNoOp(t *testing.T) {
	dp := &DeckProfile{}
	profiles := []CardProfile{
		{Name: "Hardened Scales", TypeLine: "Enchantment"},
	}
	computeCounterDensity(dp, counterReport(profiles...), nil)
	if dp.CountersMatterTheme || len(dp.CounterProducers) > 0 || len(dp.CounterPayoffs) > 0 {
		t.Errorf("nil oracle should leave fields untouched")
	}
}

// TestComputeCounterDensity_SortedOutput pins the alphabetical sort
// invariant: CounterProducers and CounterPayoffs come back sorted
// for stable downstream rendering.
func TestComputeCounterDensity_SortedOutput(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Walking Ballista", TypeLine: "Artifact Creature"},
		{Name: "Avenger of Zendikar", TypeLine: "Creature"},
		{Name: "Hangarback Walker", TypeLine: "Artifact Creature"},
	}
	oracle := stubCounterOracle(map[string]string{
		"Walking Ballista":    "walking ballista enters with x +1/+1 counters on it.",
		"Avenger of Zendikar": "landfall — at the beginning of combat on your turn, put a +1/+1 counter on each plant you control.",
		"Hangarback Walker":   "hangarback walker enters with x +1/+1 counters on it.",
	})

	dp := &DeckProfile{}
	computeCounterDensity(dp, counterReport(profiles...), oracle)

	// Should be alphabetical: Avenger, Hangarback, Walking
	if len(dp.CounterProducers) != 3 {
		t.Fatalf("expected 3 producers; got %d (%v)", len(dp.CounterProducers), dp.CounterProducers)
	}
	expected := []string{"Avenger of Zendikar", "Hangarback Walker", "Walking Ballista"}
	for i, name := range expected {
		if dp.CounterProducers[i] != name {
			t.Errorf("producers[%d] = %q, want %q (got full list %v)",
				i, dp.CounterProducers[i], name, dp.CounterProducers)
		}
	}
}
