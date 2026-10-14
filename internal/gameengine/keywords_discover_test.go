package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Discover (CR §701.57a)
// ---------------------------------------------------------------------------

func newDiscoverGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(101))
	return NewGameState(2, rng, nil)
}

// nonlandCard builds a simple nonland card with the given CMC + types
// (defaulting to {"instant"} when none supplied).
func nonlandCard(owner int, name string, cmc int, types ...string) *Card {
	if len(types) == 0 {
		types = []string{"instant"}
	}
	return &Card{
		Name:  name,
		Owner: owner,
		CMC:   cmc,
		Types: append([]string(nil), types...),
	}
}

// landCard builds a basic-land-like card.
func landCard(owner int, name string) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		CMC:   0,
		Types: []string{"land"},
	}
}

// stackLibraryTop seats `cards` at the top of `seat`'s library in the
// given order — cards[0] is drawn first.
func stackLibraryTop(gs *GameState, seat int, cards ...*Card) {
	// Prepend so cards[0] ends up at index 0.
	prefix := make([]*Card, 0, len(cards)+len(gs.Seats[seat].Library))
	prefix = append(prefix, cards...)
	prefix = append(prefix, gs.Seats[seat].Library...)
	gs.Seats[seat].Library = prefix
}

// countDiscoverEvents counts event-log entries by kind for assertion.
func countDiscoverEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// ===========================================================================
// HasDiscover / DiscoverCount
// ===========================================================================

func TestHasDiscover_Detects(t *testing.T) {
	c := &Card{Name: "Geological Appraiser"}
	if HasDiscover(c) {
		t.Fatal("HasDiscover without an AST should be false")
	}
	c.AST = mustParseDiscoverAST(t, 3)
	if !HasDiscover(c) {
		t.Fatal("HasDiscover should detect the keyword from AST")
	}
	if HasDiscover(nil) {
		t.Fatal("HasDiscover(nil) should be false")
	}
}

func TestDiscoverCount_ParsesArg(t *testing.T) {
	c := &Card{Name: "Geological Appraiser", AST: mustParseDiscoverAST(t, 3)}
	if got := DiscoverCount(c); got != 3 {
		t.Fatalf("DiscoverCount: want 3, got %d", got)
	}
}

// ===========================================================================
// (a) Hits first nonland with CMC <= N and casts it for free
// ===========================================================================

func TestApplyDiscover_HitsFirstNonlandUnderN_CastsFree(t *testing.T) {
	gs := newDiscoverGame(t)

	// Library top-down: a 5-cmc Sorcery (too expensive for N=3),
	// then a 2-cmc Instant (HIT under N=3), then filler.
	tooBig := nonlandCard(0, "Big Sorcery", 5, "sorcery")
	hit := nonlandCard(0, "Cheap Bolt", 2, "instant")
	tail := nonlandCard(0, "Random Filler", 0, "instant")
	stackLibraryTop(gs, 0, tooBig, hit, tail)

	manaBefore := gs.Seats[0].ManaPool
	got := ApplyDiscover(gs, 0, 3)

	if got != hit {
		t.Fatalf("ApplyDiscover should return the first nonland under N; got %v want %v", got, hit)
	}
	// The hit is on the stack (cast for free); not in hand.
	if len(gs.Stack) != 1 {
		t.Fatalf("expected hit pushed to stack, stack=%d", len(gs.Stack))
	}
	if gs.Stack[0].Card != hit {
		t.Errorf("stack top should be the hit card, got %v", gs.Stack[0].Card)
	}
	// Free cast: mana untouched.
	if gs.Seats[0].ManaPool != manaBefore {
		t.Errorf("free cast should not spend mana; before=%d after=%d", manaBefore, gs.Seats[0].ManaPool)
	}
	// Hit not in hand.
	for _, c := range gs.Seats[0].Hand {
		if c == hit {
			t.Error("hit must not also be in hand after a cast-free disposition")
		}
	}
	// One discover_cast event with rule + n + cards_exiled.
	if got := countDiscoverEvents(gs, "discover_cast"); got != 1 {
		t.Errorf("expected 1 discover_cast event, got %d", got)
	}
	// The 5-cmc miss was returned to the bottom of the library.
	if len(gs.Seats[0].Library) < 1 || gs.Seats[0].Library[len(gs.Seats[0].Library)-1] != tooBig {
		t.Errorf("the 5-cmc miss should be at the bottom of the library after discover")
	}
}

