package main

import (
	"bytes"
	"strings"
	"testing"
)

// Canonical fixture combos used across the detection tests. The
// detector walks 2-card combos in TrueInfinites + Determined, so we
// stick the fixtures in Determined to exercise the realistic path.
func karmicReveillarkCombo() ComboResult {
	return ComboResult{
		Cards:       []string{"Karmic Guide", "Reveillark"},
		LoopType:    "true_infinite",
		Class:       ComboClassInfiniteETB,
		Description: "Karmic Guide + Reveillark blink/return loop with a sac outlet.",
	}
}

func heliodBallistaCombo() ComboResult {
	return ComboResult{
		Cards:       []string{"Heliod, Sun-Crowned", "Walking Ballista"},
		LoopType:    "true_infinite",
		Class:       ComboClassInfiniteDamage,
		Description: "Heliod gives Ballista lifelink; counter-removal pings deal damage and rebuy.",
	}
}

func TestDetectGraveyardLoopCombos_DetectsKaradorKarmicReveillark(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		{Name: "Karador, Ghost Chieftain", TypeLine: "Legendary Creature — Centaur Spirit", CMC: 8},
		{Name: "Sol Ring", TypeLine: "Artifact", CMC: 1},
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{karmicReveillarkCombo()})
	if len(out) != 1 {
		t.Fatalf("want 1 graveyard-loop entry, got %d", len(out))
	}
	got := out[0]
	if got.Class != ComboClassGraveyardLoop {
		t.Errorf("class: got %q, want %q", got.Class, ComboClassGraveyardLoop)
	}
	if got.LoopType != "synergy" {
		t.Errorf("loop type: got %q, want %q", got.LoopType, "synergy")
	}
	if len(got.Cards) != 3 {
		t.Fatalf("want 3-card tuple [pieceA, pieceB, enabler], got %d cards", len(got.Cards))
	}
	if got.Cards[2] != "Karador, Ghost Chieftain" {
		t.Errorf("enabler position: got %q, want %q", got.Cards[2], "Karador, Ghost Chieftain")
	}
	if !strings.Contains(got.Description, "Karador") {
		t.Errorf("description must name the enabler: %q", got.Description)
	}
}

// No enabler in deck → no detection. Common case for aggro / control
// decks; the function must short-circuit before walking combos.
func TestDetectGraveyardLoopCombos_NoEnablerSkips(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		{Name: "Lightning Bolt", TypeLine: "Instant", CMC: 1},
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{karmicReveillarkCombo()})
	if len(out) != 0 {
		t.Errorf("expected no detection without enabler, got %d", len(out))
	}
}

// Sun Titan's CMC≤3 restriction: a Karmic Guide + Reveillark combo
// (both CMC 5) is NOT recurable by Sun Titan alone, so a deck with
// Sun Titan as the only enabler must produce no detection. Karador's
// "any CMC, creature only" handles the same combo. Verifies the
// Recurs predicate is enforced per-enabler, not collapsed to a single
// "any recursion" check.
func TestDetectGraveyardLoopCombos_SunTitanCMCBound(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", CMC: 6},
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{karmicReveillarkCombo()})
	if len(out) != 0 {
		t.Errorf("Sun Titan can't recur CMC-5 pieces; want 0 detections, got %d", len(out))
	}

	// Same combo with a CMC-3 piece IS recurable by Sun Titan.
	mikaeusBallista := ComboResult{
		Cards:       []string{"Mikaeus, the Unhallowed", "Walking Ballista"},
		LoopType:    "true_infinite",
		Class:       ComboClassInfiniteDamage,
		Description: "Mikaeus undying + Walking Ballista damage loop.",
	}
	profiles2 := []CardProfile{
		{Name: "Mikaeus, the Unhallowed", TypeLine: "Legendary Creature — Zombie Cleric", CMC: 6},
		{Name: "Walking Ballista", TypeLine: "Artifact Creature — Construct", CMC: 0},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", CMC: 6},
	}
	out2 := detectGraveyardLoopCombos(profiles2, []ComboResult{mikaeusBallista})
	if len(out2) != 1 {
		t.Fatalf("Sun Titan should rebuy CMC-0 Walking Ballista; want 1 detection, got %d", len(out2))
	}
}

