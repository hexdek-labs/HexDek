package main

import (
	"strings"
	"testing"
)

// reportWith builds the minimal FreyaReport shape the blurb reads from.
func reportWith(wl ...WinLine) *FreyaReport {
	return &FreyaReport{WinLines: &WinLineAnalysis{WinLines: wl}}
}

// TestPersonalityBlurb_NamesInfiniteFinisher pins the headline improvement:
// a combo deck's blurb should NAME the kill, not just count win lines. The
// pre-r60 version closed with "With 3 paths to victory, it always has a
// plan B." for a Thoracle deck — useful as a tally, useless as flavor.
func TestPersonalityBlurb_NamesInfiniteFinisher(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Combo",
		AvgCMC:           2.4,
		WinLineCount:     3,
		PrimaryWinLine:   "Thassa's Oracle + Demonic Consultation",
		HasTutorAccess:   true,
		Bracket:          4,
		BracketLabel:     "Optimized",
	}
	rep := reportWith(WinLine{
		Pieces: []string{"Thassa's Oracle", "Demonic Consultation"},
		Type:   "infinite",
	})
	blurb := buildPersonalityBlurb(dp, rep)
	if !strings.Contains(blurb, "Thassa's Oracle") || !strings.Contains(blurb, "Demonic Consultation") {
		t.Errorf("expected blurb to name the combo pieces, got %q", blurb)
	}
	if !strings.Contains(blurb, "infinite") && !strings.Contains(blurb, "ends the game") {
		t.Errorf("expected infinite-loop framing in blurb, got %q", blurb)
	}
	if strings.Contains(blurb, "With 3 paths to victory, it always has a plan B.") {
		t.Errorf("blurb should not fall back to generic win-line count when a named kill exists: %q", blurb)
	}
}

// TestPersonalityBlurb_FinisherCard pins the finisher-type framing: a
// solo finisher like Craterhoof should be NAMED, not buried under "you
// have multiple win lines."
func TestPersonalityBlurb_FinisherCard(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Aggro / Go Wide",
		AvgCMC:           2.9,
		WinLineCount:     2,
		PrimaryWinLine:   "Craterhoof Behemoth",
		Bracket:          3,
		BracketLabel:     "Upgraded",
	}
	rep := reportWith(WinLine{
		Pieces: []string{"Craterhoof Behemoth"},
		Type:   "finisher",
	})
	blurb := buildPersonalityBlurb(dp, rep)
	if !strings.Contains(blurb, "Craterhoof Behemoth") {
		t.Errorf("expected blurb to name the finisher card, got %q", blurb)
	}
	if !strings.Contains(blurb, "ends games") {
		t.Errorf("expected finisher framing in blurb, got %q", blurb)
	}
}

// TestPersonalityBlurb_VoltronCommanderDamage pins the commander_damage
// branch — the closer should reference the actual commander, not a Pieces
// list (which for commander_damage lines is just the commander's name
// anyway, but the framing should call out the 21-damage clock).
func TestPersonalityBlurb_VoltronCommanderDamage(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Voltron",
		Commander:        "Uril, the Miststalker",
		AvgCMC:           2.6,
		WinLineCount:     1,
		PrimaryWinLine:   "Uril, the Miststalker",
		Bracket:          3,
		BracketLabel:     "Upgraded",
	}
	rep := reportWith(WinLine{
		Pieces: []string{"Uril, the Miststalker"},
		Type:   "commander_damage",
	})
	blurb := buildPersonalityBlurb(dp, rep)
	if !strings.Contains(blurb, "Uril, the Miststalker") {
		t.Errorf("expected commander name in voltron blurb, got %q", blurb)
	}
	if !strings.Contains(blurb, "commander damage") {
		t.Errorf("expected '21 commander damage' framing, got %q", blurb)
	}
	if !strings.Contains(blurb, "suits up its commander") {
		t.Errorf("expected voltron approach line, got %q", blurb)
	}
}

// TestPersonalityBlurb_StaxNotConflatedWithControl ensures stax decks no
// longer share the control blurb. Pre-r60, "It grinds opponents into
// submission... accumulating an insurmountable advantage" was used for
// both — but stax doesn't accumulate, it denies. Different voice.
func TestPersonalityBlurb_StaxNotConflatedWithControl(t *testing.T) {
	stax := &DeckProfile{
		PrimaryArchetype: "Stax",
		AvgCMC:           3.1,
		Bracket:          4,
		BracketLabel:     "Optimized",
	}
	ctrl := &DeckProfile{
		PrimaryArchetype: "Control",
		AvgCMC:           3.1,
		Bracket:          4,
		BracketLabel:     "Optimized",
	}
	staxBlurb := buildPersonalityBlurb(stax, reportWith())
	ctrlBlurb := buildPersonalityBlurb(ctrl, reportWith())

	if !strings.Contains(staxBlurb, "locks the table down") && !strings.Contains(staxBlurb, "resource denial") {
		t.Errorf("expected stax-specific denial framing, got %q", staxBlurb)
	}
	if strings.Contains(staxBlurb, "drawing extra cards") {
		t.Errorf("stax blurb leaked control phrasing: %q", staxBlurb)
	}
	if !strings.Contains(ctrlBlurb, "Answers threats") && !strings.Contains(ctrlBlurb, "answers threats") {
		t.Errorf("expected control answer-the-threats framing, got %q", ctrlBlurb)
	}
}

