package main

import (
	"strings"
	"testing"
)

// TestCEDHFreeInteractionList_BasicShape sanity-checks the curated free
// interaction list. Catches accidental list collapse from refactors and
// pins the canonical entries that downstream tests rely on.
func TestCEDHFreeInteractionList_BasicShape(t *testing.T) {
	if len(cedhFreeInteractionList) < 20 {
		t.Errorf("cedhFreeInteractionList collapsed to %d entries (expected 20+)",
			len(cedhFreeInteractionList))
	}
	// Anchor entries — each comes from a distinct mechanic family. If
	// any one of these falls out, the bracket scoring loses coverage of
	// that whole free-interaction category.
	mustHave := []string{
		"force of will",        // pitch counter
		"fierce guardianship",  // commander-free
		"mental misstep",       // phyrexian-mana
		"pact of negation",     // pact cycle
		"deflecting swat",      // commander-free redirect
		"deadly rollick",       // commander-free removal
		"subtlety",             // evoke counter
		"endurance",            // evoke graveyard hate
	}
	for _, name := range mustHave {
		if !cedhFreeInteractionList[name] {
			t.Errorf("free interaction list missing canonical entry %q", name)
		}
	}
	// Negative check: cheap-but-not-free interaction must NOT be in the
	// list (those are B4 staples, not the B5 discriminator).
	mustNotHave := []string{
		"counterspell",            // 2 mana
		"swan song",               // 1 mana
		"an offer you can't refuse", // 2 mana
		"sol ring",                // mana rock, not interaction
	}
	for _, name := range mustNotHave {
		if cedhFreeInteractionList[name] {
			t.Errorf("free interaction list incorrectly includes %q (not free)", name)
		}
	}
}

// TestEstimateBracket_FreeInteractionLifts verifies free-interaction
// count contributes to the bracket score. A deck identical except for
// adding free interaction should score higher.
func TestEstimateBracket_FreeInteractionLifts(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	baseCtx := func() *classifyContext {
		return &classifyContext{
			roleRatios:       map[RoleTag]float64{},
			avgCMC:           2.4,
			tutorDensity:     0.05,
			comboCount:       1,
			fastManaCount:    4,
			gameChangerCount: 2,
		}
	}

	noFree := baseCtx()
	withFree := baseCtx()
	withFree.freeInteractionCount = 4

	bNoFree, _, _ := estimateBracket(noFree, report, "")
	bWithFree, _, _ := estimateBracket(withFree, report, "")

	if bWithFree < bNoFree {
		t.Errorf("free interaction shouldn't lower bracket: noFree=B%d, withFree=B%d",
			bNoFree, bWithFree)
	}
}

// TestEstimateBracket_B5GateRequiresCEDHMarker verifies the B5
// confirmation gate: a deck that scores 12+ via GC + finishers + fast
// mana alone but lacks free interaction, deep tutor density, or 8+ GCs
// gets demoted to B4. Otherwise "optimized big-mana piles" would
// falsely register as cEDH.
func TestEstimateBracket_B5GateRequiresCEDHMarker(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(8), // +2 finisher score bonus
	}

	// Deck that hits score >= 12 via GC=4 (+3), fast mana=10 (+3), avg
	// CMC=2.4 (+1), tutor=8% (+2), combo=2 (+2), low land (+1), finisher
	// >=8 (+2) = 14 → would be B5. But: freeInt=0, tutor=8% (< 12%),
	// GC=4 (< 8). Gate demotes to B4.
	demotedCtx := &classifyContext{
		roleRatios:       map[RoleTag]float64{RoleLand: 0.28},
		avgCMC:           2.4,
		tutorDensity:     0.08,
		comboCount:       2,
		fastManaCount:    10,
		gameChangerCount: 4,
		// no free interaction
	}
	bDemoted, lblDemoted, _ := estimateBracket(demotedCtx, report, "")
	if bDemoted != 4 {
		t.Errorf("demotion gate: want B4, got B%d (%s) — high-score deck without cEDH markers should fall back",
			bDemoted, lblDemoted)
	}

	// Same shape PLUS 2 free interaction pieces → cEDH marker present,
	// stays at B5.
	keepCtx := &classifyContext{
		roleRatios:           map[RoleTag]float64{RoleLand: 0.28},
		avgCMC:               2.4,
		tutorDensity:         0.08,
		comboCount:           2,
		fastManaCount:        10,
		gameChangerCount:     4,
		freeInteractionCount: 2,
	}
	bKeep, lblKeep, _ := estimateBracket(keepCtx, report, "")
	if bKeep != 5 {
		t.Errorf("free-int marker present: want B5, got B%d (%s)", bKeep, lblKeep)
	}
}