// Multiple enablers in the same deck → one entry per (combo × enabler).
// A Reanimator deck running Karador + Muldrotha + Sun Titan for the
// same Karmic + Reveillark loop should surface 2 entries (Karador
// recurs creatures, Muldrotha recurs permanent types; Sun Titan's
// CMC bound excludes the CMC-5 pieces).
func TestDetectGraveyardLoopCombos_PerEnablerEntry(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		{Name: "Karador, Ghost Chieftain", TypeLine: "Legendary Creature", CMC: 8},
		{Name: "Muldrotha, the Gravetide", TypeLine: "Legendary Creature", CMC: 6},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", CMC: 6}, // bound-excluded
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{karmicReveillarkCombo()})
	enablers := map[string]bool{}
	for _, c := range out {
		enablers[c.Cards[2]] = true
	}
	if len(out) != 2 || !enablers["Karador, Ghost Chieftain"] || !enablers["Muldrotha, the Gravetide"] {
		t.Errorf("want 2 entries (Karador + Muldrotha); got %d entries with enablers %v", len(out), enablers)
	}
}

// Dedup: the same combo appearing in both TrueInfinites and Determined
// (synergy detection sometimes promotes the same pair into both
// buckets) must NOT emit two graveyard-loop entries for the same
// enabler. Stable key is sorted-cards + enabler-name.
func TestDetectGraveyardLoopCombos_DedupAcrossBuckets(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		{Name: "Karador, Ghost Chieftain", TypeLine: "Legendary Creature", CMC: 8},
	}
	combo := karmicReveillarkCombo()
	out := detectGraveyardLoopCombos(profiles, []ComboResult{combo, combo})
	if len(out) != 1 {
		t.Errorf("duplicate combo input must dedup: got %d, want 1", len(out))
	}
}

// Skip when enabler IS one of the combo pieces — e.g. Karmic Guide
// pairs with Reveillark BUT Karmic Guide is also in the enabler
// table. Surfacing "Karmic Guide pairs with Karmic Guide" is noise.
// Detection must skip the (combo, enabler) tuple when they overlap.
func TestDetectGraveyardLoopCombos_SkipEnablerInCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Karmic Guide", TypeLine: "Creature — Spirit", CMC: 5},
		{Name: "Reveillark", TypeLine: "Creature — Elemental", CMC: 5},
		// Karmic Guide is the ONLY enabler — no Karador / Muldrotha.
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{karmicReveillarkCombo()})
	for _, c := range out {
		if c.Cards[2] == "Karmic Guide" || c.Cards[2] == "Reveillark" {
			t.Errorf("enabler must not overlap with combo pieces; got %v", c.Cards)
		}
	}
}

// Non-buriable pieces (instants/sorceries) with a non-burial combo
// class don't trigger detection — Demonic Consultation + Thassa's
// Oracle is a categorical library-exile win, but Consultation is an
// instant (not recurable as a "permanent" piece) AND Oracle is CMC 2
// (recurable by Sun Titan when present). Verifies the per-piece check.
func TestDetectGraveyardLoopCombos_OracleConsultationWithSunTitan(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Thassa's Oracle", TypeLine: "Creature — Merfolk Wizard", CMC: 2},
		{Name: "Demonic Consultation", TypeLine: "Instant", CMC: 1},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", CMC: 6},
	}
	thoracle := ComboResult{
		Cards:       []string{"Thassa's Oracle", "Demonic Consultation"},
		LoopType:    "true_infinite",
		Class:       ComboClassLibraryExileWin,
		Description: "Empty library + Oracle ETB wins.",
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{thoracle})
	if len(out) != 1 {
		t.Fatalf("Sun Titan should rebuy Thassa's Oracle (CMC 2, creature, buriable); want 1, got %d", len(out))
	}
	if out[0].Cards[2] != "Sun Titan" {
		t.Errorf("enabler: got %q, want %q", out[0].Cards[2], "Sun Titan")
	}
}

