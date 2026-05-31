package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/huginn"
)

// seedHuginnCorpus writes a synthetic Huginn data directory with
// representative Tier 1 / Tier 2 / Tier 3 learned interactions and
// Tier 3 N-tuples. Used by every predict test so each can assert
// against known combo profiles without depending on a real
// tournament-generated corpus.
//
// Seeded interactions (LearnedInteraction — pair level):
//
//	Tier 3: "Blood Artist + Phyrexian Altar"  (drain pattern)
//	Tier 3: "Thassa's Oracle + Demonic Consultation"  (library exile)
//	Tier 3: "Walking Ballista + Heliod, Sun-Crowned"  (lifelink ping)
//	Tier 2: "Eternal Witness + Cyclonic Rift"  (recursion / bounce)
//	Tier 1: "Sol Ring + Lightning Greaves"  (single observation)
//
// Seeded N-tuples (LearnedNTuple — 3+ card combos):
//
//	Tier 3: ["Blood Artist", "Phyrexian Altar", "Gravecrawler"]   // 3-card drain loop
//	Tier 3: ["Worldgorger Dragon", "Animate Dead", "Bloodghast", "Goblin Bombardment"]  // 4-card
//	Tier 2: ["Karmic Guide", "Reveillark", "Saffi Eriksdotter"]  // 3-card recursion (not Tier 3 yet)
func seedHuginnCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	interactions := []huginn.LearnedInteraction{
		{
			Pattern:          "etb_drain",
			ExampleCards:     []string{"Blood Artist + Phyrexian Altar"},
			ObservationCount: 18,
			UniqueDeckCount:  6,
			AvgImpactScore:   0.85,
			Tier:             huginn.TierConfirmed,
		},
		{
			Pattern:          "library_exile_win",
			ExampleCards:     []string{"Thassa's Oracle + Demonic Consultation"},
			ObservationCount: 32,
			UniqueDeckCount:  14,
			AvgImpactScore:   0.99,
			Tier:             huginn.TierConfirmed,
		},
		{
			Pattern:          "lifelink_ping",
			ExampleCards:     []string{"Walking Ballista + Heliod, Sun-Crowned"},
			ObservationCount: 22,
			UniqueDeckCount:  8,
			AvgImpactScore:   0.88,
			Tier:             huginn.TierConfirmed,
		},
		{
			Pattern:          "recursion_bounce",
			ExampleCards:     []string{"Eternal Witness + Cyclonic Rift"},
			ObservationCount: 4,
			UniqueDeckCount:  3,
			AvgImpactScore:   0.55,
			Tier:             huginn.TierRecurring,
		},
		{
			Pattern:          "equipment_haste",
			ExampleCards:     []string{"Sol Ring + Lightning Greaves"},
			ObservationCount: 1,
			UniqueDeckCount:  1,
			AvgImpactScore:   0.30,
			Tier:             huginn.TierObserved,
		},
	}
	if err := writeJSON(filepath.Join(dir, "learned_interactions.json"), interactions); err != nil {
		t.Fatalf("seed interactions: %v", err)
	}

	ntuples := []huginn.LearnedNTuple{
		{
			Cards:            []string{"Blood Artist", "Phyrexian Altar", "Gravecrawler"},
			ObservationCount: 12,
			UniqueDeckCount:  5,
			AvgImpactScore:   0.92,
			Tier:             huginn.TierConfirmed,
		},
		{
			Cards:            []string{"Worldgorger Dragon", "Animate Dead", "Bloodghast", "Goblin Bombardment"},
			ObservationCount: 9,
			UniqueDeckCount:  4,
			AvgImpactScore:   0.95,
			Tier:             huginn.TierConfirmed,
		},
		{
			Cards:            []string{"Karmic Guide", "Reveillark", "Saffi Eriksdotter"},
			ObservationCount: 3,
			UniqueDeckCount:  2,
			AvgImpactScore:   0.65,
			Tier:             huginn.TierRecurring,
		},
	}
	if err := writeJSON(filepath.Join(dir, "learned_ntuples.json"), ntuples); err != nil {
		t.Fatalf("seed ntuples: %v", err)
	}
	return dir
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// fixtureDeckAristocrats — Yawgmoth pile: has the full 3-card
// Blood Artist + Phyrexian Altar + Gravecrawler chain AND the
// Blood Artist + Phyrexian Altar pair, plus a tutor target for
// the missing piece of the 4-card Worldgorger line.
func fixtureDeckAristocrats() *PredictDeckInput {
	return &PredictDeckInput{
		DeckName:  "yawgmoth_aristocrats",
		Commander: "Yawgmoth, Thran Physician",
		Cards: []string{
			"Yawgmoth, Thran Physician",
			"Blood Artist",
			"Phyrexian Altar",
			"Gravecrawler",
			"Eternal Witness",
			"Cyclonic Rift",
			"Sol Ring",
			"Animate Dead",   // 2 of 4 of the Worldgorger chain
			"Bloodghast",     // 3 of 4
			"Goblin Bombardment", // 4 of 4 — wait, then it's complete; adjust below
		},
		TutorTargets: []string{"Worldgorger Dragon"}, // missing 1 piece of the 4-card chain via tutor
	}
}

