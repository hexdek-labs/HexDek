package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r158 — HasKeyword was re-normalizing, on every single call, strings
// that never change: the caller's literal and every Keyword name on the
// card's AST. Measured at 2.12s of a 30s production profile.

// r158Inputs is deliberately nastier than production: the fast path is
// only safe if it agrees with strings.ToLower(strings.TrimSpace(s)) on
// EVERY input, so the table carries the cases that could break it.
var r158Inputs = []string{
	"", "flying", "trample",
	"first strike", // interior space must survive
	"Flying", "FLYING", "FiRsT StRiKe",
	" flying", "flying ", "  flying  ",
	"\tflying", "flying\n", "\vflying\f", "\rflying\r",
	"\t Flying \n",
	"ward", "protection from red",
	"Lifelink", "double strike",
	"café",         // non-ASCII, already lower
	"CAFÉ",         // non-ASCII, needs lowering
	" flying ",     // U+00A0 NBSP — TrimSpace trims it, ASCII check must not claim it
	"\u0085flying", // U+0085 NEL — likewise
	"ŁÓDŹ",
	"-", "+1/+1", "kw:flying",
}

// Direction 1: the fast path returns exactly what the slow path does.
// Direction 2: it does so without allocating when it takes the fast
// route. Both are required — an implementation that is correct but
// allocates has bought nothing.
func TestR158_NormalizeKeywordMatchesTheSlowPath(t *testing.T) {
	for _, in := range r158Inputs {
		want := strings.ToLower(strings.TrimSpace(in))
		got := normalizeKeyword(in)
		if got != want {
			t.Errorf("normalizeKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestR158_FastPathAllocatesNothing(t *testing.T) {
	// Already-trimmed lowercase ASCII: the overwhelmingly common shape.
	for _, in := range []string{"flying", "first strike", "protection from red"} {
		n := testing.AllocsPerRun(200, func() {
			if normalizeKeyword(in) == "" {
				t.Fatal("unexpected empty")
			}
		})
		if n != 0 {
			t.Errorf("normalizeKeyword(%q) allocated %v times, want 0", in, n)
		}
	}
}

// A capitalized keyword in the corpus must still answer a lowercase
// query, which is the behaviour the old per-call ToLower provided.
func TestR158_CapitalizedASTKeywordStillMatches(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "r158 Bird", 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Keyword{Name: "  Flying  "},
		&gameast.Keyword{Name: "First Strike"},
	}}

	if !p.HasKeyword("flying") {
		t.Errorf("capitalized+padded AST keyword did not match a lowercase query")
	}
	if !p.HasKeyword("FIRST STRIKE") {
		t.Errorf("uppercase query did not match")
	}
	if p.HasKeyword("trample") {
		t.Errorf("matched a keyword the card does not have")
	}
}

// The cache is keyed on the AST pointer, so a DFC transform must change
// the answer. This is r157's lesson applied forward rather than
// rediscovered: the bug r157 fixed was a cache that could not see a
// face swap, and this round adds another cache on the same struct.
func TestR158_KeywordCacheFollowsATransform(t *testing.T) {
	gs := newFixtureGame(t)
	front := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "flying"}}}
	back := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "trample"}}}

	p := addBattlefield(gs, 0, "r158 Wolf", 2, 2, "creature")
	p.Card.AST = front
	p.FrontFaceAST = front
	p.BackFaceAST = back
	p.FrontFaceName = "r158 Wolf"
	p.BackFaceName = "r158 Wolf, Howling"

	if !p.HasKeyword("flying") || p.HasKeyword("trample") {
		t.Fatalf("precondition: front face should have flying and not trample")
	}

	if !TransformPermanent(gs, p, "r158") {
		t.Fatalf("TransformPermanent refused")
	}

	if p.HasKeyword("flying") {
		t.Errorf("STALE: still reports flying from the front face after transform")
	}
	if !p.HasKeyword("trample") {
		t.Errorf("STALE: does not report trample from the back face after transform")
	}
}

// A card with no keyword abilities must cache that fact rather than
// rescanning, and must still record the AST so the second call is
// served. Checked behaviourally: the answer is stable and the AST
// stamp is set.
func TestR158_EmptyKeywordSetIsCachedNotRescanned(t *testing.T) {
	ast := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "Creatures you control get +1/+1."},
	}}
	c := &Card{AST: ast}

	if got := astKeywordNames(c); got != nil {
		t.Fatalf("precondition: want no keyword names, got %v", got)
	}
	if c.kwNamesAST != ast {
		t.Errorf("the empty result was not stamped with its AST, so every call rescans")
	}
}

// Derived gate: the new cache must not be keyed on a readiness bool.
// Swapping the AST must change what astKeywordNames returns, with no
// transform machinery involved at all.
func TestR158_DerivedGate_KeywordCacheIsKeyedOnTheAST(t *testing.T) {
	a := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "flying"}}}
	b := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "menace"}}}
	c := &Card{AST: a}

	if got := astKeywordNames(c); len(got) != 1 || got[0] != "flying" {
		t.Fatalf("precondition: got %v", got)
	}
	c.AST = b
	got := astKeywordNames(c)
	if len(got) != 1 || got[0] != "menace" {
		t.Errorf("cache did not follow the AST swap: got %v, want [menace]", got)
	}
}