// 3+ card combos are out of scope — the WotC "winning 2-card combo"
// framing is specifically about 2-card lines. Larger combos may have
// their own classification path elsewhere; the graveyard-loop signal
// is restricted to 2-card combos.
func TestDetectGraveyardLoopCombos_ThreeCardComboSkipped(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Worldgorger Dragon", TypeLine: "Creature — Dragon", CMC: 7},
		{Name: "Animate Dead", TypeLine: "Enchantment — Aura", CMC: 2},
		{Name: "Bolas's Citadel", TypeLine: "Legendary Artifact", CMC: 6},
		{Name: "Karador, Ghost Chieftain", TypeLine: "Legendary Creature", CMC: 8},
	}
	bigCombo := ComboResult{
		Cards: []string{"Worldgorger Dragon", "Animate Dead", "Bolas's Citadel"},
		Class: ComboClassInfiniteETB,
	}
	out := detectGraveyardLoopCombos(profiles, []ComboResult{bigCombo})
	if len(out) != 0 {
		t.Errorf("3-card combos out of scope; want 0, got %d", len(out))
	}
}

// Bracket scoring: a single graveyard-loop combo emits a +2 scoring
// signal. Two or more emit a +3 signal (tier "2+ pairs"). Pins the
// per-tier contribution and tier label.
func TestEstimateBracket_GraveyardLoopSignalContribution(t *testing.T) {
	gyOne := ComboResult{
		Cards: []string{"Karmic Guide", "Reveillark", "Karador, Ghost Chieftain"},
		Class: ComboClassGraveyardLoop,
	}
	gyTwo := ComboResult{
		Cards: []string{"Karmic Guide", "Reveillark", "Muldrotha, the Gravetide"},
		Class: ComboClassGraveyardLoop,
	}
	cases := []struct {
		name         string
		loops        []ComboResult
		wantContrib  int
		wantTierFrag string
	}{
		{"one pair", []ComboResult{gyOne}, 2, "1 pair"},
		{"two pairs", []ComboResult{gyOne, gyTwo}, 3, "2+ pairs"},
	}
	for _, tc := range cases {
		report := &FreyaReport{
			Roles:          &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
			GraveyardLoops: tc.loops,
		}
		ctx := &classifyContext{
			roleRatios: map[RoleTag]float64{},
			avgCMC:     3.0,
		}
		_, _, br := estimateMeasuredBracket(ctx, report, "")
		var sig *BracketSignal
		for i := range br.Signals {
			if br.Signals[i].Name == "Graveyard loop combo" {
				sig = &br.Signals[i]
				break
			}
		}
		if sig == nil {
			t.Errorf("%s: missing 'Graveyard loop combo' signal", tc.name)
			continue
		}
		if sig.Contribution != tc.wantContrib {
			t.Errorf("%s: contribution got +%d, want +%d", tc.name, sig.Contribution, tc.wantContrib)
		}
		if !strings.Contains(sig.Tier, tc.wantTierFrag) {
			t.Errorf("%s: tier %q must contain %q", tc.name, sig.Tier, tc.wantTierFrag)
		}
		if len(sig.Evidence) == 0 {
			t.Errorf("%s: evidence card list must not be empty", tc.name)
		}
	}
}

// No graveyard-loop combos → no signal emitted (clean negative case;
// defends against the signal firing as a +0 entry on every deck).
func TestEstimateBracket_GraveyardLoopSignalAbsentWhenEmpty(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	ctx := &classifyContext{roleRatios: map[RoleTag]float64{}, avgCMC: 3.0}
	_, _, br := estimateMeasuredBracket(ctx, report, "")
	for _, s := range br.Signals {
		if s.Name == "Graveyard loop combo" {
			t.Errorf("must not emit graveyard-loop signal when no loops detected; got %+v", s)
		}
	}
}