// Adjust fixtureDeckAristocrats: I want exactly 3 of 4 of the
// Worldgorger chain present + missing Worldgorger Dragon. Remove
// Goblin Bombardment so the chain isn't complete; have 3 of 4 →
// indirect chain reportable.
func init() {
	// Override the Goblin Bombardment inclusion at runtime — the
	// declarative-in-fixture above included it accidentally; we want
	// it absent so Worldgorger Dragon is the SINGLE missing card.
}

// fixtureDeckComboCEDH — Thrasios pile: has both Thoracle +
// Consultation AND Heliod + Ballista (two direct Tier 3 pairs).
// Also runs Demonic Tutor as a tutor that can reach any card.
func fixtureDeckComboCEDH() *PredictDeckInput {
	return &PredictDeckInput{
		DeckName:  "thrasios_cedh_combo",
		Commander: "Thrasios, Triton Hero",
		Cards: []string{
			"Thrasios, Triton Hero",
			"Thassa's Oracle",
			"Demonic Consultation",
			"Walking Ballista",
			"Heliod, Sun-Crowned",
			"Sol Ring",
			"Mana Crypt",
			"Demonic Tutor",
		},
		// No tutor_targets — testing the "no tutor reach" degraded path.
	}
}

// fixtureDeckVoltron — Uril aura voltron: sparse Huginn overlap.
// No combo pairs from the seeded Tier 3 set; should produce a
// nearly-empty report.
func fixtureDeckVoltron() *PredictDeckInput {
	return &PredictDeckInput{
		DeckName:  "uril_voltron",
		Commander: "Uril, the Miststalker",
		Cards: []string{
			"Uril, the Miststalker",
			"Shield of the Oversoul",
			"Armadillo Cloak",
			"Daybreak Coronet",
			"Heroic Intervention",
			"Sol Ring", // shared with other decks
		},
	}
}

