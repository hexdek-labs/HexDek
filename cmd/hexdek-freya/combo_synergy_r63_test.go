package main

import "testing"

// R63 Phase 3 — SYNERGY axis + context re-tag regressions.

// aristocratsCommander is a death-trigger commander whose profile yields the
// "sacrifice" theme.
func aristocratsCommander() CardProfile {
	return ClassifyCard("Bastion of Bones",
		"Whenever a creature you control dies, each opponent loses 1 life and you gain 1 life.",
		"Legendary Creature — Spirit", "{2}{B}", 3, "3")
}

// goodstuffCommander is a themeless value commander (card draw only).
func goodstuffCommander() CardProfile {
	return ClassifyCard("Archivist of Ages",
		"{2}, {T}: Draw a card.",
		"Legendary Creature — Wizard", "{2}{U}", 3, "2")
}

func evisceratorCard() CardProfile {
	return ClassifyCard("Eviscerator's Insight",
		"As an additional cost to cast this spell, sacrifice an artifact or creature.\nDraw two cards.\nFlashback {4}{B}",
		"Instant", "{2}{B}", 3, "")
}

func orchardCard() CardProfile {
	return ClassifyCard("Orchard Strider",
		"When Orchard Strider enters, create two Food tokens. (They're artifacts with \"{2}, {T}, Sacrifice this token: You gain 3 life.\")\nLandcycling {1}{G} ({1}{G}, Discard this card: Search your library for a basic land card, reveal it, put it into your hand, then shuffle.)",
		"Creature — Spider", "{4}{G}", 5, "3")
}

// A tribal lord is a SYNERGY piece.
func TestSynergy_TribalLordIsSynergy(t *testing.T) {
	lord := ClassifyCard("Goblin King",
		"Other Goblins get +1/+1 and have mountainwalk.",
		"Creature — Goblin", "{1}{R}{R}", 3, "2")
	if !lord.IsTribalLord {
		t.Fatalf("Goblin King must be a tribal lord")
	}
	goblin := ClassifyCard("Mogg Fanatic",
		"Sacrifice Mogg Fanatic: It deals 1 damage to any target.",
		"Creature — Goblin", "{R}", 1, "1")

	report := AnalyzeDeck([]CardProfile{lord, goblin}, "goblins", "", "Goblin King")
	if !comboInBucket(report.SynergyPackages, "Goblin King") {
		t.Fatalf("tribal lord must be tagged SYNERGY; SynergyPackages=%+v", report.SynergyPackages)
	}
}

// A symmetric "each player sacrifices" effect is SYNERGY (not value).
func TestSynergy_SymmetricSacIsSynergy(t *testing.T) {
	edict := ClassifyCard("Even Hand of Doom",
		"Each player sacrifices a creature.",
		"Sorcery", "{1}{B}", 2, "")
	report := AnalyzeDeck([]CardProfile{edict, aristocratsCommander()}, "edict", "", "Bastion of Bones")
	if comboInBucket(report.ValueEngines, "Even Hand of Doom") {
		t.Fatalf("symmetric sac must NOT be VALUE")
	}
	if !comboInBucket(report.SynergyPackages, "Even Hand of Doom") {
		t.Fatalf("symmetric sac must be SYNERGY; SynergyPackages=%+v", report.SynergyPackages)
	}
}

// A card that advances the commander's plan (a sac outlet in a sacrifice deck)
// is a SYNERGY commander-piece.
func TestSynergy_CommanderPieceIsSynergy(t *testing.T) {
	outlet := ClassifyCard("Viscera Seer",
		"Sacrifice a creature: Scry 1.",
		"Creature — Vampire Wizard", "{B}", 1, "1")
	report := AnalyzeDeck([]CardProfile{outlet, aristocratsCommander()}, "aristo", "", "Bastion of Bones")
	if !comboInBucket(report.SynergyPackages, "Viscera Seer") {
		t.Fatalf("sac outlet in a sacrifice shell must be SYNERGY; SynergyPackages=%+v", report.SynergyPackages)
	}
}

// CONTEXT RE-TAG (the capstone): Eviscerator's Insight + Orchard Strider is
// VALUE-only in a goodstuff shell but flips to VALUE+SYNERGY in an aristocrats
// shell. The deck-independent VALUE baseline never changes; only Layer B adds
// the synergy tag.
func TestSynergy_ContextRetag_EvisceratorOrchard(t *testing.T) {
	// Goodstuff shell — themeless commander.
	goodstuff := AnalyzeDeck(
		[]CardProfile{evisceratorCard(), orchardCard(), goodstuffCommander()},
		"goodstuff", "", "Archivist of Ages")
	if !comboInBucket(goodstuff.ValueEngines, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("pair must be VALUE in goodstuff; ValueEngines=%+v", goodstuff.ValueEngines)
	}
	if comboInBucket(goodstuff.SynergyPackages, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("pair must NOT be SYNERGY in goodstuff shell; SynergyPackages=%+v", goodstuff.SynergyPackages)
	}

	// Aristocrats shell — sacrifice-theme commander. Eviscerator (sac outlet)
	// advances the plan → the pair flips to value+synergy.
	aristo := AnalyzeDeck(
		[]CardProfile{evisceratorCard(), orchardCard(), aristocratsCommander()},
		"aristo", "", "Bastion of Bones")
	if !comboInBucket(aristo.ValueEngines, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("pair must still be VALUE in aristocrats (baseline unchanged); ValueEngines=%+v", aristo.ValueEngines)
	}
	if !comboInBucket(aristo.SynergyPackages, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("pair must gain SYNERGY in aristocrats shell; SynergyPackages=%+v", aristo.SynergyPackages)
	}
	// Co-tag: the VALUE entry itself carries both tags.
	for _, v := range aristo.ValueEngines {
		if comboContains(v.Cards, "Eviscerator's Insight") && comboContains(v.Cards, "Orchard Strider") {
			if !hasCategory(v.Categories, CategoryValue) || !hasCategory(v.Categories, CategorySynergy) {
				t.Fatalf("aristocrats pair VALUE entry must co-tag value+synergy; got %+v", v.Categories)
			}
		}
	}
}

// Q4 carry-over: an enabler feeding a compounding-math payoff stays COMBO
// (deck-independent), unaffected by the synergy axis.
func TestSynergy_EnablerMathPayoffStillCombo(t *testing.T) {
	outlet := ClassifyCard("Viscera Seer",
		"Sacrifice a creature: Scry 1.",
		"Creature — Vampire Wizard", "{B}", 1, "1")
	payoff := ClassifyCard("Blood Tithe Engine",
		"Whenever a creature you control dies, this deals damage to each opponent equal to the number of creatures you control.",
		"Enchantment", "{2}{B}", 3, "")
	report := AnalyzeDeck([]CardProfile{outlet, payoff, aristocratsCommander()}, "aristo-combo", "", "Bastion of Bones")
	if !comboInBucket(report.Combos, "Viscera Seer", "Blood Tithe Engine") {
		t.Fatalf("enabler→math-payoff must remain COMBO; Combos=%+v", report.Combos)
	}
}