// ===========================================================================
// (b) Whiff: no qualifying card found before library empties
// ===========================================================================

func TestApplyDiscover_WhiffWhenNoQualifyingCard(t *testing.T) {
	gs := newDiscoverGame(t)

	// Three lands and two too-expensive nonlands — none qualify for N=2.
	l1 := landCard(0, "Mountain")
	l2 := landCard(0, "Plains")
	expensive1 := nonlandCard(0, "Mega Bomb", 7, "sorcery")
	expensive2 := nonlandCard(0, "Massive Threat", 6, "creature")
	stackLibraryTop(gs, 0, l1, l2, expensive1, expensive2)

	got := ApplyDiscover(gs, 0, 2)

	if got != nil {
		t.Fatalf("ApplyDiscover should whiff when no qualifying card exists, got %v", got)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("stack should be empty on whiff, got %d items", len(gs.Stack))
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should be untouched on whiff, got %d cards", len(gs.Seats[0].Hand))
	}
	if got := countDiscoverEvents(gs, "discover_whiff"); got != 1 {
		t.Errorf("expected 1 discover_whiff event, got %d", got)
	}
	if got := countDiscoverEvents(gs, "discover_cast"); got != 0 {
		t.Errorf("expected 0 discover_cast events on whiff, got %d", got)
	}
	// All 4 cards returned to the bottom (now they ARE the whole
	// library), in some order. Exile should be empty.
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("exile should be empty after whiff cleanup, got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Library) != 4 {
		t.Errorf("all 4 cards should be back in library, got %d", len(gs.Seats[0].Library))
	}
}

// ===========================================================================
// (c) Lands skip past — the hit is the first nonland that qualifies
// ===========================================================================

func TestApplyDiscover_LandsSkipPast(t *testing.T) {
	gs := newDiscoverGame(t)

	// Two lands ahead of a 2-cmc nonland. Discover should skip both
	// lands and pick the nonland.
	l1 := landCard(0, "Forest")
	l2 := landCard(0, "Island")
	hit := nonlandCard(0, "Counterspell", 2, "instant")
	stackLibraryTop(gs, 0, l1, l2, hit)

	got := ApplyDiscover(gs, 0, 4)
	if got != hit {
		t.Fatalf("lands should not satisfy the hit; want %v, got %v", hit, got)
	}
	// Both lands should now be on the bottom of the library.
	libLen := len(gs.Seats[0].Library)
	if libLen < 2 {
		t.Fatalf("expected the 2 skipped lands back in library, got %d", libLen)
	}
	bottomTwo := map[*Card]bool{
		gs.Seats[0].Library[libLen-1]: true,
		gs.Seats[0].Library[libLen-2]: true,
	}
	if !bottomTwo[l1] || !bottomTwo[l2] {
		t.Error("both skipped lands should be at the bottom of the library after discover")
	}
}

func TestApplyDiscover_NonlandOverN_AlsoSkipsPast(t *testing.T) {
	// A nonland whose CMC is GREATER than N should also be skipped
	// (only nonland AND CMC<=N is a hit). Confirms the conjunction
	// in discoverIsHit.
	gs := newDiscoverGame(t)

	overN := nonlandCard(0, "Over-Cost", 8, "creature") // not a hit at N=4
	hit := nonlandCard(0, "Just Right", 3, "instant")   // hit at N=4
	stackLibraryTop(gs, 0, overN, hit)

	got := ApplyDiscover(gs, 0, 4)
	if got != hit {
		t.Fatalf("expected to skip over the high-CMC nonland and hit the cheap one; got %v", got)
	}
}

