package main

import (
	"strings"
	"testing"
)

// valuechains_cycling_r60_test.go — regressions for the Cycling Engine
// chain template added in the r60 value-engine sweep. CLAUDE.md's done
// list cited Storm / Artifact / Enchantress / Counters Matter as the
// already-shipped templates; Cycling was the cleanest remaining gap.
//
// The pipeline is a 2-step chain:
//
//   CYCLE  (hand → graveyard, resource=cycle_source):  a card with a
//          cycling cost (Decree of Justice, Krosan Tusker, Eternal
//          Dragon, Astral Slide, etc.)
//   PAYOFF (battlefield → battlefield, resource=cycle_payoff): a
//          permanent that triggers when you cycle (Astral Drift,
//          Drake Haven, Lightning Rift, Ominous Seas, Faith of the
//          Devoted, New Perspectives).
//
// Detection lives in classifyZoneFlows (cycling cost OR
// landcycling/typecycling → cycle_source flow; whenever-you-cycle
// triggers → cycle_payoff flow). The chain template requires both
// steps to match — a deck of pure cyclers with no payoffs or pure
// payoffs with no cyclers won't trigger the engine.

// classifyCyclingCard is a tiny wrapper around ClassifyCard for tests —
// hides the ManaCost / CMC / Power positional args.
func classifyCyclingCard(name, ot, typeLine string) CardProfile {
	return ClassifyCard(name, ot, typeLine, "{2}", 2, "")
}

// -----------------------------------------------------------------------------
// classifyZoneFlows — cycling source detection
// -----------------------------------------------------------------------------

func TestCyclingFlows_CyclingCostDetected(t *testing.T) {
	cases := []struct {
		name string
		ot   string
	}{
		{"Decree of Justice", "Create X 4/4 white Angel creature tokens with flying. Cycling {2}{W}"},
		{"Krosan Tusker", "Cycling {2}{G} ({2}{G}, Discard this card: Draw a card.) When you cycle this card, you may search your library for a basic land card."},
		{"Eternal Dragon", "Plainscycling {2}. {1}{W}{W}: Return this card from your graveyard to your hand."},
		{"Astral Slide", "Whenever a player cycles a card, you may exile target creature. If you do, return that card to the battlefield. Cycling {2}"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := classifyCyclingCard(tc.name, tc.ot, "Sorcery")
			found := false
			for _, fl := range p.ZoneFlows {
				if fl.From == "hand" && fl.To == "graveyard" && fl.Resource == "cycle_source" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected cycle_source flow; got %v", tc.name, p.ZoneFlows)
			}
		})
	}
}

// Cards that REFERENCE cycling but don't have it (no cycling cost) must
// NOT be flagged as cycle sources. Astral Drift mentions "Whenever a
// player cycles a card" — that's the PAYOFF text, not the cost.
// New Perspectives ("Cycling costs you pay cost {0}") references the
// keyword but isn't itself a cycle source.
func TestCyclingFlows_PayoffOnlyNotFlaggedAsSource(t *testing.T) {
	ot := "Whenever you cycle a card, you may exile target creature. " +
		"If you do, return that card to the battlefield under its owner's control at the beginning of the next end step."
	p := classifyCyclingCard("Astral Drift", ot, "Enchantment")

	for _, fl := range p.ZoneFlows {
		if fl.Resource == "cycle_source" {
			t.Errorf("Astral Drift (payoff only) should NOT have cycle_source flow; got %v", p.ZoneFlows)
		}
	}
}

// -----------------------------------------------------------------------------
// classifyZoneFlows — cycling payoff detection
// -----------------------------------------------------------------------------

