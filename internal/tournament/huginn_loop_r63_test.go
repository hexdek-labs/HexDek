package tournament

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/huginn"
)

// huginn_loop_r63_test.go — the post-batch graduation hook that closes
// the Huginn learning loop: persisted raw observations must actually
// graduate through tiers and refresh the tier-3 Freya exports.

func TestGraduateHuginn_FoldsBatchIntoTiersAndExports(t *testing.T) {
	dir := t.TempDir()

	// A batch that meets the tier-3 bar in one go: 5 observations of
	// the same resource-flow pattern across 2 decks, impact 6.0.
	var obs []huginn.RawObservation
	for i := 0; i < 5; i++ {
		obs = append(obs, huginn.RawObservation{
			CardA: "Ashnod's Altar", CardB: "Nim Deathmantle",
			ImpactScore:   6.0,
			EffectPattern: "Ashnod's Altar produces mana, Nim Deathmantle consumes mana",
			DeckNames:     []string{[]string{"deck1", "deck2"}[i%2]},
		})
	}
	if err := huginn.PersistRawObservationsRaw(dir, obs); err != nil {
		t.Fatal(err)
	}

	graduateHuginn(dir, 5)

	// Graduated: learned DB has the confirmed pattern.
	learned, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(learned) != 1 || learned[0].Tier != huginn.TierConfirmed {
		t.Fatalf("learned = %+v, want one CONFIRMED pattern", learned)
	}

	// Exported: the file Freya's findEmergentSynergies reads.
	pairs, err := huginn.ReadTier3ForFreya(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 || pairs[0].CardA != "Ashnod's Altar" {
		t.Fatalf("tier3 export = %+v", pairs)
	}

	// Drained: the inbox is consumed, so the NEXT batch hook run cannot
	// double-count this one (the bug that kept the loop from being
	// wired for so long).
	if _, err := os.Stat(filepath.Join(dir, "raw_observations.json")); !os.IsNotExist(err) {
		t.Fatal("raw inbox not drained after graduation")
	}

	// Idempotent on an empty inbox.
	graduateHuginn(dir, 0)
	learned, _ = huginn.ReadLearnedInteractions(dir)
	if len(learned) != 1 || learned[0].ObservationCount != 5 {
		t.Fatalf("re-run changed counts: %+v", learned)
	}
}
