package tournament

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// balanced_pool_test.go — snake-draft seeding for balanced pods.
//
// Three layers:
//   1. Registry pin: "balanced-pool" appears in SupportedBracketStyles.
//   2. Pure seeding (BalancedPodSeeding) — equal pod strength,
//      uniform-strength fallback, divisibility constraint, length
//      mismatch, determinism.
//   3. RunBalancedPool validation + tiny end-to-end with real decks.

// -----------------------------------------------------------------------------
// Registry pin
// -----------------------------------------------------------------------------

func TestSupportedBracketStyles_IncludesBalancedPool(t *testing.T) {
	got := SupportedBracketStyles()
	for _, s := range got {
		if s == "balanced-pool" {
			return
		}
	}
	t.Errorf("expected balanced-pool in SupportedBracketStyles, got %v", got)
}

// -----------------------------------------------------------------------------
// Pure seeding
// -----------------------------------------------------------------------------

func TestBalancedPodSeeding_EvenPodStrengthOnDivisibleInput(t *testing.T) {
	// 8 decks with strengths 8..1. NSeats=4 → 2 pods. Snake-draft:
	//   Round 0 fw: pod 0 gets D8, pod 1 gets D7
	//   Round 1 rv: pod 1 gets D6, pod 0 gets D5
	//   Round 2 fw: pod 0 gets D4, pod 1 gets D3
	//   Round 3 rv: pod 1 gets D2, pod 0 gets D1
	// Pod 0 = {8, 5, 4, 1} = 18; Pod 1 = {7, 6, 3, 2} = 18. Spread=0.
	strengths := []float64{8, 7, 6, 5, 4, 3, 2, 1}
	pods, err := BalancedPodSeeding(8, 4, strengths)
	if err != nil {
		t.Fatalf("BalancedPodSeeding: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	spread := BalancedPodStrengthSpread(pods, strengths)
	if spread != 0 {
		t.Errorf("expected 0 strength spread, got %v (pods=%v)", spread, pods)
	}
}

func TestBalancedPodSeeding_SmallerSpreadThanFixedOrder(t *testing.T) {
	// 12 decks with strengths 12..1. Naïve fixed-order pod
	// assignment (D1-D4, D5-D8, D9-D12) would give pods of strength
	// 42 / 26 / 10 — spread 32. Snake-draft should be tighter.
	strengths := []float64{12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	pods, err := BalancedPodSeeding(12, 4, strengths)
	if err != nil {
		t.Fatalf("BalancedPodSeeding: %v", err)
	}
	if len(pods) != 3 {
		t.Fatalf("expected 3 pods, got %d", len(pods))
	}
	spread := BalancedPodStrengthSpread(pods, strengths)
	// Snake-draft gives 12+7+6+1=26, 11+8+5+2=26, 10+9+4+3=26 → spread=0.
	if spread != 0 {
		t.Errorf("expected 0 strength spread for clean 12-deck case, got %v", spread)
	}
}

func TestBalancedPodSeeding_UniformStrengthFallsBackToIndexOrder(t *testing.T) {
	// Empty strengths → all decks treated as equal. Snake-draft on
	// stable index order: pod 0 gets idx 0; pod 1 gets idx 1;
	// (reverse) pod 1 gets idx 2; pod 0 gets idx 3; ...
	pods, err := BalancedPodSeeding(4, 4, nil)
	if err != nil {
		t.Fatalf("BalancedPodSeeding: %v", err)
	}
	// 4 decks / 4 seats = 1 pod with all decks 0..3 in fixed order.
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	want := []int{0, 1, 2, 3}
	for i, idx := range pods[0] {
		if idx != want[i] {
			t.Errorf("pod[0][%d] = %d, want %d", i, idx, want[i])
		}
	}
}

func TestBalancedPodSeeding_RejectsUnevenDivision(t *testing.T) {
	// 5 decks, 4 seats — snake-draft has no graceful undersize-pod
	// story.
	_, err := BalancedPodSeeding(5, 4, nil)
	if err == nil {
		t.Errorf("expected error for NDecks=5 NSeats=4 (non-divisible)")
	}
}

func TestBalancedPodSeeding_RejectsLengthMismatch(t *testing.T) {
	_, err := BalancedPodSeeding(8, 4, []float64{1, 2, 3}) // 3 != 8
	if err == nil {
		t.Errorf("expected error for Strengths length mismatch")
	}
}

func TestBalancedPodSeeding_RejectsTooFewSeats(t *testing.T) {
	_, err := BalancedPodSeeding(8, 1, nil)
	if err == nil {
		t.Errorf("expected error for NSeats < 2")
	}
}

func TestBalancedPodSeeding_DeterministicForSameInput(t *testing.T) {
	strengths := []float64{5.0, 4.5, 4.5, 4.0, 3.5, 3.0, 2.5, 2.0} // ties present
	a, err := BalancedPodSeeding(8, 4, strengths)
	if err != nil {
		t.Fatalf("seeding A: %v", err)
	}
	b, err := BalancedPodSeeding(8, 4, strengths)
	if err != nil {
		t.Fatalf("seeding B: %v", err)
	}
	for i := range a {
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				t.Errorf("seeding not deterministic at [%d][%d]: %d vs %d", i, j, a[i][j], b[i][j])
			}
		}
	}
}

func TestBalancedPodSeeding_TopSeedsSpreadAcrossPods(t *testing.T) {
	// Confirm structural property: in each snake-draft round, no
	// two top-N decks land in the same pod. The two strongest decks
	// (idxs in strengths-sorted slots 0 and 1) MUST go to different
	// pods when there are >= 2 pods — otherwise the seeding is
	// broken regardless of strength sums.
	strengths := []float64{10, 9, 8, 7, 6, 5, 4, 3}
	pods, err := BalancedPodSeeding(8, 4, strengths)
	if err != nil {
		t.Fatalf("BalancedPodSeeding: %v", err)
	}
	// Find which pod holds deck 0 (strongest) and deck 1
	// (second-strongest) — they must be in different pods.
	podOfDeck0, podOfDeck1 := -1, -1
	for i, pod := range pods {
		for _, idx := range pod {
			if idx == 0 {
				podOfDeck0 = i
			}
			if idx == 1 {
				podOfDeck1 = i
			}
		}
	}
	if podOfDeck0 == podOfDeck1 {
		t.Errorf("strongest two decks landed in the same pod (%d) — snake-draft broken", podOfDeck0)
	}
}

func TestBalancedPodStrengthSpread_EmptyStrengthsReturnsZero(t *testing.T) {
	pods := [][]int{{0, 1, 2, 3}}
	got := BalancedPodStrengthSpread(pods, nil)
	if got != 0 {
		t.Errorf("expected 0 with empty strengths, got %v", got)
	}
}

func TestBalancedPodStrengthSpread_KnownDelta(t *testing.T) {
	// Hand-built pods to verify the spread calculation itself
	// (not the seeding output).
	pods := [][]int{
		{0, 1}, // strengths 10 + 1 = 11
		{2, 3}, // strengths 5 + 5 = 10
	}
	strengths := []float64{10, 1, 5, 5}
	got := BalancedPodStrengthSpread(pods, strengths)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("spread = %v, want 1.0", got)
	}
}