func TestCyclingFlows_PayoffDetected(t *testing.T) {
	cases := []struct {
		name string
		ot   string
	}{
		{
			name: "Astral Drift",
			ot:   "Whenever you cycle a card, you may exile target creature. If you do, return that card to the battlefield.",
		},
		{
			name: "Drake Haven",
			ot:   "Whenever you cycle or discard a card, you may pay {1}. If you do, create a 2/2 blue Drake creature token with flying.",
		},
		{
			name: "Lightning Rift",
			ot:   "Whenever you cycle a card, you may pay {1}. If you do, this enchantment deals 2 damage to any target.",
		},
		{
			name: "Ominous Seas",
			ot:   "Whenever you cycle or discard a card, put a foretell counter on this enchantment.",
		},
		{
			name: "Faith of the Devoted",
			ot:   "Whenever you cycle or discard a card, you may pay {1}. If you do, each opponent loses 2 life and you gain 2 life.",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := classifyCyclingCard(tc.name, tc.ot, "Enchantment")
			found := false
			for _, fl := range p.ZoneFlows {
				if fl.From == "battlefield" && fl.To == "battlefield" && fl.Resource == "cycle_payoff" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: expected cycle_payoff flow; got %v", tc.name, p.ZoneFlows)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Engine detection — full chain
// -----------------------------------------------------------------------------

// A canonical UWr cycling deck shell. Should produce the "Cycling Engine"
// chain with both steps populated.
func TestDetectValueChains_CyclingEngine_FullShell(t *testing.T) {
	profiles := []CardProfile{
		// Payoffs (3+ for redundancy).
		classifyCyclingCard("Astral Drift",
			"Whenever you cycle a card, you may exile target creature. If you do, return that card to the battlefield.",
			"Enchantment"),
		classifyCyclingCard("Drake Haven",
			"Whenever you cycle or discard a card, you may pay {1}. If you do, create a 2/2 blue Drake creature token with flying.",
			"Enchantment"),
		classifyCyclingCard("Lightning Rift",
			"Whenever you cycle a card, you may pay {1}. If you do, this enchantment deals 2 damage to any target.",
			"Enchantment"),
		// Sources (4+ for redundancy).
		classifyCyclingCard("Decree of Justice",
			"Create X 4/4 white Angel creature tokens with flying. Cycling {2}{W}",
			"Sorcery"),
		classifyCyclingCard("Krosan Tusker",
			"Cycling {2}{G}. When you cycle this card, you may search your library for a basic land card.",
			"Creature — Beast"),
		classifyCyclingCard("Eternal Dragon",
			"Flying. Plainscycling {2}. {1}{W}{W}: Return this card from your graveyard to your hand.",
			"Creature — Dragon Spirit"),
		classifyCyclingCard("Astral Slide",
			"Whenever a player cycles a card, you may exile target creature. If you do, return that card to the battlefield. Cycling {2}",
			"Enchantment"),
	}

	chains := DetectValueChains(profiles)

	var cycling *ValueChain
	for i, c := range chains {
		if c.Name == "Cycling Engine" {
			cycling = &chains[i]
			break
		}
	}
	if cycling == nil {
		names := []string{}
		for _, c := range chains {
			names = append(names, c.Name)
		}
		t.Fatalf("Cycling Engine not detected; got engines=%v", names)
	}
	if cycling.Depth != 2 {
		t.Errorf("Cycling Engine depth want 2, got %d", cycling.Depth)
	}
	// Step 0 = CYCLE; should have 4 sources (Decree, Tusker, Eternal
	// Dragon, Astral Slide).
	if cycling.Steps[0].Label != "CYCLE" {
		t.Errorf("step 0 label want CYCLE, got %q", cycling.Steps[0].Label)
	}
	if len(cycling.Steps[0].Cards) < 4 {
		t.Errorf("step 0 cards want >=4 sources, got %d (%v)",
			len(cycling.Steps[0].Cards), cycling.Steps[0].Cards)
	}
	// Step 1 = PAYOFF; should have 3 (Drift, Haven, Rift; Astral Slide
	// is BOTH but bridges as well).
	if cycling.Steps[1].Label != "PAYOFF" {
		t.Errorf("step 1 label want PAYOFF, got %q", cycling.Steps[1].Label)
	}
	if len(cycling.Steps[1].Cards) < 3 {
		t.Errorf("step 1 cards want >=3 payoffs, got %d (%v)",
			len(cycling.Steps[1].Cards), cycling.Steps[1].Cards)
	}
	// Astral Slide both cycles AND is a payoff → bridge card.
	foundBridge := false
	for _, b := range cycling.BridgeCards {
		if b == "Astral Slide" {
			foundBridge = true
			break
		}
	}
	if !foundBridge {
		t.Errorf("Astral Slide should be flagged as bridge card (cycles + is payoff); "+
			"bridges=%v", cycling.BridgeCards)
	}
}

// Negative-of-the-fix: a deck with cyclers but NO payoffs should NOT
// trigger the Cycling Engine — the chain requires both steps.
func TestDetectValueChains_CyclingEngine_PayoffsOnlyNoChain(t *testing.T) {
	profiles := []CardProfile{
		classifyCyclingCard("Decree of Justice",
			"Create X 4/4 white Angel creature tokens with flying. Cycling {2}{W}",
			"Sorcery"),
		classifyCyclingCard("Krosan Tusker",
			"Cycling {2}{G}. When you cycle this card, you may search your library for a basic land card.",
			"Creature — Beast"),
		classifyCyclingCard("Eternal Dragon",
			"Flying. Plainscycling {2}. {1}{W}{W}: Return this card from your graveyard to your hand.",
			"Creature — Dragon Spirit"),
		// NO Astral Drift / Drake Haven / Lightning Rift / etc.
	}

	chains := DetectValueChains(profiles)
	for _, c := range chains {
		if c.Name == "Cycling Engine" {
			t.Errorf("Cycling Engine should NOT trigger without payoff step; got %+v", c)
		}
	}
}

// Rationale assertion — confirm the engine has a populated rationale.
func TestDetectValueChains_CyclingEngine_Rationale(t *testing.T) {
	profiles := []CardProfile{
		classifyCyclingCard("Astral Drift",
			"Whenever you cycle a card, you may exile target creature. If you do, return that card to the battlefield.",
			"Enchantment"),
		classifyCyclingCard("Drake Haven",
			"Whenever you cycle or discard a card, you may pay {1}. If you do, create a 2/2 blue Drake creature token with flying.",
			"Enchantment"),
		classifyCyclingCard("Decree of Justice",
			"Create X 4/4 white Angel creature tokens with flying. Cycling {2}{W}",
			"Sorcery"),
		classifyCyclingCard("Krosan Tusker",
			"Cycling {2}{G}. When you cycle this card, you may search your library for a basic land card.",
			"Creature — Beast"),
	}

	chains := DetectValueChains(profiles)
	var cycling *ValueChain
	for i, c := range chains {
		if c.Name == "Cycling Engine" {
			cycling = &chains[i]
			break
		}
	}
	if cycling == nil {
		t.Fatal("Cycling Engine not detected")
	}
	if cycling.Rationale == nil {
		t.Fatal("Cycling Engine rationale is nil")
	}
	if !strings.Contains(cycling.Rationale.Trigger, "cycling") {
		t.Errorf("rationale Trigger should mention cycling; got %q", cycling.Rationale.Trigger)
	}
	if !strings.Contains(cycling.Rationale.HowItWorks, "Astral Drift") &&
		!strings.Contains(cycling.Rationale.HowItWorks, "Drake Haven") &&
		!strings.Contains(cycling.Rationale.HowItWorks, "Lightning Rift") {
		t.Errorf("rationale HowItWorks should name a canonical payoff; got %q",
			cycling.Rationale.HowItWorks)
	}
}