// ===========================================================================
// (d) Misses go to bottom of library in a randomized order
// ===========================================================================

func TestApplyDiscover_MissesShuffledToBottom(t *testing.T) {
	// Verify two things together: (1) ALL misses end up at the
	// bottom of the library (the hit does NOT); (2) at least one of
	// many trials produces a non-input order (probabilistic
	// shuffle confirmation).
	matchedInputOrderRuns := 0
	const trials = 32

	for trial := 0; trial < trials; trial++ {
		// Use a different seed per trial so we get RNG diversity.
		rng := rand.New(rand.NewSource(int64(trial * 137)))
		gs := NewGameState(2, rng, nil)

		// 4 distinct lands ahead of a 2-cmc hit. After discover, all
		// 4 lands should be at the bottom, the hit on the stack.
		l1 := landCard(0, "L1")
		l2 := landCard(0, "L2")
		l3 := landCard(0, "L3")
		l4 := landCard(0, "L4")
		hit := nonlandCard(0, "Bolt", 2, "instant")
		stackLibraryTop(gs, 0, l1, l2, l3, l4, hit)

		got := ApplyDiscover(gs, 0, 2)
		if got != hit {
			t.Fatalf("trial %d: discover should have hit Bolt, got %v", trial, got)
		}
		libLen := len(gs.Seats[0].Library)
		if libLen != 4 {
			t.Fatalf("trial %d: all 4 lands should be back in library; got %d", trial, libLen)
		}
		bottom := gs.Seats[0].Library[libLen-4:]
		// Confirm all 4 lands are accounted for in the bottom 4.
		seen := map[*Card]bool{}
		for _, c := range bottom {
			seen[c] = true
		}
		for _, l := range []*Card{l1, l2, l3, l4} {
			if !seen[l] {
				t.Fatalf("trial %d: land %s not in bottom 4 of library", trial, l.Name)
			}
		}
		// Was the order identical to the input order [l1, l2, l3, l4]?
		ordered := bottom[0] == l1 && bottom[1] == l2 && bottom[2] == l3 && bottom[3] == l4
		if ordered {
			matchedInputOrderRuns++
		}
	}

	// 4! = 24 permutations; the probability of matching the input
	// order over 32 trials with a working shuffle is (1/24)^32 ≈ 0,
	// so "ALL trials matched input order" is a near-certainty signal
	// that the shuffle isn't running. We only fail if every single
	// trial preserved the input order.
	if matchedInputOrderRuns == trials {
		t.Fatalf("misses never reordered across %d trials; shuffle not running", trials)
	}
}

// ===========================================================================
// (e) CostMeta["discover_cast"]=true stamped on the cast StackItem
// ===========================================================================

func TestApplyDiscover_StampsCostMetaOnCast(t *testing.T) {
	gs := newDiscoverGame(t)

	hit := nonlandCard(0, "Cheap Bolt", 2, "instant")
	stackLibraryTop(gs, 0, hit)

	if got := ApplyDiscover(gs, 0, 3); got != hit {
		t.Fatalf("expected hit on Bolt, got %v", got)
	}

	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	item := gs.Stack[0]
	if item.CostMeta == nil {
		t.Fatal("StackItem.CostMeta must not be nil after a discover cast")
	}
	if v, _ := item.CostMeta["discover_cast"].(bool); !v {
		t.Errorf("CostMeta[discover_cast] = %v, want true", item.CostMeta["discover_cast"])
	}
	if v, _ := item.CostMeta["alt_cost"].(string); v != "discover" {
		t.Errorf("CostMeta[alt_cost] = %v, want \"discover\"", item.CostMeta["alt_cost"])
	}
	if v, _ := item.CostMeta["discover_n"].(int); v != 3 {
		t.Errorf("CostMeta[discover_n] = %v, want 3", item.CostMeta["discover_n"])
	}
	if v, _ := item.CostMeta["free_cast"].(bool); !v {
		t.Errorf("CostMeta[free_cast] = %v, want true (cascade-shape compat)", item.CostMeta["free_cast"])
	}
	if item.CastZone != ZoneExile {
		t.Errorf("CastZone = %v, want ZoneExile", item.CastZone)
	}
	// The discover grant should have been cleared from ZoneCastGrants
	// after the push consumed it.
	if g, ok := gs.ZoneCastGrants[hit]; ok && g != nil {
		t.Errorf("ZoneCastGrants for the hit should have been cleared post-push; still present: %+v", g)
	}
}

