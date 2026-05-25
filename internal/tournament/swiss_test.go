package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// swiss_test.go — Swiss bracket style.
//
// Three layers:
//   1. SupportedBracketStyles registry — pins the survey result so
//      a future "ship a sixth style" PR has to update both the
//      function and the test (registry drift detector).
//   2. pairSwissRound pure-pairing tests — repeat-pairing avoidance,
//      bye assignment, sort stability. No game engine needed.
//   3. RunSwiss validation + tiny end-to-end with real decks.

// -----------------------------------------------------------------------------
// Registry survey
// -----------------------------------------------------------------------------

func TestSupportedBracketStyles_ListsAllImplementedRunners(t *testing.T) {
	got := SupportedBracketStyles()
	want := map[string]bool{
		"rotate":      true,
		"pool":        true,
		"lazy-pool":   true,
		"round-robin": true,
		"swiss":       true,
	}
	if len(got) != len(want) {
		t.Errorf("got %d styles, want %d (drift between SupportedBracketStyles and this test)",
			len(got), len(want))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected bracket style %q in registry", s)
		}
		delete(want, s)
	}
	for missing := range want {
		t.Errorf("missing bracket style %q from registry", missing)
	}
}

// -----------------------------------------------------------------------------
// Pairing — pure (no game engine).
// -----------------------------------------------------------------------------

func newStandings(scores ...int) []SwissStanding {
	out := make([]SwissStanding, len(scores))
	for i, s := range scores {
		out[i] = SwissStanding{
			CommanderName: commanderLabel(i),
			Points:        s,
		}
	}
	return out
}

func commanderLabel(i int) string {
	// Deterministic name so tests can reason about who's in which pod.
	return "D" + string(rune('A'+i))
}

func TestPairSwissRound_TopScoresPairTogether(t *testing.T) {
	// 8 decks. Decks A-D are top-of-table (5pts each); E-H are
	// bottom (0pts). Pairing should put A-D in pod 1, E-H in pod 2.
	standings := newStandings(5, 5, 5, 5, 0, 0, 0, 0)
	past := make([]map[int]struct{}, 8)
	for i := range past {
		past[i] = map[int]struct{}{}
	}
	pods, byes := pairSwissRound(standings, past, 4, 42)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	if len(byes) != 0 {
		t.Errorf("expected no byes with 8 decks / 4 seats, got %d", len(byes))
	}
	// Pod 0 must be all top-tier (indices 0-3 in some order).
	for _, idx := range pods[0] {
		if idx > 3 {
			t.Errorf("pod[0] contains bottom-tier deck %d (top-tier should pair together)", idx)
		}
	}
	for _, idx := range pods[1] {
		if idx < 4 {
			t.Errorf("pod[1] contains top-tier deck %d", idx)
		}
	}
}

func TestPairSwissRound_AvoidsRepeatPodmates(t *testing.T) {
	// 8 decks all tied at 0 points. Decks 0-3 already played together
	// last round (pastPairings). Pairing should NOT put all 4 of them
	// in the same pod again.
	standings := newStandings(0, 0, 0, 0, 0, 0, 0, 0)
	past := make([]map[int]struct{}, 8)
	for i := range past {
		past[i] = map[int]struct{}{}
	}
	// 0-1-2-3 played together.
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if i != j {
				past[i][j] = struct{}{}
			}
		}
	}
	pods, _ := pairSwissRound(standings, past, 4, 7)
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	allRepeats := true
	for _, idx := range pods[0] {
		if idx >= 4 {
			allRepeats = false
			break
		}
	}
	if allRepeats {
		t.Errorf("pod[0] = %v re-paired the same 4 decks from last round", pods[0])
	}
}

func TestPairSwissRound_ToleratesRepeatsWhenForced(t *testing.T) {
	// 4 decks, all tied, all having played each other before.
	// Pairing should still produce a single full pod (tolerated repeat).
	standings := newStandings(0, 0, 0, 0)
	past := make([]map[int]struct{}, 4)
	for i := range past {
		past[i] = map[int]struct{}{}
		for j := 0; j < 4; j++ {
			if i != j {
				past[i][j] = struct{}{}
			}
		}
	}
	pods, byes := pairSwissRound(standings, past, 4, 1)
	if len(pods) != 1 || len(byes) != 0 {
		t.Errorf("expected 1 full pod, 0 byes; got pods=%d byes=%d", len(pods), len(byes))
	}
	if len(pods) > 0 && len(pods[0]) != 4 {
		t.Errorf("pod size = %d, want 4", len(pods[0]))
	}
}

func TestPairSwissRound_ByeForExtraDecks(t *testing.T) {
	// 5 decks, 4 seats → 1 pod + 1 bye.
	standings := newStandings(3, 2, 1, 0, 0)
	past := make([]map[int]struct{}, 5)
	for i := range past {
		past[i] = map[int]struct{}{}
	}
	pods, byes := pairSwissRound(standings, past, 4, 99)
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	if len(byes) != 1 {
		t.Fatalf("expected 1 bye, got %d", len(byes))
	}
}

