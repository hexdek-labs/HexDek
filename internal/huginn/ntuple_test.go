package huginn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// makeNTupleAnalysis builds a synthetic GameAnalysis whose CoTriggerNTuples
// field contains a single observation with the given cards and impact.
// DeckNames are attached at the persistence layer, not here.
func makeNTupleAnalysis(cards []string, impact float64) *analytics.GameAnalysis {
	return &analytics.GameAnalysis{
		CoTriggerNTuples: []analytics.CoTriggerNTuple{
			{Cards: cards, ImpactScore: impact, TurnWindow: 5, GameID: "g1"},
		},
	}
}

func TestNTuple_KeyDedupesReorderedCards(t *testing.T) {
	a := nTupleKey([]string{"Birds of Paradise", "Sol Ring", "Mana Crypt"})
	b := nTupleKey([]string{"Mana Crypt", "Birds of Paradise", "Sol Ring"})
	c := nTupleKey([]string{"Sol Ring", "Mana Crypt", "Birds of Paradise"})
	if a != b || a != c {
		t.Fatalf("reordered cards must collapse to same key: %q %q %q", a, b, c)
	}
}

func TestIngestNTuples_TierGraduationThroughThreeTiers(t *testing.T) {
	dir := t.TempDir()
	cards := []string{"Card A", "Card B", "Card C"}

	// Each ingest DRAINS the raw inbox (r63 loop close), so every batch
	// below is counted exactly once; tier state and deck identity carry
	// across batches in the learned DB itself.

	// Tier 1: single observation from DeckOne.
	a1 := makeNTupleAnalysis(cards, 6.0)
	if err := PersistRawNTuples(dir, []*analytics.GameAnalysis{a1}, []string{"DeckOne"}); err != nil {
		t.Fatalf("persist 1: %v", err)
	}
	if _, err := IngestNTuples(dir, 0); err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	learned, err := ReadLearnedNTuples(dir)
	if err != nil {
		t.Fatalf("read learned: %v", err)
	}
	if len(learned) != 1 || learned[0].Tier != TierObserved {
		t.Fatalf("after 1 obs: want 1 entry tier %d, got %+v", TierObserved, learned)
	}

	// Tier 2: append 2 more obs from DeckTwo. Cumulative: 3 obs across 2 decks.
	for i := 0; i < 2; i++ {
		if err := PersistRawNTuples(dir, []*analytics.GameAnalysis{makeNTupleAnalysis(cards, 6.0)}, []string{"DeckTwo"}); err != nil {
			t.Fatalf("persist t2 #%d: %v", i, err)
		}
	}
	if _, err := IngestNTuples(dir, 0); err != nil {
		t.Fatalf("ingest t2: %v", err)
	}
	learned, _ = ReadLearnedNTuples(dir)
	if len(learned) != 1 || learned[0].Tier != TierRecurring {
		t.Fatalf("after t2 obs: want tier %d, got %+v", TierRecurring, learned)
	}
	if learned[0].UniqueDeckCount < 2 {
		t.Errorf("expected unique deck count >=2, got %d", learned[0].UniqueDeckCount)
	}

	// Tier 3: append 2 more obs from DeckThree at impact 6.0. Cumulative
	// reaches 5 obs total with avg impact 6.0 (>= tier3MinImpact 5.0).
	for i := 0; i < 2; i++ {
		if err := PersistRawNTuples(dir, []*analytics.GameAnalysis{makeNTupleAnalysis(cards, 6.0)}, []string{"DeckThree"}); err != nil {
			t.Fatalf("persist t3 #%d: %v", i, err)
		}
	}
	promotions, err := IngestNTuples(dir, 0)
	if err != nil {
		t.Fatalf("ingest t3: %v", err)
	}
	learned, _ = ReadLearnedNTuples(dir)
	if len(learned) != 1 || learned[0].Tier != TierConfirmed {
		t.Fatalf("after t3 obs: want tier %d, got %+v", TierConfirmed, learned)
	}
	if len(promotions) != 1 {
		t.Fatalf("expected 1 promotion, got %d", len(promotions))
	}

	// Tier 3 export should now contain the combo.
	exp, err := ReadTier3NTupleExport(dir)
	if err != nil {
		t.Fatalf("read tier3 export: %v", err)
	}
	if len(exp.NTuples) != 1 {
		t.Fatalf("expected 1 ntuple in tier3 export, got %d", len(exp.NTuples))
	}
	if len(exp.NTuples[0].Cards) != 3 {
		t.Errorf("expected 3 cards in exported combo, got %d", len(exp.NTuples[0].Cards))
	}
}

func TestIngestNTuples_DedupCanonicalKey(t *testing.T) {
	dir := t.TempDir()
	// Same 3 cards, different orders across two persistence calls.
	a1 := makeNTupleAnalysis([]string{"Alpha", "Beta", "Gamma"}, 4.0)
	a2 := makeNTupleAnalysis([]string{"Gamma", "Alpha", "Beta"}, 4.0)
	if err := PersistRawNTuples(dir, []*analytics.GameAnalysis{a1, a2}, []string{"D"}); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if _, err := IngestNTuples(dir, 0); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	learned, _ := ReadLearnedNTuples(dir)
	if len(learned) != 1 {
		t.Fatalf("expected 1 deduped entry, got %d", len(learned))
	}
	if learned[0].ObservationCount != 2 {
		t.Errorf("expected obs count 2, got %d", learned[0].ObservationCount)
	}
	// Cards should be in sorted canonical order.
	want := []string{"Alpha", "Beta", "Gamma"}
	for i, c := range want {
		if learned[0].Cards[i] != c {
			t.Errorf("cards[%d] = %q, want %q", i, learned[0].Cards[i], c)
		}
	}
}

