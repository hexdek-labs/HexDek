package main

import (
	"strings"
	"testing"
)

// archetype_equipment_voltron_r60_test.go — regressions for the
// Equipment-Voltron sub-archetype split off from generic Voltron.
//
// The generic Voltron fingerprint gates on `equipAuraCount >= 8 &&
// RoleProtection >= 0.06` — agnostic to whether the deck commits to
// Equipment or Auras. That coarse gate landed Bogles-style aura decks
// AND Bruenor / Akiri equipment shells on the same archetype tag,
// which matters because:
//
//   - Equipment carries an extra deck-building tax (equip-cost mana
//     per turn) but survives single-target bounce. The engine REQUIRES
//     payoff pieces (Puresteel Paladin / Sigarda's Aid / Sram /
//     Stoneforge Mystic) to be tractable; 8 equipment with no payoffs
//     is a midrange-with-toolbox shape, not Voltron.
//   - Aura-Voltron loses everything to a single removal spell but
//     wins faster off Bogles-style hexproof + ramping totem armor.
//
// Different gameplay shape → different MCTS weight profile downstream.
// The split lets the strategy bridge route the correct profile.
//
// Three layers of coverage:
//
//  1. cardIsEquipPayoff — unit-level detection of the canonical payoff
//     cards (curated names + oracle-text shapes).
//  2. buildClassifyContext — pins that equipmentCount, auraCount, and
//     equipTriggerPayoffCount are populated correctly.
//  3. ClassifyArchetype end-to-end — pins Equipment-Voltron lands when
//     the gates pass, AND that an aura-heavy or payoff-light variant
//     still falls through correctly.

// ---------------------------------------------------------------------------
// 1. cardIsEquipPayoff unit-level shape detection
// ---------------------------------------------------------------------------

func TestCardIsEquipPayoff_CuratedPuresteelPaladin(t *testing.T) {
	if !cardIsEquipPayoff("Puresteel Paladin", "whenever an equipment enters the battlefield under your control, you may draw a card.") {
		t.Error("Puresteel Paladin should be detected as an equip-payoff (curated)")
	}
}

func TestCardIsEquipPayoff_CuratedSigardasAid(t *testing.T) {
	if !cardIsEquipPayoff("Sigarda's Aid", "auras and equipment you control have flash.") {
		t.Error("Sigarda's Aid should be detected as an equip-payoff (curated)")
	}
}

func TestCardIsEquipPayoff_OracleTextEquipmentETB(t *testing.T) {
	// Non-curated card with the canonical "whenever an equipment enters"
	// trigger phrasing.
	got := cardIsEquipPayoff("Some Unknown Card",
		"whenever an equipment enters the battlefield under your control, scry 1.")
	if !got {
		t.Error("'whenever an equipment enters' should match the equip-payoff phrase scan")
	}
}

func TestCardIsEquipPayoff_OracleTextCostReduction(t *testing.T) {
	got := cardIsEquipPayoff("Heavy Arbalest", // not a real payoff, just shape
		"equipment spells you cast cost {1} less to cast.")
	if !got {
		t.Error("'equipment spells you cast cost' should match the equip-payoff phrase scan")
	}
}

func TestCardIsEquipPayoff_BareEquipmentRejected(t *testing.T) {
	// Whispersilk Cloak — pure Equipment, no payoff trigger, just an
	// equip cost and equipped-creature buff. NOT a payoff.
	got := cardIsEquipPayoff("Whispersilk Cloak",
		"equipped creature can't be blocked. equipped creature has shroud. equip {3}")
	if got {
		// "equipped creature" appears in the text. The phrase scan
		// must NOT match bare "equipped creature" — only the payoff
		// shapes ("equipped creature gets", "equipped creatures you
		// control") count. This is the load-bearing exclusion.
		t.Error("bare Equipment piece should NOT be classified as a payoff")
	}
}

func TestCardIsEquipPayoff_GenericArtifactRejected(t *testing.T) {
	got := cardIsEquipPayoff("Sol Ring", "{t}: add {c}{c}.")
	if got {
		t.Error("Sol Ring should NOT be classified as an equip-payoff")
	}
}

func TestCardIsEquipPayoff_EmptyText(t *testing.T) {
	if got := cardIsEquipPayoff("", ""); got {
		t.Error("empty text + empty name should return false")
	}
}

func TestCardIsEquipPayoff_NameMatchIsCaseInsensitive(t *testing.T) {
	if !cardIsEquipPayoff("PURESTEEL PALADIN", "") {
		t.Error("name match should be case-insensitive (curated lookup lowercases)")
	}
	if !cardIsEquipPayoff("  Sram, Senior Edificer  ", "") {
		t.Error("name match should trim whitespace")
	}
}