func TestPairSwissRound_ByePrefersFewestPriorByes(t *testing.T) {
	// 5 decks tied at 0 points. Deck 0 already has 1 bye; the others
	// have 0. The new bye should NOT be deck 0.
	standings := newStandings(0, 0, 0, 0, 0)
	standings[0].Byes = 1
	past := make([]map[int]struct{}, 5)
	for i := range past {
		past[i] = map[int]struct{}{}
	}
	pods, byes := pairSwissRound(standings, past, 4, 13)
	if len(pods) != 1 || len(byes) != 1 {
		t.Fatalf("expected 1 pod + 1 bye, got pods=%d byes=%d", len(pods), len(byes))
	}
	// Deck 0 must be in the pod, not the bye list.
	for _, b := range byes {
		if b == 0 {
			t.Errorf("deck 0 byed twice; bye distribution not balancing")
		}
	}
}

func TestPairSwissRound_DeterministicForSameSeed(t *testing.T) {
	standings := newStandings(2, 2, 2, 2, 1, 1, 1, 1)
	makePast := func() []map[int]struct{} {
		p := make([]map[int]struct{}, 8)
		for i := range p {
			p[i] = map[int]struct{}{}
		}
		return p
	}
	a, _ := pairSwissRound(standings, makePast(), 4, 12345)
	b, _ := pairSwissRound(standings, makePast(), 4, 12345)
	if len(a) != len(b) {
		t.Fatalf("pod count differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			t.Fatalf("pod %d size differs", i)
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				t.Errorf("pairing not deterministic at pod=%d slot=%d: %d vs %d", i, j, a[i][j], b[i][j])
			}
		}
	}
}

// -----------------------------------------------------------------------------
// RunSwiss config validation
// -----------------------------------------------------------------------------

func TestRunSwiss_ValidatesNSeats(t *testing.T) {
	_, err := RunSwiss(SwissConfig{NSeats: 1, Rounds: 2})
	if err == nil {
		t.Errorf("expected error for NSeats < 2")
	}
}

func TestRunSwiss_ValidatesDeckCount(t *testing.T) {
	d := &deckparser.TournamentDeck{
		CommanderName:  "X",
		CommanderCards: []*gameengine.Card{{Name: "X"}},
	}
	_, err := RunSwiss(SwissConfig{
		Decks:  []*deckparser.TournamentDeck{d},
		NSeats: 4,
		Rounds: 2,
	})
	if err == nil {
		t.Errorf("expected error: 1 deck for 4-seat tournament")
	}
}

func TestRunSwiss_ValidatesRounds(t *testing.T) {
	decks := make([]*deckparser.TournamentDeck, 4)
	for i := range decks {
		decks[i] = &deckparser.TournamentDeck{
			CommanderName:  commanderLabel(i),
			CommanderCards: []*gameengine.Card{{Name: commanderLabel(i)}},
		}
	}
	_, err := RunSwiss(SwissConfig{Decks: decks, NSeats: 4, Rounds: 0})
	if err == nil {
		t.Errorf("expected error for Rounds <= 0")
	}
}

func TestRunSwiss_ValidatesHatFactoryCount(t *testing.T) {
	decks := make([]*deckparser.TournamentDeck, 4)
	for i := range decks {
		decks[i] = &deckparser.TournamentDeck{
			CommanderName:  commanderLabel(i),
			CommanderCards: []*gameengine.Card{{Name: commanderLabel(i)}},
		}
	}
	_, err := RunSwiss(SwissConfig{
		Decks:        decks,
		NSeats:       4,
		Rounds:       2,
		HatFactories: []HatFactory{func() gameengine.Hat { return &hat.GreedyHat{} }, func() gameengine.Hat { return &hat.GreedyHat{} }},
	})
	if err == nil {
		t.Errorf("expected error for HatFactories count = 2 != 4 decks (must be 0, 1, or NDecks)")
	}
}

// -----------------------------------------------------------------------------
// RunSwiss end-to-end (requires corpus + decks; mirrors existing pattern).
// -----------------------------------------------------------------------------

func TestRunSwiss_EndToEnd(t *testing.T) {
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

	cfg := SwissConfig{
		Decks:         decks,
		NSeats:        4,
		Rounds:        3,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       2,
		CommanderMode: true,
	}
	r, err := RunSwiss(cfg)
	if err != nil {
		t.Fatalf("RunSwiss: %v", err)
	}
	// 3 rounds × 1 pod × 1 game = 3 games (or up to 3 crashes).
	if r.Games+r.Crashes != 3 {
		t.Errorf("expected 3 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	if len(r.SwissStandings) != 4 {
		t.Fatalf("expected 4 standings rows, got %d", len(r.SwissStandings))
	}
	// Standings must be sorted by Points desc.
	for i := 1; i < len(r.SwissStandings); i++ {
		if r.SwissStandings[i-1].Points < r.SwissStandings[i].Points {
			t.Errorf("standings not sorted at index %d: %d < %d",
				i, r.SwissStandings[i-1].Points, r.SwissStandings[i].Points)
		}
	}
	// Every deck plays each round (4 decks ÷ 4 seats = 1 pod, no byes).
	for _, s := range r.SwissStandings {
		if s.Games > 3 {
			t.Errorf("deck %s played %d games, exceeds 3 rounds", s.CommanderName, s.Games)
		}
	}
}
