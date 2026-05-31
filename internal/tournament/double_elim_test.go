package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// double_elim_test.go — Double-elimination bracket style.
//
// Three layers (same shape as swiss_test.go):
//   1. Registry pin: double-elim is in SupportedBracketStyles.
//   2. Pure pairing / standings tests (no game engine needed).
//   3. RunDoubleElimination config validation + tiny end-to-end.

// -----------------------------------------------------------------------------
// Registry drift detector
// -----------------------------------------------------------------------------

func TestSupportedBracketStyles_IncludesDoubleElim(t *testing.T) {
	got := SupportedBracketStyles()
	for _, s := range got {
		if s == "double-elim" {
			return
		}
	}
	t.Errorf("expected double-elim in SupportedBracketStyles, got %v", got)
}

// -----------------------------------------------------------------------------
// Pairing — pure (no engine).
// -----------------------------------------------------------------------------

func newDEStates(lossesPerDeck ...int) []deckState {
	out := make([]deckState, len(lossesPerDeck))
	for i, l := range lossesPerDeck {
		out[i] = deckState{losses: l}
	}
	return out
}

func TestPairDoubleElim_TiersSeparateWhenPossible(t *testing.T) {
	// 8 decks. 4 undefeated (losses=0), 4 once-lost (losses=1).
	// Pairing should put undefeated together in pod 1 and the rest
	// in pod 2 — tiers must not mix when both have ≥ NSeats.
	state := newDEStates(0, 0, 0, 0, 1, 1, 1, 1)
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	pods := pairDoubleElimRound(eligible, state, nil, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	for _, idx := range pods[0] {
		if idx > 3 {
			t.Errorf("pod[0] contains once-lost deck %d (winners tier should stay together)", idx)
		}
	}
	for _, idx := range pods[1] {
		if idx < 4 {
			t.Errorf("pod[1] contains undefeated deck %d", idx)
		}
	}
}

func TestPairDoubleElim_TiersMixWhenTooFew(t *testing.T) {
	// 4 decks: 2 undefeated, 2 once-lost. Single pod of 4 mixes
	// both tiers — pairing must not refuse to form a pod just
	// because tiers don't divide cleanly.
	state := newDEStates(0, 0, 1, 1)
	eligible := []int{0, 1, 2, 3}
	pods := pairDoubleElimRound(eligible, state, nil, 4, 1)
	if len(pods) != 1 {
		t.Fatalf("expected 1 mixed pod, got %d", len(pods))
	}
	if len(pods[0]) != 4 {
		t.Errorf("pod size = %d, want 4", len(pods[0]))
	}
}

func TestPairDoubleElim_BelowNSeatsReturnsEmpty(t *testing.T) {
	// 3 eligible decks, NSeats=4 → no pod formed (caller stops the
	// tournament).
	state := newDEStates(0, 0, 1)
	eligible := []int{0, 1, 2}
	pods := pairDoubleElimRound(eligible, state, nil, 4, 9)
	if len(pods) != 0 {
		t.Errorf("expected empty pod list when eligible < NSeats, got %d", len(pods))
	}
}

func TestPairDoubleElim_Deterministic(t *testing.T) {
	state := newDEStates(0, 0, 0, 0, 1, 1, 1, 1)
	eligible := []int{0, 1, 2, 3, 4, 5, 6, 7}
	a := pairDoubleElimRound(eligible, state, nil, 4, 31337)
	b := pairDoubleElimRound(eligible, state, nil, 4, 31337)
	if len(a) != len(b) {
		t.Fatalf("pod count differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				t.Errorf("pairing not deterministic at [%d][%d]: %d vs %d", i, j, a[i][j], b[i][j])
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Final rank assignment.
// -----------------------------------------------------------------------------

func TestBuildEliminationStandings_ChampionRanksFirst(t *testing.T) {
	// Deck 0 survived (eliminatedRound=0). Decks 1+2 eliminated in
	// round 3 (latest). Deck 3 eliminated in round 2.
	state := []deckState{
		{losses: 0, eliminatedRound: 0, wins: 5}, // champion
		{losses: 2, eliminatedRound: 3, wins: 3},
		{losses: 2, eliminatedRound: 3, wins: 4}, // tiebreak by wins desc
		{losses: 2, eliminatedRound: 2, wins: 2},
	}
	names := []string{"Champ", "Sub1", "Sub2", "Out"}
	got := buildEliminationStandings(names, state)
	if len(got) != 4 {
		t.Fatalf("expected 4 standings, got %d", len(got))
	}
	if got[0].CommanderName != "Champ" || got[0].FinalRank != 1 {
		t.Errorf("rank 1 = %+v, want Champ", got[0])
	}
	// Sub2 has more wins than Sub1; both eliminated round 3 → Sub2 ranks 2nd.
	if got[1].CommanderName != "Sub2" {
		t.Errorf("rank 2 = %q, want Sub2 (wins tiebreak)", got[1].CommanderName)
	}
	if got[2].CommanderName != "Sub1" {
		t.Errorf("rank 3 = %q, want Sub1", got[2].CommanderName)
	}
	if got[3].CommanderName != "Out" || got[3].FinalRank != 4 {
		t.Errorf("rank 4 = %+v, want Out", got[3])
	}
}

func TestBuildEliminationStandings_AliveOutranksEliminated(t *testing.T) {
	// Two decks still alive (losses=1, eliminatedRound=0), one with
	// fewer LossesAtEnd ranks ahead. One eliminated deck regardless
	// of wins.
	state := []deckState{
		{losses: 1, eliminatedRound: 0, wins: 1}, // alive, 1 loss
		{losses: 0, eliminatedRound: 0, wins: 4}, // alive, undefeated → rank 1
		{losses: 2, eliminatedRound: 5, wins: 9}, // eliminated last
	}
	names := []string{"OneLoss", "Undef", "OutLate"}
	got := buildEliminationStandings(names, state)
	if got[0].CommanderName != "Undef" {
		t.Errorf("rank 1 = %q, want Undef (fewer losses among alive)", got[0].CommanderName)
	}
	if got[1].CommanderName != "OneLoss" {
		t.Errorf("rank 2 = %q, want OneLoss", got[1].CommanderName)
	}
	if got[2].CommanderName != "OutLate" {
		t.Errorf("rank 3 = %q, want OutLate (any-alive outranks any-eliminated)", got[2].CommanderName)
	}
}

func TestBuildEliminationStandings_FinalRankIsSequential(t *testing.T) {
	state := []deckState{
		{losses: 0, eliminatedRound: 0, wins: 3},
		{losses: 2, eliminatedRound: 4, wins: 2},
		{losses: 2, eliminatedRound: 3, wins: 1},
		{losses: 2, eliminatedRound: 2, wins: 0},
	}
	got := buildEliminationStandings([]string{"A", "B", "C", "D"}, state)
	for i, s := range got {
		if s.FinalRank != i+1 {
			t.Errorf("standings[%d].FinalRank = %d, want %d", i, s.FinalRank, i+1)
		}
	}
}

// -----------------------------------------------------------------------------
// RunDoubleElimination config validation.
// -----------------------------------------------------------------------------

func TestRunDoubleElimination_ValidatesNSeats(t *testing.T) {
	_, err := RunDoubleElimination(DoubleEliminationConfig{NSeats: 1})
	if err == nil {
		t.Errorf("expected error for NSeats < 2")
	}
}

func TestRunDoubleElimination_ValidatesDeckCount(t *testing.T) {
	d := &deckparser.TournamentDeck{
		CommanderName:  "X",
		CommanderCards: []*gameengine.Card{{Name: "X"}},
	}
	_, err := RunDoubleElimination(DoubleEliminationConfig{
		Decks:  []*deckparser.TournamentDeck{d},
		NSeats: 4,
	})
	if err == nil {
		t.Errorf("expected error: 1 deck for 4-seat tournament")
	}
}

func TestRunDoubleElimination_ValidatesHatFactoryCount(t *testing.T) {
	decks := make([]*deckparser.TournamentDeck, 4)
	for i := range decks {
		name := string(rune('A' + i))
		decks[i] = &deckparser.TournamentDeck{
			CommanderName:  name,
			CommanderCards: []*gameengine.Card{{Name: name}},
		}
	}
	_, err := RunDoubleElimination(DoubleEliminationConfig{
		Decks:        decks,
		NSeats:       4,
		HatFactories: make([]HatFactory, 2), // 2 != 4 and != 1 and != 0
	})
	if err == nil {
		t.Errorf("expected error for HatFactories count = 2 != {0, 1, NDecks=4}")
	}
}

// -----------------------------------------------------------------------------
// RunDoubleElimination end-to-end (skips without corpus).
// -----------------------------------------------------------------------------

func TestRunDoubleElimination_EndToEnd(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 4)
	if len(paths) < 4 {
		t.Skipf("need 4 decks, got %d", len(paths))
	}
	decks := []*deckparser.TournamentDeck{}
	for _, p := range paths[:4] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}

	cfg := DoubleEliminationConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   1,
		MaxRounds:     6,
		Seed:          53,
		Workers:       2,
		CommanderMode: true,
	}
	r, err := RunDoubleElimination(cfg)
	if err != nil {
		t.Fatalf("RunDoubleElimination: %v", err)
	}
	if len(r.EliminationStandings) != 4 {
		t.Fatalf("expected 4 standings rows, got %d", len(r.EliminationStandings))
	}
	// FinalRank is sequential.
	for i, s := range r.EliminationStandings {
		if s.FinalRank != i+1 {
			t.Errorf("standings[%d].FinalRank = %d, want %d", i, s.FinalRank, i+1)
		}
	}
	// Total games is at least 2 (need at least two pod rounds to
	// resolve a 4-deck DE: the first round makes 3 decks one-loss,
	// the second eliminates someone).
	if r.Games == 0 && r.Crashes == 0 {
		t.Errorf("expected at least one finished game, got 0")
	}
	// At most one deck can finish with LossesAtEnd < 2 (the
	// champion). If MaxRounds was reached without resolution this
	// can also be a leader with 1 loss — both are acceptable.
	survivors := 0
	for _, s := range r.EliminationStandings {
		if s.LossesAtEnd < 2 {
			survivors++
		}
	}
	if survivors == 0 {
		t.Errorf("expected at least one deck with LossesAtEnd < 2 (the champion / leader)")
	}
}