func TestPruneNTuples_RemovesStaleTier1And2(t *testing.T) {
	dir := t.TempDir()
	// Hand-craft learned entries.
	entries := []LearnedNTuple{
		{
			Cards: []string{"A", "B", "C"}, NormalizedKey: nTupleKey([]string{"A", "B", "C"}),
			Tier: TierObserved, GamesSinceLastSeen: tier1PruneGames + 5, ObservationCount: 1,
		},
		{
			Cards: []string{"D", "E", "F"}, NormalizedKey: nTupleKey([]string{"D", "E", "F"}),
			Tier: TierRecurring, GamesSinceLastSeen: tier2PruneGames + 5, ObservationCount: 3,
		},
		{
			Cards: []string{"G", "H", "I"}, NormalizedKey: nTupleKey([]string{"G", "H", "I"}),
			Tier: TierConfirmed, GamesSinceLastSeen: 99999, ObservationCount: 10,
		},
		{
			Cards: []string{"J", "K", "L"}, NormalizedKey: nTupleKey([]string{"J", "K", "L"}),
			Tier: TierObserved, GamesSinceLastSeen: 10, ObservationCount: 1,
		},
	}
	if err := atomicWriteJSON(filepath.Join(dir, learnedNTupleFile), entries); err != nil {
		t.Fatalf("write: %v", err)
	}
	removed, err := PruneNTuples(dir)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed (stale t1 + stale t2), got %d", removed)
	}
	kept, _ := ReadLearnedNTuples(dir)
	if len(kept) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(kept))
	}
	// Tier 3 must always survive.
	foundTier3 := false
	foundFreshTier1 := false
	for _, ln := range kept {
		if ln.Tier == TierConfirmed {
			foundTier3 = true
		}
		if ln.Tier == TierObserved && ln.GamesSinceLastSeen == 10 {
			foundFreshTier1 = true
		}
	}
	if !foundTier3 || !foundFreshTier1 {
		t.Errorf("expected tier3 and fresh tier1 kept, got %+v", kept)
	}
}

func TestTier3NTupleExport_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	entries := []LearnedNTuple{
		{
			Cards: []string{"A", "B", "C"}, NormalizedKey: nTupleKey([]string{"A", "B", "C"}),
			Tier: TierConfirmed, AvgImpactScore: 7.5, ObservationCount: 6,
		},
		{
			Cards: []string{"X", "Y"}, NormalizedKey: nTupleKey([]string{"X", "Y"}),
			Tier: TierRecurring, AvgImpactScore: 3.0, ObservationCount: 3,
		},
	}
	if err := exportTier3NTuplesForFreya(dir, entries); err != nil {
		t.Fatalf("export: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, tier3NTupleFile))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var exp Tier3NTupleExport
	if err := json.Unmarshal(data, &exp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(exp.NTuples) != 1 {
		t.Fatalf("expected 1 confirmed combo, got %d", len(exp.NTuples))
	}
	if exp.NTuples[0].Confidence != 6 || exp.NTuples[0].AvgImpact != 7.5 {
		t.Errorf("metadata mismatch: %+v", exp.NTuples[0])
	}

	roundTrip, err := ReadTier3NTupleExport(dir)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if len(roundTrip.NTuples) != 1 {
		t.Errorf("round trip lost combos: %d", len(roundTrip.NTuples))
	}
}

// TestDetectCoTriggerNTuples_FindsThreeCardCombo synthesizes an event log
// where three cards all fire in the same turn for the same seat. A 3-card
// tuple must appear in the output.
func TestDetectCoTriggerNTuples_FindsThreeCardCombo(t *testing.T) {
	events := []gameengine.Event{
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 1}},
		// Three different cards on seat 0, same turn.
		{Kind: "triggered_ability", Seat: 0, Source: "Card A"},
		{Kind: "pool_drain", Seat: 0, Source: "Card A"},
		{Kind: "cast", Seat: 0, Source: "Card B", Amount: 1},
		{Kind: "pay_mana", Seat: 0, Source: "Card B", Amount: 2},
		{Kind: "draw_card", Seat: 0, Source: "Card C"},
		// Add resource motion so impact score is nonzero.
		{Kind: "life_change", Seat: 0, Source: "Card B", Amount: -3},
		{Kind: "turn_start", Seat: 0, Details: map[string]interface{}{"turn": 2}},
	}
	tuples := analytics.DetectCoTriggerNTuples(events, 4, 0, 3, 5)
	if len(tuples) == 0 {
		t.Fatalf("expected at least one 3-card tuple, got 0")
	}
	// Find the A/B/C tuple.
	found := false
	for _, tup := range tuples {
		if len(tup.Cards) == 3 &&
			tup.Cards[0] == "Card A" && tup.Cards[1] == "Card B" && tup.Cards[2] == "Card C" {
			found = true
			if tup.ImpactScore <= 0 {
				t.Errorf("expected positive impact, got %f", tup.ImpactScore)
			}
			if tup.TurnWindow != 1 {
				t.Errorf("expected turn 1, got %d", tup.TurnWindow)
			}
		}
	}
	if !found {
		t.Errorf("expected sorted [A,B,C] tuple in output, got %+v", tuples)
	}
}
