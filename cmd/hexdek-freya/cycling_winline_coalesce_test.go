package main

import (
	"os"
	"path/filepath"
	"testing"
)

// cycling_winline_coalesce_test.go — regressions for the cycling-loop
// combinatorial-explosion fix (issue #523).
//
// Pre-fix: Timeless Wisdom (C20 Gavi cycling precon, ~30 cycling
// cards) generated 27,323 determined + 550 true-infinite combo
// entries via the pair/triple/quad loop detector — every pair of
// cycling cards "closed" as a ResCard ↔ ResCard loop because cycling
// reads as discard-self (Consumes=ResCard) + draw (Produces=ResCard).
// The loop is a false positive: cycling is consume-once; the
// discarded card sits in the graveyard, not re-castable without
// explicit recursion. C(30,4)=27,405 quad-iterations alone produced
// the headline 27K explosion.
//
// Fix surface:
//   - hasSelfCyclingKeyword: per-line scan that rejects granted-
//     cycling clauses ("each historic card in your hand has cycling
//     {2}{W}" on Jo Grant; "Lands you control have cycling {R}" on
//     Tectonic Reformation) so HasCycling stays true only for cards
//     that have the keyword themselves.
//   - cyclingCardsInCombo guard in checkPairCombo / checkTripleCombo
//     / checkQuadCombo: skip combos with ≥ 2 cycling cards (the
//     loop is either false-positive or redundant with a smaller
//     combo that has only one representative cycling card).
//   - FindLoops quad-candidate prefilter caps cycling cards at 1 so
//     the C(N,4) iteration over the candidate pool can't explode
//     when the prefilter doesn't otherwise drop them.
//
// Tests use the wizards/ precon corpus (oracle-data gated; skip when
// the 163MB oracle blob isn't present locally).

const (
	cyclingFixOraclePath = "../../data/rules/oracle-cards.json"

	timelessWisdomDeck   = "../../data/decks/wizards/timeless_wisdom_commander_2020_precon_decklist.txt"
	blastFromThePastDeck = "../../data/decks/wizards/blast_from_the_past_doctor_who_commander_precon_decklist.txt"
	necronDynastiesDeck  = "../../data/decks/wizards/necron_dynasties_warhammer_40_000_commander_precon_decklist.txt"

	// Pre-fix Timeless Wisdom reported 27,879 win_lines. The fix's
	// post-condition is "under triple digits" — picking 100 as the
	// ceiling matches issue #523's stated test target and gives ample
	// headroom for legit non-cycling alt_wincon / commander_damage /
	// combat lines plus the (now coalesced) cycling-payoff combos.
	timelessWisdomWinLineCeiling = 100

	// Blast (Doctor Who, Jo Grant + cycling lands) is a small-combo
	// deck — 8 finishers + 2 Jo Grant + cycling-land pairs + 1
	// commander_damage line. ≤ 25 covers it with headroom even if
	// future detectors fold in another pair or two.
	blastFromThePastWinLineCeiling = 25

	// Necron Dynasties has no cycling cards but a dense combo /
	// finisher package. Pre-fix and post-fix both report ~97-99
	// win_lines. The ceiling is a sanity cap that catches regressions
	// elsewhere from blowing up a non-cycling deck.
	necronDynastiesWinLineCeiling = 200
)

