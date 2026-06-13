package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/huginn"
)

// emergent_synergy_pickup_r63_test.go — the consumption end of the
// Huginn loop: a tier-3 entry in data/huginn/tier3_for_freya.json must
// surface in findEmergentSynergies for a deck containing both cards.
// findEmergentSynergies reads cwd-relative "data/huginn", so the test
// chdirs into a temp root.

func TestFindEmergentSynergies_PicksUpTier3Entry(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	dir := filepath.Join("data", "huginn")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	export := huginn.Tier3Export{Pairs: []huginn.FreyaInteraction{{
		CardA: "Ashnod's Altar", CardB: "Nim Deathmantle",
		Pattern: "produces mana → consumes mana", AvgImpact: 6.0, Confidence: 5,
	}}}
	data, _ := json.Marshal(export)
	if err := os.WriteFile(filepath.Join(dir, "tier3_for_freya.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	report := &FreyaReport{Profiles: []CardProfile{
		{Name: "Ashnod's Altar"},
		{Name: "Nim Deathmantle"},
		{Name: "Sol Ring"},
	}}
	syn := findEmergentSynergies(report)
	if len(syn) != 1 {
		t.Fatalf("synergies = %+v, want exactly the tier-3 pair", syn)
	}
	if syn[0].Tier != huginn.TierConfirmed || syn[0].ObservationCount != 5 ||
		len(syn[0].Cards) != 2 || syn[0].Cards[0] != "Ashnod's Altar" {
		t.Fatalf("synergy = %+v", syn[0])
	}

	// A deck holding only ONE of the pair must not claim the synergy.
	half := &FreyaReport{Profiles: []CardProfile{{Name: "Ashnod's Altar"}}}
	if got := findEmergentSynergies(half); len(got) != 0 {
		t.Fatalf("half-pair deck claimed synergy: %+v", got)
	}
}
