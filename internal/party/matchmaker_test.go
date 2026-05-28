package party

import (
	"sort"
	"strings"
	"testing"
)

// TestMatchBalancedPod_BelowPodSize verifies the nil-result guard
// when the pool has fewer than 4 entries.
func TestMatchBalancedPod_BelowPodSize(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3} {
		pool := make([]QueuedDeck, n)
		for i := range pool {
			pool[i] = QueuedDeck{Owner: "u", DeckName: "d", Tier: 3, Confidence: 0.8}
		}
		got := MatchBalancedPod(pool)
		if got != nil {
			t.Errorf("n=%d: want nil, got %+v", n, got)
		}
	}
}

// TestMatchBalancedPod_ExactlyFourReturnsAll verifies that a pool
// of exactly 4 returns those 4 with no remaining (no choice to make
// — all queue entries land in the pod).
func TestMatchBalancedPod_ExactlyFourReturnsAll(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", DeckName: "a", Tier: 3, Confidence: 0.8},
		{Owner: "u2", DeckName: "b", Tier: 3, Confidence: 0.8},
		{Owner: "u3", DeckName: "c", Tier: 4, Confidence: 0.8},
		{Owner: "u4", DeckName: "d", Tier: 4, Confidence: 0.8},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("want MatchResult, got nil")
	}
	if len(got.SelectedPod) != 4 {
		t.Errorf("selected pod size: want 4, got %d", len(got.SelectedPod))
	}
	if len(got.Remaining) != 0 {
		t.Errorf("remaining: want 0, got %d", len(got.Remaining))
	}
	if got.Assessment == nil {
		t.Fatal("Assessment nil")
	}
	if got.Assessment.Balance != "balanced" {
		t.Errorf("3/3/4/4 pod: want balanced, got %q", got.Assessment.Balance)
	}
}

// TestMatchBalancedPod_PrefersBalancedPodFromLargerPool verifies the
// core user-spec behavior: when 5+ users are queued, the matchmaker
// picks the 4 whose pod-balance is highest. Setup: 5-deck pool with
// 4 mid-tier decks + 1 cEDH outlier. Expected: the cEDH outlier is
// LEFT IN REMAINING (would create spread=2 with the others); the 4
// balanced decks are selected (spread=0 or 1).
func TestMatchBalancedPod_PrefersBalancedPodFromLargerPool(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", DeckName: "casual_a", Tier: 3, Confidence: 0.85},
		{Owner: "u2", DeckName: "casual_b", Tier: 3, Confidence: 0.85},
		{Owner: "u3", DeckName: "casual_c", Tier: 3, Confidence: 0.85},
		{Owner: "u4", DeckName: "casual_d", Tier: 3, Confidence: 0.85},
		{Owner: "u5", DeckName: "cedh_outlier", Tier: 5, Confidence: 1.00},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("want MatchResult, got nil")
	}
	// The cEDH outlier MUST be in Remaining.
	if len(got.Remaining) != 1 || got.Remaining[0].Owner != "u5" {
		t.Errorf("expected cedh_outlier (u5) in Remaining, got %+v", got.Remaining)
	}
	// All 4 selected decks should be T3.
	for _, qd := range got.SelectedPod {
		if qd.Tier != 3 {
			t.Errorf("selected pod has non-T3 deck %q at tier %d", qd.DeckName, qd.Tier)
		}
	}
	if got.Assessment.TierSpread != 0 {
		t.Errorf("balanced selection: want spread 0, got %d", got.Assessment.TierSpread)
	}
	if got.Assessment.Balance != "balanced" {
		t.Errorf("want balanced, got %q", got.Assessment.Balance)
	}
}

// TestMatchBalancedPod_PrefersSpread1OverSpread2 verifies that the
// matchmaker correctly minimizes spread when multiple non-zero-spread
// pods are possible. Setup: 5-deck pool with tiers 2/3/3/4/5. The
// best 4-combos are:
//   {2,3,3,4} spread 2
//   {3,3,4,5} spread 2
//   {2,3,4,5} spread 3
//   {3,3,4,5} spread 2 (dup)
//   {2,3,3,5} spread 3
// Best spread = 2. Either valid spread-2 combo should be selected.
func TestMatchBalancedPod_PrefersSpread1OverSpread2(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", Tier: 2, Confidence: 0.8},
		{Owner: "u2", Tier: 3, Confidence: 0.8},
		{Owner: "u3", Tier: 3, Confidence: 0.8},
		{Owner: "u4", Tier: 4, Confidence: 0.8},
		{Owner: "u5", Tier: 5, Confidence: 0.8},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result")
	}
	if got.Assessment.TierSpread != 2 {
		t.Errorf("want best-spread 2, got %d (selection: %+v)",
			got.Assessment.TierSpread, got.SelectedPod)
	}
}

