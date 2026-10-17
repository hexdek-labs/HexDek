package hat

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r164 — the memo's own tests live in internal/gameengine and call
// TextPredicate directly, so they cannot tell whether the evaluator
// actually routes through it. Without this, a control that reverts the
// evaluator to the uncached predicates stays green and tests nothing.
// That is exactly what happened in r163, so it is covered up front
// here rather than discovered by a control that would not go red.
func TestR164_EvaluatorRoutesThroughTheMemo(t *testing.T) {
	gameengine.ResetTextPredicateStatsForTest()
	t.Cleanup(gameengine.ResetTextPredicateStatsForTest)

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(1)), nil)
	card := &gameengine.Card{
		Name:  "R164 Flashback Bolt",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "Flashback"}},
		},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	e := &GameStateEvaluator{}
	e.scoreGraveyard(gs, 0)

	h1, m1, _ := gameengine.TextPredicateStats()
	if h1+m1 == 0 {
		t.Fatalf("scoreGraveyard consulted the memo zero times — the " +
			"evaluator is not wired through TextPredicate")
	}

	// Second pass over the SAME card must be served from the memo.
	e.scoreGraveyard(gs, 0)
	h2, m2, _ := gameengine.TextPredicateStats()
	if h2 <= h1 {
		t.Fatalf("second pass produced no cache hit (hits %d -> %d, misses %d -> %d)",
			h1, h2, m1, m2)
	}
	if m2 != m1 {
		t.Fatalf("second pass recomputed the predicate (misses %d -> %d)", m1, m2)
	}
}

// The converted wrappers must agree with the string functions they
// replaced, on the real oracle-text path, in both directions. A faster
// predicate that answers differently is a rules change, not an
// optimisation.
func TestR164_CachedWrappersAgreeWithTheOriginals(t *testing.T) {
	gameengine.ResetTextPredicateStatsForTest()
	t.Cleanup(gameengine.ResetTextPredicateStatsForTest)

	mk := func(text string) *gameengine.Card {
		return &gameengine.Card{
			Name: "R164 " + text,
			AST: &gameast.CardAST{
				Abilities: []gameast.Ability{&gameast.Static{Raw: text}},
			},
		}
	}
	cases := []string{
		"Flashback {2}{R}",
		"Escape—{1}{B}, exile three other cards from your graveyard.",
		"Flying",
		"{T}: Add {G}.",
		"{T}: Add one mana of any color.",
		"{2}, {T}: Draw a card.",
		"At the beginning of your upkeep, you draw a card.",
		"Whenever you draw a card, create a Treasure token.",
		"Whenever this creature attacks, exile the top card of your library. You may play it this turn.",
		"At the beginning of your end step, scry 1.",
		"",
	}
	for _, text := range cases {
		c := mk(text)
		ot := gameengine.OracleTextLower(c)

		if got, want := cachedGraveyardCastKeyword(c), hasGraveyardCastKeyword(ot); got != want {
			t.Errorf("graveyardCastKeyword(%q) = %v, original %v", text, got, want)
		}
		if got, want := cachedManaOnlyAbility(c), isManaOnlyAbility(ot); got != want {
			t.Errorf("manaOnlyAbility(%q) = %v, original %v", text, got, want)
		}
		if got, want := cachedImpulseDrawEngine(c), isImpulseDrawEngine(ot); got != want {
			t.Errorf("impulseDrawEngine(%q) = %v, original %v", text, got, want)
		}
		if got, want := cachedPersistentScryEngine(c), isPersistentScryEngine(ot); got != want {
			t.Errorf("persistentScryEngine(%q) = %v, original %v", text, got, want)
		}
		if got, want := cachedPersistentDrawEngine(c), isPersistentDrawEngineText(ot); got != want {
			t.Errorf("persistentDrawEngine(%q) = %v, original %v", text, got, want)
		}
	}
}

// isPersistentDrawEngine was split into a Permanent shim and a pure
// text form; the shim must still answer identically.
func TestR164_PermanentShimMatchesTheTextForm(t *testing.T) {
	c := &gameengine.Card{
		Name: "R164 Upkeep Draw",
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Static{Raw: "At the beginning of your upkeep, you draw a card."},
		}},
	}
	p := &gameengine.Permanent{Card: c}
	if got, want := isPersistentDrawEngine(p), isPersistentDrawEngineText(gameengine.OracleTextLower(c)); got != want {
		t.Fatalf("shim = %v, text form = %v", got, want)
	}
	if isPersistentDrawEngine(nil) {
		t.Fatalf("nil permanent must be false")
	}
	if isPersistentDrawEngine(&gameengine.Permanent{}) {
		t.Fatalf("permanent with no card must be false")
	}
}