// TestEstimateBracket_B5GateRequiresLowCMC verifies the gate also
// demotes high-score decks with too-high average CMC — cEDH decks are
// lean (sub-2.8 CMC); a 3.0+ pile won't win on turn 3-4 regardless of
// how many Game Changers it stacks.
func TestEstimateBracket_B5GateRequiresLowCMC(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(8),
	}

	// 8 GCs (+4) and 12% tutors (+3) and 5 combos (+3) and fast mana=10
	// (+3) and finishers=8 (+2) — but CMC=3.2 (-0 because >3.5 is the
	// negative threshold) → score ~15. Has cEDH markers but slow curve.
	slowCtx := &classifyContext{
		roleRatios:           map[RoleTag]float64{RoleLand: 0.28},
		avgCMC:               3.2,
		tutorDensity:         0.12,
		comboCount:           5,
		fastManaCount:        10,
		gameChangerCount:     8,
		freeInteractionCount: 2,
	}
	bSlow, lblSlow, _ := estimateBracket(slowCtx, report, "")
	if bSlow != 4 {
		t.Errorf("slow-CMC gate: want B4, got B%d (%s) — high-CMC pile shouldn't reach cEDH",
			bSlow, lblSlow)
	}

	// Same shape with lean CMC=2.2 → stays at B5.
	leanCtx := &classifyContext{
		roleRatios:           map[RoleTag]float64{RoleLand: 0.28},
		avgCMC:               2.2,
		tutorDensity:         0.12,
		comboCount:           5,
		fastManaCount:        10,
		gameChangerCount:     8,
		freeInteractionCount: 2,
	}
	bLean, _, _ := estimateBracket(leanCtx, report, "")
	if bLean != 5 {
		t.Errorf("lean-CMC keep: want B5, got B%d", bLean)
	}
}

// TestBuildSignals_FreeInteractionEmitted verifies that decks with 2+
// free interaction pieces surface the signal string so report readers
// can see WHY the deck reads as cEDH.
func TestBuildSignals_FreeInteractionEmitted(t *testing.T) {
	ctx := &classifyContext{
		roleRatios:           map[RoleTag]float64{},
		freeInteractionCount: 3,
	}
	ac := &ArchetypeClassification{}
	signals := buildSignals(ctx, ac)

	found := false
	for _, s := range signals {
		if strings.Contains(s, "free interaction suite") && strings.Contains(s, "3") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("free interaction signal missing from signals: %v", signals)
	}

	// Below threshold (1 piece) should NOT emit — noise filter.
	quietCtx := &classifyContext{
		roleRatios:           map[RoleTag]float64{},
		freeInteractionCount: 1,
	}
	quietSignals := buildSignals(quietCtx, ac)
	for _, s := range quietSignals {
		if strings.Contains(s, "free interaction suite") {
			t.Errorf("1-piece free interaction shouldn't emit signal, got: %q", s)
		}
	}
}

// makeComboList builds a slice of ComboResult stubs of size n for tests
// that need to satisfy report.Finishers length checks without caring
// about contents.
func makeComboList(n int) []ComboResult {
	out := make([]ComboResult, n)
	for i := range out {
		out[i].Cards = []string{"x"}
	}
	return out
}
