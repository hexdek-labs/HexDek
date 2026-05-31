package main

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Interaction-floor tests — assert per-combo floor values + deck-level
// rollup across decks with known disruption profiles.
//
// Fixtures use 5+ combo decks (the task requirement) covering: bare
// no-defense baseline, counterspell-heavy stack-defense, protection-
// heavy permanent-defense, mixed graveyard/non-graveyard combos, and
// instant-only combos that can only be answered by counterspells.
// ---------------------------------------------------------------------------

// fiveComboBareDeck builds a deck with 5 distinct combos and ZERO
// defensive layer. Every combo should report InteractionFloor = 1
// (cheapest answer is single removal on an unprotected permanent).
func fiveComboBareDeck() (*ComboMetaInteraction, InteractionPackage) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "A1 + A2", Source: "true_infinite",
				PermanentPieces: 2, RemovalRequiredToBreak: 1, DominantThreat: "removal"},
			{ComboIndex: 1, Label: "B1 + B2", Source: "true_infinite",
				PermanentPieces: 2, RemovalRequiredToBreak: 1, DominantThreat: "removal"},
			{ComboIndex: 2, Label: "C1 + C2", Source: "determined",
				PermanentPieces: 2, RemovalRequiredToBreak: 1, DominantThreat: "removal"},
			{ComboIndex: 3, Label: "D1 + D2 + D3", Source: "determined",
				PermanentPieces: 3, RemovalRequiredToBreak: 1, DominantThreat: "removal",
				StaxScore: 3, StaxHosers: []string{"Rule of Law"}}, // multi-cast triggered
			{ComboIndex: 4, Label: "E1 + E2", Source: "graveyard_loop",
				PermanentPieces: 2, RemovalRequiredToBreak: 1, DominantThreat: "graveyard",
				GraveyardScore: 3, GraveyardHosers: []string{"Rest in Peace"}},
		},
	}
	return cma, InteractionPackage{} // no defenders
}

// TestFloor_FiveComboBare: every combo floor = 1, MinFloor=1, MaxFloor=1,
// CounterspellCount=0, ProtectionCount=0.
func TestFloor_FiveComboBare(t *testing.T) {
	cma, pkg := fiveComboBareDeck()
	r := BuildComboInteractionFloor(cma, pkg)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	if len(r.PerCombo) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(r.PerCombo))
	}
	for i, p := range r.PerCombo {
		if p.InteractionFloor != 1 {
			t.Errorf("combo %d (%s): InteractionFloor=%d, want 1", i, p.Label, p.InteractionFloor)
		}
		if p.DefensiveLayerTax != 0 {
			t.Errorf("combo %d: DefensiveLayerTax=%d, want 0", i, p.DefensiveLayerTax)
		}
	}
	if r.MinFloor != 1 || r.MaxFloor != 1 || r.MedianFloor != 1 {
		t.Errorf("Min/Max/Median: got %d/%d/%d, want 1/1/1",
			r.MinFloor, r.MaxFloor, r.MedianFloor)
	}
	if !strings.Contains(r.DeckbuildAdvice, "cheapest combo folds to 1") {
		t.Errorf("advice should warn about 1-interaction fold, got: %s", r.DeckbuildAdvice)
	}
}

// TestFloor_CounterspellHeavyDeck: 6 counterspells → CounterspellTax = 2.
// Combos whose cheapest axis is counter/stax/graveyard get +2; removal-only
// path stays at 1 (no protection).
func TestFloor_CounterspellHeavyDeck(t *testing.T) {
	cma, _ := fiveComboBareDeck()
	pkg := InteractionPackage{
		Counterspells: []string{"Counterspell", "Force of Will", "Mana Drain", "Mental Misstep", "Pact of Negation", "Swan Song"},
	}
	r := BuildComboInteractionFloor(cma, pkg)
	if r == nil {
		t.Fatal("expected non-nil")
	}
	// Every combo has an unprotected permanent, so removal axis is
	// cheapest at cost 1 with no protection tax → floor stays at 1.
	// CounterspellTax isn't applied because the removal axis wins.
	for _, p := range r.PerCombo {
		if p.InteractionFloor != 1 {
			t.Errorf("combo %s: floor=%d, want 1 (removal axis wins regardless of counterspell tax)",
				p.Label, p.InteractionFloor)
		}
	}
}

// TestFloor_ProtectionRaisesRemovalAxis: a deck with 6 protection
// pieces AND combos with no unprotected permanents (RemovalRequiredToBreak=2)
// + no stax/gy → cheapest is counter (1) or removal (2 + 2 tax = 4).
// Counter wins.
func TestFloor_ProtectionRaisesRemovalAxis(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "Protected1 + Protected2", Source: "true_infinite",
				PermanentPieces: 2, ProtectedPieces: 2, RemovalRequiredToBreak: 2},
		},
	}
	pkg := InteractionPackage{
		Protection: []string{"Veil of Summer", "Heroic Intervention", "Lightning Greaves", "Swiftfoot Boots", "Mother of Runes", "Silence"},
	}
	r := BuildComboInteractionFloor(cma, pkg)
	v := r.PerCombo[0]
	// Counter axis = 1 with 0 counterspell tax = 1.
	// Removal axis = 2 base + 2 protection tax = 4.
	// Cheapest = counter at 1.
	if v.CheapestAxis != "counter" {
		t.Errorf("CheapestAxis: got %q, want \"counter\" (counter wins over taxed removal)",
			v.CheapestAxis)
	}
	if v.InteractionFloor != 1 {
		t.Errorf("InteractionFloor: got %d, want 1", v.InteractionFloor)
	}
}