// ---------------------------------------------------------------------------
// 2. buildClassifyContext counter wiring
// ---------------------------------------------------------------------------

func TestBuildClassifyContext_EquipmentAndAuraCountsSplit(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Hammer of Nazahn", TypeLine: "Artifact — Equipment"},
		{Name: "Shadowspear", TypeLine: "Legendary Artifact — Equipment"},
		{Name: "Sword of Feast and Famine", TypeLine: "Artifact — Equipment"},
		{Name: "Pacifism", TypeLine: "Enchantment — Aura"},
		{Name: "Rancor", TypeLine: "Enchantment — Aura"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
		{Name: "Forest", TypeLine: "Basic Land — Forest"},
	}
	oracleText := map[string]string{
		"Hammer of Nazahn":          "equipped creature gets +2/+0 and has indestructible. equip {3}",
		"Shadowspear":               "equipped creature gets +1/+1, has trample and lifelink. equip {1}",
		"Sword of Feast and Famine": "equipped creature gets +2/+2 and has protection from black and from green. equip {2}",
		"Pacifism":                  "enchant creature. enchanted creature can't attack or block.",
		"Rancor":                    "enchant creature. enchanted creature gets +2/+0 and has trample.",
		"Sol Ring":                  "{t}: add {c}{c}.",
		"Forest":                    "{t}: add {g}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.equipmentCount != 3 {
		t.Errorf("equipmentCount=%d, want 3", ctx.equipmentCount)
	}
	if ctx.auraCount != 2 {
		t.Errorf("auraCount=%d, want 2", ctx.auraCount)
	}
	if ctx.equipAuraCount != 5 {
		t.Errorf("equipAuraCount (legacy combined) should still equal 5; got %d", ctx.equipAuraCount)
	}
}

func TestBuildClassifyContext_EquipTriggerPayoffCount(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Puresteel Paladin", TypeLine: "Creature — Human Knight"},
		{Name: "Sigarda's Aid", TypeLine: "Enchantment"},
		{Name: "Sram, Senior Edificer", TypeLine: "Legendary Creature — Dwarf Advisor"},
		// Bare equipment with NO payoff text — counts toward
		// equipmentCount but NOT payoff. Bonesplitter is the canonical
		// bare-Equipment piece: just +3/+0 + equip {3}, no trigger.
		// (Hammer of Nazahn is in the curated list because of its real
		// ETB-attach payoff text, so it doesn't work as the "bare"
		// example here.)
		{Name: "Bonesplitter", TypeLine: "Artifact — Equipment"},
		// Generic artifact — neither.
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Puresteel Paladin":     "metalcraft — whenever an equipment enters the battlefield under your control, you may draw a card.",
		"Sigarda's Aid":         "auras and equipment you control have flash. whenever an equipment enters the battlefield under your control, you may attach it to target creature.",
		"Sram, Senior Edificer": "whenever you cast an aura, equipment, or vehicle spell, draw a card.",
		"Bonesplitter":          "equipped creature gets +3/+0. equip {1}",
		"Sol Ring":              "{t}: add {c}{c}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.equipTriggerPayoffCount != 3 {
		t.Errorf("equipTriggerPayoffCount=%d, want 3 (Puresteel + Sigarda's Aid + Sram)",
			ctx.equipTriggerPayoffCount)
	}
}

// ---------------------------------------------------------------------------
// 3. ClassifyArchetype end-to-end
// ---------------------------------------------------------------------------

