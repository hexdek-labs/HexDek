package hat

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r172 — scoreOpponentGraveyard's inline flashback/escape/unearth/
// retrace cluster moved onto cached predicate bit 5.
//
// The round began by testing whether it could instead reuse bit 3
// (hasGraveyardCastKeyword). It cannot: that predicate tests ten
// needles, and over a 38,753-card oracle corpus it fires on 132 cards
// this cluster does not. These tests pin the narrow answer so that a
// future attempt to collapse the two goes red here instead of
// silently re-weighting the evaluator.

func mkYardCard(t *testing.T, name string, kw ...string) *gameengine.Card {
	t.Helper()
	abilities := make([]gameast.Ability, 0, len(kw))
	for _, k := range kw {
		abilities = append(abilities, &gameast.Keyword{Name: k})
	}
	return &gameengine.Card{
		Name:  name,
		Types: []string{"instant"},
		CMC:   1,
		AST:   &gameast.CardAST{Abilities: abilities},
	}
}

// threatFromYard puts one card in an OPPONENT's graveyard and returns
// the score. Seat 0 is us; seat 1 is the opponent.
func threatFromYard(t *testing.T, c *gameengine.Card) float64 {
	t.Helper()
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	if c != nil {
		gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, c)
	}
	e := &GameStateEvaluator{}
	return e.scoreOpponentGraveyard(gs, 0)
}

// Direction 1: a card that SHOULD score. Flashback is one of the four.
func TestR172_OpponentGraveyard_ScoresRecastKeyword(t *testing.T) {
	base := threatFromYard(t, nil)
	got := threatFromYard(t, mkYardCard(t, "R172 Flashback Spell", "Flashback"))
	if delta := got - base; delta > -0.149 || delta < -0.151 {
		t.Fatalf("flashback card: threat delta = %.4f, want -0.15 "+
			"(base %.4f, got %.4f)", delta, base, got)
	}
}

// Direction 2: a card that should NOT score. Encore is a graveyard
// recast keyword in the WIDE predicate but not in this cluster, so it
// must contribute nothing. This is the assertion that catches a
// collapse onto bit 3.
func TestR172_OpponentGraveyard_DoesNotScoreWideOnlyKeyword(t *testing.T) {
	base := threatFromYard(t, nil)
	for _, kw := range []string{"Encore", "Embalm", "Disturb", "Eternalize", "Jump-start"} {
		got := threatFromYard(t, mkYardCard(t, "R172 "+kw, kw))
		if got != base {
			t.Errorf("%s card changed opponent-graveyard threat: base %.4f -> %.4f; "+
				"the narrow cluster must not widen to hasGraveyardCastKeyword", kw, base, got)
		}
	}
	// A card with no relevant text at all also scores nothing.
	if got := threatFromYard(t, mkYardCard(t, "R172 Vanilla")); got != base {
		t.Errorf("vanilla card changed threat: base %.4f -> %.4f", base, got)
	}
}

// The cached wrapper must agree with the string function in BOTH
// directions on the real AST-derived oracle-text path.
func TestR172_CachedWrapperAgreesWithOriginal(t *testing.T) {
	cases := []struct {
		kw   string
		want bool
	}{
		{"Flashback", true}, {"Escape", true}, {"Unearth", true}, {"Retrace", true},
		{"Encore", false}, {"Embalm", false}, {"Disturb", false},
		{"Eternalize", false}, {"Aftermath", false}, {"Jump-start", false},
		{"Flying", false},
	}
	for _, tc := range cases {
		c := mkYardCard(t, "R172 "+tc.kw, tc.kw)
		ot := gameengine.OracleTextLower(c)
		direct := hasOpponentGraveyardRecastKeyword(ot)
		cached := cachedOpponentGraveyardRecastKeyword(c)
		if direct != cached {
			t.Errorf("%s: cached %v != direct %v (text %q)", tc.kw, cached, direct, ot)
		}
		if direct != tc.want {
			t.Errorf("%s: got %v, want %v (text %q)", tc.kw, direct, tc.want, ot)
		}
		// Second call must be served from the memo and agree.
		if again := cachedOpponentGraveyardRecastKeyword(c); again != cached {
			t.Errorf("%s: memoized call disagreed with first (%v != %v)", tc.kw, again, cached)
		}
	}
}

// Without this, a control that reverts the call site to the uncached
// inline cluster would stay green and prove nothing — the r164 lesson,
// which that round learned the hard way.
func TestR172_OpponentGraveyardRoutesThroughTheMemo(t *testing.T) {
	gameengine.ResetTextPredicateStatsForTest()
	t.Cleanup(gameengine.ResetTextPredicateStatsForTest)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard,
		mkYardCard(t, "R172 Memo Probe", "Flashback"))

	e := &GameStateEvaluator{}
	e.scoreOpponentGraveyard(gs, 0)
	h1, m1, _ := gameengine.TextPredicateStats()
	if h1+m1 == 0 {
		t.Fatalf("scoreOpponentGraveyard consulted the memo zero times — " +
			"the call site is not wired through TextPredicate")
	}

	e.scoreOpponentGraveyard(gs, 0)
	h2, m2, _ := gameengine.TextPredicateStats()
	if h2 <= h1 {
		t.Fatalf("second pass produced no cache hit (hits %d -> %d)", h1, h2)
	}
	if m2 != m1 {
		t.Fatalf("second pass recomputed the predicate (misses %d -> %d)", m1, m2)
	}
}

// Bit 5 must be distinct from bit 3. A renumbering or a copy-paste
// that reused tpGraveyardCastKeyword would make the two predicates
// clobber each other's memo entries.
func TestR172_BitIsDistinctFromGraveyardCastKeyword(t *testing.T) {
	if tpOpponentGraveyardRecastKeyword == tpGraveyardCastKeyword {
		t.Fatalf("bit collision: tpOpponentGraveyardRecastKeyword == tpGraveyardCastKeyword (%d)",
			tpGraveyardCastKeyword)
	}
	// An Encore card distinguishes them; if they shared a bit, whichever
	// ran first would poison the other.
	c := mkYardCard(t, "R172 Bit Probe", "Encore")
	wide := cachedGraveyardCastKeyword(c)
	narrow := cachedOpponentGraveyardRecastKeyword(c)
	if !wide {
		t.Fatalf("wide predicate should match Encore, got false")
	}
	if narrow {
		t.Fatalf("narrow predicate matched Encore — memo collision or widening")
	}
}
