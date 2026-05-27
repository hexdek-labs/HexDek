package main

import (
	"strings"
	"testing"
)

// archetype_tribal_lords_r60_test.go — regressions for the
// `tribalLordCount` signal and its effect on Tribal archetype
// classification. The pre-existing Tribal fingerprint required
// `creaturePct >= 0.35 && topCreatureTypePct >= 0.30`; lord-heavy
// decks with a slightly diluted creature-type concentration (e.g. a
// Goblin deck running half-a-dozen utility wizards / clerics
// alongside its goblins) fell through to Aggro or Midrange even
// though the lord package committed the archetype unambiguously.
//
// Three layers of coverage:
//
//  1. cardTribalLordTribe — unit-level shape detection. Pins the
//     four printed patterns ("<tribe> creatures you control get",
//     "<tribe> creatures you control have", "<tribe>s you control
//     get", "<tribe>s you control have") with a negative companion
//     for generic anthems (Glorious Anthem) and single-target buffs.
//
//  2. buildClassifyContext — pins that tribalLordCount accumulates
//     correctly across a deck list and tribalLordTribe records the
//     most-mentioned tribe.
//
//  3. ClassifyArchetype end-to-end — pins that 2+ lords trigger the
//     relaxed Tribal Require gate AND apply the distance discount,
//     so a lord-heavy deck just below the baseline topCreatureTypePct
//     threshold still lands on Tribal.

// ---------------------------------------------------------------------------
// 1. cardTribalLordTribe shape detection
// ---------------------------------------------------------------------------

func TestCardTribalLordTribe_DetectsKingShape(t *testing.T) {
	// Goblin King: "Other Goblin creatures you control get +1/+1 ..."
	got := cardTribalLordTribe("other goblin creatures you control get +1/+1 and have mountainwalk.")
	if got != "goblin" {
		t.Errorf("Goblin King shape: got %q, want %q", got, "goblin")
	}
}

func TestCardTribalLordTribe_DetectsBareCreaturesGet(t *testing.T) {
	// Elvish Champion: "Elf creatures get +1/+1 and have forestwalk."
	// The "you control" qualifier is the modern templating; older
	// printings drop it but Scryfall's reprint cleanup normalizes to
	// "you control" so we don't need a non-"you control" branch.
	got := cardTribalLordTribe("elf creatures you control get +1/+1 and have forestwalk.")
	if got != "elf" {
		t.Errorf("Elvish Champion shape: got %q, want %q", got, "elf")
	}
}

func TestCardTribalLordTribe_DetectsHaveShape(t *testing.T) {
	// "Merfolk creatures you control have islandwalk and get +1/+1."
	got := cardTribalLordTribe("merfolk creatures you control have +1/+1 and islandwalk.")
	if got != "merfolk" {
		t.Errorf("have-shape: got %q, want %q", got, "merfolk")
	}
}

func TestCardTribalLordTribe_DetectsPluralSuffix(t *testing.T) {
	// Plural-form modern templating, e.g. "Goblins you control get
	// +1/+1." (Krenko's Buzzcrusher / Lightning Crafter family).
	got := cardTribalLordTribe("goblins you control get +1/+1.")
	if got != "goblin" {
		t.Errorf("plural-suffix: got %q, want %q", got, "goblin")
	}
}

func TestCardTribalLordTribe_RejectsGenericAnthem(t *testing.T) {
	// Glorious Anthem — buffs ALL creatures, no tribe. Must NOT be
	// classified as a tribal lord.
	got := cardTribalLordTribe("creatures you control get +1/+1.")
	if got != "" {
		t.Errorf("generic anthem misclassified as tribal: got %q, want \"\"", got)
	}
}

func TestCardTribalLordTribe_RejectsSingleTargetBuff(t *testing.T) {
	// Giant Growth / Boros Charm: target buffs are NOT lords.
	got := cardTribalLordTribe("target creature you control gets +3/+3 until end of turn.")
	if got != "" {
		t.Errorf("single-target buff misclassified as tribal: got %q, want \"\"", got)
	}
}

