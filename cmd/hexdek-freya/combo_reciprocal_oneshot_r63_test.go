package main

import "testing"

// R63 regression guard (7174n1c-reported false positive). Freya flagged
// "Eviscerator's Insight + Orchard Strider" as a combo with a fabricated
// reciprocal interaction ("A produces a card for B, B produces a token
// back"). Neither card is a repeatable engine:
//
//   - Eviscerator's Insight is an INSTANT: "as an additional cost ...
//     sacrifice an artifact or creature ... draw two cards" + flashback. A
//     one-shot sacrifice-outlet draw spell.
//   - Orchard Strider is a CREATURE whose only relevant text is a single
//     enters-the-battlefield trigger ("create two Food tokens") + landcycling.
//
// The resource types overlap (the spell consumes a token to sacrifice and
// draws a card; the creature makes Food tokens) but there is no repeatable
// closed loop — each card fires once. It must NOT be a categorical combo.
func TestReciprocalOneShot_EvisceratorOrchardNotCombo(t *testing.T) {
	eviscerator := ClassifyCard(
		"Eviscerator's Insight",
		"As an additional cost to cast this spell, sacrifice an artifact or creature.\nDraw two cards.\nFlashback {4}{B}",
		"Instant",
		"{2}{B}",
		3,
		"",
	)
	orchard := ClassifyCard(
		"Orchard Strider",
		"When Orchard Strider enters, create two Food tokens. (They're artifacts with \"{2}, {T}, Sacrifice this token: You gain 3 life.\")\nLandcycling {1}{G} ({1}{G}, Discard this card: Search your library for a basic land card, reveal it, put it into your hand, then shuffle.)",
		"Creature — Spider",
		"{4}{G}",
		5,
		"3",
	)

	if eviscerator.RepeatableEngine {
		t.Errorf("one-shot instant Eviscerator's Insight must not be a repeatable engine")
	}
	if orchard.RepeatableEngine {
		t.Errorf("ETB-only creature Orchard Strider must not be a repeatable engine")
	}

	report := AnalyzeDeck(
		[]CardProfile{eviscerator, orchard},
		"eviscerator+orchard",
		"",
		"",
	)

	for _, bucket := range [][]ComboResult{report.TrueInfinites, report.Determined, report.Finishers} {
		for _, c := range bucket {
			if comboContains(c.Cards, "Eviscerator's Insight") &&
				comboContains(c.Cards, "Orchard Strider") {
				t.Fatalf("one-shot reciprocal pair must not be a categorical combo: %+v", c)
			}
		}
	}
}

// Conservative guardrail: a genuine reciprocal loop between two REPEATABLE
// engines (both permanents with activated abilities forming a card<->token
// resource cycle) must still be detected as a combo, not silently downgraded
// by the new repeatability gate.
func TestReciprocalRepeatable_TwoEnginesStillCombo(t *testing.T) {
	// Token engine: discards a card to repeatedly mint tokens.
	tokenForge := ClassifyCard(
		"Servo Foundry",
		"{2}, {T}, Discard a card: Create two 1/1 colorless Servo artifact creature tokens.",
		"Artifact",
		"{3}",
		3,
		"",
	)
	// Sac engine: repeatedly sacrifices an artifact (the tokens) to draw.
	sacEngine := ClassifyCard(
		"Macabre Reservoir",
		"Sacrifice an artifact: Draw two cards. Activate only once each turn.",
		"Artifact",
		"{2}",
		2,
		"",
	)

	if !tokenForge.RepeatableEngine {
		t.Fatalf("activated-ability permanent Servo Foundry must be a repeatable engine")
	}
	if !sacEngine.RepeatableEngine {
		t.Fatalf("activated-ability permanent Macabre Reservoir must be a repeatable engine")
	}

	report := AnalyzeDeck(
		[]CardProfile{tokenForge, sacEngine},
		"twoengines",
		"",
		"",
	)

	found := false
	for _, bucket := range [][]ComboResult{report.TrueInfinites, report.Determined} {
		for _, c := range bucket {
			if comboContains(c.Cards, "Servo Foundry") &&
				comboContains(c.Cards, "Macabre Reservoir") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("genuine repeatable reciprocal loop must still be detected as a combo; "+
			"TrueInfinites=%+v Determined=%+v Synergies=%+v",
			report.TrueInfinites, report.Determined, report.Synergies)
	}
}