// equipmentVoltronFixture builds a Bruenor / Akiri shell:
//   - 4 equip-trigger payoffs
//   - 10 equipment pieces
//   - 6 protection-tagged cards (Bruenor's Voltron build runs
//     hexproof / ward / totem armor effects)
//   - 1 commander-stand-in creature
//
// Role counts approximate a real Voltron build: Protection=8,
// Threat=4, Ramp=10, Removal=5, Draw=4, Land=38.
func equipmentVoltronFixture() ([]CardProfile, map[string]string, map[RoleTag]int) {
	profiles := []CardProfile{
		// 4 equip-trigger payoffs.
		{Name: "Puresteel Paladin", TypeLine: "Creature — Human Knight"},
		{Name: "Sigarda's Aid", TypeLine: "Enchantment"},
		{Name: "Sram, Senior Edificer", TypeLine: "Legendary Creature — Dwarf Advisor"},
		{Name: "Stoneforge Mystic", TypeLine: "Creature — Kor Artificer"},
		// 10 equipment pieces.
		{Name: "Hammer of Nazahn", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Feast and Famine", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Fire and Ice", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Light and Shadow", TypeLine: "Artifact — Equipment"},
		{Name: "Shadowspear", TypeLine: "Legendary Artifact — Equipment"},
		{Name: "Colossus Hammer", TypeLine: "Artifact — Equipment"},
		{Name: "Embercleave", TypeLine: "Legendary Artifact — Equipment"},
		{Name: "Skullclamp", TypeLine: "Artifact — Equipment"},
		{Name: "Lightning Greaves", TypeLine: "Artifact — Equipment"},
		{Name: "Swiftfoot Boots", TypeLine: "Artifact — Equipment"},
		// Commander stand-in.
		{Name: "Bruenor Battlehammer", TypeLine: "Legendary Creature — Dwarf Soldier"},
	}
	oracleText := map[string]string{
		"Puresteel Paladin":         "whenever an equipment enters the battlefield under your control, you may draw a card.",
		"Sigarda's Aid":             "auras and equipment you control have flash. whenever an equipment enters the battlefield under your control, you may attach it to target creature.",
		"Sram, Senior Edificer":     "whenever you cast an aura, equipment, or vehicle spell, draw a card.",
		"Stoneforge Mystic":         "{1}, {t}: search your library for an equipment, reveal it, put it into your hand.",
		"Hammer of Nazahn":          "equipped creature gets +2/+0 and has indestructible.",
		"Sword of Feast and Famine": "equipped creature gets +2/+2 and has protection from black and from green.",
		"Sword of Fire and Ice":     "equipped creature gets +2/+2 and has protection from red and from blue.",
		"Sword of Light and Shadow": "equipped creature gets +2/+2 and has protection from white and from black.",
		"Shadowspear":               "equipped creature gets +1/+1, has trample and lifelink.",
		"Colossus Hammer":           "equipped creature gets +10/+10.",
		"Embercleave":               "equipped creature gets +1/+1 and has double strike and trample.",
		"Skullclamp":                "equipped creature gets +1/-1.",
		"Lightning Greaves":         "equipped creature has haste and shroud.",
		"Swiftfoot Boots":           "equipped creature has hexproof and haste.",
		"Bruenor Battlehammer":      "you may cast the first equipment spell each turn for 2 less. creatures you control with equipment attached get +2/+0.",
	}
	roleCounts := map[RoleTag]int{
		RoleProtection: 8,
		RoleThreat:     4,
		RoleRamp:       10,
		RoleRemoval:    5,
		RoleDraw:       4,
		RoleLand:       38,
	}
	return profiles, oracleText, roleCounts
}

func TestClassifyArchetype_EquipmentVoltronLandsOnSubArchetype(t *testing.T) {
	profiles, oracleText, roleCounts := equipmentVoltronFixture()
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Equipment-Voltron" {
		t.Errorf("primary archetype = %q, want Equipment-Voltron (10 equipment + 4 payoffs)", ac.Primary)
	}
	// Equipment-engine signal must surface.
	sawSignal := false
	for _, s := range ac.Signals {
		if strings.Contains(s, "equipment engine") {
			sawSignal = true
			break
		}
	}
	if !sawSignal {
		t.Errorf("expected 'equipment engine' signal in ac.Signals; got %v", ac.Signals)
	}
}

// TestClassifyArchetype_EquipmentNoPayoffsFallsThrough pins the
// negative-companion: 10 equipment with ZERO payoffs should NOT trip
// Equipment-Voltron — it's a midrange-with-toolbox shape, not the
// committed sub-archetype. Without the equip-payoff gate this deck
// would false-positive into Equipment-Voltron just from the equipment
// count.
func TestClassifyArchetype_EquipmentNoPayoffsFallsThrough(t *testing.T) {
	profiles := []CardProfile{
		// 10 equipment — but no payoffs.
		{Name: "Hammer of Nazahn", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Feast and Famine", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Fire and Ice", TypeLine: "Artifact — Equipment"},
		{Name: "Sword of Light and Shadow", TypeLine: "Artifact — Equipment"},
		{Name: "Shadowspear", TypeLine: "Legendary Artifact — Equipment"},
		{Name: "Colossus Hammer", TypeLine: "Artifact — Equipment"},
		{Name: "Embercleave", TypeLine: "Legendary Artifact — Equipment"},
		{Name: "Skullclamp", TypeLine: "Artifact — Equipment"},
		{Name: "Lightning Greaves", TypeLine: "Artifact — Equipment"},
		{Name: "Swiftfoot Boots", TypeLine: "Artifact — Equipment"},
	}
	oracleText := map[string]string{
		"Hammer of Nazahn":          "equipped creature gets +2/+0 and has indestructible.",
		"Sword of Feast and Famine": "equipped creature gets +2/+2.",
		"Sword of Fire and Ice":     "equipped creature gets +2/+2.",
		"Sword of Light and Shadow": "equipped creature gets +2/+2.",
		"Shadowspear":               "equipped creature gets +1/+1.",
		"Colossus Hammer":           "equipped creature gets +10/+10.",
		"Embercleave":               "equipped creature gets +1/+1.",
		"Skullclamp":                "equipped creature gets +1/-1.",
		"Lightning Greaves":         "equipped creature has haste and shroud.",
		"Swiftfoot Boots":           "equipped creature has hexproof and haste.",
	}
	roleCounts := map[RoleTag]int{
		RoleProtection: 8, RoleThreat: 4, RoleRamp: 10, RoleRemoval: 5,
		RoleDraw: 4, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Equipment-Voltron" {
		t.Errorf("primary archetype = Equipment-Voltron without payoffs; "+
			"the sub-archetype gate requires >=3 equip-trigger payoffs (got %v)",
			ac.Signals)
	}
}