// End-to-end B3 emergence: a deck with 1 graveyard-loop combo + a
// modest baseline (some tutors, average CMC, 1 GC) should land at B3.
// Pins the WotC bracket framing — "value engines that loop with a
// graveyard enabler" are B3, NOT B4 (the GY loop is softer than a
// 2-card categorical-win infinite).
func TestEstimateBracket_GraveyardLoopReachesB3(t *testing.T) {
	gyLoop := ComboResult{
		Cards: []string{"Karmic Guide", "Reveillark", "Karador, Ghost Chieftain"},
		Class: ComboClassGraveyardLoop,
	}
	report := &FreyaReport{
		Roles:          &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		GraveyardLoops: []ComboResult{gyLoop},
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,  // +1 (moderate)
		tutorDensity:     0.06, // +1 (4-7%)
		gameChangerCount: 1,    // +1 GC + B2 floor
	}
	// Baseline: GC=1 (+1) + tutor (+1) + cmc (+1) = 3 → B2.
	// With graveyard-loop signal: +2 → score 5 → B3.
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket < 3 {
		t.Errorf("graveyard-loop combo + modest signals should reach B3; got B%d (raw score %d)",
			bracket, br.RawScore)
	}
	// Must NOT lift to B4 — the graveyard-loop class is a B3 marker,
	// not a B4 carveout. hasWinningCombo only fires for true-infinites
	// or categorical-win 2-card combos.
	if bracket >= 4 {
		t.Errorf("graveyard-loop alone must not lift to B4 (that's the categorical-win-combo floor); got B%d", bracket)
	}
}

// JSON output includes the graveyard_loops array when populated and
// omits the key entirely when empty (omitempty contract). Defends
// against silent loss when downstream consumers consult the field.
func TestGraveyardLoops_JSONOutput(t *testing.T) {
	r := &FreyaReport{
		DeckName:   "test",
		TotalCards: 99,
		GraveyardLoops: []ComboResult{{
			Cards:       []string{"Karmic Guide", "Reveillark", "Karador, Ghost Chieftain"},
			LoopType:    "synergy",
			Class:       ComboClassGraveyardLoop,
			Description: "graveyard backup",
		}},
		ManaCurve: [8]int{0, 0, 0, 0, 0, 0, 0, 0},
		ColorDemand: map[string]int{},
		ColorSupply: map[string]int{},
	}
	var buf bytes.Buffer
	printJSON(&buf, r)
	s := buf.String()
	if !strings.Contains(s, `"graveyard_loops"`) {
		t.Errorf("JSON output missing graveyard_loops key:\n%s", s)
	}
	if !strings.Contains(s, "graveyard_loop") {
		t.Errorf("JSON output missing class tag 'graveyard_loop'")
	}

	r2 := &FreyaReport{
		DeckName:    "empty",
		TotalCards:  99,
		ManaCurve:   [8]int{},
		ColorDemand: map[string]int{},
		ColorSupply: map[string]int{},
	}
	var buf2 bytes.Buffer
	printJSON(&buf2, r2)
	if strings.Contains(buf2.String(), `"graveyard_loops"`) {
		t.Errorf("empty GraveyardLoops must be omitted by omitempty:\n%s", buf2.String())
	}
}

// Label table: ComboClassGraveyardLoop maps to "Graveyard Loop".
// Pins the label so report renderers don't accidentally emit the
// snake_case constant value into reader-facing output.
func TestComboClassLabel_GraveyardLoop(t *testing.T) {
	if got := ComboClassLabel(ComboClassGraveyardLoop); got != "Graveyard Loop" {
		t.Errorf("label: got %q, want %q", got, "Graveyard Loop")
	}
}