// TestFloor_BothLayersFullDefense: deck with 6 counters + 6 protection.
// CounterspellTax = 2, ProtectionTax = 2. Combo with permanent piece +
// stax + graveyard angles. All axes cost ≥ 3.
//
// Cheapest axis still picks the lowest TOTAL. With base cost 1
// across all 4 axes (1 removal cost, 1 counter, 1 stax, 1 graveyard):
//   - removal: 1 + 2 = 3
//   - counter: 1 + 2 = 3
//   - stax:    1 + 2 = 3
//   - graveyard: 1 + 2 = 3
// Tie at 3 → priority is removal > counter > stax > graveyard.
func TestFloor_BothLayersFullDefense(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "FullVuln + Piece", Source: "true_infinite",
				PermanentPieces: 2, RemovalRequiredToBreak: 1,
				StaxScore: 3, GraveyardScore: 3},
		},
	}
	pkg := InteractionPackage{
		Counterspells: []string{"a", "b", "c", "d", "e", "f"},
		Protection:    []string{"p", "q", "r", "s", "t", "u"},
	}
	r := BuildComboInteractionFloor(cma, pkg)
	v := r.PerCombo[0]
	if v.InteractionFloor != 3 {
		t.Errorf("InteractionFloor: got %d, want 3 (1 base + 2 tax)", v.InteractionFloor)
	}
	if v.CheapestAxis != "removal" {
		t.Errorf("CheapestAxis: got %q, want \"removal\" (tied at 3, removal wins priority)",
			v.CheapestAxis)
	}
	if v.DefensiveLayerTax != 2 {
		t.Errorf("DefensiveLayerTax: got %d, want 2", v.DefensiveLayerTax)
	}
	if !strings.Contains(r.DeckbuildAdvice, "robust") {
		t.Errorf("advice should call deck robust (MinFloor >= 3), got: %s", r.DeckbuildAdvice)
	}
}

// TestFloor_InstantOnlyComboCounterOnly: combo with no permanent pieces
// (Demonic Consultation + Tainted Pact shape). RemovalAnswerCost = 0
// (infeasible). Cheapest axis is counter at 1.
func TestFloor_InstantOnlyComboCounterOnly(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "Demonic Consultation + Tainted Pact",
				Source: "determined", PermanentPieces: 0, RemovalRequiredToBreak: 0},
		},
	}
	r := BuildComboInteractionFloor(cma, InteractionPackage{})
	v := r.PerCombo[0]
	if v.CheapestAxis != "counter" {
		t.Errorf("CheapestAxis: got %q, want \"counter\" (no permanents → removal infeasible)",
			v.CheapestAxis)
	}
	if v.InteractionFloor != 1 {
		t.Errorf("InteractionFloor: got %d, want 1", v.InteractionFloor)
	}
}

// TestFloor_MixedDeckRollup: 5-combo deck with mixed shapes. Without a
// counterspell tax every combo's counter axis (cost 1) wins, so all
// floors collapse to 1. Adding 3 counterspells to the deck pushes the
// counter axis to cost 2 — protected combos whose removal axis is
// also 2 now bubble up to floor 2, fragile combos stay at 1. Pins
// the Cheapest / Hardest identification, MinFloor / MaxFloor / Median.
func TestFloor_MixedDeckRollup(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			// Fragile combo: removal cost 1 + 0 tax = 1.
			{ComboIndex: 0, Label: "Fragile + Pair", Source: "true_infinite",
				PermanentPieces: 2, RemovalRequiredToBreak: 1},
			// Protected combo: removal 2 + 0 tax = 2, counter 1 + 1 tax = 2.
			// Tied at 2; removal wins priority. Floor = 2.
			{ComboIndex: 1, Label: "Protected1 + Protected2", Source: "true_infinite",
				PermanentPieces: 2, ProtectedPieces: 2, RemovalRequiredToBreak: 2},
			// Instant-only combo: removal infeasible, counter 1 + 1 = 2.
			{ComboIndex: 2, Label: "Inst + Inst", Source: "determined",
				PermanentPieces: 0, RemovalRequiredToBreak: 0},
			// Graveyard combo: removal cost 1 + 0 = 1 wins.
			{ComboIndex: 3, Label: "GY1 + GY2", Source: "graveyard_loop",
				PermanentPieces: 2, RemovalRequiredToBreak: 1, GraveyardScore: 3},
			// Storm/stax combo: removal cost 1 + 0 = 1 wins.
			{ComboIndex: 4, Label: "Storm + Storm + Storm", Source: "determined",
				PermanentPieces: 1, RemovalRequiredToBreak: 1, StaxScore: 3},
		},
	}
	pkg := InteractionPackage{Counterspells: []string{"a", "b", "c"}} // tax = 1
	r := BuildComboInteractionFloor(cma, pkg)
	if r.MinFloor != 1 {
		t.Errorf("MinFloor: got %d, want 1", r.MinFloor)
	}
	if r.MaxFloor != 2 {
		t.Errorf("MaxFloor: got %d, want 2", r.MaxFloor)
	}
	// Hardest is one of {Protected1+Protected2 [removal/2], Inst+Inst
	// [counter/2]} — both have floor 2. Tie-break is index ascending,
	// so the first one (Protected, index 1) wins.
	if r.HardestComboIndex != 1 {
		t.Errorf("HardestComboIndex: got %d, want 1 (Protected1+Protected2)",
			r.HardestComboIndex)
	}
	if r.HardestFloor != 2 {
		t.Errorf("HardestFloor: got %d, want 2", r.HardestFloor)
	}
	if r.CheapestFloor != 1 {
		t.Errorf("CheapestFloor: got %d, want 1", r.CheapestFloor)
	}
}