// ===========================================================================
// Choice: ToHand
// ===========================================================================

func TestApplyDiscoverWithChoice_ToHandPutsCardInHand(t *testing.T) {
	gs := newDiscoverGame(t)

	hit := nonlandCard(0, "Bolt", 2, "instant")
	stackLibraryTop(gs, 0, hit)

	got := ApplyDiscoverWithChoice(gs, 0, 3, DiscoverChoiceToHand)
	if got != hit {
		t.Fatalf("expected hit, got %v", got)
	}
	// Stack untouched.
	if len(gs.Stack) != 0 {
		t.Errorf("stack should not receive the hit on ToHand choice, got %d items", len(gs.Stack))
	}
	// Hit is in hand.
	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == hit {
			inHand = true
			break
		}
	}
	if !inHand {
		t.Error("hit should be in hand under DiscoverChoiceToHand")
	}
	// Event log records the to-hand path.
	if got := countDiscoverEvents(gs, "discover_to_hand"); got != 1 {
		t.Errorf("expected 1 discover_to_hand event, got %d", got)
	}
	if got := countDiscoverEvents(gs, "discover_cast"); got != 0 {
		t.Errorf("expected 0 discover_cast events under ToHand, got %d", got)
	}
}

// ===========================================================================
// NewDiscoverCastFromExilePermission shape
// ===========================================================================

func TestNewDiscoverCastFromExilePermission_Shape(t *testing.T) {
	p := NewDiscoverCastFromExilePermission(2)
	if p.Zone != ZoneExile {
		t.Errorf("Zone = %v, want ZoneExile", p.Zone)
	}
	if p.ManaCost != 0 {
		t.Errorf("ManaCost = %d, want 0 (free cast)", p.ManaCost)
	}
	if p.Keyword != "discover" {
		t.Errorf("Keyword = %q, want \"discover\"", p.Keyword)
	}
	if p.RequireController != 2 {
		t.Errorf("RequireController = %d, want 2", p.RequireController)
	}
}

// ===========================================================================
// Nil safety + degenerate inputs
// ===========================================================================

func TestApplyDiscover_NilOrInvalid(t *testing.T) {
	if c := ApplyDiscover(nil, 0, 3); c != nil {
		t.Error("ApplyDiscover(nil, ...) should return nil")
	}
	gs := newDiscoverGame(t)
	if c := ApplyDiscover(gs, -1, 3); c != nil {
		t.Error("ApplyDiscover with invalid seat should return nil")
	}
	if c := ApplyDiscover(gs, 0, -1); c != nil {
		t.Error("ApplyDiscover with negative n should return nil")
	}
}

func TestApplyDiscover_EmptyLibraryIsWhiff(t *testing.T) {
	gs := newDiscoverGame(t)
	gs.Seats[0].Library = nil // empty

	got := ApplyDiscover(gs, 0, 5)
	if got != nil {
		t.Errorf("empty library should whiff, got %v", got)
	}
	if got := countDiscoverEvents(gs, "discover_whiff"); got != 1 {
		t.Errorf("expected 1 discover_whiff event for empty library, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

// mustParseDiscoverAST builds a minimal AST carrying the discover
// keyword with `n` as its first arg. Used only by HasDiscover /
// DiscoverCount tests since the ApplyDiscover tests don't depend on
// the keyword being on the source card (they pass n explicitly).
func mustParseDiscoverAST(t *testing.T, n int) *gameast.CardAST {
	t.Helper()
	return &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "discover", Args: []any{float64(n)}},
		},
	}
}
