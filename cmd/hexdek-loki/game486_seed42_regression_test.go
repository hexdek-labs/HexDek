package main

import (
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// game486_seed42_regression_test.go — r63 1-in-500 strict-census leak.
//
// Seed-42 game 486 was the only violating game in a 500-game strict
// census: Banisher Priest linked-exiled seat 1's commander, the
// §704.6d/§903.9a SBA moved the commander exile → command zone without
// severing the Priest's §406.7 claim, and ExileLinkageIntegrity fired 24
// times (turn 16 until the Priest left play). Fixed by UnlinkExiledCard
// in sba704_6d + ExileLinked stamping linkage only on a real exile
// landing (state.go).
//
// This test replays exactly that game under the strict census and pins
// it to zero violations of ANY invariant. Skips when the gitignored
// data files are absent. Runtime ~60–90s (corpus load dominates).
func TestGame486Seed42_StrictCensusClean(t *testing.T) {
	if testing.Short() {
		t.Skip("full single-game replay; skipped in -short mode")
	}
	if _, err := os.Stat("../../data/rules/ast_dataset.jsonl"); err != nil {
		t.Skip("data/rules/ast_dataset.jsonl not present (gitignored corpus)")
	}
	if _, err := os.Stat("../../data/rules/oracle-cards.json"); err != nil {
		t.Skip("data/rules/oracle-cards.json not present (gitignored corpus)")
	}

	gameengine.SetStrictCensusDefault(true)
	defer gameengine.SetStrictCensusDefault(false)

	corpus, err := astload.Load("../../data/rules/ast_dataset.jsonl")
	if err != nil {
		t.Fatalf("astload: %v", err)
	}
	meta, err := deckparser.LoadMetaFromJSONL("../../data/rules/ast_dataset.jsonl")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if err := meta.SupplementWithOracleJSON("../../data/rules/oracle-cards.json"); err != nil {
		t.Fatalf("oracle supplement: %v", err)
	}
	chaosCorpus, err := loadOracleCorpus("../../data/rules/oracle-cards.json")
	if err != nil {
		t.Fatalf("oracle corpus: %v", err)
	}

	result := runChaosGame(486, 0, chaosCorpus, corpus, meta, 4, 42, 60, nil, "", nil)

	for _, c := range result.Crashes {
		t.Errorf("crash: %s", c.PanicValue)
	}
	for _, v := range result.Violations {
		t.Errorf("violation (turn %d, %s): %s", v.Turn, v.InvariantName, v.Message)
	}
	if len(result.Violations) == 0 && len(result.Crashes) == 0 {
		t.Logf("game 486 seed 42: clean (%d turns)", result.Turns)
	}
}
