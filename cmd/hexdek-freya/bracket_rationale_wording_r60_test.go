package main

import (
	"strings"
	"testing"
)

// This suite pins the user-facing wording rewrite of three rationale
// messages identified as cryptic in the 8-precon survey under
// docs/precon-rationale-survey-r60.md. The Names (programmatic IDs)
// stay stable so existing tooling, docs, and JSON consumers keep
// working — only the human-readable Note text changes.
//
// Surveyed precons that drove each finding:
//   - Tuned-redundancy floor: Blame Game, Creative Energy, Corrupting
//     Influence (all stock B2 precons lifted by the floor)
//   - GC=0 ceiling: Animated Army, Buckle Up, Counter Intelligence
//   - Vulnerability cap: Coven Counters, Cabaretti Cacophony, Creative
//     Energy (all combo decks with unprotected pieces)

// findFloorSignalByName returns the BracketSignal with the given Name
// and matching Kind, or nil. Test helper for asserting Note prose.
func findFloorSignalByName(br *BracketRationale, name, kind string) *BracketSignal {
	if br == nil {
		return nil
	}
	for i := range br.Signals {
		if br.Signals[i].Name == name && br.Signals[i].Kind == kind {
			return &br.Signals[i]
		}
	}
	return nil
}

// TestRationaleWording_TunedRedundancyFloor_ExplainsWhyInPlainEnglish
// pins that the Tuned-redundancy floor's Note text explains WHY many
// finishers + cheap mana = Bracket 4, rather than just stating the
// numbers. A non-technical reader should understand "the deck reliably
// draws both a finisher and the mana to cast it" from the message.
func TestRationaleWording_TunedRedundancyFloor_ExplainsWhyInPlainEnglish(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.6,
		fastManaCount:    7,
		gameChangerCount: 1, // gate this floor open per PR #566 / dev-11 OR-gate
	}
	_, _, br := estimateMeasuredBracket(ctx, report, "")
	sig := findFloorSignalByName(br, "Tuned-redundancy floor", "floor")
	if sig == nil {
		t.Fatalf("Tuned-redundancy floor signal not present; got: %+v", br.Signals)
	}
	// "Bracket 4" spelled out, not "B4" abbreviation, in user-facing prose.
	if !strings.Contains(sig.Note, "Bracket 4") {
		t.Errorf("note should spell out \"Bracket 4\" (not \"B4\") for non-technical readers, got: %q", sig.Note)
	}
	// Plain-language WHY: "close the game" + "cheap mana producers" + reliability claim.
	for _, want := range []string{"close the game", "cheap mana", "same turn"} {
		if !strings.Contains(sig.Note, want) {
			t.Errorf("note should contain plain-language hook %q to explain why finishers+fastMana=B4, got: %q", want, sig.Note)
		}
	}
	// The cryptic old phrasing "X finishers + Y fast-mana pieces" alone is gone —
	// numbers still appear (with new framing), but the bare arithmetic listing
	// is replaced with prose. The raw-score reference uses "raw score was B%d"
	// not the older "(was B%d)" parenthetical.
	if strings.Contains(sig.Note, "fast-mana pieces (was B") {
		t.Errorf("note should not retain the old bare-arithmetic format \"X fast-mana pieces (was BN)\", got: %q", sig.Note)
	}
}