// TestClassifyArchetype_EquipmentVoltronPreferredOverGenericVoltron
// pins the tie-break: when both Equipment-Voltron and Voltron gates
// pass, the more specific sub-archetype wins. Same fixture as the
// canonical equip-voltron test — both gates pass — Equipment-Voltron
// must win deterministically.
func TestClassifyArchetype_EquipmentVoltronPreferredOverGenericVoltron(t *testing.T) {
	profiles, oracleText, roleCounts := equipmentVoltronFixture()
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	// Both fingerprints have identical role-ratio templates so the
	// distance discount edges Equipment-Voltron ahead deterministically.
	if ac.Primary != "Equipment-Voltron" {
		t.Errorf("primary archetype = %q, want Equipment-Voltron "+
			"(both Voltron gates pass; sub-archetype tie-break should win)",
			ac.Primary)
	}
}

// TestClassifyArchetype_AuraVoltronStaysOnGenericVoltron pins the
// converse: a 10-aura Bogles-style shell should land on generic
// Voltron, NOT Equipment-Voltron, even with a payoff piece in the
// list. Defends against the new equipmentCount counter accidentally
// matching auras.
func TestClassifyArchetype_AuraVoltronStaysOnGenericVoltron(t *testing.T) {
	profiles := []CardProfile{
		// Single equip-trigger payoff that ALSO buffs auras (Sigarda's
		// Aid). Only 1 payoff < 3, so Equipment-Voltron gate fails.
		{Name: "Sigarda's Aid", TypeLine: "Enchantment"},
		// 10 auras.
		{Name: "Rancor", TypeLine: "Enchantment — Aura"},
		{Name: "Daybreak Coronet", TypeLine: "Enchantment — Aura"},
		{Name: "Ethereal Armor", TypeLine: "Enchantment — Aura"},
		{Name: "Spirit Mantle", TypeLine: "Enchantment — Aura"},
		{Name: "Hyena Umbra", TypeLine: "Enchantment — Aura"},
		{Name: "Spider Umbra", TypeLine: "Enchantment — Aura"},
		{Name: "Cartouche of Solidarity", TypeLine: "Enchantment — Aura Cartouche"},
		{Name: "Sentinel's Eyes", TypeLine: "Enchantment — Aura"},
		{Name: "Indestructibility", TypeLine: "Enchantment — Aura"},
		{Name: "Holy Mantle", TypeLine: "Enchantment — Aura"},
	}
	oracleText := map[string]string{
		"Sigarda's Aid":           "auras and equipment you control have flash.",
		"Rancor":                  "enchant creature. enchanted creature gets +2/+0 and has trample.",
		"Daybreak Coronet":        "enchanted creature gets +3/+3, has first strike, vigilance, and lifelink.",
		"Ethereal Armor":          "enchanted creature gets +1/+1 for each enchantment you control and has first strike.",
		"Spirit Mantle":           "enchanted creature gets +1/+1 and has protection from creatures.",
		"Hyena Umbra":             "enchant creature you control.",
		"Spider Umbra":            "enchant creature you control.",
		"Cartouche of Solidarity": "enchanted creature gets +1/+1.",
		"Sentinel's Eyes":         "enchanted creature gets +1/+1 and has vigilance.",
		"Indestructibility":       "enchanted permanent has indestructible.",
		"Holy Mantle":             "enchanted creature gets +2/+2 and has protection from creatures.",
	}
	roleCounts := map[RoleTag]int{
		RoleProtection: 8, RoleThreat: 4, RoleRamp: 10, RoleRemoval: 5,
		RoleDraw: 4, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Equipment-Voltron" {
		t.Errorf("Aura-heavy deck classified as Equipment-Voltron; equipmentCount " +
			"should be 0 since auras aren't equipment")
	}
}