// TestPersonalityBlurb_CoversExpandedArchetypes pins that the 11 new
// archetype branches added in r60 (storm, artifacts, lifegain,
// spellslinger, counters matter, extra combats, theft, ninjutsu,
// discard, tribal, ramp) no longer fall through to the generic
// "It plays a flexible game" default.
func TestPersonalityBlurb_CoversExpandedArchetypes(t *testing.T) {
	cases := []struct {
		arch       string
		mustContain string
	}{
		{"Storm", "storm count"},
		{"Artifacts", "artifact"},
		{"Lifegain", "life buffer"},
		{"Spellslinger", "magecraft"},
		{"Counters Matter", "+1/+1 counters"},
		{"Extra Combats", "attack steps"},
		{"Theft / Clone", "hijacks"},
		{"Ninjutsu / Evasion", "ninjutsus"},
		{"Discard / Hand Attack", "strips opponents"},
		{"Tribal", "tribal swarm"},
		{"Ramp", "ahead of curve"},
	}
	for _, tc := range cases {
		dp := &DeckProfile{
			PrimaryArchetype: tc.arch,
			AvgCMC:           3.0,
			Bracket:          3,
			BracketLabel:     "Upgraded",
		}
		blurb := buildPersonalityBlurb(dp, reportWith())
		if !strings.Contains(strings.ToLower(blurb), strings.ToLower(tc.mustContain)) {
			t.Errorf("%s blurb missing %q: %q", tc.arch, tc.mustContain, blurb)
		}
		if strings.Contains(blurb, "plays a flexible game") {
			t.Errorf("%s blurb fell through to generic default: %q", tc.arch, blurb)
		}
	}
}

// TestPersonalityBlurb_SpeedFactorsRamp pins the speed-label fix: a
// high-CMC deck with heavy ramp is "explosive" (the ramp is the plan),
// not "slow but devastating". Likewise high-CMC + no ramp is "lumbering"
// rather than the misleading "devastating".
func TestPersonalityBlurb_SpeedFactorsRamp(t *testing.T) {
	explosive := &DeckProfile{
		PrimaryArchetype: "Ramp",
		AvgCMC:           4.2,
		RampCount:        15,
	}
	lumbering := &DeckProfile{
		PrimaryArchetype: "Midrange",
		AvgCMC:           4.2,
		RampCount:        4,
	}
	if got := describeSpeed(explosive); got != "explosive" {
		t.Errorf("high-CMC heavy-ramp deck should be 'explosive', got %q", got)
	}
	if got := describeSpeed(lumbering); got != "lumbering" {
		t.Errorf("high-CMC no-ramp deck should be 'lumbering', got %q", got)
	}
	// Pre-r60 sanity: lean curves still read as lightning-fast regardless
	// of ramp, and the original "methodical" mid-tier still exists.
	lean := &DeckProfile{PrimaryArchetype: "Aggro", AvgCMC: 2.2}
	if got := describeSpeed(lean); got != "lightning-fast" {
		t.Errorf("lean curve should stay 'lightning-fast', got %q", got)
	}
	mid := &DeckProfile{PrimaryArchetype: "Midrange", AvgCMC: 3.3, RampCount: 8}
	if got := describeSpeed(mid); got != "methodical" {
		t.Errorf("midrange curve with normal ramp should be 'methodical', got %q", got)
	}
}

// TestPersonalityBlurb_FallbackWhenNoNamedFinisher pins that when the
// primary win line is generic "combat" with no named card, the closer
// falls back to bracket/synergy framing instead of awkwardly emitting
// an empty Pieces string.
func TestPersonalityBlurb_FallbackWhenNoNamedFinisher(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Aggro",
		AvgCMC:           2.6,
		WinLineCount:     1,
		Bracket:          2,
		BracketLabel:     "Core",
	}
	rep := reportWith(WinLine{Pieces: []string{}, Type: "combat"})
	blurb := buildPersonalityBlurb(dp, rep)
	if strings.Contains(blurb, "  ") || strings.Contains(blurb, "is the trigger") {
		t.Errorf("blurb should fall back cleanly, got %q", blurb)
	}
	if !strings.Contains(blurb, "casual") && !strings.Contains(blurb, "Core") {
		t.Errorf("low-bracket fallback should reference Core/casual framing, got %q", blurb)
	}
}