// TestRationaleWording_GCZeroCeiling_ExplainsHeldNotCapped pins that
// the GC=0 ceiling's note uses "held at Bracket 2" framing and tells
// the reader WHAT they could change to unlock the higher bracket.
// The old phrasing "capped at B2: no Game Changers and no true-infinite
// combo (was B3 on raw score)" was scannable but offered no action.
func TestRationaleWording_GCZeroCeiling_ExplainsHeldNotCapped(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(7), // below tunedRedundancy threshold so the floor doesn't re-lift
	}
	// Animated Army shape: dense combo lines + heavy curve + 0 GCs, no true
	// infinites. The raw score will be high (multiple +2/+3 bands) but
	// GC=0 ceiling should hold at B2.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           4.0,
		fastManaCount:    11,
		gameChangerCount: 0,
		comboCount:       10, // many "determined" loops, none categorically winning
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket != 2 {
		t.Fatalf("GC=0 no-combo deck should be held at B2, got B%d", bracket)
	}
	sig := findFloorSignalByName(br, "GC=0 ceiling", "ceiling")
	if sig == nil {
		t.Fatalf("GC=0 ceiling signal not present; got: %+v", br.Signals)
	}
	// "held at Bracket 2" framing (not the old "capped at B2").
	if !strings.Contains(sig.Note, "held at Bracket 2") {
		t.Errorf("note should use \"held at Bracket 2\" framing (was \"capped at B2\"), got: %q", sig.Note)
	}
	// Action hook — tell the reader HOW to unlock the next bracket.
	if !strings.Contains(sig.Note, "Adding a Game Changer") &&
		!strings.Contains(sig.Note, "real 2-card combo") {
		t.Errorf("note should suggest what to add (Game Changer or 2-card combo) to unlock the next bracket, got: %q", sig.Note)
	}
	// "raw score" still mentioned so power users can correlate with the score table above.
	if !strings.Contains(sig.Note, "raw score") {
		t.Errorf("note should still reference \"raw score\" so the score-table rows are correlated, got: %q", sig.Note)
	}
}

// TestRationaleWording_VulnerabilityCap_ActionableNotBureaucratic pins
// the Vulnerability cap note rewrite. Old prose: "felt power closer to
// B3 (single Path / Pongify / Imprisoned in the Moon resets the line).
// Bracket call unchanged; informational only." — three problems:
// (1) "felt power" is awkward, (2) hardcoded card-name examples don't
// match every meta, (3) "informational only" reads bureaucratic. New
// prose: plain-language "plays closer to Bracket 3 at the table", names
// the protection cards as an upgrade suggestion, ends with a friendly
// "heads-up, not a downgrade".
func TestRationaleWording_VulnerabilityCap_ActionableNotBureaucratic(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 4,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Thassa's Oracle", CMC: 2, Reason: "combo piece — no built-in protection"},
		},
		BracketRationale: &BracketRationale{
			FinalBracket: 4,
			FinalLabel:   "Optimized",
		},
	}
	appendVulnerabilityBracketNote(dp)
	note := findVulnerabilityNote(dp.BracketRationale)
	if note == nil {
		t.Fatalf("expected Vulnerability cap note")
	}
	// (1) "Bracket 3" spelled out, not bare "B3".
	if !strings.Contains(note.Note, "Bracket 3") {
		t.Errorf("note should spell out \"Bracket 3\", got: %q", note.Note)
	}
	// (2) Protection-card upgrade suggestions present.
	for _, want := range []string{"Lightning Greaves", "Heroic Intervention", "Veil of Summer"} {
		if !strings.Contains(note.Note, want) {
			t.Errorf("note should suggest %q as a protection upgrade, got: %q", want, note.Note)
		}
	}
	// (3) Friendly framing ("heads-up, not a downgrade"), not the old
	// "informational only" / "felt power" wording.
	if strings.Contains(note.Note, "felt power") {
		t.Errorf("note should not retain awkward \"felt power\" phrasing, got: %q", note.Note)
	}
	if strings.Contains(note.Note, "informational only") {
		t.Errorf("note should not retain bureaucratic \"informational only\" phrasing, got: %q", note.Note)
	}
	if !strings.Contains(note.Note, "heads-up") && !strings.Contains(note.Note, "bracket call itself is unchanged") {
		t.Errorf("note should make clear the bracket call is unchanged in friendly terms, got: %q", note.Note)
	}
	// Hardcoded Path/Pongify/Imprisoned card list is gone (was meta-specific).
	if strings.Contains(note.Note, "Path / Pongify / Imprisoned in the Moon") {
		t.Errorf("note should not retain hardcoded Path/Pongify/Imprisoned removal list (meta-specific), got: %q", note.Note)
	}
	// The piece-name evidence is still threaded into the note (kept from old prose).
	if !strings.Contains(note.Note, "Thassa's Oracle") {
		t.Errorf("note should still name the vulnerable piece, got: %q", note.Note)
	}
}
