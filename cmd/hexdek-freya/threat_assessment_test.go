package main

import (
	"strings"
	"testing"
)

// TestHoserDB_AllHaveSeverity guards against future additions forgetting
// the Severity field — Severity is what the sort + critical-tag logic
// keys off, and a zero-valued entry would silently sort last.
func TestHoserDB_AllHaveSeverity(t *testing.T) {
	for i, h := range hoserDB {
		if h.Severity < hoserSeverityMinor || h.Severity > hoserSeverityCritical {
			t.Errorf("hoserDB[%d] %q has invalid Severity %d (want 1-3)", i, h.Hoser, h.Severity)
		}
	}
}

// makeProfileWithEffect returns a CardProfile carrying the given effect
// tag, used to drive condition detection in the threat-assessment tests
// without needing a full oracle text → ClassifyCard pipeline.
func makeProfileWithEffect(name string, effects ...string) CardProfile {
	return CardProfile{
		Name:     name,
		TypeLine: "Sorcery", // non-land so it counts in the analysis loop
		Effects:  effects,
	}
}

// TestComputeThreatAssessment_ReanimatorCondition asserts that archetype-
// flagged reanimator decks get the reanimator-tier hosers AND the
// graveyard-heavy hosers (the more general condition still applies).
func TestComputeThreatAssessment_ReanimatorCondition(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Reanimator"}
	report := &FreyaReport{
		Profiles: []CardProfile{
			// Enough recursion to trigger graveyard_heavy (8+).
			{Name: "Reanimate", TypeLine: "Sorcery", IsRecursion: true},
			{Name: "Animate Dead", TypeLine: "Enchantment", IsRecursion: true},
			{Name: "Necromancy", TypeLine: "Enchantment", IsRecursion: true},
			{Name: "Sevinne's Reclamation", TypeLine: "Sorcery", IsRecursion: true},
			{Name: "Victimize", TypeLine: "Sorcery", IsRecursion: true},
			{Name: "Unburial Rites", TypeLine: "Sorcery", IsRecursion: true},
			{Name: "Doomed Necromancer", TypeLine: "Creature", IsRecursion: true},
			{Name: "Living Death", TypeLine: "Sorcery", IsRecursion: true},
		},
	}
	computeThreatAssessment(dp, report)
	if !vulnerableToContains(dp, "Faerie Macabre") {
		t.Errorf("reanimator deck missing Faerie Macabre — got %v", dp.VulnerableTo)
	}
	if !vulnerableToContains(dp, "Bojuka Bog") {
		t.Errorf("reanimator deck missing Bojuka Bog — got %v", dp.VulnerableTo)
	}
	// Reanimator extends graveyard_heavy — Rest in Peace must still surface.
	if !vulnerableToContains(dp, "Rest in Peace") {
		t.Errorf("reanimator deck missing Rest in Peace (graveyard_heavy extension) — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_StormExplicitCondition asserts that a deck
// with a storm-finisher win line gets the storm-specific hosers AND the
// generic spellslinger hosers. Storm doesn't replace spellslinger — the
// two condition tiers stack.
func TestComputeThreatAssessment_StormExplicitCondition(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Spellslinger"} // not "storm" archetype-wise
	report := &FreyaReport{
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Aetherflux Reservoir"}, Class: ComboClassStormFinisher},
			},
		},
	}
	computeThreatAssessment(dp, report)
	if !vulnerableToContains(dp, "Eidolon of Rhetoric") {
		t.Errorf("storm deck missing Eidolon of Rhetoric — got %v", dp.VulnerableTo)
	}
	if !vulnerableToContains(dp, "Damping Sphere") {
		t.Errorf("storm deck missing Damping Sphere — got %v", dp.VulnerableTo)
	}
	if !vulnerableToContains(dp, "Deafening Silence") {
		t.Errorf("storm deck (also spellslinger) missing Deafening Silence — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_CommanderCentricCondition asserts that the
// Voltron / engine-commander vulnerability class surfaces the
// commander-targeting enchantments when dp.IsCommanderCentric is set.
func TestComputeThreatAssessment_CommanderCentricCondition(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype:   "Voltron",
		IsCommanderCentric: true,
	}
	report := &FreyaReport{}
	computeThreatAssessment(dp, report)
	want := []string{"Imprisoned in the Moon", "Song of the Dryads", "Darksteel Mutation"}
	for _, w := range want {
		if !vulnerableToContains(dp, w) {
			t.Errorf("commander-centric deck missing %q — got %v", w, dp.VulnerableTo)
		}
	}
}

// TestComputeThreatAssessment_CountersMatterCondition asserts that decks
// with 5+ +1/+1 counter producers (heuristic threshold) get the counters-
// matter hosers. Tested via the effect-tag path rather than archetype.
func TestComputeThreatAssessment_CountersMatterCondition(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Midrange"} // no counter in name
	report := &FreyaReport{
		Profiles: []CardProfile{
			makeProfileWithEffect("Hardened Scales", "counter_add"),
			makeProfileWithEffect("Doubling Season", "counter_add"),
			makeProfileWithEffect("Forgotten Ancient", "counter_add"),
			makeProfileWithEffect("Inexorable Tide", "proliferate"),
			makeProfileWithEffect("Karn's Bastion", "proliferate"),
		},
	}
	computeThreatAssessment(dp, report)
	if !vulnerableToContains(dp, "Solemnity") {
		t.Errorf("counters-matter deck missing Solemnity — got %v", dp.VulnerableTo)
	}
	if !vulnerableToContains(dp, "Hex Parasite") {
		t.Errorf("counters-matter deck missing Hex Parasite — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_WheelsCondition asserts the by-name wheel
// detection fires when 2+ canonical wheel spells are present.
func TestComputeThreatAssessment_WheelsCondition(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Midrange"}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Wheel of Fortune", TypeLine: "Sorcery"},
			{Name: "Windfall", TypeLine: "Sorcery"},
			{Name: "Time Spiral", TypeLine: "Sorcery"},
		},
	}
	computeThreatAssessment(dp, report)
	if !vulnerableToContains(dp, "Notion Thief") {
		t.Errorf("wheels deck missing Notion Thief — got %v", dp.VulnerableTo)
	}
	if !vulnerableToContains(dp, "Hullbreacher") {
		t.Errorf("wheels deck missing Hullbreacher — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_WheelsCondition_BelowThreshold asserts a
// deck with only one wheel doesn't trigger the wheels condition — the
// threshold is 2 to avoid surfacing the warning for incidental Windfall
// inclusions in non-wheel decks.
func TestComputeThreatAssessment_WheelsCondition_BelowThreshold(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Midrange"}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Wheel of Fortune", TypeLine: "Sorcery"},
		},
	}
	computeThreatAssessment(dp, report)
	if vulnerableToContains(dp, "Notion Thief") {
		t.Errorf("single-wheel deck should not trigger wheels condition — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_CriticalSortingAndTagging asserts the
// output ordering and the "(critical)" inline tag. A mixed-condition
// deck must have its critical hosers listed before its major / minor
// hosers, and critical entries must be marked.
func TestComputeThreatAssessment_CriticalSortingAndTagging(t *testing.T) {
	// Token-heavy deck: Massacre Wurm (critical) + Rakdos Charm (minor).
	dp := &DeckProfile{PrimaryArchetype: "Tokens"}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Goblin Spawning Engine", TypeLine: "Creature", Produces: []ResourceType{ResToken}},
			{Name: "Krenko, Mob Boss", TypeLine: "Creature", Produces: []ResourceType{ResToken}},
			{Name: "Krenko's Command", TypeLine: "Sorcery", Produces: []ResourceType{ResToken}},
			{Name: "Goblin Rabblemaster", TypeLine: "Creature", Produces: []ResourceType{ResToken}},
			{Name: "Empty the Warrens", TypeLine: "Sorcery", Produces: []ResourceType{ResToken}},
			{Name: "Dragon Fodder", TypeLine: "Sorcery", Produces: []ResourceType{ResToken}},
			{Name: "Goblin Instigator", TypeLine: "Creature", Produces: []ResourceType{ResToken}},
			{Name: "Mardu Ascendancy", TypeLine: "Enchantment", Produces: []ResourceType{ResToken}},
		},
	}
	computeThreatAssessment(dp, report)

	// Find positions of the critical (Massacre Wurm) vs minor (Rakdos
	// Charm) hosers in the output.
	posMassacre, posRakdos := -1, -1
	for i, v := range dp.VulnerableTo {
		if strings.HasPrefix(v, "Massacre Wurm") {
			posMassacre = i
		}
		if strings.HasPrefix(v, "Rakdos Charm") {
			posRakdos = i
		}
	}
	if posMassacre < 0 || posRakdos < 0 {
		t.Fatalf("expected both Massacre Wurm and Rakdos Charm in VulnerableTo, got %v", dp.VulnerableTo)
	}
	if posMassacre >= posRakdos {
		t.Errorf("critical hoser Massacre Wurm (pos %d) should sort before minor Rakdos Charm (pos %d): %v",
			posMassacre, posRakdos, dp.VulnerableTo)
	}
	// Massacre Wurm is critical (severity 3) — must carry the inline tag.
	if !strings.Contains(dp.VulnerableTo[posMassacre], "(critical)") {
		t.Errorf("critical hoser missing inline tag: %q", dp.VulnerableTo[posMassacre])
	}
	// Rakdos Charm is minor — must NOT carry the inline tag.
	if strings.Contains(dp.VulnerableTo[posRakdos], "(critical)") {
		t.Errorf("minor hoser should not carry critical tag: %q", dp.VulnerableTo[posRakdos])
	}
}

// vulnerableToContains returns true when any VulnerableTo entry begins
// with the given hoser name. The entries carry trailing " — reason"
// (and possibly "(critical)") text, so prefix-match is the right shape.
func vulnerableToContains(dp *DeckProfile, hoser string) bool {
	for _, v := range dp.VulnerableTo {
		if strings.HasPrefix(v, hoser+" ") {
			return true
		}
	}
	return false
}
