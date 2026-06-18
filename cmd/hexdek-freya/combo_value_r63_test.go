package main

import "testing"

// R63 Phase 2 — VALUE axis regressions.

// Eviscerator's Insight + Orchard Strider is card-advantage VALUE (not combo,
// not symmetric).
func TestValue_EvisceratorOrchardIsValue(t *testing.T) {
	eviscerator := ClassifyCard("Eviscerator's Insight",
		"As an additional cost to cast this spell, sacrifice an artifact or creature.\nDraw two cards.\nFlashback {4}{B}",
		"Instant", "{2}{B}", 3, "")
	orchard := ClassifyCard("Orchard Strider",
		"When Orchard Strider enters, create two Food tokens. (They're artifacts with \"{2}, {T}, Sacrifice this token: You gain 3 life.\")\nLandcycling {1}{G} ({1}{G}, Discard this card: Search your library for a basic land card, reveal it, put it into your hand, then shuffle.)",
		"Creature — Spider", "{4}{G}", 5, "3")

	report := AnalyzeDeck([]CardProfile{eviscerator, orchard}, "ev+orchard", "", "")

	// Must NOT be a combo.
	if comboInBucket(report.Combos, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("pair must not be COMBO; Combos=%+v", report.Combos)
	}
	// The pair interaction must be VALUE.
	if !comboInBucket(report.ValueEngines, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("Eviscerator+Orchard must be tagged VALUE; ValueEngines=%+v", report.ValueEngines)
	}
	// Eviscerator alone is card advantage.
	if !comboInBucket(report.ValueEngines, "Eviscerator's Insight") {
		t.Fatalf("Eviscerator's Insight (draw) must be a VALUE baseline; ValueEngines=%+v", report.ValueEngines)
	}
}

// A one-sided board wipe is VALUE (asymmetric board swing).
func TestValue_OneSidedBoardWipeIsValue(t *testing.T) {
	wipe := ClassifyCard("Culling Sun",
		"Destroy all creatures you don't control.",
		"Sorcery", "{3}{W}{B}", 5, "")
	if vp := buildValueProfile(wipe); !vp.isValue() {
		t.Fatalf("one-sided wipe must be asymmetric VALUE; got %+v", vp)
	}
	report := AnalyzeDeck([]CardProfile{wipe}, "wipe", "", "")
	if !comboInBucket(report.ValueEngines, "Culling Sun") {
		t.Fatalf("one-sided board wipe must be VALUE; ValueEngines=%+v", report.ValueEngines)
	}
}

// A pure ramp rock is VALUE (mana advantage).
func TestValue_RampRockIsValue(t *testing.T) {
	rock := ClassifyCard("Mind Stone",
		"{T}: Add {C}.\n{1}, {T}, Sacrifice Mind Stone: Draw a card.",
		"Artifact", "{2}", 2, "")
	vp := buildValueProfile(rock)
	if !containsStr(vp.Advantage, "mana") {
		t.Fatalf("ramp rock must carry mana advantage; got %+v", vp)
	}
	report := AnalyzeDeck([]CardProfile{rock}, "rock", "", "")
	if !comboInBucket(report.ValueEngines, "Mind Stone") {
		t.Fatalf("ramp rock must be VALUE; ValueEngines=%+v", report.ValueEngines)
	}
}

// A symmetric "each player sacrifices" effect is NOT value.
func TestValue_SymmetricSacrificeNotValue(t *testing.T) {
	edict := ClassifyCard("Even Hand of Doom",
		"Each player sacrifices a creature.",
		"Sorcery", "{1}{B}", 2, "")
	if !edict.Symmetric {
		t.Fatalf("'each player sacrifices' must set Symmetric")
	}
	report := AnalyzeDeck([]CardProfile{edict}, "edict", "", "")
	if comboInBucket(report.ValueEngines, "Even Hand of Doom") {
		t.Fatalf("symmetric sacrifice must NOT be VALUE; ValueEngines=%+v", report.ValueEngines)
	}
}

// The symmetry discriminator gates even a card that WOULD be value: a symmetric
// effect that produces card advantage is excluded from VALUE.
func TestValue_SymmetricDrawGatedOut(t *testing.T) {
	// Construct the profile directly to isolate the discriminator from oracle
	// draw-parsing edge cases: card advantage present, but symmetric.
	groupHug := CardProfile{
		Name:      "Shared Insight Ritual",
		TypeLine:  "Sorcery",
		Produces:  []ResourceType{ResCard},
		Symmetric: true,
	}
	vp := buildValueProfile(groupHug)
	if !containsStr(vp.Advantage, "card") {
		t.Fatalf("group draw should register card advantage; got %+v", vp)
	}
	if vp.Symmetry != "symmetric" {
		t.Fatalf("group draw must be symmetric; got %q", vp.Symmetry)
	}
	if vp.isValue() {
		t.Fatalf("symmetric group draw must NOT qualify as VALUE")
	}
}

func containsStr(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