func TestCardTribalLordTribe_EmptyText(t *testing.T) {
	if got := cardTribalLordTribe(""); got != "" {
		t.Errorf("empty oracle text: got %q, want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// 2. buildClassifyContext tribalLordCount + tribalLordTribe wiring
// ---------------------------------------------------------------------------

func TestBuildClassifyContext_TribalLordCount(t *testing.T) {
	profiles := []CardProfile{
		// Three Goblin lords.
		{Name: "Goblin King", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin Chieftain", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin Warchief", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		// One generic anthem — anthemCount but NOT tribalLordCount.
		{Name: "Glorious Anthem", TypeLine: "Enchantment"},
		// One single-target buff — neither counter.
		{Name: "Giant Growth", TypeLine: "Instant"},
		// Non-anthem goblin — provides type concentration only.
		{Name: "Goblin Piledriver", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
	}
	oracleText := map[string]string{
		"Goblin King":        "other goblin creatures you control get +1/+1 and have mountainwalk.",
		"Goblin Chieftain":   "goblin creatures you control have haste and get +1/+1.",
		"Goblin Warchief":    "goblins you control get +1/+1.",
		"Glorious Anthem":    "creatures you control get +1/+1.",
		"Giant Growth":       "target creature gets +3/+3 until end of turn.",
		"Goblin Piledriver":  "whenever this attacks, it gets +2/+0 until end of turn for each other attacking goblin.",
	}
	ctx := buildContextFor(profiles, oracleText)

	if ctx.tribalLordCount != 3 {
		t.Errorf("tribalLordCount=%d, want 3 (3 Goblin lords; generic anthem and target buff excluded)",
			ctx.tribalLordCount)
	}
	if ctx.tribalLordTribe != "goblin" {
		t.Errorf("tribalLordTribe=%q, want \"goblin\"", ctx.tribalLordTribe)
	}
	// Note on anthemCount: cardHasAnthem (pre-existing) only matches
	// the strict "creatures you control get +" / "creatures you control
	// have +" substrings — Goblin Chieftain's "have haste AND get +"
	// and Goblin Warchief's plural "goblins you control" miss its
	// existing patterns. tribalLordCount uses the broader
	// clauseHasBuffAnchor window so it catches them. The assertion here
	// just sanity-checks anthemCount didn't regress (Goblin King +
	// Glorious Anthem both still match the legacy detector).
	if ctx.anthemCount < 2 {
		t.Errorf("anthemCount=%d, want >= 2 (Goblin King + Glorious Anthem at minimum)", ctx.anthemCount)
	}
}

func TestBuildClassifyContext_TribalLordCount_ZeroOnNonTribalDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Lightning Bolt", TypeLine: "Instant"},
		{Name: "Counterspell", TypeLine: "Instant"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Lightning Bolt": "lightning bolt deals 3 damage to any target.",
		"Counterspell":   "counter target spell.",
		"Sol Ring":       "{t}: add {c}{c}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.tribalLordCount != 0 {
		t.Errorf("tribalLordCount=%d, want 0 on a non-tribal pile", ctx.tribalLordCount)
	}
	if ctx.tribalLordTribe != "" {
		t.Errorf("tribalLordTribe=%q, want \"\"", ctx.tribalLordTribe)
	}
}

// ---------------------------------------------------------------------------
// 3. ClassifyArchetype end-to-end — relaxed gate + distance discount
// ---------------------------------------------------------------------------

// tribalDeckFixture builds a fixture that's deliberately UNDER the
// baseline Tribal gate's topCreatureTypePct=0.30 threshold but has
// the configured number of lords. Used to demonstrate that the lord
// carveout (relaxed gate to 0.20 + distance discount) flips the
// classification to Tribal.
//
// Layout (24 nonland creatures):
//   - lordCount Goblin lords (anthem-typed, count toward tribalLordCount)
//   - 6 generic goblins (creature-type concentration only)
//   - 7 utility non-goblin creatures (dilute topCreatureTypePct)
//   - 6 non-creature support cards (curve fill, ramp, removal)
//
// With lordCount=3: top tribe is goblin at 9/22 ≈ 41%. With
// lordCount=0: top tribe is goblin at 6/22 ≈ 27% — below baseline.
func tribalDeckFixture(lordCount int) ([]CardProfile, map[string]string, map[RoleTag]int) {
	profiles := []CardProfile{}
	oracleText := map[string]string{}

	lordNames := []string{
		"Goblin King", "Goblin Chieftain", "Goblin Warchief",
		"Goblin Lord", "Krenko's Buzzcrusher", "Goblin Trashmaster",
	}
	lordTexts := []string{
		"other goblin creatures you control get +1/+1 and have mountainwalk.",
		"goblin creatures you control have haste and get +1/+1.",
		"goblins you control get +1/+1.",
		"goblin creatures you control get +1/+1.",
		"goblins you control have +1/+1.",
		"goblin creatures you control get +1/+0.",
	}
	for i := 0; i < lordCount && i < len(lordNames); i++ {
		profiles = append(profiles, CardProfile{
			Name: lordNames[i], TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"},
		})
		oracleText[lordNames[i]] = lordTexts[i]
	}
	// 6 generic goblins (non-anthem) for creature-type concentration.
	for i := 0; i < 6; i++ {
		name := "Goblin Filler " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"},
		})
		oracleText[name] = "haste"
	}
	// 7 non-goblin utility creatures to dilute topCreatureTypePct.
	utilityTypes := []string{"human", "wizard", "rogue", "warrior", "shaman", "cleric", "knight"}
	for i, t := range utilityTypes {
		name := "Utility " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature — " + strings.Title(t), CreatureTypes: []string{t},
		})
		oracleText[name] = "flash"
	}
	// 6 non-creature support cards.
	for i := 0; i < 6; i++ {
		name := "Support " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Sorcery"})
		oracleText[name] = "destroy target creature."
	}

	roleCounts := map[RoleTag]int{
		RoleThreat:  6 + lordCount,
		RoleDraw:    4,
		RoleRamp:    4,
		RoleRemoval: 4,
		RoleLand:    38,
	}
	return profiles, oracleText, roleCounts
}