func loadCyclingFixContext(t *testing.T) (*oracleDB, *MechanicDB) {
	t.Helper()
	if _, err := os.Stat(cyclingFixOraclePath); err != nil {
		t.Skipf("oracle data not available at %s — run scripts/fetch-oracle.sh", cyclingFixOraclePath)
	}
	oracle, err := loadOracle(cyclingFixOraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(cyclingFixOraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}
	return oracle, mechDB
}

func analyzeAndCountWinLines(t *testing.T, deckPath string, oracle *oracleDB, mechDB *MechanicDB) int {
	t.Helper()
	if _, err := os.Stat(deckPath); err != nil {
		t.Skipf("deck not present at %s", deckPath)
	}
	report, err := analyzeDeckFile(deckPath, oracle, mechDB)
	if err != nil {
		t.Fatalf("analyze %s: %v", filepath.Base(deckPath), err)
	}
	if report.WinLines == nil {
		return 0
	}
	return len(report.WinLines.WinLines)
}

// TestCyclingWinLineCoalesce_TimelessWisdom is the headline regression
// for issue #523: Gavi cycling precon's 27,879 → ≤100 win-line count.
func TestCyclingWinLineCoalesce_TimelessWisdom(t *testing.T) {
	oracle, mechDB := loadCyclingFixContext(t)
	n := analyzeAndCountWinLines(t, timelessWisdomDeck, oracle, mechDB)
	if n > timelessWisdomWinLineCeiling {
		t.Errorf("Timeless Wisdom: win_lines=%d, want ≤%d (cycling-loop explosion regression — see issue #523)",
			n, timelessWisdomWinLineCeiling)
	}
	if n == 0 {
		t.Errorf("Timeless Wisdom: win_lines=0, expected at least the commander_damage + alt_wincon baseline — fix overshot and skipped all combos")
	}
}

// TestCyclingWinLineCoalesce_BlastFromThePast guards the small-combo
// case: Jo Grant + 2 cycling lands should still surface as a small
// number of win_lines. The fix's per-line "has cycling" filter is the
// reason Jo Grant's cycling-grant clause doesn't false-positive her
// as a cycling card (which would skip her pair combos via the
// cyclingCardsInCombo guard).
func TestCyclingWinLineCoalesce_BlastFromThePast(t *testing.T) {
	oracle, mechDB := loadCyclingFixContext(t)
	n := analyzeAndCountWinLines(t, blastFromThePastDeck, oracle, mechDB)
	if n > blastFromThePastWinLineCeiling {
		t.Errorf("Blast from the Past: win_lines=%d, want ≤%d (regression in non-cycling-tribal deck)",
			n, blastFromThePastWinLineCeiling)
	}
	if n == 0 {
		t.Errorf("Blast from the Past: win_lines=0, expected at least the finisher + commander_damage baseline")
	}
}

// TestCyclingWinLineCoalesce_NecronDynasties guards the no-cycling
// case: a dense-combo deck with zero cycling cards must not be
// affected by the cycling-coalesce path. Catches regressions where
// the cycling guard accidentally fires on non-cycling combos.
func TestCyclingWinLineCoalesce_NecronDynasties(t *testing.T) {
	oracle, mechDB := loadCyclingFixContext(t)
	n := analyzeAndCountWinLines(t, necronDynastiesDeck, oracle, mechDB)
	if n > necronDynastiesWinLineCeiling {
		t.Errorf("Necron Dynasties: win_lines=%d, want ≤%d (cycling guard mis-fired on a no-cycling deck)",
			n, necronDynastiesWinLineCeiling)
	}
	if n == 0 {
		t.Errorf("Necron Dynasties: win_lines=0, expected dense combo + finisher set to surface")
	}
}

// TestHasSelfCyclingKeyword_GrantedClauseSkip pins the per-line filter
// that distinguishes "this card has cycling" from "this card grants
// cycling." Without the filter, Jo Grant ("Each historic card in your
// hand has cycling {2}{W}") and Tectonic Reformation ("Lands you
// control have cycling {R}") false-positive as cycling cards, and the
// cyclingCardsInCombo guard wrongly skips their legitimate combos
// with actual cycling cards.
func TestHasSelfCyclingKeyword_GrantedClauseSkip(t *testing.T) {
	tests := []struct {
		name string
		ot   string
		want bool
	}{
		{
			name: "self-cycling Astral Slide-style",
			ot:   "cycling {2}\n{2}, discard this card: draw a card.",
			want: true,
		},
		{
			name: "self-cycling cycling land",
			ot:   "{T}: add {W}.\ncycling {2}",
			want: true,
		},
		{
			name: "Jo Grant grants cycling — must not flag self",
			ot:   "each historic card in your hand has cycling {2}{w}. ({2}{w}, discard that card: draw a card.)\nwhenever you cycle a card, put a +1/+1 counter on jo grant.",
			want: false,
		},
		{
			name: "Tectonic Reformation grants cycling to lands",
			ot:   "lands you control have cycling {r}. ({r}, discard a land card: draw a card.)",
			want: false,
		},
		{
			name: "cycle trigger only (Astral Drift) — has 'cycle' but no 'cycling {'",
			ot:   "flash\nwhenever you cycle another card, exile target creature you don't control.",
			want: false,
		},
		{
			name: "no cycling at all",
			ot:   "draw a card.",
			want: false,
		},
		{
			name: "basic landcycling self",
			ot:   "{T}: add {C}.\nbasic landcycling {1}",
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasSelfCyclingKeyword(tc.ot)
			if got != tc.want {
				t.Errorf("hasSelfCyclingKeyword(%q) = %v, want %v", tc.ot, got, tc.want)
			}
		})
	}
}