// TestFloor_TaxCap: 12 counterspells should still cap tax at 2 (cap-at-2
// is documented in the file header — beyond ~6 the opponent's interaction
// slot count is the real ceiling).
func TestFloor_TaxCap(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "Inst + Inst", Source: "determined",
				PermanentPieces: 0, RemovalRequiredToBreak: 0},
		},
	}
	pkg := InteractionPackage{}
	for i := 0; i < 12; i++ {
		pkg.Counterspells = append(pkg.Counterspells, "C")
	}
	r := BuildComboInteractionFloor(cma, pkg)
	v := r.PerCombo[0]
	if v.DefensiveLayerTax != 2 {
		t.Errorf("DefensiveLayerTax: got %d, want 2 (capped at 2)", v.DefensiveLayerTax)
	}
}

// TestFloor_NilOnNoCombos: nil ComboMetaInteraction → nil floor report.
func TestFloor_NilOnNoCombos(t *testing.T) {
	if r := BuildComboInteractionFloor(nil, InteractionPackage{}); r != nil {
		t.Errorf("expected nil for nil input, got %+v", r)
	}
	if r := BuildComboInteractionFloor(&ComboMetaInteraction{}, InteractionPackage{}); r != nil {
		t.Errorf("expected nil for empty PerCombo, got %+v", r)
	}
}

// TestFloor_DeckbuildAdvice_Tail: tail strings match the
// counterspell/protection deficit conditions.
func TestFloor_DeckbuildAdvice_Tail(t *testing.T) {
	cma := &ComboMetaInteraction{
		PerCombo: []ComboMetaVulnerability{
			{ComboIndex: 0, Label: "Bare", Source: "true_infinite",
				PermanentPieces: 2, RemovalRequiredToBreak: 1},
		},
	}
	cases := []struct {
		pkg  InteractionPackage
		want string
	}{
		{InteractionPackage{}, "add 3-4 counterspells AND 2-3 protection slots"},
		{InteractionPackage{Counterspells: []string{"a", "b", "c"}}, "add 2-3 protection slots"},
		{InteractionPackage{Protection: []string{"a", "b", "c"}}, "add 3-4 counterspells"},
		{InteractionPackage{
			Counterspells: []string{"a", "b", "c"},
			Protection:    []string{"a", "b", "c"},
		}, "defensive layer is already healthy"},
	}
	for i, c := range cases {
		r := BuildComboInteractionFloor(cma, c.pkg)
		if !strings.Contains(r.DeckbuildAdvice, c.want) {
			t.Errorf("case %d: advice %q does not contain %q", i, r.DeckbuildAdvice, c.want)
		}
	}
}

// TestPrintComboInteractionFloor_RendersExpectedShape: text output
// contains canonical headers, per-combo floor lines, and the advice tail.
func TestPrintComboInteractionFloor_RendersExpectedShape(t *testing.T) {
	cma, pkg := fiveComboBareDeck()
	r := BuildComboInteractionFloor(cma, pkg)
	var buf bytes.Buffer
	printComboInteractionFloor(&buf, r)
	out := buf.String()
	musts := []string{
		"INTERACTION FLOOR",
		"min 1, max 1, median 1",
		"Cheapest line to break:",
		"axis (cost",
		"Advice:",
		"cheapest combo folds to 1 interaction",
	}
	for _, s := range musts {
		if !strings.Contains(out, s) {
			t.Errorf("text output missing %q\nfull:\n%s", s, out)
		}
	}
}

// TestPrintComboInteractionFloor_NilNoOp: nil input renders nothing.
func TestPrintComboInteractionFloor_NilNoOp(t *testing.T) {
	var buf bytes.Buffer
	printComboInteractionFloor(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected zero output for nil, got: %s", buf.String())
	}
}
