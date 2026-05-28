package tournament

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	gameengine "github.com/hexdek/hexdek/internal/gameengine"
)

// roundrobin_validation_r60_test pins the input-validation guard
// branches in RunRoundRobin plus the pure helper combinations(). These
// fire before any engine code runs, closing the bulk of the 0%
// coverage gap on roundrobin.go without needing real tournament
// simulation.

// TestCombinations_SmallExhaustive pins exact lexicographic output for
// C(n, k) on small inputs where every enumeration is hand-checkable.
// combinations() is the only logic in roundrobin.go that's purely
// algorithmic (no engine call), and downstream code rotates pods
// based on this ordering — a regression here would silently change
// which deck pairs face off across the corpus.
func TestCombinations_SmallExhaustive(t *testing.T) {
	cases := []struct {
		n, k int
		want [][]int
	}{
		{4, 2, [][]int{{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}}},
		{3, 1, [][]int{{0}, {1}, {2}}},
		{3, 3, [][]int{{0, 1, 2}}},
		{5, 4, [][]int{
			{0, 1, 2, 3}, {0, 1, 2, 4}, {0, 1, 3, 4},
			{0, 2, 3, 4}, {1, 2, 3, 4},
		}},
	}
	for _, tc := range cases {
		got := combinations(tc.n, tc.k)
		if len(got) != len(tc.want) {
			t.Errorf("C(%d,%d): got %d combos, want %d", tc.n, tc.k, len(got), len(tc.want))
			continue
		}
		for i := range tc.want {
			if !intSliceEqual(got[i], tc.want[i]) {
				t.Errorf("C(%d,%d)[%d] = %v, want %v", tc.n, tc.k, i, got[i], tc.want[i])
			}
		}
	}
}

// TestCombinations_DegenerateInputs pins behavior at the edges. k=0
// yields a single empty combination (the empty set is the only 0-of-n
// pick); k>n yields no combinations.
func TestCombinations_DegenerateInputs(t *testing.T) {
	if got := combinations(5, 0); len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("C(5,0) = %v, want one empty combo", got)
	}
	if got := combinations(2, 5); len(got) != 0 {
		t.Errorf("C(2,5) should be empty, got %v", got)
	}
}

// TestCombinations_LexicographicOrder verifies the contract that
// combinations come back in lexicographic ascending order — pod
// indexing in RunRoundRobin uses this implicitly (the per-pod seed
// offset = podIdx*10000 means a reordering would change the random
// stream for every pod, breaking reproducibility).
func TestCombinations_LexicographicOrder(t *testing.T) {
	got := combinations(6, 3)
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		// Compare element-wise; the first differing index decides.
		for k := 0; k < len(prev); k++ {
			if prev[k] == cur[k] {
				continue
			}
			if prev[k] > cur[k] {
				t.Errorf("combinations not lexicographic at idx %d: %v > %v", i, prev, cur)
			}
			break
		}
	}
}

// TestRunRoundRobin_NSeatsTooSmall pins the first input-validation
// branch: NSeats < 2 is rejected with a clear error.
func TestRunRoundRobin_NSeatsTooSmall(t *testing.T) {
	_, err := RunRoundRobin(RoundRobinConfig{NSeats: 1, GamesPerPod: 1})
	if err == nil || !strings.Contains(err.Error(), "NSeats must be >= 2") {
		t.Errorf("expected NSeats error, got: %v", err)
	}
}

// TestRunRoundRobin_TooFewDecks pins the deck-count guard: there must
// be at least NSeats decks to form even a single pod.
func TestRunRoundRobin_TooFewDecks(t *testing.T) {
	_, err := RunRoundRobin(RoundRobinConfig{
		NSeats:      4,
		GamesPerPod: 1,
		Decks:       []*deckparser.TournamentDeck{stubTournamentDeck("A"), stubTournamentDeck("B")},
	})
	if err == nil || !strings.Contains(err.Error(), "need at least 4 decks") {
		t.Errorf("expected too-few-decks error, got: %v", err)
	}
}

// TestRunRoundRobin_GamesPerPodNonPositive pins the third guard.
func TestRunRoundRobin_GamesPerPodNonPositive(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		stubTournamentDeck("A"), stubTournamentDeck("B"),
	}
	_, err := RunRoundRobin(RoundRobinConfig{NSeats: 2, GamesPerPod: 0, Decks: decks})
	if err == nil || !strings.Contains(err.Error(), "GamesPerPod must be > 0") {
		t.Errorf("expected GamesPerPod error, got: %v", err)
	}
}

// TestRunRoundRobin_NilDeckErrors pins the nil-deck guard inside the
// for-each loop — a deck slot with a nil pointer must surface a clear
// error rather than panic when the runner dereferences it later.
func TestRunRoundRobin_NilDeckErrors(t *testing.T) {
	decks := []*deckparser.TournamentDeck{
		stubTournamentDeck("A"),
		nil, // <-- the offender
	}
	_, err := RunRoundRobin(RoundRobinConfig{NSeats: 2, GamesPerPod: 1, Decks: decks})
	if err == nil || !strings.Contains(err.Error(), "decks[1] is nil") {
		t.Errorf("expected nil-deck error citing index 1, got: %v", err)
	}
}

// TestRunRoundRobin_NoCommanderErrors pins the empty-CommanderCards
// guard. A deck with no commander can't drive a tournament — the
// commander seat is load-bearing.
func TestRunRoundRobin_NoCommanderErrors(t *testing.T) {
	noCmdr := &deckparser.TournamentDeck{CommanderName: "X", CommanderCards: nil}
	decks := []*deckparser.TournamentDeck{stubTournamentDeck("A"), noCmdr}
	_, err := RunRoundRobin(RoundRobinConfig{NSeats: 2, GamesPerPod: 1, Decks: decks})
	if err == nil || !strings.Contains(err.Error(), "no commander") {
		t.Errorf("expected no-commander error, got: %v", err)
	}
}

// TestRunRoundRobin_HatFactoriesWrongLengthErrors pins the
// HatFactories arity check: must be 0, 1, or len(Decks). Three
// factories for two decks should fail cleanly rather than letting the
// later copy() truncate silently.
func TestRunRoundRobin_HatFactoriesWrongLengthErrors(t *testing.T) {
	stub := func() gameengine.Hat { return nil }
	decks := []*deckparser.TournamentDeck{
		stubTournamentDeck("A"), stubTournamentDeck("B"), stubTournamentDeck("C"),
	}
	// 2 factories for 3 decks is not 0, 1, or len(Decks).
	_, err := RunRoundRobin(RoundRobinConfig{
		NSeats: 2, GamesPerPod: 1, Decks: decks,
		HatFactories: []HatFactory{stub, stub},
	})
	if err == nil || !strings.Contains(err.Error(), "HatFactories") {
		t.Errorf("expected HatFactories arity error, got: %v", err)
	}
}

// intSliceEqual is a small []int equality helper used by the
// combinations tests above.
func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