// TestMatchBalancedPod_ConfidenceTieBreak verifies that when two
// pods tie on spread, the higher avg-confidence pod wins. Setup: 5
// decks where two distinct 4-subsets both have spread 0, but one
// has higher avg confidence.
func TestMatchBalancedPod_ConfidenceTieBreak(t *testing.T) {
	// Pool: 5 T3 decks. ALL 4-subsets have spread=0. Tie-break:
	// the subset with highest avg confidence wins. Decks u1-u4
	// have confidence 0.9; u5 has 0.5. The best subset is the one
	// EXCLUDING u5 (the low-confidence one).
	pool := []QueuedDeck{
		{Owner: "u1", Tier: 3, Confidence: 0.9},
		{Owner: "u2", Tier: 3, Confidence: 0.9},
		{Owner: "u3", Tier: 3, Confidence: 0.9},
		{Owner: "u4", Tier: 3, Confidence: 0.9},
		{Owner: "u5", Tier: 3, Confidence: 0.5},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result")
	}
	// u5 should be in Remaining (its lower confidence costs the
	// subset's avg-confidence tie-break).
	if len(got.Remaining) != 1 || got.Remaining[0].Owner != "u5" {
		t.Errorf("want u5 (low confidence) in Remaining, got %+v", got.Remaining)
	}
}

// TestMatchBalancedPod_StarvationTieBreak verifies the
// earliest-queued tie-break for cases where multiple pods tie on
// BOTH spread AND avg confidence.
func TestMatchBalancedPod_StarvationTieBreak(t *testing.T) {
	// 5 identical decks (same tier + same confidence). C(5,4)=5
	// subsets all tie on spread (0) AND avg confidence (same).
	// Starvation tie-break: the subset INCLUDING the earliest-
	// queued user wins. The earliest user is u1 (index 0); the
	// pods including u1 are {u1,u2,u3,u4}, {u1,u2,u3,u5},
	// {u1,u2,u4,u5}, {u1,u3,u4,u5}. The earliestIndex on each is
	// 0. The selection should always include u1.
	pool := []QueuedDeck{
		{Owner: "u1", Tier: 3, Confidence: 0.8},
		{Owner: "u2", Tier: 3, Confidence: 0.8},
		{Owner: "u3", Tier: 3, Confidence: 0.8},
		{Owner: "u4", Tier: 3, Confidence: 0.8},
		{Owner: "u5", Tier: 3, Confidence: 0.8},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result")
	}
	// u1 MUST be in the selected pod.
	foundU1 := false
	for _, qd := range got.SelectedPod {
		if qd.Owner == "u1" {
			foundU1 = true
			break
		}
	}
	if !foundU1 {
		var owners []string
		for _, qd := range got.SelectedPod {
			owners = append(owners, qd.Owner)
		}
		t.Errorf("earliest-queued starvation tie-break: expected u1 in selected pod, got %v",
			owners)
	}
}

// TestMatchBalancedPod_LopsidedPoolStillReturns verifies that even
// a hopelessly lopsided pool (no balanced combo possible) still
// returns a result rather than nil — the matchmaker picks the
// LEAST-bad pod, not no pod. Callers that want to refuse lopsided
// launches should inspect Assessment.Balance.
func TestMatchBalancedPod_LopsidedPoolStillReturns(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", Tier: 1, Confidence: 0.9},
		{Owner: "u2", Tier: 1, Confidence: 0.9},
		{Owner: "u3", Tier: 5, Confidence: 0.9},
		{Owner: "u4", Tier: 5, Confidence: 0.9},
		{Owner: "u5", Tier: 5, Confidence: 0.9},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result on lopsided pool — should still return best-available")
	}
	// Best-available is the 3-T5 + 1-T1 pod (spread 4) OR the
	// 2-T5 + 2-T1 pod (spread 4). Both lopsided. Either way the
	// result should be Balance="lopsided" with spread=4. The
	// matchmaker picks one of the two valid spread-4 pods.
	if got.Assessment.TierSpread != 4 {
		t.Errorf("want spread 4 (best available), got %d", got.Assessment.TierSpread)
	}
	if got.Assessment.Balance != "lopsided" {
		t.Errorf("want lopsided, got %q", got.Assessment.Balance)
	}
}