// -----------------------------------------------------------------------------
// RunBalancedPool validation
// -----------------------------------------------------------------------------

func TestRunBalancedPool_ValidatesNSeats(t *testing.T) {
	_, err := RunBalancedPool(BalancedPoolConfig{NSeats: 1})
	if err == nil {
		t.Errorf("expected error for NSeats < 2")
	}
}

func TestRunBalancedPool_RejectsNonDivisibleDeckCount(t *testing.T) {
	// 5 decks, 4 seats — should fail at the seeding step.
	decks := make([]*deckparser.TournamentDeck, 5)
	for i := range decks {
		name := string(rune('A' + i))
		decks[i] = &deckparser.TournamentDeck{
			CommanderName:  name,
			CommanderCards: []*gameengine.Card{{Name: name}},
		}
	}
	_, err := RunBalancedPool(BalancedPoolConfig{
		Decks:  decks,
		NSeats: 4,
	})
	if err == nil {
		t.Errorf("expected error for NDecks=5 NSeats=4")
	}
}

func TestRunBalancedPool_RejectsStrengthsLengthMismatch(t *testing.T) {
	decks := make([]*deckparser.TournamentDeck, 4)
	for i := range decks {
		name := string(rune('A' + i))
		decks[i] = &deckparser.TournamentDeck{
			CommanderName:  name,
			CommanderCards: []*gameengine.Card{{Name: name}},
		}
	}
	_, err := RunBalancedPool(BalancedPoolConfig{
		Decks:     decks,
		NSeats:    4,
		Strengths: []float64{1, 2, 3}, // 3 != 4
	})
	if err == nil {
		t.Errorf("expected error for Strengths length mismatch")
	}
}

// -----------------------------------------------------------------------------
// RunBalancedPool end-to-end (skips without corpus).
// -----------------------------------------------------------------------------

func TestRunBalancedPool_EndToEnd(t *testing.T) {
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

	cfg := BalancedPoolConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   2,
		Seed:          71,
		Workers:       2,
		CommanderMode: true,
		Strengths:     []float64{2.0, 1.5, 1.0, 0.5},
	}
	r, err := RunBalancedPool(cfg)
	if err != nil {
		t.Fatalf("RunBalancedPool: %v", err)
	}
	// 4 decks / 4 seats = 1 pod × 2 games = 2 outcomes.
	if r.Games+r.Crashes != 2 {
		t.Errorf("expected 2 outcomes, got games=%d crashes=%d", r.Games, r.Crashes)
	}
	if len(r.BalancedPodPlan) != 1 {
		t.Fatalf("expected 1 pod plan entry, got %d", len(r.BalancedPodPlan))
	}
	pod := r.BalancedPodPlan[0]
	if len(pod.DeckIndices) != 4 || len(pod.CommanderNames) != 4 {
		t.Errorf("pod plan shape wrong: indices=%d names=%d (want 4/4)",
			len(pod.DeckIndices), len(pod.CommanderNames))
	}
	wantSum := 2.0 + 1.5 + 1.0 + 0.5
	if math.Abs(pod.PodStrength-wantSum) > 1e-9 {
		t.Errorf("PodStrength = %v, want %v", pod.PodStrength, wantSum)
	}
}