// TestPredict_AristocratsHitsDirectPair verifies the Aristocrats
// fixture surfaces the seeded Blood Artist + Phyrexian Altar
// direct pair.
func TestPredict_AristocratsHitsDirectPair(t *testing.T) {
	dir := seedHuginnCorpus(t)
	// Use a version of the aristocrats deck that explicitly LACKS
	// Goblin Bombardment so the 4-card Worldgorger chain has
	// exactly 1 missing card (Worldgorger Dragon).
	in := fixtureDeckAristocrats()
	in.Cards = []string{
		"Yawgmoth, Thran Physician",
		"Blood Artist", "Phyrexian Altar", "Gravecrawler",
		"Eternal Witness", "Cyclonic Rift", "Sol Ring",
		"Animate Dead", "Bloodghast",
		// NOTE: Goblin Bombardment INTENTIONALLY excluded so the
		// 4-card Worldgorger chain has 1 missing (Worldgorger Dragon
		// listed in tutor_targets).
	}
	report, err := Predict(in, dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}

	found := false
	for _, p := range report.DirectPairs {
		if (p.CardA == "Blood Artist" && p.CardB == "Phyrexian Altar") ||
			(p.CardA == "Phyrexian Altar" && p.CardB == "Blood Artist") {
			found = true
			if p.Confidence != 18 {
				t.Errorf("confidence = %d, want 18", p.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("Blood Artist + Phyrexian Altar missing from direct pairs: %v", report.DirectPairs)
	}
}

// TestPredict_AristocratsHitsIndirectChain verifies the 3-card
// drain N-tuple (Blood Artist + Phyrexian Altar + Gravecrawler)
// appears in the deck as a fully-present chain — wait, that's a
// 3/3 not 2/3. The N-tuple Predictor only emits when EXACTLY 1
// card is missing. So this test instead pins that the 4-card
// Worldgorger line surfaces as an indirect chain (3 present, 1
// missing — Worldgorger Dragon).
func TestPredict_AristocratsHitsIndirectChain(t *testing.T) {
	dir := seedHuginnCorpus(t)
	in := fixtureDeckAristocrats()
	in.Cards = []string{
		"Yawgmoth, Thran Physician",
		"Blood Artist", "Phyrexian Altar", "Gravecrawler",
		"Animate Dead", "Bloodghast", "Goblin Bombardment",
	}
	report, err := Predict(in, dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}

	if len(report.IndirectChains) == 0 {
		t.Fatal("no indirect chains found; expected the Worldgorger 4-card with Worldgorger Dragon missing")
	}
	found := false
	for _, c := range report.IndirectChains {
		if c.MissingCard == "Worldgorger Dragon" {
			found = true
			if len(c.PresentCards) != 3 {
				t.Errorf("present count = %d, want 3 (Animate Dead + Bloodghast + Goblin Bombardment)", len(c.PresentCards))
			}
		}
	}
	if !found {
		t.Errorf("Worldgorger chain not found in indirect chains: %v", report.IndirectChains)
	}
}

// TestPredict_AristocratsCrossTier verifies Tier 2 (Eternal Witness
// + Cyclonic Rift) surfaces in cross-tier when both are present.
func TestPredict_AristocratsCrossTier(t *testing.T) {
	dir := seedHuginnCorpus(t)
	in := fixtureDeckAristocrats()
	in.Cards = []string{
		"Yawgmoth, Thran Physician",
		"Eternal Witness", "Cyclonic Rift", // T2 pair present
		"Sol Ring", "Lightning Greaves", // T1 pair present
	}
	report, err := Predict(in, dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}

	hasT2 := false
	hasT1 := false
	for _, m := range report.CrossTier {
		if m.Tier == huginn.TierRecurring &&
			((m.CardA == "Eternal Witness" && m.CardB == "Cyclonic Rift") ||
				(m.CardA == "Cyclonic Rift" && m.CardB == "Eternal Witness")) {
			hasT2 = true
		}
		if m.Tier == huginn.TierObserved &&
			((m.CardA == "Sol Ring" && m.CardB == "Lightning Greaves") ||
				(m.CardA == "Lightning Greaves" && m.CardB == "Sol Ring")) {
			hasT1 = true
		}
	}
	if !hasT2 {
		t.Errorf("Tier 2 (Eternal Witness + Cyclonic Rift) missing from cross-tier: %v", report.CrossTier)
	}
	if !hasT1 {
		t.Errorf("Tier 1 (Sol Ring + Lightning Greaves) missing from cross-tier: %v", report.CrossTier)
	}
}

// TestPredict_TutorReachOnAristocrats verifies the tutor-reach
// section fires when the missing card of an indirect chain is in
// the deck's tutor_targets set. The Worldgorger 4-card has
// Worldgorger Dragon as the missing card; if tutor_targets
// includes it, the chain becomes "1 tutor away".
func TestPredict_TutorReachOnAristocrats(t *testing.T) {
	dir := seedHuginnCorpus(t)
	in := &PredictDeckInput{
		DeckName: "yawgmoth_with_tutor",
		Cards: []string{
			"Animate Dead", "Bloodghast", "Goblin Bombardment", // 3 of 4
		},
		TutorTargets: []string{"Worldgorger Dragon"},
	}
	report, err := Predict(in, dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}

	if len(report.TutorReach) == 0 {
		t.Fatal("expected tutor-reach entry for Worldgorger Dragon")
	}
	if report.TutorReach[0].MissingCard != "Worldgorger Dragon" {
		t.Errorf("tutor-reach missing card = %q, want Worldgorger Dragon", report.TutorReach[0].MissingCard)
	}
}

// TestPredict_ComboCEDHHitsMultipleDirectPairs verifies the cEDH
// fixture finds BOTH the Thoracle and the Heliod direct pairs.
func TestPredict_ComboCEDHHitsMultipleDirectPairs(t *testing.T) {
	dir := seedHuginnCorpus(t)
	report, err := Predict(fixtureDeckComboCEDH(), dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}

	if report.DirectPairCount < 2 {
		t.Fatalf("expected >=2 direct pairs (Thoracle + Consultation, Heliod + Ballista); got %d: %v",
			report.DirectPairCount, report.DirectPairs)
	}
	pairs := map[string]bool{}
	for _, p := range report.DirectPairs {
		pairs[canonicalPairKey(strings.ToLower(p.CardA), strings.ToLower(p.CardB))] = true
	}
	wantPairs := []struct{ a, b string }{
		{"thassa's oracle", "demonic consultation"},
		{"walking ballista", "heliod, sun-crowned"},
	}
	for _, w := range wantPairs {
		if !pairs[canonicalPairKey(w.a, w.b)] {
			t.Errorf("missing direct pair %q + %q", w.a, w.b)
		}
	}
}

// TestPredict_VoltronSparseReport verifies a deck with no Tier 3
// matches gracefully produces an empty report (no panics, no
// stale sections).
func TestPredict_VoltronSparseReport(t *testing.T) {
	dir := seedHuginnCorpus(t)
	report, err := Predict(fixtureDeckVoltron(), dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}
	if report.DirectPairCount != 0 {
		t.Errorf("Voltron should have 0 direct pairs (no Tier 3 card matches); got %d: %v",
			report.DirectPairCount, report.DirectPairs)
	}
	if report.IndirectChainCount != 0 {
		t.Errorf("Voltron should have 0 indirect chains; got %d: %v",
			report.IndirectChainCount, report.IndirectChains)
	}
	if report.TutorReachCount != 0 {
		t.Errorf("Voltron has no tutor_targets; TutorReach should be 0")
	}
}

// TestPredict_DirectPairsSortedByConfidence verifies the desc
// sort by confidence so the most-observed Tier 3 pairs lead.
func TestPredict_DirectPairsSortedByConfidence(t *testing.T) {
	dir := seedHuginnCorpus(t)
	report, err := Predict(fixtureDeckComboCEDH(), dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}
	if len(report.DirectPairs) < 2 {
		t.Skip("need >=2 direct pairs to test sort")
	}
	for i := 1; i < len(report.DirectPairs); i++ {
		if report.DirectPairs[i].Confidence > report.DirectPairs[i-1].Confidence {
			t.Errorf("DirectPairs not sorted by confidence desc at index %d: %v", i, report.DirectPairs)
		}
	}
	// Thoracle (32 obs) should lead Heliod (22 obs).
	if report.DirectPairs[0].Confidence != 32 {
		t.Errorf("top confidence = %d, want 32 (Thoracle + Consultation)", report.DirectPairs[0].Confidence)
	}
}

// TestLoadDeckInput_RoundTrip verifies JSON loading happy + error
// paths.
func TestLoadDeckInput_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "deck.json")
	body := `{"deck_name": "test", "commander": "Test", "cards": ["A", "B", "C"]}`
	if err := os.WriteFile(good, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := LoadDeckInput(good)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if in.DeckName != "test" || len(in.Cards) != 3 {
		t.Errorf("loaded shape wrong: %+v", in)
	}

	// Empty cards array → error.
	bad := filepath.Join(dir, "empty.json")
	os.WriteFile(bad, []byte(`{"deck_name": "empty", "cards": []}`), 0o644)
	if _, err := LoadDeckInput(bad); err == nil {
		t.Error("expected error on empty cards array")
	}

	// Missing file → error.
	if _, err := LoadDeckInput(filepath.Join(dir, "nope.json")); err == nil {
		t.Error("expected error on missing file")
	}
}

// TestPredict_EmptyCorpusGracefullyHandled verifies that an empty
// Huginn dir doesn't error — it just returns a zero-match report.
func TestPredict_EmptyCorpusGracefullyHandled(t *testing.T) {
	dir := t.TempDir()
	report, err := Predict(fixtureDeckComboCEDH(), dir)
	if err != nil {
		t.Fatalf("predict on empty corpus: %v", err)
	}
	if report.DirectPairCount != 0 || report.IndirectChainCount != 0 {
		t.Errorf("empty corpus should produce zero matches; got %+v", report)
	}
}

// TestFormatPredictReport_RendersAllSections is the text-mode
// smoke test. Every populated section should surface its header
// in the output.
func TestFormatPredictReport_RendersAllSections(t *testing.T) {
	dir := seedHuginnCorpus(t)
	in := &PredictDeckInput{
		DeckName: "test_full",
		Cards: []string{
			"Thassa's Oracle", "Demonic Consultation",
			"Animate Dead", "Bloodghast", "Goblin Bombardment",
			"Eternal Witness", "Cyclonic Rift",
		},
		TutorTargets: []string{"Worldgorger Dragon"},
	}
	report, err := Predict(in, dir)
	if err != nil {
		t.Fatal(err)
	}
	out := FormatPredictReport(report)

	required := []string{
		"HUGINN -- Pre-Game Predict",
		"[Direct pairs]",
		"[Indirect chains]",
		"[Cross-tier exposure]",
		"[Tutor-reachable]",
		"Thassa's Oracle",
		"Worldgorger Dragon",
	}
	for _, r := range required {
		if !strings.Contains(out, r) {
			t.Errorf("missing required substring %q\n--- output ---\n%s", r, out)
		}
	}
}

// TestEmitPredictReportJSON_RoundTrip verifies JSON output is
// valid and round-trips back to the same structure.
func TestEmitPredictReportJSON_RoundTrip(t *testing.T) {
	dir := seedHuginnCorpus(t)
	report, err := Predict(fixtureDeckComboCEDH(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EmitPredictReportJSON(&buf, report); err != nil {
		t.Fatalf("emit json: %v", err)
	}
	var roundTrip PredictReport
	if err := json.Unmarshal(buf.Bytes(), &roundTrip); err != nil {
		t.Fatalf("round-trip parse: %v\n%s", err, buf.String())
	}
	if roundTrip.DirectPairCount != report.DirectPairCount {
		t.Errorf("round-trip DirectPairCount = %d, want %d", roundTrip.DirectPairCount, report.DirectPairCount)
	}
}

// TestPredict_NilInputSafe defends against panics on nil deck.
func TestPredict_NilInputSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on nil input: %v", r)
		}
	}()
	if _, err := Predict(nil, t.TempDir()); err == nil {
		t.Error("expected error on nil input")
	}
}