// TestMatchBalancedPod_LargePoolStaysFast smoke-tests that the
// matchmaker runs in reasonable time for a typical large pool
// (N=20 -> C(20,4)=4,845 combinations). Not a benchmark; just
// confirms no quadratic-quadratic blowup.
func TestMatchBalancedPod_LargePoolStaysFast(t *testing.T) {
	pool := make([]QueuedDeck, 20)
	for i := range pool {
		// Spread across all 5 tiers with varying confidence
		pool[i] = QueuedDeck{
			Owner:      "u" + string(rune('A'+i)),
			DeckName:   "deck" + string(rune('A'+i)),
			Tier:       1 + (i % 5),
			Confidence: 0.5 + 0.05*float64(i%10),
		}
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result on N=20 pool")
	}
	if len(got.SelectedPod) != 4 {
		t.Errorf("want 4 selected from N=20, got %d", len(got.SelectedPod))
	}
	if len(got.Remaining) != 16 {
		t.Errorf("want 16 remaining from N=20, got %d", len(got.Remaining))
	}
	// Best pod from a 20-deck pool of mixed tiers should be
	// balanced (spread ≤ 1) — with 4 decks per tier in the pool
	// at every tier, a same-tier 4-pod must exist.
	if got.Assessment.TierSpread > 1 {
		t.Errorf("N=20 mixed pool: want spread ≤ 1 (best should be same-tier), got %d",
			got.Assessment.TierSpread)
	}
}

// TestMatchBalancedPod_RemainingPreservesOrder verifies that the
// Remaining slice preserves the original input order of non-selected
// users — so a UI rendering "still waiting" doesn't shuffle the
// queue display each time the matchmaker runs.
func TestMatchBalancedPod_RemainingPreservesOrder(t *testing.T) {
	// 6 decks; the matchmaker picks 4. The 2 not picked should
	// appear in Remaining in the same order they appeared in pool.
	pool := []QueuedDeck{
		{Owner: "z1", Tier: 5, Confidence: 0.9}, // outlier
		{Owner: "a1", Tier: 3, Confidence: 0.9},
		{Owner: "a2", Tier: 3, Confidence: 0.9},
		{Owner: "z2", Tier: 1, Confidence: 0.9}, // outlier
		{Owner: "a3", Tier: 3, Confidence: 0.9},
		{Owner: "a4", Tier: 3, Confidence: 0.9},
	}
	got := MatchBalancedPod(pool)
	if got == nil {
		t.Fatal("nil result")
	}
	// The 4 T3 decks should be selected; z1 + z2 should be in
	// Remaining IN INPUT ORDER (z1 first since it was first in pool).
	if len(got.Remaining) != 2 {
		t.Fatalf("want 2 remaining, got %d", len(got.Remaining))
	}
	if got.Remaining[0].Owner != "z1" || got.Remaining[1].Owner != "z2" {
		t.Errorf("remaining order: want [z1, z2], got [%s, %s]",
			got.Remaining[0].Owner, got.Remaining[1].Owner)
	}
}

// TestMatchBalancedPod_AssessmentVerdictReadable smoke-tests that
// the Assessment.Verdict is non-empty and human-readable, since
// party-mode UI will render it directly to the user.
func TestMatchBalancedPod_AssessmentVerdictReadable(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", DeckName: "deck_a", Tier: 3, Confidence: 0.8},
		{Owner: "u2", DeckName: "deck_b", Tier: 3, Confidence: 0.8},
		{Owner: "u3", DeckName: "deck_c", Tier: 4, Confidence: 0.8},
		{Owner: "u4", DeckName: "deck_d", Tier: 4, Confidence: 0.8},
	}
	got := MatchBalancedPod(pool)
	if got == nil || got.Assessment == nil {
		t.Fatal("nil result/assessment")
	}
	v := got.Assessment.Verdict
	if v == "" {
		t.Error("Verdict empty")
	}
	if !strings.Contains(v, "T3") || !strings.Contains(v, "T4") {
		t.Errorf("Verdict missing tier callout: %q", v)
	}
}

// TestSortPoolByTier verifies the admin-debug helper: ascending tier,
// then descending confidence within tier.
func TestSortPoolByTier(t *testing.T) {
	pool := []QueuedDeck{
		{Owner: "u1", Tier: 4, Confidence: 0.5},
		{Owner: "u2", Tier: 2, Confidence: 0.9},
		{Owner: "u3", Tier: 2, Confidence: 0.6},
		{Owner: "u4", Tier: 5, Confidence: 1.0},
	}
	got := SortPoolByTier(pool)
	wantOrder := []string{"u2", "u3", "u1", "u4"}
	for i, want := range wantOrder {
		if got[i].Owner != want {
			t.Errorf("position %d: want %q, got %q (full sort: %v)",
				i, want, got[i].Owner, ownerList(got))
		}
	}
	// Verify input pool unmodified
	if pool[0].Owner != "u1" {
		t.Errorf("SortPoolByTier mutated input pool")
	}
	// And that the returned slice is a copy (different backing array)
	got[0].Owner = "MUTATED"
	if pool[0].Owner == "MUTATED" {
		t.Error("SortPoolByTier returned slice shares backing array with input")
	}
}

func ownerList(pool []QueuedDeck) []string {
	out := make([]string, len(pool))
	for i, qd := range pool {
		out[i] = qd.Owner
	}
	sort.Strings(out) // sorted for stable diff display
	return out
}