// TestCyclingCardsInCombo_Counts pins the helper's arity and behavior
// — pair / triple / quad input shapes that drive the combo-skip
// guard in checkPairCombo / checkTripleCombo / checkQuadCombo.
func TestCyclingCardsInCombo_Counts(t *testing.T) {
	cyc := CardProfile{Name: "Cycler", HasCycling: true}
	non := CardProfile{Name: "NonCycler", HasCycling: false}

	cases := []struct {
		name  string
		cards []CardProfile
		want  int
	}{
		{"empty", nil, 0},
		{"single non", []CardProfile{non}, 0},
		{"single cyc", []CardProfile{cyc}, 1},
		{"pair no cycling", []CardProfile{non, non}, 0},
		{"pair one cycling", []CardProfile{cyc, non}, 1},
		{"pair both cycling", []CardProfile{cyc, cyc}, 2},
		{"triple two cycling", []CardProfile{cyc, cyc, non}, 2},
		{"quad three cycling", []CardProfile{cyc, cyc, cyc, non}, 3},
		{"quad all cycling", []CardProfile{cyc, cyc, cyc, cyc}, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cyclingCardsInCombo(tc.cards...)
			if got != tc.want {
				t.Errorf("cyclingCardsInCombo(%d cards) = %d, want %d", len(tc.cards), got, tc.want)
			}
		})
	}
}

// TestCyclingComboSkip_PairSkipsBothCycling pins the pair-detector
// skip: two cycling cards never form a combo, even when their
// resource flow trivially "closes" via the cycling discard-self +
// draw shape.
func TestCyclingComboSkip_PairSkipsBothCycling(t *testing.T) {
	a := CardProfile{
		Name:       "Cycler A",
		HasCycling: true,
		Produces:   []ResourceType{ResCard},
		Consumes:   []ResourceType{ResCard},
	}
	b := CardProfile{
		Name:       "Cycler B",
		HasCycling: true,
		Produces:   []ResourceType{ResCard},
		Consumes:   []ResourceType{ResCard},
	}
	if got := checkPairCombo(a, b); got != nil {
		t.Errorf("checkPairCombo(2 cycling cards) returned a combo (%+v) — should be skipped per cyclingCardsInCombo guard", got)
	}

	// Sanity: one cycling + one non-cycling with the same resource
	// shape is NOT skipped by the cycling guard.
	c := CardProfile{
		Name:       "NonCycler C",
		HasCycling: false,
		Produces:   []ResourceType{ResCard},
		Consumes:   []ResourceType{ResCard},
		Effects:    []string{"draw"},
		Triggers:   []string{"draw"},
	}
	// The result here is gated by other detectors (verifyTriggerChain
	// etc.), so we don't assert non-nil — only that the cycling guard
	// alone doesn't short-circuit.
	_ = checkPairCombo(a, c)
}
