package huginn

import (
	"os"
	"path/filepath"
	"testing"
)

// ingest_loop_r63_test.go — the restored graduation step (round-2
// step-2 loop close). Ingest was unreachable from birth and removed by
// the #1054 dead-code sweep; this restoration adds the inbox drain so
// the per-batch post-tournament hook can call it unconditionally.

func seedRawObs(t *testing.T, dir string, obs []RawObservation) {
	t.Helper()
	if err := PersistRawObservationsRaw(dir, obs); err != nil {
		t.Fatal(err)
	}
}

func nObs(n int, pattern string, impact float64, decks ...string) []RawObservation {
	out := make([]RawObservation, n)
	for i := range out {
		out[i] = RawObservation{
			CardA: "Ashnod's Altar", CardB: "Nim Deathmantle",
			ImpactScore:   impact,
			EffectPattern: pattern,
			DeckNames:     []string{decks[i%len(decks)]},
		}
	}
	return out
}

// A seeded observation set meeting the tier-3 bar (5+ obs, avg impact
// ≥ 5.0) must graduate straight through to CONFIRMED, export to
// tier3_for_freya.json, and report the promotion.
func TestIngest_GraduatesSeededObservationsToTier3(t *testing.T) {
	dir := t.TempDir()
	seedRawObs(t, dir, nObs(5, "A produces mana, B consumes mana", 6.0, "deck1", "deck2"))

	proms, err := Ingest(dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(proms) != 1 || proms[0].Tier != TierConfirmed {
		t.Fatalf("promotions = %+v, want 1 tier-3 promotion", proms)
	}
	if proms[0].Pattern != "produces mana → consumes mana" {
		t.Fatalf("pattern = %q (NormalizePattern regression)", proms[0].Pattern)
	}

	learned, err := ReadLearnedInteractions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(learned) != 1 || learned[0].Tier != TierConfirmed || learned[0].ObservationCount != 5 {
		t.Fatalf("learned = %+v", learned)
	}

	// The Freya export — the file findEmergentSynergies reads.
	pairs, err := ReadTier3ForFreya(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 || pairs[0].CardA != "Ashnod's Altar" || pairs[0].CardB != "Nim Deathmantle" {
		t.Fatalf("tier3 export = %+v", pairs)
	}
}

// Ingest must DRAIN the raw inbox: a second call sees nothing new and
// counts stay exact. Without the drain, every post-batch hook run
// re-counted the entire append-only history.
func TestIngest_DrainsInboxNoDoubleCount(t *testing.T) {
	dir := t.TempDir()
	seedRawObs(t, dir, nObs(5, "A produces mana, B consumes mana", 6.0, "deck1", "deck2"))

	if _, err := Ingest(dir, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, rawObsFile)); !os.IsNotExist(err) {
		t.Fatal("raw_observations.json still exists after Ingest — inbox not drained")
	}

	// Idempotent second run.
	proms, err := Ingest(dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(proms) != 0 {
		t.Fatalf("re-ingest re-promoted: %+v", proms)
	}
	learned, _ := ReadLearnedInteractions(dir)
	if learned[0].ObservationCount != 5 {
		t.Fatalf("ObservationCount = %d after re-ingest, want 5 (double-counted)", learned[0].ObservationCount)
	}
}

// The tier ladder: 1 obs = OBSERVED; 3 obs across 2 decks = RECURRING;
// high count with LOW impact must NOT reach CONFIRMED.
func TestIngest_TierLadder(t *testing.T) {
	dir := t.TempDir()
	seedRawObs(t, dir, nObs(1, "A produces life, B consumes life", 6.0, "deck1"))
	if _, err := Ingest(dir, 1); err != nil {
		t.Fatal(err)
	}
	learned, _ := ReadLearnedInteractions(dir)
	if learned[0].Tier != TierObserved {
		t.Fatalf("1 obs tier = %d, want OBSERVED", learned[0].Tier)
	}

	// Accumulate to 3 obs / 2 decks across SEPARATE batches — the
	// learned DB carries state between drained inboxes.
	seedRawObs(t, dir, nObs(2, "A produces life, B consumes life", 6.0, "deck2"))
	if _, err := Ingest(dir, 2); err != nil {
		t.Fatal(err)
	}
	learned, _ = ReadLearnedInteractions(dir)
	if learned[0].Tier != TierRecurring || learned[0].ObservationCount != 3 {
		t.Fatalf("3 obs / 2 decks = tier %d count %d, want RECURRING/3", learned[0].Tier, learned[0].ObservationCount)
	}

	// Low impact never confirms regardless of count.
	dir2 := t.TempDir()
	seedRawObs(t, dir2, nObs(8, "A produces cards, B consumes cards", 1.0, "deck1", "deck2"))
	if _, err := Ingest(dir2, 8); err != nil {
		t.Fatal(err)
	}
	learned, _ = ReadLearnedInteractions(dir2)
	if learned[0].Tier == TierConfirmed {
		t.Fatal("avg impact 1.0 graduated to CONFIRMED — impact gate broken")
	}
}

// IngestNTuples gets the same drain contract.
func TestIngestNTuples_DrainsInbox(t *testing.T) {
	dir := t.TempDir()
	raw := []RawNTuple{}
	for i := 0; i < 5; i++ {
		raw = append(raw, RawNTuple{
			Cards:       []string{"A", "B", "C"},
			ImpactScore: 6.0,
			DeckNames:   []string{[]string{"deck1", "deck2"}[i%2]},
		})
	}
	if err := atomicWriteJSON(filepath.Join(dir, rawNTupleFile), raw); err != nil {
		t.Fatal(err)
	}
	proms, err := IngestNTuples(dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(proms) != 1 {
		t.Fatalf("ntuple promotions = %d, want 1", len(proms))
	}
	if _, err := os.Stat(filepath.Join(dir, rawNTupleFile)); !os.IsNotExist(err) {
		t.Fatal("raw_ntuples.json still exists after IngestNTuples — inbox not drained")
	}
	learned, _ := ReadLearnedNTuples(dir)
	if len(learned) != 1 || learned[0].ObservationCount != 5 {
		t.Fatalf("learned ntuples = %+v", learned)
	}
}

// Prune ages out stale tier 1/2; tier 3 is permanent.
func TestPrune_AgesOutLowTiersKeepsConfirmed(t *testing.T) {
	dir := t.TempDir()
	interactions := []LearnedInteraction{
		{Pattern: "stale-t1", Tier: TierObserved, GamesSinceLastSeen: tier1PruneGames},
		{Pattern: "fresh-t1", Tier: TierObserved, GamesSinceLastSeen: 1},
		{Pattern: "stale-t2", Tier: TierRecurring, GamesSinceLastSeen: tier2PruneGames},
		{Pattern: "eternal-t3", Tier: TierConfirmed, GamesSinceLastSeen: 99999},
	}
	if err := atomicWriteJSON(filepath.Join(dir, learnedFile), interactions); err != nil {
		t.Fatal(err)
	}
	removed, err := Prune(dir)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	kept, _ := ReadLearnedInteractions(dir)
	names := map[string]bool{}
	for _, li := range kept {
		names[li.Pattern] = true
	}
	if !names["eternal-t3"] || !names["fresh-t1"] || names["stale-t1"] || names["stale-t2"] {
		t.Fatalf("kept = %v", names)
	}
}