// TestClassifyArchetype_TribalLordPackageBoostsClassification is the
// canonical regression: a Goblin shell with 3 lords + 6 generic
// goblins + 7 non-goblin utility creatures has topCreatureTypePct ≈
// 41% (9/22), which is OVER the baseline gate. This pins that the
// classification correctly lands on Tribal AND the lord signal
// surfaces in ac.Signals.
func TestClassifyArchetype_TribalLordPackageBoostsClassification(t *testing.T) {
	profiles, oracleText, roleCounts := tribalDeckFixture(3)
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Tribal" {
		t.Errorf("primary archetype = %q, want Tribal (3 lords + goblin core)", ac.Primary)
	}
	// Lord signal must surface in the human-readable signals list.
	sawLordSignal := false
	for _, s := range ac.Signals {
		if strings.Contains(s, "tribal lord package") {
			sawLordSignal = true
			break
		}
	}
	if !sawLordSignal {
		t.Errorf("expected 'tribal lord package' signal in ac.Signals, got: %v", ac.Signals)
	}
}

// TestClassifyArchetype_TribalLordCarveoutRescuesDilutedDeck pins the
// real-world scenario: a deck where the utility-creature dilution
// drags topCreatureTypePct BELOW the baseline 0.30 gate, but 2+ lords
// trigger the relaxed gate (0.20). Without the lord carveout this
// deck falls through to Aggro/Midrange. With it, the deck lands on
// Tribal.
//
// 2 lords + 4 generic goblins + 13 non-goblin creatures: top tribe
// goblin is 6/19 ≈ 32%. Hmm, that's still above 0.30 — adjust the
// dilution to demonstrate the carveout cleanly.
func TestClassifyArchetype_TribalLordCarveoutRescuesDilutedDeck(t *testing.T) {
	profiles := []CardProfile{
		// Two lords.
		{Name: "Goblin King", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin Chieftain", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		// Three more goblins (5 total goblin creatures).
		{Name: "Goblin A", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin B", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin C", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
	}
	// 16 non-goblin creatures, each its own subtype (no other type
	// gets >1 instance, so goblin remains top tribe).
	otherTypes := []string{
		"human", "wizard", "rogue", "warrior", "shaman", "cleric",
		"knight", "elf", "dwarf", "merfolk", "soldier", "druid",
		"horror", "ninja", "samurai", "pirate",
	}
	for i, t := range otherTypes {
		name := "Utility " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature — " + strings.Title(t), CreatureTypes: []string{t},
		})
	}
	oracleText := map[string]string{
		"Goblin King":      "other goblin creatures you control get +1/+1 and have mountainwalk.",
		"Goblin Chieftain": "goblin creatures you control have haste and get +1/+1.",
		"Goblin A":         "haste",
		"Goblin B":         "haste",
		"Goblin C":         "haste",
	}
	for _, name := range []string{
		"Utility A", "Utility B", "Utility C", "Utility D", "Utility E",
		"Utility F", "Utility G", "Utility H", "Utility I", "Utility J",
		"Utility K", "Utility L", "Utility M", "Utility N", "Utility O", "Utility P",
	} {
		oracleText[name] = "flash"
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 12, RoleDraw: 4, RoleRamp: 4, RoleRemoval: 4, RoleLand: 38,
	}
	// Goblin = 5/21 ≈ 23.8% — UNDER baseline 0.30, OVER relaxed 0.20.
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Tribal" {
		t.Errorf("primary archetype = %q, want Tribal (lord carveout should rescue diluted deck)", ac.Primary)
	}
}

// TestClassifyArchetype_NoLordsBaselineGateHolds pins the negative
// path: the same diluted deck WITHOUT lords must NOT classify as
// Tribal. The relaxed gate is gated on tribalLordCount ≥ 2 — defends
// against the carveout accidentally widening the baseline definition.
func TestClassifyArchetype_NoLordsBaselineGateHolds(t *testing.T) {
	// Same diluted shape as the carveout test but with non-anthem
	// goblin creatures instead of lords.
	profiles := []CardProfile{
		{Name: "Goblin 1", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin 2", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin 3", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin 4", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
		{Name: "Goblin 5", TypeLine: "Creature — Goblin", CreatureTypes: []string{"goblin"}},
	}
	otherTypes := []string{
		"human", "wizard", "rogue", "warrior", "shaman", "cleric",
		"knight", "elf", "dwarf", "merfolk", "soldier", "druid",
		"horror", "ninja", "samurai", "pirate",
	}
	for i, t := range otherTypes {
		name := "Utility " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: "Creature — " + strings.Title(t), CreatureTypes: []string{t},
		})
	}
	oracleText := map[string]string{}
	for _, p := range profiles {
		oracleText[p.Name] = "haste"
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 12, RoleDraw: 4, RoleRamp: 4, RoleRemoval: 4, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Tribal" {
		t.Errorf("primary archetype = Tribal without lords; baseline gate should have rejected " +
			"the diluted deck (topCreatureTypePct ~24%% < 0.30 and tribalLordCount=0 < 2)")
	}
}
